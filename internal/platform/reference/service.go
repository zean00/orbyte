package reference

import (
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type Service struct {
	repo Repository
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RegisterType(def TypeDefinition) error {
	if strings.TrimSpace(def.Key) == "" || strings.TrimSpace(def.DisplayName) == "" {
		return shared.Validation("reference type key and display_name are required")
	}
	if def.ValueType == "" {
		def.ValueType = "json"
	}
	if len(def.AllowedScopes) == 0 {
		def.AllowedScopes = []string{"deployment", "organization", "location"}
	}
	return s.repo.SaveType(def)
}

func (s *Service) Type(key string) (TypeDefinition, bool) {
	return s.repo.GetType(strings.TrimSpace(key))
}

func (s *Service) Types() []TypeDefinition {
	return s.repo.ListTypes()
}

func (s *Service) UpsertRecord(record Record) error {
	if strings.TrimSpace(record.TypeKey) == "" || strings.TrimSpace(record.Key) == "" || strings.TrimSpace(record.DisplayName) == "" {
		return shared.Validation("reference record type_key, key, and display_name are required")
	}
	def, ok := s.repo.GetType(record.TypeKey)
	if !ok {
		return shared.NotFound("reference type not found")
	}
	scope := normalizeScope(record.Scope)
	if !contains(def.AllowedScopes, scope) {
		return shared.Validation("reference record scope is not allowed")
	}
	record.Scope = scope
	if record.Status == "" {
		record.Status = "active"
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = time.Now().UTC()
	}
	return s.repo.SaveRecord(record)
}

func (s *Service) Records(typeKey string) []Record {
	return s.repo.ListRecords(strings.TrimSpace(typeKey))
}

func (s *Service) Resolve(typeKey, organizationID, locationID string, at time.Time) (ResolvedSet, error) {
	typeKey = strings.TrimSpace(typeKey)
	if typeKey == "" {
		return ResolvedSet{}, shared.Validation("reference type key is required")
	}
	if _, ok := s.repo.GetType(typeKey); !ok {
		return ResolvedSet{}, shared.NotFound("reference type not found")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	records := s.repo.ListRecords(typeKey)
	candidates := make([]Record, 0, len(records))
	for _, record := range records {
		if !isInScope(record, organizationID, locationID) || !isEffective(record, at) || strings.TrimSpace(record.Status) == "inactive" {
			continue
		}
		candidates = append(candidates, record)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Key == candidates[j].Key {
			return scopeRank(candidates[i].Scope) > scopeRank(candidates[j].Scope)
		}
		return candidates[i].Key < candidates[j].Key
	})
	byKey := map[string]Record{}
	sourceScopes := map[string]bool{}
	for _, item := range candidates {
		if _, ok := byKey[item.Key]; ok {
			continue
		}
		byKey[item.Key] = item
		sourceScopes[item.Scope] = true
	}
	items := make([]Record, 0, len(byKey))
	for _, item := range byKey {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	scopes := make([]string, 0, len(sourceScopes))
	for scope := range sourceScopes {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return ResolvedSet{
		TypeKey:      typeKey,
		Scope:        firstNonEmpty(scopeFor(locationID, "location"), scopeFor(organizationID, "organization"), "deployment"),
		ScopeID:      firstNonEmpty(locationID, organizationID),
		ResolvedAt:   at,
		SourceScopes: scopes,
		Items:        items,
	}, nil
}

func normalizeScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "deployment"
	}
	return scope
}

func isInScope(record Record, organizationID, locationID string) bool {
	switch normalizeScope(record.Scope) {
	case "deployment":
		return true
	case "organization":
		return record.ScopeID == "" || record.ScopeID == strings.TrimSpace(organizationID)
	case "location":
		return record.ScopeID != "" && record.ScopeID == strings.TrimSpace(locationID)
	default:
		return false
	}
}

func isEffective(record Record, at time.Time) bool {
	if !record.EffectiveFrom.IsZero() && at.Before(record.EffectiveFrom) {
		return false
	}
	if !record.EffectiveTo.IsZero() && at.After(record.EffectiveTo) {
		return false
	}
	return true
}

func scopeRank(scope string) int {
	switch normalizeScope(scope) {
	case "location":
		return 3
	case "organization":
		return 2
	default:
		return 1
	}
}

func scopeFor(id, scope string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	return scope
}

func contains(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
