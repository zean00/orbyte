package featureflags

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
	svc := &Service{repo: repo}
	now := time.Now().UTC()
	for _, def := range []Definition{
		{Key: "platform.admin_console", ModuleKey: "platform.core", Description: "Enable platform admin console surface.", AllowedScopes: []string{"deployment", "organization", "location", "operating_unit"}, DefaultState: true, CreatedAt: now, UpdatedAt: now},
		{Key: "platform.mcp_templates", ModuleKey: "platform.core", Description: "Enable MCP template-designer workflow.", AllowedScopes: []string{"deployment", "organization", "location", "operating_unit"}, DefaultState: true, CreatedAt: now, UpdatedAt: now},
	} {
		_ = svc.RegisterDefinition(def)
	}
	return svc
}

func (s *Service) RegisterDefinition(def Definition) error {
	if strings.TrimSpace(def.Key) == "" {
		return shared.Validation("feature flag key is required")
	}
	if strings.TrimSpace(def.ModuleKey) == "" {
		def.ModuleKey = "platform.core"
	}
	if len(def.AllowedScopes) == 0 {
		def.AllowedScopes = []string{"deployment"}
	}
	now := time.Now().UTC()
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	def.UpdatedAt = now
	return s.repo.SaveDefinition(def)
}

func (s *Service) Definitions() []Definition {
	return s.repo.ListDefinitions()
}

func (s *Service) Values() []Value {
	return s.repo.ListValues()
}

func (s *Service) UpsertValue(value Value) error {
	def, ok := s.repo.GetDefinition(strings.TrimSpace(value.FlagKey))
	if !ok {
		return shared.NotFound("feature flag definition not found")
	}
	if value.Scope == "" {
		value.Scope = "deployment"
	}
	if !containsScope(def.AllowedScopes, value.Scope) {
		return shared.Validation("feature flag scope is not allowed")
	}
	if value.Status == "" {
		value.Status = "active"
	}
	now := time.Now().UTC()
	value.UpdatedAt = now
	if value.UpdatedBy == "" {
		value.UpdatedBy = "system"
	}
	return s.repo.SaveValue(value)
}

func (s *Service) Resolve(key, organizationID, locationID string, at time.Time) (EffectiveValue, bool) {
	return s.ResolveWithOperatingUnit(key, organizationID, locationID, "", at)
}

func (s *Service) ResolveWithOperatingUnit(key, organizationID, locationID, operatingUnitID string, at time.Time) (EffectiveValue, bool) {
	def, ok := s.repo.GetDefinition(strings.TrimSpace(key))
	if !ok {
		return EffectiveValue{}, false
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	effective := def.DefaultState
	sourceScope := "default"
	sourceScopeID := ""
	for _, candidate := range []struct {
		scope   string
		scopeID string
	}{
		{scope: "deployment", scopeID: ""},
		{scope: "organization", scopeID: organizationID},
		{scope: "location", scopeID: locationID},
		{scope: "operating_unit", scopeID: operatingUnitID},
	} {
		if candidate.scope != "deployment" && candidate.scopeID == "" {
			continue
		}
		if !containsScope(def.AllowedScopes, candidate.scope) {
			continue
		}
		value, ok := s.repo.GetValue(def.Key, candidate.scope, candidate.scopeID)
		if !ok || value.Status != "active" {
			continue
		}
		if !value.EffectiveFrom.IsZero() && at.Before(value.EffectiveFrom) {
			continue
		}
		if !value.EffectiveTo.IsZero() && at.After(value.EffectiveTo) {
			continue
		}
		effective = value.Enabled
		sourceScope = candidate.scope
		sourceScopeID = candidate.scopeID
	}
	return EffectiveValue{
		Key:           def.Key,
		Enabled:       effective,
		SourceScope:   sourceScope,
		SourceScopeID: sourceScopeID,
		OperatingUnitID: operatingUnitID,
		ResolvedAt:    at,
	}, true
}

func (s *Service) ResolveAll(organizationID, locationID string, at time.Time) []EffectiveValue {
	return s.ResolveAllWithOperatingUnit(organizationID, locationID, "", at)
}

func (s *Service) ResolveAllWithOperatingUnit(organizationID, locationID, operatingUnitID string, at time.Time) []EffectiveValue {
	defs := s.Definitions()
	items := make([]EffectiveValue, 0, len(defs))
	for _, def := range defs {
		if value, ok := s.ResolveWithOperatingUnit(def.Key, organizationID, locationID, operatingUnitID, at); ok {
			items = append(items, value)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func containsScope(scopes []string, candidate string) bool {
	for _, scope := range scopes {
		if scope == candidate {
			return true
		}
	}
	return false
}
