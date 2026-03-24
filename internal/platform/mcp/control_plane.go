package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/shared"
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
	Health              any                     `json:"health"`
	ConfigValidation    config.ValidationReport `json:"config_validation"`
	ModuleCompatibility []module.Detail         `json:"module_compatibility,omitempty"`
	Warnings            []string                `json:"warnings,omitempty"`
}

type implementationRoleGrant struct {
	RoleID        string `json:"role_id"`
	PermissionKey string `json:"permission_key"`
}

func (s *Server) readJSONControlResource(actor ActorContext, uri, permission string, provider func(ActorContext) (map[string]any, error)) ([]ResourceContent, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{permission}) {
		return nil, true, fmt.Errorf("resource is not allowed")
	}
	if provider == nil {
		return nil, true, fmt.Errorf("resource provider is unavailable")
	}
	payload, err := provider(actor)
	if err != nil {
		return nil, true, err
	}
	body, _ := json.Marshal(payload)
	return []ResourceContent{{URI: uri, MIMEType: "application/json", Text: string(body)}}, true, nil
}

func (s *Server) configCatalogResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.config == nil {
		return nil, fmt.Errorf("config is unavailable")
	}
	return map[string]any{
		"definitions": s.config.Definitions(),
		"entries":     s.config.Entries(),
	}, nil
}

func (s *Server) flagCatalogResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.flags == nil {
		return nil, fmt.Errorf("feature flags are unavailable")
	}
	return map[string]any{
		"definitions": s.flags.Definitions(),
		"values":      s.flags.Values(),
	}, nil
}

func (s *Server) roleMatrixResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.identity == nil {
		return nil, fmt.Errorf("identity is unavailable")
	}
	return map[string]any{"matrix": buildRolePermissionMatrix(s.identity)}, nil
}

func (s *Server) moduleCompatibilityResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.modules == nil {
		return nil, fmt.Errorf("modules are unavailable")
	}
	return map[string]any{
		"items":         s.modules.List(),
		"compatibility": s.modules.CompatibilityReport(),
	}, nil
}

func (s *Server) integrationHealthResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.integration == nil {
		return nil, fmt.Errorf("integrations are unavailable")
	}
	return map[string]any{
		"health":              s.integration.HealthSummary(),
		"submissions":         s.integration.ListSubmissions(),
		"dead_letters":        s.integration.ListDeadLetters(),
		"adapter_descriptors": s.integration.AdapterDescriptors(),
		"retry_policy":        s.integration.RetryPolicy(),
	}, nil
}

func (s *Server) readinessResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.config == nil || s.modules == nil {
		return nil, fmt.Errorf("readiness is unavailable")
	}
	return map[string]any{"readiness": buildAdminReadinessReport(s.config, s.modules, s.health)}, nil
}

func (s *Server) runbooksResource(actor ActorContext) (map[string]any, error) {
	_ = actor
	if s == nil || s.health == nil {
		return nil, fmt.Errorf("runtime health is unavailable")
	}
	snapshot := s.health.Snapshot(context.Background())
	items := make([]map[string]any, 0, len(snapshot.Subsystems))
	for _, subsystem := range snapshot.Subsystems {
		if subsystem.RunbookID == "" && subsystem.OperatorHint == "" {
			continue
		}
		items = append(items, map[string]any{
			"subsystem":         subsystem.Name,
			"status":            subsystem.Status,
			"failure_category":  subsystem.FailureCategory,
			"runbook_id":        subsystem.RunbookID,
			"operator_hint":     subsystem.OperatorHint,
			"impacts_readiness": subsystem.ImpactsReadiness,
		})
	}
	return map[string]any{"items": items, "snapshot": snapshot}, nil
}

func (s *Server) configDefinitionList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.config == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.config.Definitions()
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d configuration definitions.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) configEntryList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.config == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	req := configBundleRequest{
		ConfigKeys:   stringSliceArg(arguments, "config_keys"),
		ConfigScopes: stringSliceArg(arguments, "config_scopes"),
	}
	items := filteredConfigEntries(s.config.Entries(), req.ConfigKeys, req.ConfigScopes)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d configuration entries.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) configEffectiveGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.config == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.config.ResolveAll(stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Resolved %d effective configuration values.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) configCompare(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.config == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	left := compareContextFromArgs(arguments, "left")
	right := compareContextFromArgs(arguments, "right")
	items := s.config.CompareContexts(left, right)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Compared %d configuration keys.", len(items))}},
		"structuredContent": map[string]any{
			"left":  left,
			"right": right,
			"items": items,
		},
	}, true, nil
}

