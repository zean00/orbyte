package search

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"clinic/internal/platform/document"
	"clinic/internal/platform/jobs"
	"clinic/internal/platform/model"
	"clinic/internal/platform/shared"
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

type Service struct {
	repo      Repository
	backend   Backend
	embedder  Embedder
	documents *document.Service
	models    *model.Service
	jobs      *jobs.Service

	mu      sync.RWMutex
	indexes map[string]IndexDefinition
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo:     repo,
		backend:  NewMemoryBackend(),
		embedder: NewHashEmbedder(),
		indexes:  map[string]IndexDefinition{},
	}
}

func (s *Service) SetBackend(backend Backend) {
	if backend == nil {
		return
	}
	s.backend = backend
}

func (s *Service) SetEmbedder(embedder Embedder) {
	if embedder == nil {
		return
	}
	s.embedder = embedder
}

func (s *Service) AttachSources(documents *document.Service, models *model.Service) {
	s.documents = documents
	s.models = models
}

func (s *Service) AttachJobs(jobSvc *jobs.Service) {
	s.jobs = jobSvc
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
	for _, def := range s.matchingDocumentIndexes(record.Header.Type) {
		s.enqueue("search.index."+def.Key, func() (map[string]any, error) {
			if err := s.indexDocument(def, record); err != nil {
				return nil, err
			}
			return map[string]any{"index_key": def.Key, "source_id": record.Header.ID}, nil
		})
	}
	for _, def := range s.matchingProjectionIndexes("document_summary") {
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
		s.enqueue("search.index."+def.Key, func() (map[string]any, error) {
			if err := s.indexProjectionSummary(def, summary); err != nil {
				return nil, err
			}
			return map[string]any{"index_key": def.Key, "source_id": summary.DocumentID}, nil
		})
	}
}

func (s *Service) RefreshModel(record model.Record) {
	for _, def := range s.matchingModelIndexes(record.ModelKey) {
		def := def
		record := record
		s.enqueue("search.index."+def.Key, func() (map[string]any, error) {
			if err := s.indexModel(def, record); err != nil {
				return nil, err
			}
			return map[string]any{"index_key": def.Key, "source_id": record.ID}, nil
		})
	}
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
			if err := s.indexDocument(def, record); err != nil {
				return nil, err
			}
			count++
		}
		return map[string]any{"index_key": key, "reindexed": count}, nil
	case "model":
		if s.models == nil {
			return nil, shared.Validation("model source is not configured")
		}
		items := s.models.Repository().ListRecords(def.ModelKey)
		count := 0
		for _, record := range items {
			if err := s.indexModel(def, record); err != nil {
				return nil, err
			}
			count++
		}
		return map[string]any{"index_key": key, "reindexed": count}, nil
	case "projection":
		if def.ProjectionKey != "document_summary" {
			return nil, shared.Validation("projection rebuild is not supported for this projection")
		}
		count := 0
		for _, summary := range s.ListDocuments() {
			if err := s.indexProjectionSummary(def, summary); err != nil {
				return nil, err
			}
			count++
		}
		return map[string]any{"index_key": key, "reindexed": count}, nil
	default:
		return nil, shared.Validation("search index source_kind is invalid")
	}
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
	return report
}

func (s *Service) enqueue(name string, fn func() (map[string]any, error)) {
	if s.jobs == nil {
		_, _ = fn()
		return
	}
	s.jobs.Enqueue(name, fn)
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
			"payload": record.Body.Payload,
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
	envelope := cloneAnyMap(record.Values)
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
		OrganizationID: stringValue(record.Values["organization_id"]),
		LocationID:     stringValue(record.Values["location_id"]),
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
		fields[field.Key] = resolvePath(envelope, field.Path)
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
