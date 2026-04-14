package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/shared"
)

func registerAdminConfigRuntimeRoutes(mux *http.ServeMux, cfg *config.Service, flags *featureflags.Service, ident *identity.Service, modules *module.Service, auditSvc *audit.Service, health *runtimehealth.Tracker) {
	mux.HandleFunc("GET /admin/api/config/definitions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		items := cfg.Definitions()
		pagedItems, total := paginateSlice(items, intQuery(r, "page", 1), intQuery(r, "page_size", 20))
		respondJSON(w, http.StatusOK, map[string]any{"items": pagedItems, "total": total})
	})

	mux.HandleFunc("GET /admin/api/config/entries", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		entries := cfg.Entries()
		items := make([]config.Entry, 0, len(entries))
		for _, entry := range entries {
			if def, ok := cfg.Definition(entry.Key); ok {
				entry.Value = redactValue(def, entry.Value)
			}
			items = append(items, entry)
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("GET /admin/api/config/compare", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		items := cfg.CompareContexts(
			config.CompareContext{
				Label:          "left",
				OrganizationID: strings.TrimSpace(r.URL.Query().Get("left_organization_id")),
				LocationID:     strings.TrimSpace(r.URL.Query().Get("left_location_id")),
			},
			config.CompareContext{
				Label:          "right",
				OrganizationID: strings.TrimSpace(r.URL.Query().Get("right_organization_id")),
				LocationID:     strings.TrimSpace(r.URL.Query().Get("right_location_id")),
			},
		)
		for i := range items {
			if def, ok := cfg.Definition(items[i].Key); ok {
				items[i].Left.Value = redactValue(def, items[i].Left.Value)
				items[i].Right.Value = redactValue(def, items[i].Right.Value)
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("GET /admin/api/config/effective", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		orgID := strings.TrimSpace(r.URL.Query().Get("organization_id"))
		locationID := strings.TrimSpace(r.URL.Query().Get("location_id"))
		items := cfg.ResolveAll(orgID, locationID)
		for i := range items {
			if def, ok := cfg.Definition(items[i].Key); ok {
				items[i].Value = redactValue(def, items[i].Value)
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /admin/api/config/bundles/export", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read")
		if !ok {
			return
		}
		var req configBundleRequest
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid config bundle export payload"))
				return
			}
		}
		bundle := exportConfigBundle(cfg, flags, req, principalActorID(p))
		respondJSON(w, http.StatusOK, map[string]any{"bundle": bundle})
	})

	mux.HandleFunc("POST /admin/api/config/bundles/validate", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage"); !ok {
			return
		}
		var req struct {
			Bundle configBundle `json:"bundle"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid config bundle payload"))
			return
		}
		respondJSON(w, http.StatusOK, validateConfigBundle(cfg, flags, modules, req.Bundle))
	})

	mux.HandleFunc("POST /admin/api/config/bundles/apply", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		var req struct {
			Bundle configBundle `json:"bundle"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid config bundle payload"))
			return
		}
		validation := validateConfigBundle(cfg, flags, modules, req.Bundle)
		if !validation.Valid {
			respondJSON(w, http.StatusConflict, validation)
			return
		}
		appliedConfig := make([]config.EffectiveValue, 0, len(req.Bundle.ConfigEntries))
		for _, entry := range req.Bundle.ConfigEntries {
			def, ok := cfg.Definition(entry.Key)
			if !ok {
				continue
			}
			effective, err := saveConfigEntry(cfg, modules, auditSvc, def, configUpdateRequest{Scope: entry.Scope, ScopeID: entry.ScopeID, Value: entry.Value}, principalActorID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			appliedConfig = append(appliedConfig, effective)
		}
		appliedFlags := make([]featureflags.EffectiveValue, 0, len(req.Bundle.FeatureFlags))
		for _, value := range req.Bundle.FeatureFlags {
			if err := flags.UpsertValue(featureflags.Value{
				FlagKey:       value.FlagKey,
				Scope:         value.Scope,
				ScopeID:       value.ScopeID,
				Enabled:       value.Enabled,
				Status:        value.Status,
				UpdatedBy:     principalActorID(p),
				EffectiveFrom: value.EffectiveFrom,
				EffectiveTo:   value.EffectiveTo,
			}); err != nil {
				respondError(w, err)
				return
			}
			recordAudit(auditSvc, audit.Event{
				ID:            "audit:feature_flag:bundle_apply:" + value.FlagKey + ":" + time.Now().UTC().Format("20060102150405.000000000"),
				Action:        "feature_flag.bundle_apply",
				TargetType:    "feature_flag",
				TargetID:      value.FlagKey,
				ActorID:       principalActorID(p),
				OccurredAt:    time.Now().UTC(),
				CorrelationID: "feature-flag:bundle-apply:" + value.FlagKey,
				Metadata:      map[string]any{"scope": value.Scope, "scope_id": value.ScopeID, "enabled": value.Enabled, "status": value.Status},
			})
			effective, _ := flags.ResolveWithOperatingUnit(value.FlagKey, scopeIDForFeatureFlag(value.Scope, value.ScopeID, "organization"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "location"), scopeIDForFeatureFlag(value.Scope, value.ScopeID, "operating_unit"), time.Now().UTC())
			appliedFlags = append(appliedFlags, effective)
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:config:bundle_apply:" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:        "configuration.bundle_apply",
			TargetType:    "configuration_bundle",
			TargetID:      strings.TrimSpace(req.Bundle.Name),
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			Metadata:      map[string]any{"config_entries": len(req.Bundle.ConfigEntries), "feature_flags": len(req.Bundle.FeatureFlags)},
			CorrelationID: "configuration:bundle-apply",
		})
		respondJSON(w, http.StatusOK, map[string]any{"config_entries": appliedConfig, "feature_flags": appliedFlags, "validation": validation})
	})

	mux.HandleFunc("GET /admin/api/security/role-permission-matrix", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, buildRolePermissionMatrix(ident))
	})

	mux.HandleFunc("PUT /admin/api/security/roles/", func(w http.ResponseWriter, r *http.Request) {
		roleID, permissionKey, ok := adminRolePermissionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("role permission route not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
		if !ok {
			return
		}
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: roleID, PermissionKey: permissionKey}); err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:identity:role_permission:grant:" + roleID + ":" + permissionKey + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:        "identity.role_permission.grant",
			TargetType:    "role_permission",
			TargetID:      roleID + ":" + permissionKey,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			Metadata:      map[string]any{"role_id": roleID, "permission_key": permissionKey},
			CorrelationID: "identity:role-permission:grant:" + roleID,
		})
		respondJSON(w, http.StatusOK, map[string]any{"role_id": roleID, "permission_key": permissionKey})
	})

	mux.HandleFunc("DELETE /admin/api/security/roles/", func(w http.ResponseWriter, r *http.Request) {
		roleID, permissionKey, ok := adminRolePermissionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("role permission route not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
		if !ok {
			return
		}
		if err := ident.RevokeRolePermission(roleID, permissionKey); err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:identity:role_permission:revoke:" + roleID + ":" + permissionKey + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:        "identity.role_permission.revoke",
			TargetType:    "role_permission",
			TargetID:      roleID + ":" + permissionKey,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			Metadata:      map[string]any{"role_id": roleID, "permission_key": permissionKey},
			CorrelationID: "identity:role-permission:revoke:" + roleID,
		})
		respondJSON(w, http.StatusOK, map[string]any{"role_id": roleID, "permission_key": permissionKey})
	})

	mux.HandleFunc("GET /admin/api/readiness", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, buildAdminReadinessReport(cfg, modules, health))
	})

	mux.HandleFunc("GET /admin/api/auth/settings", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		def, ok := cfg.Definition("identity.auth")
		if !ok {
			respondError(w, shared.NotFound("authentication configuration definition not found"))
			return
		}
		orgID := strings.TrimSpace(r.URL.Query().Get("organization_id"))
		locationID := strings.TrimSpace(r.URL.Query().Get("location_id"))
		value, ok := cfg.Resolve("identity.auth", orgID, locationID)
		if !ok {
			respondError(w, shared.NotFound("authentication configuration not found"))
			return
		}
		value.Value = redactValue(def, value.Value)
		respondJSON(w, http.StatusOK, authSettingsResponse{Definition: def, Entry: value})
	})

	mux.HandleFunc("PUT /admin/api/auth/settings", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		def, ok := cfg.Definition("identity.auth")
		if !ok {
			respondError(w, shared.NotFound("authentication configuration definition not found"))
			return
		}
		var req configUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		entry, err := saveConfigEntry(cfg, modules, auditSvc, def, req, principalActorID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, authSettingsResponse{Definition: def, Entry: entry})
	})

	mux.HandleFunc("PUT /admin/api/config/entries/", func(w http.ResponseWriter, r *http.Request) {
		key, ok := adminConfigKeyPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("configuration entry not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		def, ok := cfg.Definition(key)
		if !ok {
			respondError(w, shared.NotFound("configuration definition not found"))
			return
		}
		if detail, ok := modules.Get(def.ModuleKey); ok && !detail.Installed.Enabled {
			respondError(w, shared.Conflict("module is disabled"))
			return
		}
		var req configUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		entry, err := saveConfigEntry(cfg, modules, auditSvc, def, req, principalActorID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, entry)
	})
}