func (s *Server) configBundleExport(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.config == nil || s.flags == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var req configBundleRequest
	if err := decodeOptionalObjectArg(arguments, "request", &req); err != nil {
		return nil, true, err
	}
	if req.Name == "" {
		req.Name = strings.TrimSpace(stringArg(arguments, "name"))
	}
	if len(req.ConfigKeys) == 0 {
		req.ConfigKeys = stringSliceArg(arguments, "config_keys")
	}
	if len(req.ConfigScopes) == 0 {
		req.ConfigScopes = stringSliceArg(arguments, "config_scopes")
	}
	if !req.IncludeFlags {
		req.IncludeFlags = boolArg(arguments, "include_flags")
	}
	if len(req.FlagKeys) == 0 {
		req.FlagKeys = stringSliceArg(arguments, "flag_keys")
	}
	if len(req.FlagScopes) == 0 {
		req.FlagScopes = stringSliceArg(arguments, "flag_scopes")
	}
	bundle := exportConfigBundle(s.config, s.flags, req, workflowActorID(actor))
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Exported bundle with %d config entries and %d feature flag values.", len(bundle.ConfigEntries), len(bundle.FeatureFlags))}},
		"structuredContent": map[string]any{"bundle": bundle},
	}, true, nil
}

func (s *Server) configBundleValidate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.config == nil || s.flags == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var bundle configBundle
	if err := decodeObjectArg(arguments, "bundle", &bundle); err != nil {
		return nil, true, err
	}
	validation := validateConfigBundle(s.config, s.flags, s.modules, bundle)
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Validated bundle: valid=%t.", validation.Valid)}},
		"structuredContent": map[string]any{"validation": validation},
	}, true, nil
}

func (s *Server) configBundleApply(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.config == nil || s.flags == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	var bundle configBundle
	if err := decodeObjectArg(arguments, "bundle", &bundle); err != nil {
		return nil, true, err
	}
	validation := validateConfigBundle(s.config, s.flags, s.modules, bundle)
	if !validation.Valid {
		return map[string]any{
			"content":           []ContentBlock{{Type: "text", Text: "Bundle apply blocked by validation issues."}},
			"structuredContent": map[string]any{"executed": false, "validation": validation},
		}, true, nil
	}
	appliedConfig := make([]config.EffectiveValue, 0, len(bundle.ConfigEntries))
	for _, entry := range bundle.ConfigEntries {
		entry.UpdatedBy = workflowActorID(actor)
		if err := s.config.Save(entry); err != nil {
			return nil, true, err
		}
		effective, _ := s.config.Resolve(entry.Key, scopeIDForConfig(entry.Scope, entry.ScopeID, "organization"), scopeIDForConfig(entry.Scope, entry.ScopeID, "location"))
		appliedConfig = append(appliedConfig, effective)
	}
	appliedFlags := make([]featureflags.Value, 0, len(bundle.FeatureFlags))
	for _, value := range bundle.FeatureFlags {
		value.UpdatedBy = workflowActorID(actor)
		if err := s.flags.UpsertValue(value); err != nil {
			return nil, true, err
		}
		appliedFlags = append(appliedFlags, value)
	}
	correlationID := shared.ChildID("mcp", "config.bundle.apply")
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Applied bundle with %d config entries and %d feature flag values.", len(appliedConfig), len(appliedFlags))}},
		"structuredContent": map[string]any{
			"executed":       true,
			"correlation_id": correlationID,
			"config_entries": appliedConfig,
			"feature_flags":  appliedFlags,
			"validation":     validation,
		},
	}, true, nil
}

