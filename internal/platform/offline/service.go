package offline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/module"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

const (
	BootstrapSchemaVersion  = "offline.bootstrap.v2"
	PackageManifestVersion  = "offline.packages.v1"
	SyncQueueModelClient    = "client_first"
	StatusAccepted          = "accepted"
	StatusConflict          = "conflict"
	StatusFailedRetryable   = "failed_retryable"
	StatusFailedTerminal    = "failed_terminal"
	StatusForbidden         = "forbidden"
)

type Repository interface {
	SaveBatch(SyncBatch) error
	SaveOutcome(string, SyncResultItem) error
	ListBatches(int) []SyncBatch
	ListOutcomes(string) []SyncResultItem
	ListRecentOutcomes(int) []SyncResultItem
}

type Service struct {
	modules   *module.Service
	reference *reference.Service
	search    *search.Service
	repo      Repository
	mu        sync.RWMutex
}

func NewService(modules *module.Service, referenceSvc *reference.Service, searchSvc *search.Service) *Service {
	return NewServiceWithRepository(modules, referenceSvc, searchSvc, NewMemoryRepository())
}

func NewServiceWithRepository(modules *module.Service, referenceSvc *reference.Service, searchSvc *search.Service, repo Repository) *Service {
	if repo == nil {
		repo = NewMemoryRepository()
	}
	return &Service{
		modules:   modules,
		reference: referenceSvc,
		search:    searchSvc,
		repo:      repo,
	}
}

type Bootstrap struct {
	SchemaVersion         string                            `json:"schema_version"`
	PackageManifestVersion string                           `json:"package_manifest_version"`
	CacheToken            string                            `json:"cache_token"`
	GeneratedAt           time.Time                         `json:"generated_at"`
	References            []module.OfflineReferenceDefinition  `json:"references,omitempty"`
	Projections           []module.OfflineProjectionDefinition `json:"projections,omitempty"`
	Documents             []module.OfflineDocumentDefinition   `json:"documents,omitempty"`
	Models                []module.OfflineModelDefinition      `json:"models,omitempty"`
	SyncCapabilities      SyncCapabilities                 `json:"sync_capabilities"`
	PackageManifest       []PackageManifestItem            `json:"package_manifest,omitempty"`
}

type SyncCapabilities struct {
	QueueModel         string   `json:"queue_model"`
	ResultStatuses     []string `json:"result_statuses"`
	ResolutionOptions  []string `json:"resolution_options"`
	SupportsRetryAfter bool     `json:"supports_retry_after"`
}

type PackageManifestItem struct {
	PackageKey  string    `json:"package_key"`
	Kind        string    `json:"kind"`
	Version     string    `json:"version"`
	Checksum    string    `json:"checksum"`
	GeneratedAt time.Time `json:"generated_at"`
}

type ReferencePackage struct {
	PackageKey  string                            `json:"package_key"`
	Version     string                            `json:"version"`
	Checksum    string                            `json:"checksum"`
	GeneratedAt time.Time                         `json:"generated_at"`
	Type        module.OfflineReferenceDefinition `json:"type"`
	ResolvedSet reference.ResolvedSet             `json:"resolved_set"`
}

type ProjectionPackage struct {
	PackageKey  string                             `json:"package_key"`
	Version     string                             `json:"version"`
	Checksum    string                             `json:"checksum"`
	GeneratedAt time.Time                          `json:"generated_at"`
	Projection  module.OfflineProjectionDefinition `json:"projection"`
	Query       search.QueryRequest                `json:"query"`
	Result      search.QueryResult                 `json:"result"`
}

