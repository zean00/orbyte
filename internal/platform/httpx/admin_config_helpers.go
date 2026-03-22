package httpx

import (
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/shared"
)

func redactValue(def config.Definition, value map[string]any) map[string]any {
	redacted := make(map[string]any, len(value))
	for key, current := range value {
		redacted[key] = current
	}
	for _, field := range def.Fields {
		if field.Sensitive {
			if _, ok := redacted[field.Key]; ok {
				redacted[field.Key] = "[redacted]"
			}
		}
	}
	return redacted
}

func preserveSensitiveValues(def config.Definition, incoming, existing map[string]any) map[string]any {
	preserved := map[string]any{}
	for key, value := range incoming {
		preserved[key] = value
	}
	for _, field := range def.Fields {
		if !field.Sensitive {
			continue
		}
		current, ok := preserved[field.Key]
		if !ok {
			continue
		}
		text, ok := current.(string)
		if !ok || text != "[redacted]" {
			continue
		}
		if existingValue, ok := existing[field.Key]; ok {
			preserved[field.Key] = existingValue
		} else {
			preserved[field.Key] = ""
		}
	}
	return preserved
}

func saveConfigEntry(cfg *config.Service, modules *module.Service, auditSvc *audit.Service, def config.Definition, req configUpdateRequest, actorID string) (config.EffectiveValue, error) {
	scope := strings.TrimSpace(req.Scope)
	scopeID := strings.TrimSpace(req.ScopeID)
	if scope == "" {
		scope = "deployment"
	}
	if detail, ok := modules.Get(def.ModuleKey); ok && !detail.Installed.Enabled {
		return config.EffectiveValue{}, shared.Conflict("module is disabled")
	}
	var existing map[string]any
	if current, ok := cfg.Resolve(def.Key, scopeIDIfOrganization(scope, scopeID), scopeIDIfLocation(scope, scopeID)); ok {
		existing = current.Value
	}
	previousRedacted := redactValue(def, existing)
	validation := cfg.ValidateEntry(config.Entry{
		Key:       def.Key,
		ModuleKey: def.ModuleKey,
		Category:  def.Category,
		Scope:     scope,
		ScopeID:   scopeID,
		Value:     preserveSensitiveValues(def, req.Value, existing),
	})
	entry := config.Entry{
		Key:         def.Key,
		ModuleKey:   def.ModuleKey,
		Category:    def.Category,
		Scope:       scope,
		ScopeID:     scopeID,
		Value:       preserveSensitiveValues(def, req.Value, existing),
		UpdatedAt:   time.Now().UTC(),
		UpdatedBy:   actorID,
		Description: def.Description,
	}
	if err := cfg.Save(entry); err != nil {
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:config:reject:" + def.Key + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:        "configuration.reject",
			TargetType:    "configuration",
			TargetID:      def.Key,
			ActorID:       actorID,
			OccurredAt:    time.Now().UTC(),
			CorrelationID: "configuration:reject:" + def.Key,
		})
		return config.EffectiveValue{}, err
	}
	newRedacted := redactValue(def, entry.Value)
	recordAudit(auditSvc, audit.Event{
		ID:            "audit:config:update:" + def.Key + ":" + time.Now().UTC().Format("20060102150405.000000000"),
		Action:        "configuration.update",
		TargetType:    "configuration",
		TargetID:      def.Key,
		ActorID:       actorID,
		OccurredAt:    time.Now().UTC(),
		CorrelationID: "configuration:update:" + def.Key,
		Metadata: map[string]any{
			"scope":             entry.Scope,
			"scope_id":          entry.ScopeID,
			"previous_value":    previousRedacted,
			"new_value":         newRedacted,
			"changed_fields":    configChangedFields(previousRedacted, newRedacted),
			"validation_valid":  validation.Valid,
			"validation_issues": validation.Issues,
		},
	})
	orgID := ""
	locationID := ""
	if scope == "organization" {
		orgID = scopeID
	}
	if scope == "location" {
		locationID = scopeID
	}
	effective, ok := cfg.Resolve(def.Key, orgID, locationID)
	if !ok {
		return config.EffectiveValue{}, shared.NotFound("configuration entry not found")
	}
	effective.Value = redactValue(def, effective.Value)
	return effective, nil
}

func configChangedFields(left, right map[string]any) []string {
	if left == nil {
		left = map[string]any{}
	}
	if right == nil {
		right = map[string]any{}
	}
	seen := map[string]struct{}{}
	for key := range left {
		seen[key] = struct{}{}
	}
	for key := range right {
		seen[key] = struct{}{}
	}
	fields := make([]string, 0, len(seen))
	for key := range seen {
		if stringifyAny(left[key]) != stringifyAny(right[key]) {
			fields = append(fields, key)
		}
	}
	sort.Strings(fields)
	return fields
}

func scopeIDIfOrganization(scope, scopeID string) string {
	if scope == "organization" {
		return scopeID
	}
	return ""
}

func scopeIDIfLocation(scope, scopeID string) string {
	if scope == "location" {
		return scopeID
	}
	return ""
}