func (s *Server) featureFlagDefinitionList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.flags == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.flags.Definitions()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d feature flag definitions.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) featureFlagValueList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.flags == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.flags.Values()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d feature flag values.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) featureFlagTargetingGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.flags == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	flagKey := strings.TrimSpace(stringArg(arguments, "flag_key"))
	if flagKey == "" {
		return nil, true, shared.Validation("flag_key is required")
	}
	view, ok := s.flags.TargetingView(flagKey, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"), stringArg(arguments, "operating_unit_id"), time.Now().UTC())
	if !ok {
		return nil, true, shared.NotFound("feature flag definition not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded targeting view for feature flag %s.", flagKey)}}, "structuredContent": view}, true, nil
}

func (s *Server) featureFlagValueUpsert(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.flags == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	var value featureflags.Value
	if err := decodeObjectArg(arguments, "value", &value); err != nil {
		return nil, true, err
	}
	value.UpdatedBy = workflowActorID(actor)
	if err := s.flags.UpsertValue(value); err != nil {
		return nil, true, err
	}
	view, _ := s.flags.TargetingView(value.FlagKey, scopeIDForFeatureFlag(value.Scope, value.ScopeID, "organization"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "location"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "operating_unit"), time.Now().UTC())
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated feature flag %s.", value.FlagKey)}}, "structuredContent": map[string]any{"executed": true, "value": value, "targeting": view}}, true, nil
}

func (s *Server) identityRolePermissionMatrixGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.identity == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	matrix := buildRolePermissionMatrix(s.identity)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded role matrix with %d roles and %d permissions.", len(matrix.Roles), len(matrix.Permissions))}}, "structuredContent": matrix}, true, nil
}

func (s *Server) identityRolePermissionGrant(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.identity == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	roleID := strings.TrimSpace(stringArg(arguments, "role_id"))
	permissionKey := strings.TrimSpace(stringArg(arguments, "permission_key"))
	if roleID == "" || permissionKey == "" {
		return nil, true, shared.Validation("role_id and permission_key are required")
	}
	if err := s.identity.GrantRolePermission(identity.RolePermission{RoleID: roleID, PermissionKey: permissionKey}); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Granted %s to %s.", permissionKey, roleID)}}, "structuredContent": map[string]any{"executed": true, "role_id": roleID, "permission_key": permissionKey}}, true, nil
}

func (s *Server) identityRolePermissionRevoke(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.identity == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	roleID := strings.TrimSpace(stringArg(arguments, "role_id"))
	permissionKey := strings.TrimSpace(stringArg(arguments, "permission_key"))
	if roleID == "" || permissionKey == "" {
		return nil, true, shared.Validation("role_id and permission_key are required")
	}
	if err := s.identity.RevokeRolePermission(roleID, permissionKey); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Revoked %s from %s.", permissionKey, roleID)}}, "structuredContent": map[string]any{"executed": true, "role_id": roleID, "permission_key": permissionKey}}, true, nil
}

func (s *Server) moduleList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.modules.List()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d modules.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) moduleCompatibilityList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.modules.CompatibilityReport()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d module compatibility records.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) integrationAdapterList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.integration.ListAdapterDescriptors()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d integration adapters.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) integrationSystemList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.integration.ListSystems()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d integration systems.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) integrationSystemConfigGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	key := strings.TrimSpace(stringArg(arguments, "system_key"))
	if key == "" {
		return nil, true, shared.Validation("system_key is required")
	}
	view, err := s.integration.ValidateSystemConfig(key)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded integration system config for %s.", key)}}, "structuredContent": map[string]any{"system_key": key, "view": view}}, true, nil
}

func (s *Server) integrationSystemConfigUpdate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	key := strings.TrimSpace(stringArg(arguments, "system_key"))
	if key == "" {
		return nil, true, shared.Validation("system_key is required")
	}
	settings := objectArg(arguments, "settings")
	system, view, err := s.integration.UpdateSystemSettings(key, settings)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated integration system config for %s.", key)}}, "structuredContent": map[string]any{"executed": true, "system": system, "view": view}}, true, nil
}