type SyncBatch struct {
	ID             string    `json:"id"`
	CorrelationID  string    `json:"correlation_id,omitempty"`
	ActorID        string    `json:"actor_id,omitempty"`
	DeviceID       string    `json:"device_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ProcessedAt    time.Time `json:"processed_at,omitempty"`
	ItemCount      int       `json:"item_count"`
	AcceptedCount  int       `json:"accepted_count"`
	ConflictCount  int       `json:"conflict_count"`
	RetryableCount int       `json:"retryable_count"`
	TerminalCount  int       `json:"terminal_count"`
}

type SyncConflict struct {
	Current           map[string]any `json:"current,omitempty"`
	Attempted         map[string]any `json:"attempted,omitempty"`
	ResolutionOptions []string       `json:"resolution_options,omitempty"`
}

type SyncResultItem struct {
	IdempotencyKey string       `json:"idempotency_key,omitempty"`
	BatchID        string       `json:"batch_id,omitempty"`
	CorrelationID  string       `json:"correlation_id,omitempty"`
	DeviceID       string       `json:"device_id,omitempty"`
	Status         string       `json:"status"`
	Kind           string       `json:"kind"`
	Operation      string       `json:"operation"`
	TargetID       string       `json:"target_id,omitempty"`
	Version        int          `json:"version,omitempty"`
	ETag           string       `json:"etag,omitempty"`
	Error          string       `json:"error,omitempty"`
	ErrorCode      string       `json:"error_code,omitempty"`
	Conflict       SyncConflict `json:"conflict,omitempty"`
	AttemptCount   int          `json:"attempt_count,omitempty"`
	ProcessedAt    time.Time    `json:"processed_at,omitempty"`
	RetryAfter     string       `json:"retry_after,omitempty"`
}

type MemoryRepository struct {
	mu       sync.RWMutex
	batches  []SyncBatch
	outcomes map[string][]SyncResultItem
	recent   []SyncResultItem
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{batches: []SyncBatch{}, outcomes: map[string][]SyncResultItem{}, recent: []SyncResultItem{}}
}

func (r *MemoryRepository) SaveBatch(batch SyncBatch) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.batches {
		if r.batches[i].ID == batch.ID {
			r.batches[i] = batch
			return nil
		}
	}
	r.batches = append(r.batches, batch)
	sort.Slice(r.batches, func(i, j int) bool { return r.batches[i].CreatedAt.After(r.batches[j].CreatedAt) })
	return nil
}

func (r *MemoryRepository) SaveOutcome(batchID string, item SyncResultItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outcomes[batchID] = append(r.outcomes[batchID], item)
	r.recent = append([]SyncResultItem{item}, r.recent...)
	if len(r.recent) > 200 {
		r.recent = r.recent[:200]
	}
	return nil
}

func (r *MemoryRepository) ListBatches(limit int) []SyncBatch {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := append([]SyncBatch(nil), r.batches...)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (r *MemoryRepository) ListOutcomes(batchID string) []SyncResultItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]SyncResultItem(nil), r.outcomes[strings.TrimSpace(batchID)]...)
}

func (r *MemoryRepository) ListRecentOutcomes(limit int) []SyncResultItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := append([]SyncResultItem(nil), r.recent...)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *Service) Bootstrap() Bootstrap {
	if s == nil || s.modules == nil {
		return Bootstrap{
			SchemaVersion:          BootstrapSchemaVersion,
			PackageManifestVersion: PackageManifestVersion,
			GeneratedAt:            time.Now().UTC(),
			SyncCapabilities:       defaultSyncCapabilities(),
		}
	}
	bootstrap := Bootstrap{
		SchemaVersion:          BootstrapSchemaVersion,
		PackageManifestVersion: PackageManifestVersion,
		GeneratedAt:            time.Now().UTC(),
		References:             append([]module.OfflineReferenceDefinition(nil), s.modules.OfflineReferences()...),
		Projections:            append([]module.OfflineProjectionDefinition(nil), s.modules.OfflineProjections()...),
		Documents:              append([]module.OfflineDocumentDefinition(nil), s.modules.OfflineDocuments()...),
		Models:                 append([]module.OfflineModelDefinition(nil), s.modules.OfflineModels()...),
		SyncCapabilities:       defaultSyncCapabilities(),
	}
	bootstrap.CacheToken = bootstrapToken(bootstrap)
	return bootstrap
}

func defaultSyncCapabilities() SyncCapabilities {
	return SyncCapabilities{
		QueueModel:         SyncQueueModelClient,
		ResultStatuses:     []string{StatusAccepted, StatusConflict, StatusFailedRetryable, StatusFailedTerminal, StatusForbidden},
		ResolutionOptions:  []string{"reload", "retry_with_latest", "duplicate_as_new"},
		SupportsRetryAfter: true,
	}
}

func bootstrapToken(bootstrap Bootstrap) string {
	payload := map[string]any{
		"schema_version":           bootstrap.SchemaVersion,
		"package_manifest_version": bootstrap.PackageManifestVersion,
		"references":               bootstrap.References,
		"projections":              bootstrap.Projections,
		"documents":                bootstrap.Documents,
		"models":                   bootstrap.Models,
	}
	checksum, _ := packageSignature(payload)
	return checksum[:16]
}

func (s *Service) ReferencePackage(typeKey, organizationID, locationID string, at time.Time) (ReferencePackage, error) {
	if s == nil || s.modules == nil || s.reference == nil {
		return ReferencePackage{}, shared.Validation("offline reference service is not configured")
	}
	def, ok := s.modules.OfflineReference(strings.TrimSpace(typeKey))
	if !ok {
		return ReferencePackage{}, shared.NotFound("offline reference package is not registered")
	}
	resolved, err := s.reference.Resolve(typeKey, organizationID, locationID, at)
	if err != nil {
		return ReferencePackage{}, err
	}
	payload := map[string]any{
		"type_key":        typeKey,
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
		"items":           resolved.Items,
	}
	checksum, version := packageSignature(payload)
	return ReferencePackage{
		PackageKey:  "reference:" + typeKey,
		Version:     version,
		Checksum:    checksum,
		GeneratedAt: time.Now().UTC(),
		Type:        def,
		ResolvedSet: resolved,
	}, nil
}

func (s *Service) ProjectionPackage(indexKey, organizationID, locationID string, req search.QueryRequest) (ProjectionPackage, error) {
	if s == nil || s.modules == nil || s.search == nil {
		return ProjectionPackage{}, shared.Validation("offline projection service is not configured")
	}
	def, ok := s.modules.OfflineProjection(strings.TrimSpace(indexKey))
	if !ok {
		return ProjectionPackage{}, shared.NotFound("offline projection package is not registered")
	}
	req = normalizeProjectionQuery(def, req)
	result, err := s.search.Query(indexKey, organizationID, locationID, req)
	if err != nil {
		return ProjectionPackage{}, err
	}
	payload := map[string]any{
		"index_key":       indexKey,
		"organization_id": strings.TrimSpace(organizationID),
		"location_id":     strings.TrimSpace(locationID),
		"query":           req,
		"hits":            result.Hits,
		"total":           result.Total,
	}
	checksum, version := packageSignature(payload)
	return ProjectionPackage{
		PackageKey:  "projection:" + indexKey,
		Version:     version,
		Checksum:    checksum,
		GeneratedAt: time.Now().UTC(),
		Projection:  def,
		Query:       req,
		Result:      result,
	}, nil
}

func (s *Service) BuildPackageManifest(organizationID, locationID string, refs []module.OfflineReferenceDefinition, projections []module.OfflineProjectionDefinition) []PackageManifestItem {
	items := make([]PackageManifestItem, 0, len(refs)+len(projections))
	for _, item := range refs {
		pkg, err := s.ReferencePackage(item.TypeKey, organizationID, locationID, time.Now().UTC())
		if err != nil {
			continue
		}
		items = append(items, PackageManifestItem{PackageKey: pkg.PackageKey, Kind: "reference", Version: pkg.Version, Checksum: pkg.Checksum, GeneratedAt: pkg.GeneratedAt})
	}
	for _, item := range projections {
		pkg, err := s.ProjectionPackage(item.IndexKey, organizationID, locationID, search.QueryRequest{})
		if err != nil {
			continue
		}
		items = append(items, PackageManifestItem{PackageKey: pkg.PackageKey, Kind: "projection", Version: pkg.Version, Checksum: pkg.Checksum, GeneratedAt: pkg.GeneratedAt})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].PackageKey < items[j].PackageKey })
	return items
}

func (s *Service) StartBatch(correlationID, actorID, deviceID string, itemCount int) SyncBatch {
	batch := SyncBatch{
		ID:            "offline-sync:" + firstNonEmpty(strings.TrimSpace(correlationID), time.Now().UTC().Format("20060102150405.000000000")),
		CorrelationID: strings.TrimSpace(correlationID),
		ActorID:       strings.TrimSpace(actorID),
		DeviceID:      strings.TrimSpace(deviceID),
		CreatedAt:     time.Now().UTC(),
		ItemCount:     itemCount,
	}
	_ = s.repo.SaveBatch(batch)
	return batch
}

func (s *Service) RecordOutcome(batch *SyncBatch, result SyncResultItem) {
	if s == nil || s.repo == nil || batch == nil {
		return
	}
	result.BatchID = batch.ID
	result.CorrelationID = batch.CorrelationID
	if result.ProcessedAt.IsZero() {
		result.ProcessedAt = time.Now().UTC()
	}
	_ = s.repo.SaveOutcome(batch.ID, result)
	switch result.Status {
	case StatusAccepted:
		batch.AcceptedCount++
	case StatusConflict:
		batch.ConflictCount++
	case StatusFailedRetryable:
		batch.RetryableCount++
	case StatusFailedTerminal, StatusForbidden:
		batch.TerminalCount++
	}
	batch.ProcessedAt = result.ProcessedAt
	_ = s.repo.SaveBatch(*batch)
}

func (s *Service) RecentBatches(limit int) []SyncBatch {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ListBatches(limit)
}

func (s *Service) BatchOutcomes(batchID string) []SyncResultItem {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ListOutcomes(batchID)
}

func (s *Service) RecentOutcomes(limit int) []SyncResultItem {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ListRecentOutcomes(limit)
}

func (s *Service) SyncSummary() map[string]any {
	items := s.RecentOutcomes(200)
	summary := map[string]int{}
	for _, item := range items {
		summary[item.Status]++
	}
	return map[string]any{
		"recent_count": len(items),
		"by_status":    summary,
	}
}

func normalizeProjectionQuery(def module.OfflineProjectionDefinition, req search.QueryRequest) search.QueryRequest {
	if req.Filters == nil {
		req.Filters = map[string]string{}
	}
	for key, value := range parseDefaultFilters(def.DefaultFilters) {
		if _, ok := req.Filters[key]; !ok {
			req.Filters[key] = value
		}
	}
	if len(req.IncludeFields) == 0 && len(def.DefaultIncludeFields) > 0 {
		req.IncludeFields = append([]string(nil), def.DefaultIncludeFields...)
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	return req
}

func parseDefaultFilters(items []string) map[string]string {
	result := map[string]string{}
	for _, item := range items {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}

func packageSignature(payload any) (checksum string, version string) {
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	checksum = hex.EncodeToString(sum[:])
	version = checksum[:16]
	return checksum, version
}

func FilterReferenceCapabilities(items []module.OfflineReferenceDefinition, allow func([]string) bool) []module.OfflineReferenceDefinition {
	filtered := make([]module.OfflineReferenceDefinition, 0, len(items))
	for _, item := range items {
		if allow == nil || allow(item.RequiredPermissions) {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].TypeKey < filtered[j].TypeKey })
	return filtered
}

func FilterProjectionCapabilities(items []module.OfflineProjectionDefinition, allow func([]string) bool) []module.OfflineProjectionDefinition {
	filtered := make([]module.OfflineProjectionDefinition, 0, len(items))
	for _, item := range items {
		if allow == nil || allow(item.RequiredPermissions) {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].IndexKey < filtered[j].IndexKey })
	return filtered
}

func FilterDocumentCapabilities(items []module.OfflineDocumentDefinition, allow func([]string) bool) []module.OfflineDocumentDefinition {
	filtered := make([]module.OfflineDocumentDefinition, 0, len(items))
	for _, item := range items {
		required := append([]string(nil), item.RequiredPermissions...)
		if item.CreatePermissionKey != "" {
			required = append(required, item.CreatePermissionKey)
		}
		if item.UpdatePermissionKey != "" {
			required = append(required, item.UpdatePermissionKey)
		}
		if allow == nil || allow(required) {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Type < filtered[j].Type })
	return filtered
}

func FilterModelCapabilities(items []module.OfflineModelDefinition, allow func([]string) bool) []module.OfflineModelDefinition {
	filtered := make([]module.OfflineModelDefinition, 0, len(items))
	for _, item := range items {
		required := append([]string(nil), item.RequiredPermissions...)
		if item.CreatePermissionKey != "" {
			required = append(required, item.CreatePermissionKey)
		}
		if item.UpdatePermissionKey != "" {
			required = append(required, item.UpdatePermissionKey)
		}
		if allow == nil || allow(required) {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ModelKey < filtered[j].ModelKey })
	return filtered
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
