package httpx

import (
	"context"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/runtimehealth"
)

type configBundle struct {
	Name          string               `json:"name,omitempty"`
	ExportedAt    time.Time            `json:"exported_at"`
	ExportedBy    string               `json:"exported_by,omitempty"`
	ConfigEntries []config.Entry       `json:"config_entries,omitempty"`
	FeatureFlags  []featureflags.Value `json:"feature_flags,omitempty"`
}

type configBundleRequest struct {
	Name         string   `json:"name,omitempty"`
	ConfigKeys   []string `json:"config_keys,omitempty"`
	ConfigScopes []string `json:"config_scopes,omitempty"`
	IncludeFlags bool     `json:"include_flags,omitempty"`
	FlagKeys     []string `json:"flag_keys,omitempty"`
	FlagScopes   []string `json:"flag_scopes,omitempty"`
}

type configBundleValidation struct {
	Valid    bool                     `json:"valid"`
	Issues   []config.ValidationIssue `json:"issues,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
	Summary  map[string]any           `json:"summary,omitempty"`
}

type rolePermissionMatrix struct {
	Roles       []identity.Role           `json:"roles"`
	Permissions []identity.Permission     `json:"permissions"`
	Grants      []identity.RolePermission `json:"grants"`
	Bindings    []identity.RoleBinding    `json:"bindings"`
	Rows        []rolePermissionMatrixRow `json:"rows"`
}

type rolePermissionMatrixRow struct {
	Permission identity.Permission `json:"permission"`
	GrantedTo  []string            `json:"granted_to"`
}

type adminReadinessReport struct {
	Status              string                  `json:"status"`
	BlockedForApply     bool                    `json:"blocked_for_apply"`
	Health              runtimehealth.Snapshot  `json:"health"`
	ConfigValidation    config.ValidationReport `json:"config_validation"`
	ModuleCompatibility []module.Detail         `json:"module_compatibility,omitempty"`
	Warnings            []string                `json:"warnings,omitempty"`
}

func exportConfigBundle(cfg *config.Service, flags *featureflags.Service, req configBundleRequest, actorID string) configBundle {
	bundle := configBundle{
		Name:          strings.TrimSpace(req.Name),
		ExportedAt:    time.Now().UTC(),
		ExportedBy:    actorID,
		ConfigEntries: filteredConfigEntries(cfg.Entries(), req.ConfigKeys, req.ConfigScopes),
	}
	if req.IncludeFlags {
		bundle.FeatureFlags = filteredFlagValues(flags.Values(), req.FlagKeys, req.FlagScopes)
	}
	return bundle
}

func validateConfigBundle(cfg *config.Service, flags *featureflags.Service, modules *module.Service, bundle configBundle) configBundleValidation {
	report := configBundleValidation{Valid: true, Summary: map[string]any{
		"config_entries": len(bundle.ConfigEntries),
		"feature_flags":  len(bundle.FeatureFlags),
	}}
	for _, entry := range bundle.ConfigEntries {
		validation := cfg.ValidateEntry(entry)
		if !validation.Valid {
			report.Valid = false
			report.Issues = append(report.Issues, validation.Issues...)
		}
		if detail, ok := modules.Get(entry.ModuleKey); ok && !detail.Installed.Enabled {
			report.Valid = false
			report.Issues = append(report.Issues, config.ValidationIssue{
				Key:      entry.Key,
				Severity: "error",
				Message:  "module is disabled",
			})
		}
		if warning := configReadinessWarning(entry.Key); warning != "" {
			report.Warnings = append(report.Warnings, warning)
		}
	}
	for _, value := range bundle.FeatureFlags {
		view, ok := flags.TargetingView(value.FlagKey, scopeIDForFeatureFlag(value.Scope, value.ScopeID, "organization"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "location"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "operating_unit"), time.Now().UTC())
		if !ok {
			report.Valid = false
			report.Issues = append(report.Issues, config.ValidationIssue{
				Key:      value.FlagKey,
				Severity: "error",
				Message:  "feature flag definition not found",
			})
			continue
		}
		if !containsAdminString(view.Definition.AllowedScopes, value.Scope) {
			report.Valid = false
			report.Issues = append(report.Issues, config.ValidationIssue{
				Key:      value.FlagKey,
				Severity: "error",
				Message:  "feature flag scope is not allowed",
			})
		}
	}
	return report
}

func buildRolePermissionMatrix(ident *identity.Service) rolePermissionMatrix {
	roles := ident.Roles()
	permissions := ident.Permissions()
	grants := ident.RolePermissions()
	bindings := ident.Bindings()
	rows := make([]rolePermissionMatrixRow, 0, len(permissions))
	for _, permission := range permissions {
		row := rolePermissionMatrixRow{Permission: permission}
		for _, grant := range grants {
			if grant.PermissionKey == permission.Key {
				row.GrantedTo = append(row.GrantedTo, grant.RoleID)
			}
		}
		sort.Strings(row.GrantedTo)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Permission.Key < rows[j].Permission.Key })
	return rolePermissionMatrix{
		Roles:       roles,
		Permissions: permissions,
		Grants:      grants,
		Bindings:    bindings,
		Rows:        rows,
	}
}

func buildAdminReadinessReport(cfg *config.Service, modules *module.Service, health *runtimehealth.Tracker) adminReadinessReport {
	report := adminReadinessReport{
		Status:           "ready",
		ConfigValidation: cfg.ValidateAll("", ""),
	}
	if health != nil {
		report.Health = health.Snapshot(context.Background())
	}
	for _, issue := range modules.CompatibilityReport() {
		if !moduleCompatibilityBlocksApply(issue) {
			continue
		}
		report.ModuleCompatibility = append(report.ModuleCompatibility, issue)
	}
	for _, issue := range report.ConfigValidation.Issues {
		if warning := configReadinessWarning(issue.Key); warning != "" {
			report.Warnings = append(report.Warnings, warning)
		}
	}
	if !report.Health.Ready || !report.ConfigValidation.Valid || len(report.ModuleCompatibility) > 0 {
		report.Status = "degraded"
	}
	if !report.ConfigValidation.Valid || len(report.ModuleCompatibility) > 0 {
		report.BlockedForApply = true
		report.Status = "blocked_for_apply"
	}
	return report
}

func moduleCompatibilityBlocksApply(detail module.Detail) bool {
	if !detail.Installed.Enabled {
		return false
	}
	switch detail.LifecycleState {
	case "", "healthy", "disabled":
		return false
	}
	for _, diagnostic := range detail.DependencyDiagnostics {
		if !diagnostic.Enabled || !diagnostic.Compatible {
			return true
		}
	}
	for _, diagnostic := range detail.KernelDiagnostics {
		if diagnostic.Severity == module.SeverityError {
			return true
		}
	}
	return detail.LifecycleState == "blocked"
}

func filteredConfigEntries(entries []config.Entry, keys, scopes []string) []config.Entry {
	filtered := make([]config.Entry, 0, len(entries))
	for _, entry := range entries {
		if len(keys) > 0 && !containsAdminString(keys, entry.Key) {
			continue
		}
		if len(scopes) > 0 && !containsAdminString(scopes, entry.Scope) {
			continue
		}
		filtered = append(filtered, entry)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Key == filtered[j].Key {
			if filtered[i].Scope == filtered[j].Scope {
				return filtered[i].ScopeID < filtered[j].ScopeID
			}
			return filtered[i].Scope < filtered[j].Scope
		}
		return filtered[i].Key < filtered[j].Key
	})
	return filtered
}

func filteredFlagValues(values []featureflags.Value, keys, scopes []string) []featureflags.Value {
	filtered := make([]featureflags.Value, 0, len(values))
	for _, value := range values {
		if len(keys) > 0 && !containsAdminString(keys, value.FlagKey) {
			continue
		}
		if len(scopes) > 0 && !containsAdminString(scopes, value.Scope) {
			continue
		}
		filtered = append(filtered, value)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].FlagKey == filtered[j].FlagKey {
			if filtered[i].Scope == filtered[j].Scope {
				return filtered[i].ScopeID < filtered[j].ScopeID
			}
			return filtered[i].Scope < filtered[j].Scope
		}
		return filtered[i].FlagKey < filtered[j].FlagKey
	})
	return filtered
}

func configReadinessWarning(key string) string {
	switch strings.TrimSpace(key) {
	case "identity.auth":
		return "authentication configuration changes can affect login and provisioning readiness"
	case "search.typesense", "search.embedding":
		return "search configuration changes can affect search and projection readiness"
	case "eventing.nats":
		return "eventing configuration changes can affect external delivery readiness"
	default:
		return ""
	}
}

func scopeIDForFeatureFlag(scope, scopeID, expected string) string {
	if scope == expected {
		return scopeID
	}
	return ""
}

func containsAdminString(items []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, item := range items {
		if strings.TrimSpace(item) == candidate {
			return true
		}
	}
	return false
}