func (s *Server) integrationEndpointList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.integration.ListEndpoints()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d integration endpoints.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) integrationEndpointConfigGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	key := strings.TrimSpace(stringArg(arguments, "endpoint_key"))
	if key == "" {
		return nil, true, shared.Validation("endpoint_key is required")
	}
	view, err := s.integration.ValidateEndpointConfig(key)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded integration endpoint config for %s.", key)}}, "structuredContent": map[string]any{"endpoint_key": key, "view": view}}, true, nil
}

func (s *Server) integrationEndpointConfigUpdate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	key := strings.TrimSpace(stringArg(arguments, "endpoint_key"))
	if key == "" {
		return nil, true, shared.Validation("endpoint_key is required")
	}
	settings := objectArg(arguments, "settings")
	endpoint, view, err := s.integration.UpdateEndpointSettings(key, settings)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated integration endpoint config for %s.", key)}}, "structuredContent": map[string]any{"executed": true, "endpoint": endpoint, "view": view}}, true, nil
}

func (s *Server) integrationSubmissionList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.integration.ListSubmissions()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d integration submissions.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) integrationSubmissionGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "submission_id"))
	if id == "" {
		return nil, true, shared.Validation("submission_id is required")
	}
	record, ok := s.integration.GetSubmission(id)
	if !ok {
		return nil, true, shared.NotFound("integration submission not found")
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded integration submission %s.", id)}}, "structuredContent": map[string]any{"record": record, "attempts": s.integration.ListSubmissionAttempts(id)}}, true, nil
}

func (s *Server) integrationDeadLetterList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.integration.ListDeadLetters()
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d integration dead letters.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) integrationDeadLetterReplay(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.integration == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	id := strings.TrimSpace(stringArg(arguments, "dead_letter_id"))
	if id == "" {
		return nil, true, shared.Validation("dead_letter_id is required")
	}
	record, err := s.integration.ReplayDeadLetter(id)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Replayed dead letter %s.", id)}}, "structuredContent": map[string]any{"executed": true, "record": record}}, true, nil
}

func (s *Server) readinessGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.config == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	report := buildAdminReadinessReport(s.config, s.modules, s.health)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Current readiness status is %s.", report.Status)}}, "structuredContent": report}, true, nil
}

func (s *Server) opsHealthGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.health == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	snapshot := s.health.Snapshot(context.Background())
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Runtime health is %s.", snapshot.Status)}}, "structuredContent": snapshot}, true, nil
}

