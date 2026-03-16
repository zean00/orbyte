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

type Service struct {
	modules    *module.Service
	reference  *reference.Service
	search     *search.Service
	mu         sync.RWMutex
	syncResult map[string]SyncResultItem
}

func NewService(modules *module.Service, referenceSvc *reference.Service, searchSvc *search.Service) *Service {
	return &Service{
		modules:    modules,
		reference:  referenceSvc,
		search:     searchSvc,
		syncResult: map[string]SyncResultItem{},
	}
}

type Bootstrap struct {
	GeneratedAt time.Time                            `json:"generated_at"`
	References  []module.OfflineReferenceDefinition  `json:"references,omitempty"`
	Projections []module.OfflineProjectionDefinition `json:"projections,omitempty"`
	Documents   []module.OfflineDocumentDefinition   `json:"documents,omitempty"`
	Models      []module.OfflineModelDefinition      `json:"models,omitempty"`
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

type SyncResultItem struct {
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Status         string         `json:"status"`
	Kind           string         `json:"kind"`
	Operation      string         `json:"operation"`
	TargetID       string         `json:"target_id,omitempty"`
	Version        int            `json:"version,omitempty"`
	ETag           string         `json:"etag,omitempty"`
	Error          string         `json:"error,omitempty"`
	Conflict       map[string]any `json:"conflict,omitempty"`
}

func (s *Service) Bootstrap() Bootstrap {
	if s == nil || s.modules == nil {
		return Bootstrap{GeneratedAt: time.Now().UTC()}
	}
	return Bootstrap{
		GeneratedAt: time.Now().UTC(),
		References:  append([]module.OfflineReferenceDefinition(nil), s.modules.OfflineReferences()...),
		Projections: append([]module.OfflineProjectionDefinition(nil), s.modules.OfflineProjections()...),
		Documents:   append([]module.OfflineDocumentDefinition(nil), s.modules.OfflineDocuments()...),
		Models:      append([]module.OfflineModelDefinition(nil), s.modules.OfflineModels()...),
	}
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

func (s *Service) RememberSyncResult(key string, result SyncResultItem) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncResult[trimmed] = result
}

func (s *Service) SyncResult(key string) (SyncResultItem, bool) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return SyncResultItem{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.syncResult[trimmed]
	return result, ok
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
