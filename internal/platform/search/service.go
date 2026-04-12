package search

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

const (
	defaultQueryPageSize = 20
	maxQueryPageSize     = 100
)

type DocumentSummary struct {
	DocumentID     string    `json:"document_id"`
	DocumentType   string    `json:"document_type"`
	Status         string    `json:"status"`
	Version        int       `json:"version"`
	ETag           string    `json:"etag"`
	OrganizationID string    `json:"organization_id"`
	LocationID     string    `json:"location_id,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProjectionConsistency struct {
	ProjectionKey       string    `json:"projection_key"`
	SourceCount         int       `json:"source_count"`
	ProjectionCount     int       `json:"projection_count"`
	StaleCount          int       `json:"stale_count"`
	MissingCount        int       `json:"missing_count"`
	LastProjectedAt     time.Time `json:"last_projected_at,omitempty"`
	MaxSourceVersion    int       `json:"max_source_version"`
	MaxProjectedVersion int       `json:"max_projected_version"`
}

type ProjectionStatus struct {
	ProjectionKey         string    `json:"projection_key"`
	LastRefreshStatus     string    `json:"last_refresh_status"`
	LastSuccessAt         time.Time `json:"last_success_at,omitempty"`
	LastFailureAt         time.Time `json:"last_failure_at,omitempty"`
	LastError             string    `json:"last_error,omitempty"`
	LastRebuildStartedAt  time.Time `json:"last_rebuild_started_at,omitempty"`
	LastRebuildFinishedAt time.Time `json:"last_rebuild_finished_at,omitempty"`
	LastRebuildCount      int       `json:"last_rebuild_count"`
	SourceCount           int       `json:"source_count"`
	ProjectionCount       int       `json:"projection_count"`
	StaleCount            int       `json:"stale_count"`
	MissingCount          int       `json:"missing_count"`
}

type Service struct {
	repo      Repository
	backend   Backend
	embedder  Embedder
	documents *document.Service
	models    *model.Service
	jobs      *jobs.Service
	fields    *securityfields.Service

	mu       sync.RWMutex
	indexes  map[string]IndexDefinition
	runtimes map[string]IndexRuntime
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo:     repo,
		backend:  NewMemoryBackend(),
		embedder: NewDevelopmentHashEmbedder(8),
		indexes:  map[string]IndexDefinition{},
		runtimes: map[string]IndexRuntime{},
	}
}

func (s *Service) SetBackend(backend Backend) {
	if backend == nil {
		return
	}
	s.mu.Lock()
	s.backend = backend
	for key, def := range s.indexes {
		runtime := s.runtimes[key]
		runtime.BackendCapabilities = backend.Capabilities(def)
		s.runtimes[key] = runtime
	}
	s.mu.Unlock()
}

func (s *Service) SetEmbedder(embedder Embedder) {
	if embedder == nil {
		return
	}
	s.embedder = embedder
}

func (s *Service) EmbeddingRuntime() EmbedderDescriptor {
	if aware, ok := s.embedder.(DescriptorAwareEmbedder); ok {
		return aware.Descriptor()
	}
	return EmbedderDescriptor{
		Provider:        "custom",
		RuntimeProvider: "custom",
		Semantic:        true,
		Description:     "Custom embedder attached at runtime.",
	}
}

func (s *Service) AttachSources(documents *document.Service, models *model.Service) {
	s.documents = documents
	s.models = models
}

func (s *Service) AttachJobs(jobSvc *jobs.Service) {
	s.jobs = jobSvc
	if jobSvc != nil {
		s.registerJobHandlers(jobSvc)
	}
}

func (s *Service) AttachFieldSecurity(fieldSecurity *securityfields.Service) {
	s.fields = fieldSecurity
}

func (s *Service) RegisterIndex(def IndexDefinition) error {
	if strings.TrimSpace(def.Key) == "" || strings.TrimSpace(def.Title) == "" {
		return shared.Validation("search index key and title are required")
	}
	if def.SourceKind == "" {
		return shared.Validation("search index source_kind is required")
	}
	switch def.SourceKind {
	case "document":
		if strings.TrimSpace(def.DocumentType) == "" {
			return shared.Validation("document search index requires document_type")
		}
	case "model":
		if strings.TrimSpace(def.ModelKey) == "" {
			return shared.Validation("model search index requires model_key")
		}
	case "projection":
		if strings.TrimSpace(def.ProjectionKey) == "" {
			return shared.Validation("projection search index requires projection_key")
		}
	case "runtime":
	default:
		return shared.Validation("search index source_kind is invalid")
	}
	if len(def.Fields) == 0 && len(def.VectorFields) == 0 {
		return shared.Validation("search index requires fields or vector fields")
	}
	if len(def.Modes) == 0 {
		def.Modes = []string{"keyword"}
	}
	for _, mode := range def.Modes {
		switch strings.TrimSpace(mode) {
		case "keyword", "vector", "hybrid":
		default:
			return shared.Validation("search index mode is invalid")
		}
	}
	knownFields := map[string]bool{
		"id":              true,
		"source_id":       true,
		"organization_id": true,
		"location_id":     true,
		"updated_at":      true,
	}
	for _, field := range def.Fields {
		if strings.TrimSpace(field.Key) == "" || strings.TrimSpace(field.Path) == "" || strings.TrimSpace(field.Type) == "" {
			return shared.Validation("search field key, path, and type are required")
		}
		knownFields[field.Key] = true
	}
	for _, field := range def.VectorFields {
		if strings.TrimSpace(field.Key) == "" || len(field.SourcePaths) == 0 {
			return shared.Validation("vector field key and source_paths are required")
		}
		if field.EmbeddingMode != "typesense_auto" && field.EmbeddingMode != "external" {
			return shared.Validation("vector field embedding_mode is invalid")
		}
		knownFields[field.Key] = true
	}
	for _, key := range def.QueryFilterFields {
		if !knownFields[key] {
			return shared.Validation("search query filter field is not declared")
		}
	}
	for _, key := range def.QuerySortFields {
		if !knownFields[key] {
			return shared.Validation("search query sort field is not declared")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.indexes[def.Key]; exists {
		return shared.Conflict("search index key already registered")
	}
	s.indexes[def.Key] = def
	s.runtimes[def.Key] = IndexRuntime{
		IndexKey:            def.Key,
		ProjectionKey:       projectionKeyForDefinition(def),
		SourceKind:          def.SourceKind,
		RuntimeStatus:       "ready",
		ConsistencyStatus:   "unknown",
		ActiveSchemaVersion: "v1",
		LifecycleState:      "active",
		BackendCapabilities: s.backend.Capabilities(def),
	}
	return nil
}

func (s *Service) IndexDefinitions() []IndexDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]IndexDefinition, 0, len(s.indexes))
	for _, def := range s.indexes {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) IndexDefinition(key string) (IndexDefinition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	def, ok := s.indexes[key]
	return def, ok
}

func (s *Service) IndexRuntime(key string) (IndexRuntime, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.runtimes[key]
	return item, ok
}

func (s *Service) IndexRuntimes() []IndexRuntime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]IndexRuntime, 0, len(s.runtimes))
	for _, item := range s.runtimes {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].IndexKey < items[j].IndexKey })
	return items
}

func (s *Service) PlanIndexSchemaVersion(key, version string) (IndexRuntime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	runtime, ok := s.runtimes[key]
	if !ok {
		return IndexRuntime{}, shared.NotFound("search index not found")
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return IndexRuntime{}, shared.Validation("schema version is required")
	}
	runtime.CandidateSchemaVersion = version
	runtime.LifecycleState = "cutover_pending"
	s.runtimes[key] = runtime
	return runtime, nil
}

func (s *Service) BuildCandidateIndex(key string) (IndexRuntime, error) {
	s.mu.Lock()
	runtime, ok := s.runtimes[key]
	if !ok {
		s.mu.Unlock()
		return IndexRuntime{}, shared.NotFound("search index not found")
	}
	if runtime.CandidateSchemaVersion == "" {
		s.mu.Unlock()
		return IndexRuntime{}, shared.Validation("candidate schema version is not planned")
	}
	runtime.LifecycleState = "building"
	runtime.RuntimeStatus = "building_candidate"
	runtime.LastRebuildStartedAt = time.Now().UTC()
	s.runtimes[key] = runtime
	s.mu.Unlock()
	def, _ := s.IndexDefinition(key)
	if _, err := s.RebuildIndex(key); err != nil {
		s.updateRuntime(key, func(current *IndexRuntime) {
			current.RuntimeStatus = "failed"
			current.LastFailureAt = time.Now().UTC()
			current.LastError = err.Error()
		})
		return IndexRuntime{}, err
	}
	s.updateRuntime(key, func(current *IndexRuntime) {
		current.LifecycleState = "validating"
		current.RuntimeStatus = "candidate_built"
		current.LastRebuildFinishedAt = time.Now().UTC()
		current.BackendCapabilities = s.backend.Capabilities(def)
	})
	runtime, _ = s.IndexRuntime(key)
	return runtime, nil
}

func (s *Service) ActivateCandidateIndex(key string) (IndexRuntime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	runtime, ok := s.runtimes[key]
	if !ok {
		return IndexRuntime{}, shared.NotFound("search index not found")
	}
	if runtime.CandidateSchemaVersion == "" {
		return IndexRuntime{}, shared.Validation("candidate schema version is not planned")
	}
	runtime.ActiveSchemaVersion = runtime.CandidateSchemaVersion
	runtime.CandidateSchemaVersion = ""
	runtime.LifecycleState = "active"
	runtime.RuntimeStatus = "ready"
	s.runtimes[key] = runtime
	return runtime, nil
}

func (s *Service) RefreshDocument(record document.Record) {
	_ = s.repo.SaveDocument(DocumentSummary{
		DocumentID:     record.Header.ID,
		DocumentType:   record.Header.Type,
		Status:         record.Header.Status,
		Version:        record.Header.Version,
		ETag:           record.Header.ETag,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		UpdatedAt:      record.Header.UpdatedAt,
	})
	s.enqueueDocumentRefresh(record)
}

func (s *Service) RefreshModel(record model.Record) {
	s.enqueueModelRefresh(record)
}

func (s *Service) RefreshModelByID(modelKey, recordID string) error {
	if s.models == nil {
		return shared.Validation("search model source is not configured")
	}
	record, err := s.models.Get(modelKey, recordID)
	if err != nil {
		return err
	}
	s.RefreshModel(record)
	return nil
}

func (s *Service) ListDocuments() []DocumentSummary {
	return s.repo.ListDocuments()
}

func (s *Service) RebuildDocument(record document.Record) {
	s.RefreshDocument(record)
}

func (s *Service) RebuildAll(records []document.Record) {
	for _, record := range records {
		s.RefreshDocument(record)
	}
}

func (s *Service) RebuildIndex(key string) (map[string]any, error) {
	def, ok := s.IndexDefinition(key)
	if !ok {
		return nil, shared.NotFound("search index not found")
	}
	projectionKey := projectionKeyForDefinition(def)
	s.markRebuildStart(projectionKey)
	var (
		result map[string]any
		err    error
	)
	switch def.SourceKind {
	case "document":
		if s.documents == nil {
			return nil, shared.Validation("document source is not configured")
		}
		count := 0
		for _, record := range s.documents.List() {
			if def.DocumentType != "" && record.Header.Type != def.DocumentType {
				continue
			}
			if indexErr := s.indexDocument(def, record); indexErr != nil {
				err = indexErr
				break
			}
			count++
		}
		if err == nil {
			result = map[string]any{"index_key": key, "reindexed": count}
		}
	case "model":
		if s.models == nil {
			return nil, shared.Validation("model source is not configured")
		}
		items := s.models.Repository().ListRecords(def.ModelKey)
		count := 0
		for _, record := range items {
			if indexErr := s.indexModel(def, record); indexErr != nil {
				err = indexErr
				break
			}
			count++
		}
		if err == nil {
			result = map[string]any{"index_key": key, "reindexed": count}
		}
	case "projection":
		if def.ProjectionKey != "document_summary" {
			return nil, shared.Validation("projection rebuild is not supported for this projection")
		}
		count := 0
		for _, summary := range s.ListDocuments() {
			if indexErr := s.indexProjectionSummary(def, summary); indexErr != nil {
				err = indexErr
				break
			}
			count++
		}
		if err == nil {
			result = map[string]any{"index_key": key, "reindexed": count}
		}
	case "runtime":
		err = shared.Validation("runtime search indexes must be refreshed through ReplaceRuntimeRecords")
	default:
		err = shared.Validation("search index source_kind is invalid")
	}
	if err != nil {
		s.markRebuildFailure(projectionKey, err)
		s.updateRuntime(key, func(runtime *IndexRuntime) {
			runtime.RuntimeStatus = "failed"
			runtime.LastFailureAt = time.Now().UTC()
			runtime.LastError = err.Error()
		})
		return nil, err
	}
	s.markRebuildSuccess(projectionKey, intValue(result["reindexed"]))
	s.updateRuntime(key, func(runtime *IndexRuntime) {
		runtime.RuntimeStatus = "ready"
		runtime.LastSuccessAt = time.Now().UTC()
		runtime.LastRebuildStartedAt = time.Now().UTC()
		runtime.LastRebuildFinishedAt = time.Now().UTC()
		runtime.LastError = ""
	})
	return result, nil
}

func (s *Service) ReplaceRuntimeRecords(indexKey string, records []RuntimeSourceRecord) (map[string]any, error) {
	def, ok := s.IndexDefinition(indexKey)
	if !ok {
		return nil, shared.NotFound("search index not found")
	}
	if def.SourceKind != "runtime" {
		return nil, shared.Validation("search index is not a runtime index")
	}
	byOrganization := make(map[string][]RuntimeSourceRecord)
	for _, record := range records {
		organizationID := strings.TrimSpace(record.OrganizationID)
		if organizationID == "" {
			organizationID = "global"
		}
		record.OrganizationID = organizationID
		if record.SourceID == "" {
			record.SourceID = record.ID
		}
		if record.UpdatedAt.IsZero() {
			record.UpdatedAt = time.Now().UTC()
		}
		byOrganization[organizationID] = append(byOrganization[organizationID], record)
	}
	total := 0
	if len(byOrganization) == 0 {
		byOrganization["global"] = nil
	}
	for organizationID, items := range byOrganization {
		if err := s.backend.EnsureIndex(def, organizationID); err != nil {
			return nil, err
		}
		existing, err := s.backend.List(def, organizationID)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]struct{}, len(items))
		for _, item := range items {
			indexed, err := s.runtimeRecord(def, item)
			if err != nil {
				return nil, err
			}
			seen[indexed.ID] = struct{}{}
			if err := s.backend.Upsert(def, organizationID, indexed); err != nil {
				return nil, err
			}
			total++
		}
		for _, item := range existing {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			if err := s.backend.Delete(def, organizationID, item.ID); err != nil {
				return nil, err
			}
		}
	}
	s.updateRuntime(indexKey, func(runtime *IndexRuntime) {
		runtime.RuntimeStatus = "ready"
		runtime.LastSuccessAt = time.Now().UTC()
		runtime.LastRebuildStartedAt = time.Now().UTC()
		runtime.LastRebuildFinishedAt = time.Now().UTC()
		runtime.LastError = ""
	})
	return map[string]any{"index_key": indexKey, "reindexed": total}, nil
}

func (s *Service) Query(indexKey, organizationID, locationID string, req QueryRequest) (QueryResult, error) {
	def, ok := s.IndexDefinition(indexKey)
	if !ok {
		return QueryResult{}, shared.NotFound("search index not found")
	}
	normalized, err := normalizeQueryRequest(def, req)
	if err != nil {
		return QueryResult{}, err
	}
	if normalized.Filters == nil {
		normalized.Filters = map[string]string{}
	}
	if locationID != "" {
		normalized.Filters["location_id"] = locationID
	}
	if len(normalized.Vector) == 0 && strings.TrimSpace(normalized.VectorText) != "" {
		vectorField := selectedVectorField(def, normalized.VectorField)
		if vectorField.Key == "" {
			return QueryResult{}, shared.Validation("search index does not define a vector field")
		}
		vectors, err := s.embedder.Embed([]string{normalized.VectorText}, vectorDimensions(vectorField))
		if err != nil {
			return QueryResult{}, err
		}
		if len(vectors) > 0 {
			normalized.Vector = vectors[0]
		}
	}
	return s.backend.Search(def, organizationID, normalized)
}

func (s *Service) Consistency(records []document.Record) ProjectionConsistency {
	summaries := s.ListDocuments()
	byID := make(map[string]DocumentSummary, len(summaries))
	report := ProjectionConsistency{
		ProjectionKey:   "document_summary",
		SourceCount:     len(records),
		ProjectionCount: len(summaries),
	}
	for _, item := range summaries {
		byID[item.DocumentID] = item
		if item.UpdatedAt.After(report.LastProjectedAt) {
			report.LastProjectedAt = item.UpdatedAt
		}
		if item.Version > report.MaxProjectedVersion {
			report.MaxProjectedVersion = item.Version
		}
	}
	for _, record := range records {
		if record.Header.Version > report.MaxSourceVersion {
			report.MaxSourceVersion = record.Header.Version
		}
		current, ok := byID[record.Header.ID]
		if !ok {
			report.MissingCount++
			continue
		}
		if current.Version < record.Header.Version || current.ETag != record.Header.ETag || current.Status != record.Header.Status {
			report.StaleCount++
		}
	}
	status := s.currentProjectionStatus("document_summary")
	status.ProjectionKey = "document_summary"
	status.SourceCount = report.SourceCount
	status.ProjectionCount = report.ProjectionCount
	status.StaleCount = report.StaleCount
	status.MissingCount = report.MissingCount
	if status.LastRefreshStatus == "" {
		status.LastRefreshStatus = "idle"
	}
	_ = s.repo.SaveProjectionStatus(status)
	return report
}

func (s *Service) ProjectionStatuses(records []document.Record) []ProjectionStatus {
	_ = s.Consistency(records)
	items := s.repo.ListProjectionStatuses()
	sort.Slice(items, func(i, j int) bool { return items[i].ProjectionKey < items[j].ProjectionKey })
	return items
}

func (s *Service) enqueue(name string, payload map[string]any, fn func() error) {
	if s.jobs == nil {
		if fn != nil {
			_ = fn()
		}
		return
	}
	if _, err := s.jobs.Enqueue(name, payload); err != nil && fn != nil {
		_ = fn()
	}
}

func (s *Service) enqueueDocumentRefresh(record document.Record) {
	if s.jobs == nil || s.documents == nil {
		_ = s.indexDocumentAndProjection(record)
		return
	}
	payload := map[string]any{
		"document_id": record.Header.ID,
	}
	s.enqueue(JobRefreshDocument, payload, func() error {
		return s.indexDocumentAndProjection(record)
	})
}

func (s *Service) enqueueModelRefresh(record model.Record) {
	if s.jobs == nil || s.models == nil {
		_ = s.indexModelByRecord(record)
		return
	}
	payload := map[string]any{
		"model_key": record.ModelKey,
		"record_id": record.ID,
	}
	s.enqueue(JobRefreshModel, payload, func() error {
		return s.indexModelByRecord(record)
	})
}

func (s *Service) indexDocumentAndProjection(record document.Record) error {
	for _, def := range s.matchingDocumentIndexes(record.Header.Type) {
		if err := s.indexDocument(def, record); err != nil {
			return err
		}
	}
	summary := DocumentSummary{
		DocumentID:     record.Header.ID,
		DocumentType:   record.Header.Type,
		Status:         record.Header.Status,
		Version:        record.Header.Version,
		ETag:           record.Header.ETag,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		UpdatedAt:      record.Header.UpdatedAt,
	}
	for _, def := range s.matchingProjectionIndexes("document_summary") {
		if err := s.indexProjectionSummary(def, summary); err != nil {
			s.markRefreshFailure("document_summary", err)
			return err
		}
	}
	s.markRefreshSuccess("document_summary")
	return nil
}

func (s *Service) markRefreshSuccess(projectionKey string) {
	status := s.currentProjectionStatus(projectionKey)
	status.ProjectionKey = projectionKey
	status.LastRefreshStatus = "succeeded"
	status.LastSuccessAt = time.Now().UTC()
	status.LastError = ""
	_ = s.repo.SaveProjectionStatus(status)
}

func (s *Service) markRefreshFailure(projectionKey string, err error) {
	status := s.currentProjectionStatus(projectionKey)
	status.ProjectionKey = projectionKey
	status.LastRefreshStatus = "failed"
	status.LastFailureAt = time.Now().UTC()
	if err != nil {
		status.LastError = err.Error()
	}
	_ = s.repo.SaveProjectionStatus(status)
}

func (s *Service) markRebuildStart(projectionKey string) {
	if projectionKey == "" {
		return
	}
	status := s.currentProjectionStatus(projectionKey)
	status.ProjectionKey = projectionKey
	status.LastRefreshStatus = "rebuilding"
	status.LastRebuildStartedAt = time.Now().UTC()
	_ = s.repo.SaveProjectionStatus(status)
}

func (s *Service) markRebuildSuccess(projectionKey string, count int) {
	if projectionKey == "" {
		return
	}
	status := s.currentProjectionStatus(projectionKey)
	status.ProjectionKey = projectionKey
	status.LastRefreshStatus = "succeeded"
	status.LastSuccessAt = time.Now().UTC()
	status.LastRebuildFinishedAt = time.Now().UTC()
	status.LastRebuildCount = count
	status.LastError = ""
	_ = s.repo.SaveProjectionStatus(status)
}

func (s *Service) markRebuildFailure(projectionKey string, err error) {
	if projectionKey == "" {
		return
	}
	status := s.currentProjectionStatus(projectionKey)
	status.ProjectionKey = projectionKey
	status.LastRefreshStatus = "failed"
	status.LastFailureAt = time.Now().UTC()
	status.LastRebuildFinishedAt = time.Now().UTC()
	if err != nil {
		status.LastError = err.Error()
	}
	_ = s.repo.SaveProjectionStatus(status)
}

func projectionKeyForDefinition(def IndexDefinition) string {
	switch def.SourceKind {
	case "projection":
		return def.ProjectionKey
	case "document":
		return "document_summary"
	default:
		return def.Key
	}
}

func intValue(value any) int {
	switch current := value.(type) {
	case int:
		return current
	case int64:
		return int(current)
	case float64:
		return int(current)
	default:
		return 0
	}
}

func (s *Service) currentProjectionStatus(projectionKey string) ProjectionStatus {
	items := s.repo.ListProjectionStatuses()
	for _, item := range items {
		if item.ProjectionKey == projectionKey {
			return item
		}
	}
	return ProjectionStatus{}
}

func (s *Service) indexModelByRecord(record model.Record) error {
	for _, def := range s.matchingModelIndexes(record.ModelKey) {
		if err := s.indexModel(def, record); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) matchingDocumentIndexes(documentType string) []IndexDefinition {
	items := s.IndexDefinitions()
	filtered := make([]IndexDefinition, 0, len(items))
	for _, def := range items {
		if def.SourceKind == "document" && def.DocumentType == documentType {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

func (s *Service) matchingModelIndexes(modelKey string) []IndexDefinition {
	items := s.IndexDefinitions()
	filtered := make([]IndexDefinition, 0, len(items))
	for _, def := range items {
		if def.SourceKind == "model" && def.ModelKey == modelKey {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

func (s *Service) matchingProjectionIndexes(projectionKey string) []IndexDefinition {
	items := s.IndexDefinitions()
	filtered := make([]IndexDefinition, 0, len(items))
	for _, def := range items {
		if def.SourceKind == "projection" && def.ProjectionKey == projectionKey {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

func (s *Service) runtimeRecord(def IndexDefinition, record RuntimeSourceRecord) (IndexedRecord, error) {
	envelope := cloneAnyMap(record.Payload)
	if envelope == nil {
		envelope = map[string]any{}
	}
	envelope["id"] = record.ID
	envelope["source_id"] = firstNonEmptyString(record.SourceID, record.ID)
	envelope["organization_id"] = record.OrganizationID
	envelope["location_id"] = record.LocationID
	envelope["updated_at"] = record.UpdatedAt
	fields, vectors, err := s.extractIndexPayload(def, envelope)
	if err != nil {
		return IndexedRecord{}, err
	}
	return IndexedRecord{
		ID:             record.ID,
		SourceID:       firstNonEmptyString(record.SourceID, record.ID),
		SourceKind:     "runtime",
		OrganizationID: record.OrganizationID,
		LocationID:     record.LocationID,
		Version:        record.Version,
		UpdatedAt:      record.UpdatedAt,
		Fields:         fields,
		Vectors:        vectors,
	}, nil
}

func (s *Service) indexDocument(def IndexDefinition, record document.Record) error {
	organizationID := record.Header.OrganizationID
	if err := s.backend.EnsureIndex(def, organizationID); err != nil {
		return err
	}
	indexed, err := s.documentRecord(def, record)
	if err != nil {
		return err
	}
	return s.backend.Upsert(def, organizationID, indexed)
}

func (s *Service) indexModel(def IndexDefinition, record model.Record) error {
	organizationID := stringValue(record.Values["organization_id"])
	if organizationID == "" {
		organizationID = "global"
	}
	if err := s.backend.EnsureIndex(def, organizationID); err != nil {
		return err
	}
	indexed, err := s.modelRecord(def, record)
	if err != nil {
		return err
	}
	return s.backend.Upsert(def, organizationID, indexed)
}

func (s *Service) indexProjectionSummary(def IndexDefinition, summary DocumentSummary) error {
	organizationID := summary.OrganizationID
	if err := s.backend.EnsureIndex(def, organizationID); err != nil {
		return err
	}
	indexed, err := s.projectionRecord(def, summary)
	if err != nil {
		return err
	}
	return s.backend.Upsert(def, organizationID, indexed)
}

func (s *Service) documentRecord(def IndexDefinition, record document.Record) (IndexedRecord, error) {
	payload := record.Body.Payload
	if s.fields != nil {
		profile := s.fields.DocumentProfile(securityfields.AccessContext{
			OrganizationID: record.Header.OrganizationID,
			LocationID:     record.Header.LocationID,
			ScopeID:        record.Header.LocationID,
			Channel:        "search",
			State:          record.Header.Status,
		}, record)
		for key, access := range profile.Fields {
			if !access.SearchVisible {
				access.Visible = false
				access.Mask = ""
				profile.Fields[key] = access
			}
		}
		payload = s.fields.SanitizeDocumentPayload(profile, payload)
	}
	envelope := map[string]any{
		"header": map[string]any{
			"id":              record.Header.ID,
			"type":            record.Header.Type,
			"status":          record.Header.Status,
			"version":         record.Header.Version,
			"etag":            record.Header.ETag,
			"organization_id": record.Header.OrganizationID,
			"location_id":     record.Header.LocationID,
			"number":          record.Header.Number,
			"created_at":      record.Header.CreatedAt,
			"updated_at":      record.Header.UpdatedAt,
		},
		"body": map[string]any{
			"payload": payload,
		},
	}
	fields, vectors, err := s.extractIndexPayload(def, envelope)
	if err != nil {
		return IndexedRecord{}, err
	}
	return IndexedRecord{
		ID:             record.Header.ID,
		SourceID:       record.Header.ID,
		SourceKind:     "document",
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		Version:        record.Header.Version,
		UpdatedAt:      record.Header.UpdatedAt,
		Fields:         fields,
		Vectors:        vectors,
	}, nil
}

func (s *Service) modelRecord(def IndexDefinition, record model.Record) (IndexedRecord, error) {
	values := cloneAnyMap(record.Values)
	if s.fields != nil && s.models != nil {
		if modelDef, ok := s.models.Definition(def.ModelKey); ok {
			profile := s.fields.ModelProfile(securityfields.AccessContext{
				OrganizationID: stringValue(record.Values["organization_id"]),
				LocationID:     stringValue(record.Values["location_id"]),
				ScopeID:        stringValue(record.Values["location_id"]),
				Channel:        "search",
			}, modelDef)
			for key, access := range profile.Fields {
				if !access.SearchVisible {
					access.Visible = false
					access.Mask = ""
					profile.Fields[key] = access
				}
			}
			record = s.fields.SanitizeModelRecord(profile, record)
			values = cloneAnyMap(record.Values)
		}
	}
	envelope := values
	envelope["id"] = record.ID
	envelope["created_at"] = record.CreatedAt
	envelope["updated_at"] = record.UpdatedAt
	fields, vectors, err := s.extractIndexPayload(def, envelope)
	if err != nil {
		return IndexedRecord{}, err
	}
	return IndexedRecord{
		ID:             record.ID,
		SourceID:       record.ID,
		SourceKind:     "model",
		OrganizationID: stringValue(values["organization_id"]),
		LocationID:     stringValue(values["location_id"]),
		Version:        record.Version,
		UpdatedAt:      record.UpdatedAt,
		Fields:         fields,
		Vectors:        vectors,
	}, nil
}

func (s *Service) projectionRecord(def IndexDefinition, summary DocumentSummary) (IndexedRecord, error) {
	envelope := map[string]any{
		"id":              summary.DocumentID,
		"document_id":     summary.DocumentID,
		"document_type":   summary.DocumentType,
		"status":          summary.Status,
		"version":         summary.Version,
		"etag":            summary.ETag,
		"organization_id": summary.OrganizationID,
		"location_id":     summary.LocationID,
		"updated_at":      summary.UpdatedAt,
	}
	fields, vectors, err := s.extractIndexPayload(def, envelope)
	if err != nil {
		return IndexedRecord{}, err
	}
	return IndexedRecord{
		ID:             summary.DocumentID,
		SourceID:       summary.DocumentID,
		SourceKind:     "projection",
		OrganizationID: summary.OrganizationID,
		LocationID:     summary.LocationID,
		Version:        summary.Version,
		UpdatedAt:      summary.UpdatedAt,
		Fields:         fields,
		Vectors:        vectors,
	}, nil
}

func (s *Service) extractIndexPayload(def IndexDefinition, envelope map[string]any) (map[string]any, map[string][]float32, error) {
	fields := map[string]any{}
	for _, field := range def.Fields {
		value := resolvePath(envelope, field.Path)
		if value == nil {
			continue
		}
		fields[field.Key] = value
	}
	vectors := map[string][]float32{}
	for _, field := range def.VectorFields {
		texts := make([]string, 0, len(field.SourcePaths))
		for _, path := range field.SourcePaths {
			text := strings.TrimSpace(fmt.Sprint(resolvePath(envelope, path)))
			if text != "" {
				texts = append(texts, text)
			}
		}
		if len(texts) == 0 {
			continue
		}
		combined := strings.Join(texts, "\n")
		fields[field.Key+"_text"] = combined
		vectorsOut, err := s.embedder.Embed([]string{combined}, vectorDimensions(field))
		if err != nil {
			return nil, nil, err
		}
		if len(vectorsOut) > 0 {
			vectors[field.Key] = vectorsOut[0]
		}
	}
	return fields, vectors, nil
}

func normalizeQueryRequest(def IndexDefinition, req QueryRequest) (QueryRequest, error) {
	mode := normalizeQueryMode(req.Mode, def)
	req.Mode = mode
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultQueryPageSize
	}
	if req.PageSize > maxQueryPageSize {
		req.PageSize = maxQueryPageSize
	}
	allowedFilters := stringSet(def.QueryFilterFields)
	for key := range req.Filters {
		if !allowedFilters[key] && key != "location_id" {
			return QueryRequest{}, shared.Validation("unsupported search filter field: " + key)
		}
	}
	if req.SortBy != "" {
		allowedSorts := stringSet(def.QuerySortFields)
		if !allowedSorts[req.SortBy] {
			return QueryRequest{}, shared.Validation("unsupported search sort field: " + req.SortBy)
		}
	}
	return req, nil
}

func normalizeQueryMode(mode string, def IndexDefinition) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		if len(def.Modes) > 0 {
			return def.Modes[0]
		}
		return "keyword"
	}
	return mode
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[strings.TrimSpace(value)] = true
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func selectedVectorField(def IndexDefinition, key string) VectorFieldDefinition {
	key = strings.TrimSpace(key)
	if key == "" && len(def.VectorFields) > 0 {
		return def.VectorFields[0]
	}
	for _, field := range def.VectorFields {
		if field.Key == key {
			return field
		}
	}
	return VectorFieldDefinition{}
}

func vectorDimensions(field VectorFieldDefinition) int {
	if field.Dimensions > 0 {
		return field.Dimensions
	}
	return 8
}

func resolvePath(payload map[string]any, path string) any {
	current := any(payload)
	for _, segment := range strings.Split(strings.TrimSpace(path), ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := asMap[segment]
		if !ok {
			return nil
		}
		current = next
	}
	return current
}