func (s *Server) opsAuditCorrelationGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.audit == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	correlationID := strings.TrimSpace(stringArg(arguments, "correlation_id"))
	if correlationID == "" {
		return nil, true, shared.Validation("correlation_id is required")
	}
	items := make([]audit.Event, 0)
	for _, item := range s.audit.List() {
		if strings.TrimSpace(item.CorrelationID) == correlationID {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d correlated audit events.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func (s *Server) opsTraceGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	correlationID := strings.TrimSpace(stringArg(arguments, "correlation_id"))
	if correlationID == "" {
		return nil, true, shared.Validation("correlation_id is required")
	}
	trace := buildOperationalTrace(correlationID, s.audit, s.eventing, s.workflows, s.jobs, s.integration, s.offline)
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Built operational trace with %d step(s).", len(anySlice(trace["steps"])))}}, "structuredContent": trace}, true, nil
}

func (s *Server) implementationTenantInspect(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	orgID := stringArg(arguments, "organization_id")
	locID := stringArg(arguments, "location_id")
	ouID := stringArg(arguments, "operating_unit_id")
	var readiness any
	if s.config != nil && s.modules != nil {
		readiness = buildAdminReadinessReport(s.config, s.modules, s.health)
	}
	payload := map[string]any{
		"effective_config": func() any {
			if s.config == nil {
				return nil
			}
			return s.config.ResolveAll(orgID, locID)
		}(),
		"feature_flags": func() any {
			if s.flags == nil {
				return nil
			}
			return s.flags.ResolveAllWithOperatingUnit(orgID, locID, ouID, time.Now().UTC())
		}(),
		"module_compatibility": func() any {
			if s.modules == nil {
				return nil
			}
			return s.modules.CompatibilityReport()
		}(),
		"role_matrix": func() any {
			if s.identity == nil {
				return nil
			}
			return buildRolePermissionMatrix(s.identity)
		}(),
		"integration_health": func() any {
			if s.integration == nil {
				return nil
			}
			return s.integration.HealthSummary()
		}(),
		"readiness": readiness,
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Aggregated tenant implementation state."}}, "structuredContent": payload}, true, nil
}

func (s *Server) implementationConfigPlan(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var bundle configBundle
	if err := decodeOptionalObjectArg(arguments, "bundle", &bundle); err != nil {
		return nil, true, err
	}
	roleGrants := decodeRoleGrants(arguments)
	validation := configBundleValidation{Valid: true}
	if s.config != nil && s.flags != nil && s.modules != nil {
		validation = validateConfigBundle(s.config, s.flags, s.modules, bundle)
	}
	payload := map[string]any{
		"bundle":          bundle,
		"role_grants":     roleGrants,
		"validation":      validation,
		"requires_apply":  len(bundle.ConfigEntries) > 0 || len(bundle.FeatureFlags) > 0 || len(roleGrants) > 0,
		"affected_scopes": summarizeBundleScopes(bundle),
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Prepared implementation plan with %d config entries, %d feature flags, and %d role grants.", len(bundle.ConfigEntries), len(bundle.FeatureFlags), len(roleGrants))}}, "structuredContent": payload}, true, nil
}

func (s *Server) implementationConfigApply(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	if !boolArg(arguments, "confirm_apply") {
		return nil, true, shared.Validation("confirm_apply must be true")
	}
	var bundle configBundle
	if err := decodeOptionalObjectArg(arguments, "bundle", &bundle); err != nil {
		return nil, true, err
	}
	roleGrants := decodeRoleGrants(arguments)
	validation := configBundleValidation{Valid: true}
	if s.config != nil && s.flags != nil && s.modules != nil {
		validation = validateConfigBundle(s.config, s.flags, s.modules, bundle)
	}
	if !validation.Valid {
		return map[string]any{"content": []ContentBlock{{Type: "text", Text: "Implementation apply blocked by validation issues."}}, "structuredContent": map[string]any{"executed": false, "validation": validation}}, true, nil
	}
	configResult, _, err := s.configBundleApply(actor, map[string]any{"bundle": bundle, "confirm_apply": true})
	if err != nil {
		return nil, true, err
	}
	appliedRoleGrants := make([]implementationRoleGrant, 0, len(roleGrants))
	for _, grant := range roleGrants {
		if s.identity == nil {
			break
		}
		if err := s.identity.GrantRolePermission(identity.RolePermission{RoleID: grant.RoleID, PermissionKey: grant.PermissionKey}); err != nil {
			return nil, true, err
		}
		appliedRoleGrants = append(appliedRoleGrants, grant)
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Applied implementation plan with %d role grants.", len(appliedRoleGrants))}},
		"structuredContent": map[string]any{
			"executed":            true,
			"config_result":       configResult["structuredContent"],
			"applied_role_grants": appliedRoleGrants,
			"validation":          validation,
		},
	}, true, nil
}

func (s *Server) implementationReadinessCheck(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.readinessGet(actor, arguments)
}

func (s *Server) implementationRollbackInspect(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := make([]audit.Event, 0)
	if s.audit != nil {
		for _, event := range s.audit.List() {
			if strings.HasPrefix(event.Action, "configuration.") || strings.HasPrefix(event.Action, "feature_flag.") || strings.HasPrefix(event.Action, "identity.role_permission.") || strings.HasPrefix(event.Action, "integration.") {
				items = append(items, event)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.After(items[j].OccurredAt) })
	if len(items) > 50 {
		items = items[:50]
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded %d recent implementation-related audit events.", len(items))}}, "structuredContent": map[string]any{"items": items}}, true, nil
}

func exportConfigBundle(cfg *config.Service, flags *featureflags.Service, req configBundleRequest, actorID string) configBundle {
	bundle := configBundle{Name: strings.TrimSpace(req.Name), ExportedAt: time.Now().UTC(), ExportedBy: actorID, ConfigEntries: filteredConfigEntries(cfg.Entries(), req.ConfigKeys, req.ConfigScopes)}
	if req.IncludeFlags && flags != nil {
		bundle.FeatureFlags = filteredFlagValues(flags.Values(), req.FlagKeys, req.FlagScopes)
	}
	return bundle
}

func validateConfigBundle(cfg *config.Service, flags *featureflags.Service, modules *module.Service, bundle configBundle) configBundleValidation {
	report := configBundleValidation{Valid: true, Summary: map[string]any{"config_entries": len(bundle.ConfigEntries), "feature_flags": len(bundle.FeatureFlags)}}
	for _, entry := range bundle.ConfigEntries {
		validation := cfg.ValidateEntry(entry)
		if !validation.Valid {
			report.Valid = false
			report.Issues = append(report.Issues, validation.Issues...)
		}
		if detail, ok := modules.Get(entry.ModuleKey); ok && !detail.Installed.Enabled {
			report.Valid = false
			report.Issues = append(report.Issues, config.ValidationIssue{Key: entry.Key, Severity: "error", Message: "module is disabled"})
		}
		if warning := configReadinessWarning(entry.Key); warning != "" {
			report.Warnings = append(report.Warnings, warning)
		}
	}
	if flags != nil {
		for _, value := range bundle.FeatureFlags {
			view, ok := flags.TargetingView(value.FlagKey, scopeIDForFeatureFlag(value.Scope, value.ScopeID, "organization"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "location"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "operating_unit"), time.Now().UTC())
			if !ok {
				report.Valid = false
				report.Issues = append(report.Issues, config.ValidationIssue{Key: value.FlagKey, Severity: "error", Message: "feature flag definition not found"})
				continue
			}
			if !containsString(view.Definition.AllowedScopes, value.Scope) {
				report.Valid = false
				report.Issues = append(report.Issues, config.ValidationIssue{Key: value.FlagKey, Severity: "error", Message: "feature flag scope is not allowed"})
			}
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
	return rolePermissionMatrix{Roles: roles, Permissions: permissions, Grants: grants, Bindings: bindings, Rows: rows}
}

func buildAdminReadinessReport(cfg *config.Service, modules *module.Service, health *runtimehealth.Tracker) adminReadinessReport {
	report := adminReadinessReport{Status: "ready", ConfigValidation: cfg.ValidateAll("", "")}
	if health != nil {
		report.Health = health.Snapshot(context.Background())
	}
	for _, issue := range modules.CompatibilityReport() {
		if len(issue.DependencyDiagnostics) == 0 && len(issue.KernelDiagnostics) == 0 {
			continue
		}
		report.ModuleCompatibility = append(report.ModuleCompatibility, issue)
	}
	for _, issue := range report.ConfigValidation.Issues {
		if warning := configReadinessWarning(issue.Key); warning != "" {
			report.Warnings = append(report.Warnings, warning)
		}
	}
	ready := true
	if snapshot, ok := report.Health.(runtimehealth.Snapshot); ok {
		ready = snapshot.Ready
	}
	if !ready || !report.ConfigValidation.Valid || len(report.ModuleCompatibility) > 0 {
		report.Status = "degraded"
	}
	if !report.ConfigValidation.Valid || len(report.ModuleCompatibility) > 0 {
		report.BlockedForApply = true
		report.Status = "blocked_for_apply"
	}
	return report
}

func buildOperationalTrace(correlationID string, auditSvc *audit.Service, eventingSvc *eventing.Service, workflows any, jobSvc *jobs.Service, integrationSvc any, offlineSvc *offline.Service) map[string]any {
	steps := make([]map[string]any, 0)
	if auditSvc != nil {
		for _, item := range auditSvc.List() {
			if strings.TrimSpace(item.CorrelationID) != correlationID {
				continue
			}
			steps = append(steps, map[string]any{
				"kind":        "audit_event",
				"occurred_at": item.OccurredAt,
				"action":      item.Action,
				"target_type": item.TargetType,
				"target_id":   item.TargetID,
			})
		}
	}
	if eventingSvc != nil {
		for _, item := range eventingSvc.ListEvents() {
			if strings.TrimSpace(item.CorrelationID) != correlationID {
				continue
			}
			steps = append(steps, map[string]any{
				"kind":        "domain_event",
				"occurred_at": item.OccurredAt,
				"event_type":  item.Type,
				"target_type": item.AggregateType,
				"target_id":   item.AggregateID,
			})
		}
	}
	if jobSvc != nil {
		for _, item := range jobSvc.List() {
			if jobCorrelationID(item) != correlationID {
				continue
			}
			steps = append(steps, map[string]any{
				"kind":        "job",
				"occurred_at": firstNonZeroTime(item.EndedAt, item.StartedAt, item.CreatedAt),
				"job_id":      item.ID,
				"name":        item.Name,
				"status":      item.Status,
			})
		}
	}
	if offlineSvc != nil {
		for _, item := range offlineSvc.RecentBatches(200) {
			if strings.TrimSpace(item.CorrelationID) != correlationID {
				continue
			}
			steps = append(steps, map[string]any{
				"kind":        "offline_sync",
				"occurred_at": firstNonZeroTime(item.ProcessedAt, item.CreatedAt),
				"batch_id":    item.ID,
				"actor_id":    item.ActorID,
			})
		}
	}
	sort.Slice(steps, func(i, j int) bool {
		return timeValue(steps[i]["occurred_at"]).Before(timeValue(steps[j]["occurred_at"]))
	})
	return map[string]any{"correlation_id": correlationID, "steps": steps}
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func filteredConfigEntries(entries []config.Entry, keys, scopes []string) []config.Entry {
	filtered := make([]config.Entry, 0, len(entries))
	for _, entry := range entries {
		if len(keys) > 0 && !containsString(keys, entry.Key) {
			continue
		}
		if len(scopes) > 0 && !containsString(scopes, entry.Scope) {
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
		if len(keys) > 0 && !containsString(keys, value.FlagKey) {
			continue
		}
		if len(scopes) > 0 && !containsString(scopes, value.Scope) {
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

func compareContextFromArgs(arguments map[string]any, key string) config.CompareContext {
	var raw map[string]any
	if item, ok := arguments[key].(map[string]any); ok {
		raw = item
	}
	return config.CompareContext{
		Label:          key,
		OrganizationID: stringMapValue(raw, "organization_id"),
		LocationID:     stringMapValue(raw, "location_id"),
	}
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

func scopeIDForConfig(scope, scopeID, expected string) string {
	if scope == expected {
		return scopeID
	}
	return ""
}

func containsString(items []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, item := range items {
		if strings.TrimSpace(item) == candidate {
			return true
		}
	}
	return false
}

func stringSliceArg(arguments map[string]any, key string) []string {
	raw, ok := arguments[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		items := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				items = append(items, strings.TrimSpace(text))
			}
		}
		return items
	default:
		return nil
	}
}

func objectArg(arguments map[string]any, key string) map[string]any {
	if raw, ok := arguments[key].(map[string]any); ok {
		return cloneMap(raw)
	}
	return map[string]any{}
}

func decodeRoleGrants(arguments map[string]any) []implementationRoleGrant {
	raw, ok := arguments["role_grants"]
	if !ok {
		return nil
	}
	body, _ := json.Marshal(raw)
	var items []implementationRoleGrant
	_ = json.Unmarshal(body, &items)
	return items
}

func summarizeBundleScopes(bundle configBundle) []string {
	seen := map[string]struct{}{}
	for _, entry := range bundle.ConfigEntries {
		seen[entry.Scope] = struct{}{}
	}
	for _, value := range bundle.FeatureFlags {
		seen[value.Scope] = struct{}{}
	}
	items := make([]string, 0, len(seen))
	for scope := range seen {
		items = append(items, scope)
	}
	sort.Strings(items)
	return items
}

func anySlice(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	default:
		return nil
	}
}

func timeValue(value any) time.Time {
	if item, ok := value.(time.Time); ok {
		return item
	}
	return time.Time{}
}

func jobCorrelationID(item jobs.Job) string {
	if correlation := strings.TrimSpace(stringifyAny(item.Payload["correlation_id"])); correlation != "" {
		return correlation
	}
	return strings.TrimSpace(stringifyAny(item.Result["correlation_id"]))
}

func stringifyAny(value any) string {
	switch item := value.(type) {
	case string:
		return item
	default:
		return fmt.Sprintf("%v", item)
	}
}
