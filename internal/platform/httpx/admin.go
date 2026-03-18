package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/shared"
)

type configUpdateRequest struct {
	Scope   string         `json:"scope"`
	ScopeID string         `json:"scope_id"`
	Value   map[string]any `json:"value"`
}

type regoUpdateRequest struct {
	Scope   string `json:"scope"`
	ScopeID string `json:"scope_id"`
	Source  string `json:"source"`
}

type authSettingsResponse struct {
	Definition config.Definition     `json:"definition"`
	Entry      config.EffectiveValue `json:"entry"`
}

func registerAdminRoutes(mux *http.ServeMux, cfg *config.Service, org *organization.Service, ident *identity.Service, modules *module.Service, auditSvc *audit.Service, policySvc *policy.Service, obsSvc *observability.Service, integrationSvc *integration.Service, referenceSvc *reference.Service) {
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			http.Redirect(w, r, "/ui", http.StatusSeeOther)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal {
			http.Redirect(w, r, "/ui", http.StatusSeeOther)
			return
		}
		if !principalAllowsAll(ident, p, []string{"module.read"}) {
			respondError(w, shared.Forbidden("module.read is required"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(adminConsoleHTML))
	})

	mux.HandleFunc("GET /admin/assets/platform.css", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(platformCSS)
	})

	mux.HandleFunc("GET /admin/api/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read")
		if !ok {
			return
		}
		menus, actions, views, entries := visibleUIContracts(ident, modules, p, module.UISurfaceAdmin)
		uiMenus, uiActions, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceUser)
		defaultPath := defaultRouteForSurface(ident, p.userID, "admin", menus, actions)
		uiPath := "/ui"
		if len(uiMenus) > 0 {
			for _, action := range uiActions {
				if action.Key == uiMenus[0].ActionKey {
					uiPath = "/ui#" + action.RoutePath
					break
				}
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"organization":      org.Root(),
			"locations":         org.Locations(),
			"roles":             ident.Roles(),
			"menus":             menus,
			"actions":           actions,
			"user_actions":      uiActions,
			"views":             views,
			"custom_entries":    entries,
			"default_path":      defaultPath,
			"ui_access":         len(uiMenus) > 0,
			"ui_path":           uiPath,
			"current_user_id":   p.userID,
			"locale":            localeFromRequest(r, ident),
			"supported_locales": i18n.SupportedLocales(),
		})
	})

	mux.HandleFunc("GET /admin/api/config/validate", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		report := cfg.ValidateAll(strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")))
		respondJSON(w, http.StatusOK, report)
	})

	mux.HandleFunc("GET /admin/api/modules", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": modules.List()})
	})

	mux.HandleFunc("GET /admin/api/modules/compatibility", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": modules.CompatibilityReport()})
	})

	mux.HandleFunc("GET /admin/api/security/role-templates", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": modules.RoleTemplates()})
	})

	mux.HandleFunc("GET /admin/api/security/policy-hooks", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items": policySvc.Runtimes(strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id"))),
		})
	})

	mux.HandleFunc("GET /admin/api/observability/contracts", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"metrics":         obsSvc.MetricDefinitions(),
			"log_events":      obsSvc.LogEventDefinitions(),
			"domain_events":   obsSvc.DomainEventDefinitions(),
			"contract_status": obsSvc.ContractStatuses(),
		})
	})

	mux.HandleFunc("GET /admin/api/integrations/systems", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListSystems()})
	})

	mux.HandleFunc("GET /admin/api/integrations/submissions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListSubmissions()})
	})

	mux.HandleFunc("GET /admin/api/references/types", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": referenceSvc.Types()})
	})

	mux.HandleFunc("GET /admin/api/references/values", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		typeKey := strings.TrimSpace(r.URL.Query().Get("type"))
		if typeKey == "" {
			respondError(w, shared.Validation("type is required"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": referenceSvc.Records(typeKey)})
	})

	mux.HandleFunc("GET /admin/api/references/resolve", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		typeKey := strings.TrimSpace(r.URL.Query().Get("type"))
		if typeKey == "" {
			respondError(w, shared.Validation("type is required"))
			return
		}
		result, err := referenceSvc.Resolve(typeKey, strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")), time.Time{})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /admin/api/modules/", func(w http.ResponseWriter, r *http.Request) {
		moduleKey, ok := adminModulePath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("module not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		item, found := modules.Get(moduleKey)
		if !found {
			respondError(w, shared.NotFound("module not found"))
			return
		}
		respondJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET /admin/api/security/policy-hooks/", func(w http.ResponseWriter, r *http.Request) {
		hookKey, ok := adminPolicyHookPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("policy hook not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		runtime, found := policySvc.Runtime(hookKey, strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")))
		if !found {
			respondError(w, shared.NotFound("policy hook rule not found"))
			return
		}
		respondJSON(w, http.StatusOK, runtime)
	})

	mux.HandleFunc("PUT /admin/api/security/policy-hooks/", func(w http.ResponseWriter, r *http.Request) {
		if hookKey, ok := adminPolicyHookRegoPath(r.URL.Path); ok {
			p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
			if !ok {
				return
			}
			var req regoUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid request body"))
				return
			}
			if err := policySvc.UpsertModule(hookKey, req.Scope, req.ScopeID, principalActorID(p), req.Source); err != nil {
				respondError(w, err)
				return
			}
			runtime, _ := policySvc.Runtime(hookKey, strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")))
			respondJSON(w, http.StatusOK, runtime)
			return
		}
		hookKey, ok := adminPolicyHookPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("policy hook not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		var req configUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		if err := policySvc.UpsertRule(hookKey, req.Scope, req.ScopeID, principalActorID(p), req.Value); err != nil {
			respondError(w, err)
			return
		}
		runtime, _ := policySvc.Runtime(hookKey, strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")))
		respondJSON(w, http.StatusOK, runtime)
	})

	mux.HandleFunc("POST /admin/api/modules/", func(w http.ResponseWriter, r *http.Request) {
		moduleKey, action, ok := adminModuleActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("module action not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "module.manage", "", "module.manage")
		if !ok {
			return
		}
		var (
			item module.InstalledModule
			err  error
		)
		switch action {
		case "enable":
			item, err = modules.Enable(moduleKey, principalActorID(p))
		case "disable":
			item, err = modules.Disable(moduleKey, principalActorID(p))
		default:
			respondError(w, shared.NotFound("module action not found"))
			return
		}
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:module:" + action + ":" + moduleKey + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:        "module." + action,
			TargetType:    "module",
			TargetID:      moduleKey,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			CorrelationID: "module:" + action + ":" + moduleKey,
		})
		respondJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("POST /admin/api/security/role-templates/", func(w http.ResponseWriter, r *http.Request) {
		moduleKey, roleKey, ok := adminRoleTemplateActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("role template action not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users")
		if !ok {
			return
		}
		detail, found := modules.Get(moduleKey)
		if !found {
			respondError(w, shared.NotFound("module not found"))
			return
		}
		var template module.RoleTemplateDefinition
		matched := false
		for _, candidate := range detail.Manifest.Security.RoleTemplates {
			if candidate.Key == roleKey {
				template = candidate
				matched = true
				break
			}
		}
		if !matched {
			respondError(w, shared.NotFound("role template not found"))
			return
		}
		roleID := "role:" + moduleKey + ":" + template.Key
		if err := ident.UpsertRole(identity.Role{ID: roleID, Key: template.Key, Name: template.Name, ScopeType: firstTemplateScope(template.AllowedScopes)}); err != nil {
			respondError(w, err)
			return
		}
		for _, permissionKey := range template.PermissionKeys {
			if err := ident.GrantRolePermission(identity.RolePermission{RoleID: roleID, PermissionKey: permissionKey}); err != nil {
				respondError(w, err)
				return
			}
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:role-template:apply:" + moduleKey + ":" + roleKey + ":" + time.Now().UTC().Format("20060102150405.000000000"),
			Action:        "security.role_template.apply",
			TargetType:    "role_template",
			TargetID:      moduleKey + ":" + roleKey,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			CorrelationID: "role-template:apply:" + moduleKey + ":" + roleKey,
		})
		respondJSON(w, http.StatusOK, map[string]any{"module_key": moduleKey, "role_id": roleID, "template": template, "permissions": template.PermissionKeys})
	})

	mux.HandleFunc("POST /admin/api/integrations/submissions", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "module.manage", "", "module.manage")
		if !ok {
			return
		}
		var req struct {
			SystemKey     string         `json:"system_key"`
			OperationType string         `json:"operation_type"`
			DocumentID    string         `json:"document_id"`
			CorrelationID string         `json:"correlation_id"`
			Payload       map[string]any `json:"payload"`
			ProcessNow    bool           `json:"process_now"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		record, err := integrationSvc.CreateSubmission(req.SystemKey, req.OperationType, req.DocumentID, req.CorrelationID, req.Payload)
		if err != nil {
			respondError(w, err)
			return
		}
		status := http.StatusOK
		response := map[string]any{"record": record}
		if req.ProcessNow {
			if job, queueErr := integrationSvc.EnqueueProcessSubmission(record.ID); queueErr == nil {
				status = http.StatusAccepted
				response["job"] = job
			} else {
				record, err = integrationSvc.ProcessSubmission(record.ID)
				if err != nil {
					respondError(w, err)
					return
				}
				response["record"] = record
			}
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:integration:create:" + record.ID,
			Action:        "integration.submission.create",
			TargetType:    "integration_submission",
			TargetID:      record.ID,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			CorrelationID: "integration:create:" + record.ID,
		})
		respondJSON(w, status, response)
	})

	mux.HandleFunc("POST /admin/api/integrations/submissions/", func(w http.ResponseWriter, r *http.Request) {
		submissionID, action, ok := adminIntegrationSubmissionActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("integration action not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "module.manage", "", "module.manage")
		if !ok {
			return
		}
		var (
			record integration.SubmissionRecord
			err    error
		)
		switch action {
		case "process":
			if job, queueErr := integrationSvc.EnqueueProcessSubmission(submissionID); queueErr == nil {
				record, _ = integrationSvc.GetSubmission(submissionID)
				recordAudit(auditSvc, audit.Event{
					ID:            "audit:integration:" + action + ":" + submissionID,
					Action:        "integration.submission." + action,
					TargetType:    "integration_submission",
					TargetID:      submissionID,
					ActorID:       principalActorID(p),
					OccurredAt:    time.Now().UTC(),
					CorrelationID: "integration:" + action + ":" + submissionID,
				})
				respondJSON(w, http.StatusAccepted, map[string]any{"record": record, "job": job})
				return
			}
			record, err = integrationSvc.ProcessSubmission(submissionID)
		case "retry":
			if job, queueErr := integrationSvc.EnqueueRetrySubmission(submissionID); queueErr == nil {
				record, _ = integrationSvc.GetSubmission(submissionID)
				recordAudit(auditSvc, audit.Event{
					ID:            "audit:integration:" + action + ":" + submissionID,
					Action:        "integration.submission." + action,
					TargetType:    "integration_submission",
					TargetID:      submissionID,
					ActorID:       principalActorID(p),
					OccurredAt:    time.Now().UTC(),
					CorrelationID: "integration:" + action + ":" + submissionID,
				})
				respondJSON(w, http.StatusAccepted, map[string]any{"record": record, "job": job})
				return
			}
			record, err = integrationSvc.RetrySubmission(submissionID)
		default:
			respondError(w, shared.NotFound("integration action not found"))
			return
		}
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:            "audit:integration:" + action + ":" + record.ID,
			Action:        "integration.submission." + action,
			TargetType:    "integration_submission",
			TargetID:      record.ID,
			ActorID:       principalActorID(p),
			OccurredAt:    time.Now().UTC(),
			CorrelationID: "integration:" + action + ":" + record.ID,
		})
		respondJSON(w, http.StatusOK, record)
	})

	mux.HandleFunc("GET /admin/api/config/definitions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": cfg.Definitions()})
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

func adminModulePath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "modules" {
		return "", false
	}
	return strings.TrimSpace(parts[3]), parts[3] != ""
}

func adminModuleActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "modules" || parts[4] != "actions" {
		return "", "", false
	}
	return strings.TrimSpace(parts[3]), strings.TrimSpace(parts[5]), parts[3] != "" && parts[5] != ""
}

func adminConfigKeyPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "config" || parts[3] != "entries" || parts[5] != "value" {
		return "", false
	}
	return strings.TrimSpace(parts[4]), parts[4] != ""
}

func adminPolicyHookPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "security" || parts[3] != "policy-hooks" {
		return "", false
	}
	return strings.TrimSpace(parts[4]), parts[4] != ""
}

func adminPolicyHookRegoPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "security" || parts[3] != "policy-hooks" || parts[5] != "rego" {
		return "", false
	}
	return strings.TrimSpace(parts[4]), parts[4] != ""
}

func adminRoleTemplateActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 8 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "security" || parts[3] != "role-templates" || parts[6] != "actions" || parts[7] != "apply" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[5]), parts[4] != "" && parts[5] != ""
}

func adminIntegrationSubmissionActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 7 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "integrations" || parts[3] != "submissions" || parts[5] != "actions" {
		return "", "", false
	}
	return strings.TrimSpace(parts[4]), strings.TrimSpace(parts[6]), parts[4] != "" && parts[6] != ""
}

func firstTemplateScope(scopes []string) string {
	if len(scopes) == 0 {
		return "deployment"
	}
	return scopes[0]
}

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
	recordAudit(auditSvc, audit.Event{
		ID:            "audit:config:update:" + def.Key + ":" + time.Now().UTC().Format("20060102150405.000000000"),
		Action:        "configuration.update",
		TargetType:    "configuration",
		TargetID:      def.Key,
		ActorID:       actorID,
		OccurredAt:    time.Now().UTC(),
		CorrelationID: "configuration:update:" + def.Key,
		Metadata: map[string]any{
			"scope":    entry.Scope,
			"scope_id": entry.ScopeID,
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

const adminConsoleHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Orbyte Admin</title>
  <link rel="stylesheet" href="/admin/assets/platform.css?v=` + platformAssetVersion + `">
</head>
<body>
  <header class="header-shell">
    <div class="toolbar">
      <div>
        <h1 id="admin-title">Platform Admin</h1>
        <p class="status" id="admin-subtitle">Modules, scoped configuration, and effective runtime settings.</p>
      </div>
      <div class="actions">
        <label class="locale-switch">
          <span id="admin-locale-label">Language</span>
          <select id="admin-locale-switcher"></select>
        </label>
        <a id="admin-ui-link" class="button secondary" href="/ui" hidden>Workspace</a>
        <button type="button" id="admin-logout-button" class="secondary">Log out</button>
      </div>
    </div>
  </header>
  <main class="admin-main">
    <nav id="admin-nav" class="admin-tabs" aria-label="Admin sections"></nav>
    <div class="admin-grid" data-admin-route="/admin/modules">
      <section class="card">
        <h2 id="modules-heading">Modules</h2>
        <div id="modules"></div>
      </section>
    </div>
    <section class="card" data-admin-route="/admin/auth">
      <h2 id="auth-heading">Authentication Settings</h2>
      <div class="admin-row">
        <label class="field"><span id="label-auth-password-enabled">Password Login</span><select id="auth-password-enabled"><option value="true">enabled</option><option value="false">disabled</option></select></label>
        <label class="field"><span id="label-auth-google-enabled">Google Login</span><select id="auth-google-enabled"><option value="true">enabled</option><option value="false">disabled</option></select></label>
      </div>
      <div class="admin-row">
        <label class="field"><span id="label-auth-login-title">Login Title</span><input id="auth-login-title"></label>
        <label class="field"><span id="label-auth-google-button-label">Google Button Label</span><input id="auth-google-button-label"></label>
      </div>
      <label class="field"><span id="label-auth-login-subtitle">Login Subtitle</span><input id="auth-login-subtitle"></label>
      <div class="admin-row">
        <label class="field"><span id="label-auth-google-client-id">Google Client ID</span><input id="auth-google-client-id"></label>
        <label class="field"><span id="label-auth-google-client-secret">Google Client Secret</span><input id="auth-google-client-secret" placeholder="[redacted]"></label>
      </div>
      <div class="admin-row">
        <label class="field"><span id="label-auth-google-redirect-url">Google Redirect URL</span><input id="auth-google-redirect-url"></label>
        <label class="field"><span id="label-auth-google-hosted-domain">Google Hosted Domain</span><input id="auth-google-hosted-domain"></label>
      </div>
      <div class="admin-row">
        <label class="field"><span id="label-auth-google-auth-url">Google Auth URL</span><input id="auth-google-auth-url"></label>
        <label class="field"><span id="label-auth-google-token-url">Google Token URL</span><input id="auth-google-token-url"></label>
      </div>
      <div class="admin-row">
        <label class="field"><span id="label-auth-google-jwks-url">Google JWKS URL</span><input id="auth-google-jwks-url"></label>
        <label class="field"><span id="label-auth-google-issuer">Google Issuer</span><input id="auth-google-issuer"></label>
      </div>
      <div class="admin-row">
        <label class="field"><span id="label-auth-google-timeout-seconds">Google Timeout Seconds</span><input id="auth-google-timeout-seconds" type="number"></label>
        <label class="field"><span id="label-auth-google-auto-provision-enabled">Provision New Users</span><select id="auth-google-auto-provision-enabled"><option value="true">enabled</option><option value="false">disabled</option></select></label>
      </div>
      <div class="admin-row">
        <label class="field"><span id="label-auth-google-auto-provision-role-id">Provision Role</span><select id="auth-google-auto-provision-role-id"></select></label>
        <label class="field"><span id="label-auth-google-auto-provision-default-location-id">Provision Default Location</span><select id="auth-google-auto-provision-default-location-id"></select></label>
      </div>
      <div class="admin-row">
        <label class="field"><span id="label-auth-google-auto-provision-scope-type">Provision Scope Type</span><select id="auth-google-auto-provision-scope-type"><option value="deployment">deployment</option><option value="organization">organization</option><option value="location">location</option></select></label>
        <label class="field"><span id="label-auth-google-auto-provision-scope-id">Provision Scope ID</span><select id="auth-google-auto-provision-scope-id"></select></label>
      </div>
      <label class="field"><span id="label-auth-google-auto-provision-allowed-domains">Provision Allowed Domains</span><input id="auth-google-auto-provision-allowed-domains" placeholder="example.com, example.org"></label>
      <div class="actions">
        <button id="load-auth-settings" class="secondary">Load Auth Settings</button>
        <button id="save-auth-settings">Save Auth Settings</button>
      </div>
      <p id="auth-settings-status" class="muted"></p>
      <pre id="auth-settings-validation"></pre>
    </section>
    <div class="admin-grid" data-admin-route="/admin/config">
      <section class="card">
        <h2 id="config-heading">Config Editor</h2>
        <div class="admin-row">
          <div class="field"><label id="config-key-label">Config Key</label><select id="config-key"></select></div>
          <div class="field"><label id="config-scope-label">Scope</label><select id="config-scope"><option value="deployment">deployment</option><option value="organization">organization</option><option value="location">location</option></select></div>
        </div>
        <div class="admin-row">
          <div class="field"><label id="organization-label">Organization</label><select id="organization-id"></select></div>
          <div class="field"><label id="location-label">Location</label><select id="location-id"></select></div>
        </div>
        <div class="field"><label id="config-value-label">Value JSON</label><textarea id="config-value"></textarea></div>
        <div class="actions">
          <button id="load-effective" class="secondary">Load Effective</button>
          <button id="save-config">Save Entry</button>
        </div>
        <p id="config-status" class="muted"></p>
      </section>
    </div>
    <div class="template-stack" data-admin-route="/admin/templates">
      <section class="card">
        <h2 id="templates-heading">Template Library</h2>
        <div class="admin-row">
          <label class="field"><span id="template-definition-label">Template</span><select id="template-definition"></select></label>
          <label class="field"><span id="template-binding-scope-label">Binding Scope</span><select id="template-binding-scope"><option value="deployment">deployment</option><option value="organization">organization</option><option value="location">location</option></select></label>
        </div>
        <div class="admin-row">
          <label class="field"><span id="template-binding-scope-id-label">Binding Scope ID</span><select id="template-binding-scope-id"><option value="">deployment</option></select></label>
          <label class="field"><span id="template-purpose-label">Purpose</span><input id="template-purpose" placeholder="official"></label>
        </div>
        <div class="admin-row">
          <label class="field"><span id="template-channel-label">Channel</span><input id="template-channel" placeholder="print"></label>
          <label class="field"><span id="template-binding-flags-label">Binding Flags</span><div class="actions"><label><input id="template-binding-default" type="checkbox" checked> <span id="template-binding-default-label">Default</span></label><label><input id="template-binding-official" type="checkbox"> <span id="template-binding-official-label">Official</span></label></div></label>
        </div>
        <div class="admin-row">
          <label class="field"><span id="template-paper-preset-label">Paper Preset</span><select id="template-paper-preset"><option value="a4">A4 Portrait</option><option value="a4-landscape">A4 Landscape</option><option value="receipt-80">Receipt 80mm</option><option value="receipt-58">Receipt 58mm</option></select></label>
          <label class="field"><span id="template-render-target-label">Preview Target ID</span><input id="template-render-target-id" placeholder="doc_..."></label>
        </div>
        <div class="admin-row">
          <label class="field"><span id="template-report-key-label">Preview Target Key</span><input id="template-render-target-key" placeholder="document_reporting"></label>
          <label class="field"><span id="template-render-mode-label">Preview Mode</span><select id="template-render-mode"><option value="sample">sample</option><option value="live">live</option></select></label>
        </div>
        <div class="actions">
          <button id="load-template-definition" class="secondary">Load Template</button>
          <button id="save-template-draft">Save Draft</button>
          <button id="publish-template-version" class="secondary">Publish Current Draft</button>
          <button id="save-template-binding" class="secondary">Save Binding</button>
          <button id="preview-template-render">Preview</button>
        </div>
        <p id="template-status" class="muted"></p>
      </section>
      <div class="template-admin-grid">
        <section class="card">
          <h3 id="template-palette-heading">Block Palette</h3>
          <div id="template-block-palette" class="template-palette"></div>
          <div class="actions">
            <button id="template-add-row" class="secondary">Add Row</button>
            <button id="template-add-column" class="secondary">Add Column</button>
          </div>
          <p class="status" id="template-active-section">Body</p>
        </section>
        <section class="card template-designer-shell">
          <div class="toolbar">
            <div>
              <h3 id="template-canvas-heading">Designer Canvas</h3>
              <p class="status" id="template-canvas-status">Drag blocks into header, body, or footer rows.</p>
            </div>
            <div class="template-section-tabs" id="template-section-tabs"></div>
          </div>
          <div id="template-canvas" class="template-canvas"></div>
          <details>
            <summary id="template-expert-heading">Expert Source</summary>
            <div class="admin-row">
              <label class="field"><span id="template-body-label">Template Body</span><textarea id="template-body"></textarea></label>
              <label class="field"><span id="template-style-label">Template Style</span><textarea id="template-style"></textarea></label>
            </div>
          </details>
        </section>
        <section class="card">
          <h3 id="template-inspector-heading">Inspector</h3>
          <div id="template-inspector" class="list"></div>
        </section>
      </div>
      <div class="admin-grid">
        <section class="card">
          <h2 id="template-preview-heading">Template Preview</h2>
          <div id="template-preview" class="template-preview-frame"></div>
        </section>
        <section class="card">
          <h3 id="template-versions-heading">Versions</h3>
          <div id="template-versions" class="list"></div>
          <h3 id="template-bindings-heading">Bindings</h3>
          <div id="template-bindings" class="list"></div>
        </section>
      </div>
    </div>
    <section class="card" data-admin-route="/admin/definitions">
      <h2 id="definitions-heading">Definitions</h2>
      <div id="definitions" class="list"></div>
    </section>
    <div class="admin-grid" data-admin-route="/admin/security">
      <section class="card">
        <h2 id="navigation-heading">Navigation Defaults</h2>
        <div id="navigation-settings-status" class="muted"></div>
        <div id="navigation-settings" class="list"></div>
      </section>
      <section class="card">
        <h2 id="role-templates-heading">Role Templates</h2>
        <pre id="role-templates"></pre>
      </section>
      <section class="card">
        <h2 id="policy-hooks-heading">Policy Hooks</h2>
        <pre id="policy-hooks"></pre>
      </section>
    </div>
    <section class="card" data-admin-route="/admin/observability">
      <h2 id="observability-heading">Observability Contracts</h2>
      <pre id="observability-contracts"></pre>
    </section>
  </main>
  <script>
    const adminMessages = {
      en: {
        admin_title: 'Platform Admin',
        admin_subtitle: 'Modules, scoped configuration, and effective runtime settings.',
        language: 'Language',
        workspace_link: 'Workspace',
        logout: 'Log out',
        modules: 'Modules',
        auth_settings: 'Authentication Settings',
        config_editor: 'Config Editor',
        templates: 'Templates',
        template_library: 'Template Library',
        template_definition: 'Template',
        template_binding_scope: 'Binding Scope',
        template_binding_scope_id: 'Binding Scope ID',
        template_purpose: 'Purpose',
        template_channel: 'Channel',
        template_binding_flags: 'Binding Flags',
        template_binding_default: 'Default',
        template_binding_official: 'Official',
        template_paper_preset: 'Paper Preset',
        template_body: 'Template Body',
        template_style: 'Template Style',
        preview_target_id: 'Preview Target ID',
        preview_target_key: 'Preview Target Key',
        preview_mode: 'Preview Mode',
        load_template: 'Load Template',
        save_template_draft: 'Save Draft',
        publish_template_version: 'Publish Current Draft',
        save_template_binding: 'Save Binding',
        preview_template_render: 'Preview',
        template_preview: 'Template Preview',
        template_versions: 'Versions',
        template_bindings: 'Bindings',
        template_palette: 'Block Palette',
        template_canvas: 'Designer Canvas',
        template_canvas_help: 'Drag blocks into header, body, or footer rows.',
        template_inspector: 'Inspector',
        template_expert: 'Expert Source',
        template_add_row: 'Add Row',
        template_add_column: 'Add Column',
        template_section_header: 'Header',
        template_section_body: 'Body',
        template_section_footer: 'Footer',
        template_block_text: 'Text',
        template_block_field: 'Field',
        template_block_table: 'Table',
        template_block_totals: 'Totals',
        template_block_divider: 'Divider',
        template_block_image: 'Image',
        template_block_barcode: 'Barcode',
        template_block_signature: 'Signature',
        template_inspector_empty: 'Select a block to edit it.',
        template_block_label: 'Label',
        template_block_text_prop: 'Text',
        template_block_path: 'Field Path',
        template_block_rows_path: 'Rows Path',
        template_block_span: 'Column Span',
        template_block_align: 'Align',
        template_block_size: 'Font Size',
        template_block_emphasis: 'Emphasis',
        template_block_visible_if: 'Visible If',
        template_block_columns: 'Table Columns',
        template_delete_block: 'Delete Block',
        template_duplicate_block: 'Duplicate Block',
        template_move_up: 'Move Up',
        template_move_down: 'Move Down',
        template_delete_row: 'Delete Row',
        template_add_column_action: 'Add Column',
        template_remove_column: 'Remove Column',
        template_block_value: 'Value',
        template_block_image_url: 'Image URL',
        template_block_alt: 'Image Alt',
        template_block_format: 'Format',
        template_add_column_definition: 'Add Column',
        template_remove_column_definition: 'Remove Column',
        template_no_columns: 'No table columns yet.',
        template_column_label: 'Column Label',
        template_column_path: 'Column Path',
        template_module_default: 'Module Default',
        template_module_default_help: 'No scoped binding is active. Module default resolution will be used.',
        template_binding_effective: 'Effective binding',
        template_binding_overrides_broader: 'Overrides broader scopes',
        template_binding_overrides_deployment: 'Overrides deployment default',
        template_binding_fallback: 'Fallback binding',
        template_preview_sample: 'sample',
        template_preview_live: 'live',
        loaded_template: 'Loaded template',
        saved_template_draft: 'Saved template draft',
        published_template_version: 'Published template version',
        saved_template_binding: 'Saved template binding',
        updated_template_binding: 'Updated template binding',
        definitions: 'Definitions',
        role_templates: 'Role Templates',
        navigation_defaults: 'Navigation Defaults',
        navigation_defaults_help: 'Set landing pages by role, user override, and role-binding priority.',
        users: 'Users',
        roles_label: 'Roles',
        role_bindings: 'Role Bindings',
        selected_user: 'Selected User',
        selected_role: 'Selected Role',
        selected_binding: 'Selected Binding',
        preferred_user_route: 'Preferred Workspace Route',
        preferred_admin_route: 'Preferred Admin Route',
        default_user_route: 'Default Workspace Route',
        default_admin_route: 'Default Admin Route',
        binding_priority: 'Binding Priority',
        save_user_preferences: 'Save User Preferences',
        save_role_defaults: 'Save Role Defaults',
        save_binding_priority: 'Save Binding Priority',
        manage_users_required: 'User navigation settings require manage users permission.',
        no_bindings: 'No bindings for selected user.',
        no_routes: 'No routes available',
        loaded_navigation_settings: 'Loaded navigation settings',
        saved_user_preferences: 'Saved user navigation preferences',
        saved_role_defaults: 'Saved role navigation defaults',
        saved_binding_priority: 'Saved role binding priority',
        policy_hooks: 'Policy Hooks',
        observability: 'Observability Contracts',
        config_key: 'Config Key',
        scope: 'Scope',
        organization: 'Organization',
        location: 'Location',
        value_json: 'Value JSON',
        password_login: 'Password Login',
        google_login: 'Google Login',
        login_title: 'Login Title',
        google_button_label: 'Google Button Label',
        login_subtitle: 'Login Subtitle',
        google_client_id: 'Google Client ID',
        google_client_secret: 'Google Client Secret',
        google_redirect_url: 'Google Redirect URL',
        google_hosted_domain: 'Google Hosted Domain',
        google_auth_url: 'Google Auth URL',
        google_token_url: 'Google Token URL',
        google_jwks_url: 'Google JWKS URL',
        google_issuer: 'Google Issuer',
        google_timeout_seconds: 'Google Timeout Seconds',
        provision_new_users: 'Provision New Users',
        provision_role: 'Provision Role',
        provision_default_location: 'Provision Default Location',
        provision_scope_type: 'Provision Scope Type',
        provision_scope_id: 'Provision Scope ID',
        provision_allowed_domains: 'Provision Allowed Domains',
        load_effective: 'Load Effective',
        save_entry: 'Save Entry',
        load_auth_settings: 'Load Auth Settings',
        save_auth_settings: 'Save Auth Settings',
        default_value: 'Default Value',
        fields_label: 'Fields',
        description_label: 'Description',
        scopes_label: 'Scopes',
        permissions_label: 'Permissions',
        module_label: 'Module',
        target_label: 'Target',
        dashboards_label: 'Dashboards',
        metrics_label: 'Metrics',
        reports_label: 'Reports',
        hooks_label: 'Hooks',
        module_col: 'Module',
        status_col: 'Status',
        deps_col: 'Dependencies',
        none: 'none',
        enabled: 'enabled',
        disabled: 'disabled',
        enable: 'Enable',
        disable: 'Disable',
        default_option: 'default',
        select_role: 'Select role',
        default_location: 'Default location',
        select_organization: 'Select organization',
        select_location: 'Select location',
        deployment_default: 'Deployment default',
        auth_validation_clear: 'No authentication validation issues.',
        loaded_auth_settings: 'Loaded authentication settings from',
        saved_auth_settings: 'Saved authentication settings at',
        loaded_effective: 'Loaded effective value from',
        saved_config: 'Saved'
      },
      id: {
        admin_title: 'Admin Platform',
        admin_subtitle: 'Modul, konfigurasi berscope, dan pengaturan runtime efektif.',
        language: 'Bahasa',
        workspace_link: 'Workspace',
        logout: 'Keluar',
        modules: 'Modul',
        auth_settings: 'Pengaturan Autentikasi',
        config_editor: 'Editor Konfigurasi',
        templates: 'Template',
        template_library: 'Pustaka Template',
        template_definition: 'Template',
        template_binding_scope: 'Scope Binding',
        template_binding_scope_id: 'ID Scope Binding',
        template_purpose: 'Tujuan',
        template_channel: 'Channel',
        template_binding_flags: 'Flag Binding',
        template_binding_default: 'Default',
        template_binding_official: 'Resmi',
        template_paper_preset: 'Preset Kertas',
        template_body: 'Isi Template',
        template_style: 'Gaya Template',
        preview_target_id: 'ID Target Preview',
        preview_target_key: 'Kunci Target Preview',
        preview_mode: 'Mode Pratinjau',
        load_template: 'Muat Template',
        save_template_draft: 'Simpan Draf',
        publish_template_version: 'Publikasikan Draf Saat Ini',
        save_template_binding: 'Simpan Binding',
        preview_template_render: 'Pratinjau',
        template_preview: 'Pratinjau Template',
        template_versions: 'Versi',
        template_bindings: 'Binding',
        template_palette: 'Palet Blok',
        template_canvas: 'Kanvas Desainer',
        template_canvas_help: 'Seret blok ke baris header, body, atau footer.',
        template_inspector: 'Inspector',
        template_expert: 'Sumber Ahli',
        template_add_row: 'Tambah Baris',
        template_add_column: 'Tambah Kolom',
        template_section_header: 'Header',
        template_section_body: 'Body',
        template_section_footer: 'Footer',
        template_block_text: 'Teks',
        template_block_field: 'Field',
        template_block_table: 'Tabel',
        template_block_totals: 'Total',
        template_block_divider: 'Pemisah',
        template_block_image: 'Gambar',
        template_block_barcode: 'Barcode',
        template_block_signature: 'Tanda Tangan',
        template_inspector_empty: 'Pilih blok untuk mengeditnya.',
        template_block_label: 'Label',
        template_block_text_prop: 'Teks',
        template_block_path: 'Path Field',
        template_block_rows_path: 'Path Rows',
        template_block_span: 'Span Kolom',
        template_block_align: 'Perataan',
        template_block_size: 'Ukuran Font',
        template_block_emphasis: 'Penekanan',
        template_block_visible_if: 'Visible If',
        template_block_columns: 'Kolom Tabel',
        template_delete_block: 'Hapus Blok',
        template_duplicate_block: 'Duplikasi Blok',
        template_move_up: 'Naikkan',
        template_move_down: 'Turunkan',
        template_delete_row: 'Hapus Baris',
        template_add_column_action: 'Tambah Kolom',
        template_remove_column: 'Hapus Kolom',
        template_block_value: 'Nilai',
        template_block_image_url: 'URL Gambar',
        template_block_alt: 'Alt Gambar',
        template_block_format: 'Format',
        template_add_column_definition: 'Tambah Kolom',
        template_remove_column_definition: 'Hapus Definisi Kolom',
        template_no_columns: 'Belum ada kolom tabel.',
        template_column_label: 'Label Kolom',
        template_column_path: 'Path Kolom',
        template_module_default: 'Default Modul',
        template_module_default_help: 'Tidak ada binding scope yang aktif. Resolusi akan memakai default modul.',
        template_binding_effective: 'Binding efektif',
        template_binding_overrides_broader: 'Menimpa scope yang lebih luas',
        template_binding_overrides_deployment: 'Menimpa default deployment',
        template_binding_fallback: 'Binding fallback',
        template_preview_sample: 'sample',
        template_preview_live: 'live',
        loaded_template: 'Memuat template',
        saved_template_draft: 'Menyimpan draf template',
        published_template_version: 'Mempublikasikan versi template',
        saved_template_binding: 'Menyimpan binding template',
        updated_template_binding: 'Memperbarui binding template',
        definitions: 'Definisi',
        role_templates: 'Template Peran',
        navigation_defaults: 'Default Navigasi',
        navigation_defaults_help: 'Atur landing page berdasarkan peran, override pengguna, dan prioritas binding peran.',
        users: 'Pengguna',
        roles_label: 'Peran',
        role_bindings: 'Binding Peran',
        selected_user: 'Pengguna Terpilih',
        selected_role: 'Peran Terpilih',
        selected_binding: 'Binding Terpilih',
        preferred_user_route: 'Route Workspace Pilihan',
        preferred_admin_route: 'Route Admin Pilihan',
        default_user_route: 'Route Workspace Default',
        default_admin_route: 'Route Admin Default',
        binding_priority: 'Prioritas Binding',
        save_user_preferences: 'Simpan Preferensi Pengguna',
        save_role_defaults: 'Simpan Default Peran',
        save_binding_priority: 'Simpan Prioritas Binding',
        manage_users_required: 'Pengaturan navigasi pengguna memerlukan izin kelola pengguna.',
        no_bindings: 'Tidak ada binding untuk pengguna terpilih.',
        no_routes: 'Tidak ada route tersedia',
        loaded_navigation_settings: 'Memuat pengaturan navigasi',
        saved_user_preferences: 'Menyimpan preferensi navigasi pengguna',
        saved_role_defaults: 'Menyimpan default navigasi peran',
        saved_binding_priority: 'Menyimpan prioritas binding peran',
        policy_hooks: 'Policy Hook',
        observability: 'Kontrak Observabilitas',
        config_key: 'Kunci Konfigurasi',
        scope: 'Cakupan',
        organization: 'Organisasi',
        location: 'Lokasi',
        value_json: 'JSON Nilai',
        password_login: 'Login Kata Sandi',
        google_login: 'Login Google',
        login_title: 'Judul Login',
        google_button_label: 'Label Tombol Google',
        login_subtitle: 'Subjudul Login',
        google_client_id: 'Google Client ID',
        google_client_secret: 'Google Client Secret',
        google_redirect_url: 'URL Redirect Google',
        google_hosted_domain: 'Domain Hosted Google',
        google_auth_url: 'URL Auth Google',
        google_token_url: 'URL Token Google',
        google_jwks_url: 'URL JWKS Google',
        google_issuer: 'Issuer Google',
        google_timeout_seconds: 'Detik Timeout Google',
        provision_new_users: 'Provision Pengguna Baru',
        provision_role: 'Peran Provision',
        provision_default_location: 'Lokasi Default Provision',
        provision_scope_type: 'Tipe Scope Provision',
        provision_scope_id: 'ID Scope Provision',
        provision_allowed_domains: 'Domain Provision yang Diizinkan',
        load_effective: 'Muat Efektif',
        save_entry: 'Simpan Entri',
        load_auth_settings: 'Muat Pengaturan Auth',
        save_auth_settings: 'Simpan Pengaturan Auth',
        default_value: 'Nilai Default',
        fields_label: 'Field',
        description_label: 'Deskripsi',
        scopes_label: 'Scope',
        permissions_label: 'Izin',
        module_label: 'Modul',
        target_label: 'Target',
        dashboards_label: 'Dashboard',
        metrics_label: 'Metrik',
        reports_label: 'Laporan',
        hooks_label: 'Hook',
        module_col: 'Modul',
        status_col: 'Status',
        deps_col: 'Dependensi',
        none: 'tidak ada',
        enabled: 'aktif',
        disabled: 'nonaktif',
        enable: 'Aktifkan',
        disable: 'Nonaktifkan',
        default_option: 'default',
        select_role: 'Pilih peran',
        default_location: 'Lokasi default',
        select_organization: 'Pilih organisasi',
        select_location: 'Pilih lokasi',
        deployment_default: 'Default deployment',
        auth_validation_clear: 'Tidak ada isu validasi autentikasi.',
        loaded_auth_settings: 'Memuat pengaturan autentikasi dari',
        saved_auth_settings: 'Menyimpan pengaturan autentikasi pada',
        loaded_effective: 'Memuat nilai efektif dari',
        saved_config: 'Menyimpan'
      }
    };
    const adminState = { bootstrap: null, locale: 'en', supportedLocales: ['en', 'id'], users: [], bindings: [], navigationManageAllowed: false, templateDefinitions: [], templateBindings: [], templateVersions: [], templateDesigner: { layout: null, sectionID: 'body', selectedBlockID: '' } };
    function normalizeLocale(locale) {
      const value = String(locale || '').trim().toLowerCase().replace(/_/g, '-');
      if (value === 'id' || value.indexOf('id-') === 0) return 'id';
      return 'en';
    }
    function t(key) {
      return (adminMessages[adminState.locale] && adminMessages[adminState.locale][key]) || adminMessages.en[key] || key;
    }
    function pickText(item, baseField) {
      if (!item) return '';
      const localized = item[baseField + '_i18n'];
      if (localized && typeof localized === 'object') {
        return localized[adminState.locale] || localized.en || localized.id || item[baseField] || '';
      }
      return item[baseField] || '';
    }
    function adminCurrentPath() {
      const raw = window.location.hash.replace(/^#/, '').trim();
      return raw || ((adminState.bootstrap && adminState.bootstrap.default_path) || '/admin/modules');
    }
    function renderAdminMenus() {
      const container = document.getElementById('admin-nav');
      if (!container) return;
      const menus = (adminState.bootstrap && adminState.bootstrap.menus) || [];
      const actions = (adminState.bootstrap && adminState.bootstrap.actions) || [];
      const path = adminCurrentPath();
      container.innerHTML = menus.map((menu) => {
        const action = actions.find((item) => item.key === menu.action_key);
        if (!action) return '';
        const selected = action.route_path === path ? 'true' : 'false';
        const classes = action.route_path === path ? 'admin-tab active' : 'admin-tab';
        return '<a class="' + classes + '" role="tab" aria-selected="' + selected + '" href="#' + action.route_path + '">' + escapeHTML(pickText(menu, 'label')) + '</a>';
      }).join('');
    }
    function applyAdminRoute() {
      const path = adminCurrentPath();
      document.querySelectorAll('[data-admin-route]').forEach((node) => {
        node.style.display = node.dataset.adminRoute === path ? '' : 'none';
      });
      renderAdminMenus();
    }
    async function persistLocale(locale) {
      try {
        const payload = await getJSON('/locale?locale=' + encodeURIComponent(locale));
        adminState.locale = normalizeLocale(payload.locale || locale);
        adminState.supportedLocales = payload.supported_locales || adminState.supportedLocales || ['en', 'id'];
      } catch (_) {
        adminState.locale = normalizeLocale(locale);
      }
    }
    async function logoutAdmin() {
      const csrf = getCookie('orbyte_csrf');
      try {
        await fetch('/auth/logout', {
          method: 'POST',
          credentials: 'same-origin',
          headers: csrf ? {'X-CSRF-Token': csrf} : {}
        });
      } catch (_) {}
      window.location.assign('/ui');
    }
    function renderLocaleSwitcher() {
      const select = document.getElementById('admin-locale-switcher');
      if (!select) return;
      select.innerHTML = (adminState.supportedLocales || ['en', 'id']).map((locale) => '<option value="' + locale + '">' + (locale === 'id' ? 'Bahasa Indonesia' : 'English') + '</option>').join('');
      select.value = adminState.locale;
      select.onchange = async () => {
        await persistLocale(select.value);
        renderAdminChrome();
        if (adminState.bootstrap) boot();
      };
    }
    function renderAdminChrome() {
      document.documentElement.lang = adminState.locale;
      document.title = t('admin_title');
      const pairs = {
        'admin-title': 'admin_title',
        'admin-subtitle': 'admin_subtitle',
        'admin-locale-label': 'language',
        'admin-ui-link': 'workspace_link',
        'admin-logout-button': 'logout',
        'modules-heading': 'modules',
        'auth-heading': 'auth_settings',
        'config-heading': 'config_editor',
        'templates-heading': 'template_library',
        'template-definition-label': 'template_definition',
        'template-binding-scope-label': 'template_binding_scope',
        'template-binding-scope-id-label': 'template_binding_scope_id',
        'template-purpose-label': 'template_purpose',
        'template-channel-label': 'template_channel',
        'template-binding-flags-label': 'template_binding_flags',
        'template-binding-default-label': 'template_binding_default',
        'template-binding-official-label': 'template_binding_official',
        'template-paper-preset-label': 'template_paper_preset',
        'template-body-label': 'template_body',
        'template-style-label': 'template_style',
        'template-render-target-label': 'preview_target_id',
        'template-report-key-label': 'preview_target_key',
        'template-render-mode-label': 'preview_mode',
        'load-template-definition': 'load_template',
        'save-template-draft': 'save_template_draft',
        'publish-template-version': 'publish_template_version',
        'save-template-binding': 'save_template_binding',
        'preview-template-render': 'preview_template_render',
        'template-preview-heading': 'template_preview',
        'template-versions-heading': 'template_versions',
        'template-bindings-heading': 'template_bindings',
        'template-palette-heading': 'template_palette',
        'template-canvas-heading': 'template_canvas',
        'template-canvas-status': 'template_canvas_help',
        'template-inspector-heading': 'template_inspector',
        'template-expert-heading': 'template_expert',
        'template-add-row': 'template_add_row',
        'template-add-column': 'template_add_column',
        'definitions-heading': 'definitions',
        'navigation-heading': 'navigation_defaults',
        'role-templates-heading': 'role_templates',
        'policy-hooks-heading': 'policy_hooks',
        'observability-heading': 'observability',
        'config-key-label': 'config_key',
        'config-scope-label': 'scope',
        'organization-label': 'organization',
        'location-label': 'location',
        'config-value-label': 'value_json',
        'label-auth-password-enabled': 'password_login',
        'label-auth-google-enabled': 'google_login',
        'label-auth-login-title': 'login_title',
        'label-auth-google-button-label': 'google_button_label',
        'label-auth-login-subtitle': 'login_subtitle',
        'label-auth-google-client-id': 'google_client_id',
        'label-auth-google-client-secret': 'google_client_secret',
        'label-auth-google-redirect-url': 'google_redirect_url',
        'label-auth-google-hosted-domain': 'google_hosted_domain',
        'label-auth-google-auth-url': 'google_auth_url',
        'label-auth-google-token-url': 'google_token_url',
        'label-auth-google-jwks-url': 'google_jwks_url',
        'label-auth-google-issuer': 'google_issuer',
        'label-auth-google-timeout-seconds': 'google_timeout_seconds',
        'label-auth-google-auto-provision-enabled': 'provision_new_users',
        'label-auth-google-auto-provision-role-id': 'provision_role',
        'label-auth-google-auto-provision-default-location-id': 'provision_default_location',
        'label-auth-google-auto-provision-scope-type': 'provision_scope_type',
        'label-auth-google-auto-provision-scope-id': 'provision_scope_id',
        'label-auth-google-auto-provision-allowed-domains': 'provision_allowed_domains',
        'load-effective': 'load_effective',
        'save-config': 'save_entry',
        'load-auth-settings': 'load_auth_settings',
        'save-auth-settings': 'save_auth_settings',
        'admin-locale-label': 'language'
      };
      Object.keys(pairs).forEach((id) => {
        const node = document.getElementById(id);
        if (node) node.textContent = t(pairs[id]);
      });
      const boolSelects = ['auth-password-enabled', 'auth-google-enabled', 'auth-google-auto-provision-enabled'];
      boolSelects.forEach((id) => {
        const select = document.getElementById(id);
        if (!select || !select.options || select.options.length < 2) return;
        select.options[0].textContent = t('enabled');
        select.options[1].textContent = t('disabled');
      });
      const scopeType = document.getElementById('auth-google-auto-provision-scope-type');
      if (scopeType && scopeType.options.length >= 3) {
        scopeType.options[0].textContent = 'deployment';
        scopeType.options[1].textContent = t('organization');
        scopeType.options[2].textContent = t('location');
      }
      const configScope = document.getElementById('config-scope');
      if (configScope && configScope.options.length >= 3) {
        configScope.options[0].textContent = 'deployment';
        configScope.options[1].textContent = t('organization');
        configScope.options[2].textContent = t('location');
      }
    }
    async function getJSON(url, options) {
      const resp = await fetch(url, Object.assign({credentials:'include'}, options || {}));
      if (!resp.ok) {
        const text = await resp.text();
        throw new Error(text || ('HTTP ' + resp.status));
      }
      return resp.json();
    }
    async function optionalJSON(url, options) {
      const resp = await fetch(url, Object.assign({credentials:'include'}, options || {}));
      if (resp.status === 403 || resp.status === 404) {
        return null;
      }
      if (!resp.ok) {
        const text = await resp.text();
        throw new Error(text || ('HTTP ' + resp.status));
      }
      return resp.json();
    }
    async function boot() {
      if (!adminState.bootstrap) {
        adminState.locale = normalizeLocale(navigator.language || 'en');
      }
      const [bootstrap, modules, definitions, roleTemplates, policyHooks, observability, authSettings, users, bindings, templateDefinitions, templateBindings] = await Promise.all([
        getJSON('/admin/api/bootstrap'),
        getJSON('/admin/api/modules'),
        getJSON('/admin/api/config/definitions'),
        getJSON('/admin/api/security/role-templates'),
        getJSON('/admin/api/security/policy-hooks'),
        getJSON('/admin/api/observability/contracts'),
        getJSON('/admin/api/auth/settings'),
        optionalJSON('/users'),
        optionalJSON('/role-bindings'),
        optionalJSON('/admin/api/templates/definitions'),
        optionalJSON('/admin/api/template-bindings')
      ]);
      adminState.bootstrap = bootstrap;
      adminState.supportedLocales = bootstrap.supported_locales || ['en', 'id'];
      adminState.users = (users && users.items) || [];
      adminState.bindings = (bindings && bindings.items) || [];
      adminState.navigationManageAllowed = !!(users && bindings);
      adminState.templateDefinitions = (templateDefinitions && templateDefinitions.items) || [];
      adminState.templateBindings = (templateBindings && templateBindings.items) || [];
      if (bootstrap.locale) {
        adminState.locale = normalizeLocale(bootstrap.locale);
      }
      const uiLink = document.getElementById('admin-ui-link');
      if (uiLink) {
        uiLink.hidden = !bootstrap.ui_access;
        uiLink.href = bootstrap.ui_path || '/ui';
      }
      renderLocaleSwitcher();
      renderAdminChrome();
      renderAdminMenus();
      document.getElementById('organization-id').innerHTML = '<option value="">' + t('default_option') + '</option><option value="' + bootstrap.organization.id + '">' + bootstrap.organization.name + '</option>';
      document.getElementById('location-id').innerHTML = '<option value="">' + t('default_option') + '</option>' + bootstrap.locations.map(loc => '<option value="' + loc.id + '">' + loc.name + '</option>').join('');
      renderModules(modules.items);
      renderDefinitions(definitions.items);
      renderTemplates();
      renderAuthSettings(authSettings.entry.value);
      renderNavigationSettings();
      renderRoleTemplates(roleTemplates.items);
      renderPolicyHooks(policyHooks.items);
      renderObservability(observability);
      applyAdminRoute();
    }
    function boolValue(id) {
      return document.getElementById(id).value === 'true';
    }
    function csvValue(id) {
      return (document.getElementById(id).value || '').split(',').map(item => item.trim()).filter(Boolean);
    }
    function selectedScopeID(scope) {
      if (scope === 'deployment') return '';
      if (scope === 'organization') return document.getElementById('organization-id').value;
      return document.getElementById('location-id').value;
    }
    function renderProvisionScopeOptions(scopeType, selectedValue) {
      const target = document.getElementById('auth-google-auto-provision-scope-id');
      if (scopeType === 'deployment') {
        target.innerHTML = '<option value="">' + t('deployment_default') + '</option>';
        target.value = '';
        target.disabled = true;
        return;
      }
      if (scopeType === 'organization') {
        const org = adminState.bootstrap && adminState.bootstrap.organization;
        target.innerHTML = '<option value="">' + t('select_organization') + '</option>' + (org ? '<option value="' + org.id + '">' + org.name + ' (' + org.id + ')</option>' : '');
        target.disabled = false;
        target.value = selectedValue || '';
        return;
      }
      const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
      target.innerHTML = '<option value="">' + t('select_location') + '</option>' + locations.map(loc => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
      target.disabled = false;
      target.value = selectedValue || '';
    }
    function setDisabled(ids, disabled) {
      ids.forEach((id) => {
        const el = document.getElementById(id);
        if (el) el.disabled = disabled;
      });
    }
    function syncAuthSettingsState() {
      const googleEnabled = boolValue('auth-google-enabled');
      const autoProvisionEnabled = googleEnabled && boolValue('auth-google-auto-provision-enabled');
      setDisabled([
        'auth-google-button-label',
        'auth-google-client-id',
        'auth-google-client-secret',
        'auth-google-redirect-url',
        'auth-google-hosted-domain',
        'auth-google-auth-url',
        'auth-google-token-url',
        'auth-google-jwks-url',
        'auth-google-issuer',
        'auth-google-timeout-seconds',
        'auth-google-auto-provision-enabled'
      ], !googleEnabled);
      setDisabled([
        'auth-google-auto-provision-role-id',
        'auth-google-auto-provision-default-location-id',
        'auth-google-auto-provision-scope-type',
        'auth-google-auto-provision-allowed-domains'
      ], !autoProvisionEnabled);
      if (!autoProvisionEnabled) {
        document.getElementById('auth-google-auto-provision-scope-id').disabled = true;
      } else {
        renderProvisionScopeOptions(document.getElementById('auth-google-auto-provision-scope-type').value, document.getElementById('auth-google-auto-provision-scope-id').value);
      }
    }
    async function loadAuthSettingsValidation() {
      const orgID = document.getElementById('organization-id').value;
      const locationID = document.getElementById('location-id').value;
      const payload = await getJSON('/admin/api/config/validate?organization_id=' + encodeURIComponent(orgID) + '&location_id=' + encodeURIComponent(locationID));
      const issues = (payload.issues || []).filter((issue) => issue.key === 'identity.auth');
      document.getElementById('auth-settings-validation').textContent = issues.length
        ? JSON.stringify(issues, null, 2)
        : t('auth_validation_clear');
    }
    function renderAuthSettings(value) {
      value = value || {};
      const roles = (adminState.bootstrap && adminState.bootstrap.roles) || [];
      const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
      document.getElementById('auth-google-auto-provision-role-id').innerHTML = '<option value="">' + t('select_role') + '</option>' + roles.map(role => '<option value="' + role.id + '">' + role.name + ' (' + role.id + ')</option>').join('');
      document.getElementById('auth-google-auto-provision-default-location-id').innerHTML = '<option value="">' + t('default_location') + '</option>' + locations.map(loc => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
      document.getElementById('auth-password-enabled').value = String(value.password_enabled !== false);
      document.getElementById('auth-google-enabled').value = String(!!value.google_enabled);
      document.getElementById('auth-login-title').value = value.login_title || '';
      document.getElementById('auth-login-subtitle').value = value.login_subtitle || '';
      document.getElementById('auth-google-button-label').value = value.google_button_label || '';
      document.getElementById('auth-google-client-id').value = value.google_client_id || '';
      document.getElementById('auth-google-client-secret').value = value.google_client_secret || '';
      document.getElementById('auth-google-redirect-url').value = value.google_redirect_url || '';
      document.getElementById('auth-google-hosted-domain').value = value.google_hosted_domain || '';
      document.getElementById('auth-google-auth-url').value = value.google_auth_url || '';
      document.getElementById('auth-google-token-url').value = value.google_token_url || '';
      document.getElementById('auth-google-jwks-url').value = value.google_jwks_url || '';
      document.getElementById('auth-google-issuer').value = value.google_issuer || '';
      document.getElementById('auth-google-timeout-seconds').value = value.google_timeout_seconds || 5;
      document.getElementById('auth-google-auto-provision-enabled').value = String(!!value.google_auto_provision_enabled);
      document.getElementById('auth-google-auto-provision-role-id').value = value.google_auto_provision_role_id || '';
      document.getElementById('auth-google-auto-provision-default-location-id').value = value.google_auto_provision_default_location_id || '';
      document.getElementById('auth-google-auto-provision-scope-type').value = value.google_auto_provision_scope_type || 'deployment';
      renderProvisionScopeOptions(value.google_auto_provision_scope_type || 'deployment', value.google_auto_provision_scope_id || '');
      document.getElementById('auth-google-auto-provision-allowed-domains').value = (value.google_auto_provision_allowed_domains || []).join(', ');
      document.getElementById('load-auth-settings').onclick = loadAuthSettings;
      document.getElementById('save-auth-settings').onclick = saveAuthSettings;
      document.getElementById('auth-google-enabled').onchange = syncAuthSettingsState;
      document.getElementById('auth-google-auto-provision-enabled').onchange = syncAuthSettingsState;
      document.getElementById('auth-google-auto-provision-scope-type').onchange = () => {
        renderProvisionScopeOptions(document.getElementById('auth-google-auto-provision-scope-type').value, '');
        syncAuthSettingsState();
      };
      syncAuthSettingsState();
      void loadAuthSettingsValidation();
    }
    async function loadAuthSettings() {
      const orgID = document.getElementById('organization-id').value;
      const locationID = document.getElementById('location-id').value;
      const payload = await getJSON('/admin/api/auth/settings?organization_id=' + encodeURIComponent(orgID) + '&location_id=' + encodeURIComponent(locationID));
      renderAuthSettings(payload.entry.value);
      document.getElementById('auth-settings-status').textContent = t('loaded_auth_settings') + ' ' + payload.entry.source_scope + (payload.entry.source_scope_id ? ':' + payload.entry.source_scope_id : '');
      await loadAuthSettingsValidation();
    }
    async function saveAuthSettings() {
      const scope = document.getElementById('config-scope').value;
      const scopeID = selectedScopeID(scope);
      const value = {
        password_enabled: boolValue('auth-password-enabled'),
        login_title: document.getElementById('auth-login-title').value,
        login_subtitle: document.getElementById('auth-login-subtitle').value,
        google_button_label: document.getElementById('auth-google-button-label').value,
        google_enabled: boolValue('auth-google-enabled'),
        google_auto_provision_enabled: boolValue('auth-google-auto-provision-enabled'),
        google_auto_provision_allowed_domains: csvValue('auth-google-auto-provision-allowed-domains'),
        google_auto_provision_role_id: document.getElementById('auth-google-auto-provision-role-id').value,
        google_auto_provision_scope_type: document.getElementById('auth-google-auto-provision-scope-type').value,
        google_auto_provision_scope_id: document.getElementById('auth-google-auto-provision-scope-id').value,
        google_auto_provision_default_location_id: document.getElementById('auth-google-auto-provision-default-location-id').value,
        google_client_id: document.getElementById('auth-google-client-id').value,
        google_client_secret: document.getElementById('auth-google-client-secret').value,
        google_redirect_url: document.getElementById('auth-google-redirect-url').value,
        google_auth_url: document.getElementById('auth-google-auth-url').value,
        google_token_url: document.getElementById('auth-google-token-url').value,
        google_jwks_url: document.getElementById('auth-google-jwks-url').value,
        google_issuer: document.getElementById('auth-google-issuer').value,
        google_hosted_domain: document.getElementById('auth-google-hosted-domain').value,
        google_timeout_seconds: parseInt(document.getElementById('auth-google-timeout-seconds').value || '5', 10)
      };
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/admin/api/auth/settings', {
        method:'PUT',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({scope: scope, scope_id: scopeID, value: value})
      });
      renderAuthSettings(payload.entry.value);
      document.getElementById('auth-settings-status').textContent = t('saved_auth_settings') + ' ' + scope + (scopeID ? ':' + scopeID : '');
      await loadAuthSettingsValidation();
    }
    function renderModules(items) {
      document.getElementById('modules').innerHTML = '<table><thead><tr><th>' + t('module_col') + '</th><th>' + t('status_col') + '</th><th>' + t('deps_col') + '</th><th></th></tr></thead><tbody>' + items.map(item => {
        const enabled = item.installed.enabled;
        const deps = (item.dependency_diagnostics || []).map(dep => dep.module_key + ':' + (dep.compatible ? 'ok' : dep.reason || 'blocked')).join(', ');
        return '<tr><td><strong>' + pickText(item.manifest, 'name') + '</strong><div class="muted">' + item.manifest.key + ' · ' + item.manifest.version + '</div></td><td><span class="pill ' + (enabled ? '' : 'off') + '">' + (enabled ? t('enabled') : t('disabled')) + '</span></td><td class="muted">' + (deps || t('none')) + '</td><td><button data-key="' + item.manifest.key + '" data-action="' + (enabled ? 'disable' : 'enable') + '" class="' + (enabled ? 'warn' : '') + '">' + (enabled ? t('disable') : t('enable')) + '</button></td></tr>';
      }).join('') + '</tbody></table>';
      document.querySelectorAll('#modules button[data-key]').forEach(btn => {
        btn.addEventListener('click', async () => {
          const csrf = getCookie('orbyte_csrf');
          await getJSON('/admin/api/modules/' + btn.dataset.key + '/actions/' + btn.dataset.action, {method:'POST', headers:{'X-CSRF-Token': csrf}});
          boot();
        });
      });
    }
    function renderDefinitions(items) {
      document.getElementById('definitions').innerHTML = items.map((item) => {
        const fields = (item.fields || []).map((field) => '<li><strong>' + pickText(field, 'label') + '</strong> <span class="muted">(' + field.key + ' · ' + field.type + ')</span></li>').join('');
        return '<article class="card"><h3>' + pickText(item, 'display_name') + '</h3><p class="muted">' + item.key + ' · ' + item.module_key + '</p>' +
          (pickText(item, 'description') ? '<p class="status">' + pickText(item, 'description') + '</p>' : '') +
          '<p><strong>' + t('default_value') + ':</strong></p><pre>' + escapeHTML(JSON.stringify(item.default_value || {}, null, 2)) + '</pre>' +
          '<p><strong>' + t('fields_label') + ':</strong></p><ul>' + fields + '</ul></article>';
      }).join('');
      document.getElementById('config-key').innerHTML = items.map(item => '<option value="' + item.key + '">' + pickText(item, 'display_name') + ' (' + item.key + ')</option>').join('');
      if (items[0]) {
        document.getElementById('config-value').value = JSON.stringify(items[0].default_value, null, 2);
      }
      document.getElementById('config-key').onchange = () => {
        const current = items.find(item => item.key === document.getElementById('config-key').value);
        if (current) document.getElementById('config-value').value = JSON.stringify(current.default_value, null, 2);
      };
      document.getElementById('load-effective').onclick = async () => {
        const key = document.getElementById('config-key').value;
        const orgID = document.getElementById('organization-id').value;
        const locationID = document.getElementById('location-id').value;
        const payload = await getJSON('/admin/api/config/effective?organization_id=' + encodeURIComponent(orgID) + '&location_id=' + encodeURIComponent(locationID));
        const match = payload.items.find(item => item.key === key);
          if (match) {
            document.getElementById('config-value').value = JSON.stringify(match.value, null, 2);
            document.getElementById('config-status').textContent = t('loaded_effective') + ' ' + match.source_scope + (match.source_scope_id ? ':' + match.source_scope_id : '');
          }
        };
      document.getElementById('save-config').onclick = async () => {
        const key = document.getElementById('config-key').value;
        const scope = document.getElementById('config-scope').value;
        const scopeID = scope === 'deployment' ? '' : (scope === 'organization' ? document.getElementById('organization-id').value : document.getElementById('location-id').value);
        const value = JSON.parse(document.getElementById('config-value').value || '{}');
        const csrf = getCookie('orbyte_csrf');
        await getJSON('/admin/api/config/entries/' + key + '/value', {
          method:'PUT',
          headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
          body: JSON.stringify({scope: scope, scope_id: scopeID, value: value})
        });
        document.getElementById('config-status').textContent = t('saved_config') + ' ' + key + ' ' + t('scope') + ' ' + scope + (scopeID ? ':' + scopeID : '');
      };
    }
    function renderRoleTemplates(items) {
      document.getElementById('role-templates').innerHTML = items.map((item) => {
        const template = item.template || {};
        return '<article class="card"><h3>' + pickText(template, 'name') + '</h3><p class="muted">' + template.key + ' · ' + (item.module_key || '') + '</p>' +
          (pickText(template, 'description') ? '<p class="status">' + pickText(template, 'description') + '</p>' : '') +
          '<p><strong>' + t('scopes_label') + ':</strong> ' + escapeHTML((template.allowed_scopes || []).join(', ') || '-')
          + '</p><p><strong>' + t('permissions_label') + ':</strong> ' + escapeHTML((template.permission_keys || []).join(', ') || '-')
          + '</p></article>';
      }).join('');
    }
    async function loadTemplateVersions(templateKey) {
      const payload = await getJSON('/admin/api/templates/versions?template_key=' + encodeURIComponent(templateKey));
      adminState.templateVersions = payload.items || [];
    }
    async function loadTemplateBindings(templateKey) {
      const payload = await getJSON('/admin/api/template-bindings?template_key=' + encodeURIComponent(templateKey));
      adminState.templateBindings = payload.items || [];
    }
    function renderTemplateBindingScopeOptions(scopeType, selectedValue) {
      const select = document.getElementById('template-binding-scope-id');
      if (!select) return;
      if (scopeType === 'organization') {
        const org = adminState.bootstrap && adminState.bootstrap.organization;
        select.disabled = false;
        select.innerHTML = '<option value="">' + t('select_organization') + '</option>' + (org ? '<option value="' + org.id + '">' + org.name + ' (' + org.id + ')</option>' : '');
        select.value = selectedValue || '';
        return;
      }
      if (scopeType === 'location') {
        const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
        select.disabled = false;
        select.innerHTML = '<option value="">' + t('select_location') + '</option>' + locations.map((loc) => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
        select.value = selectedValue || '';
        return;
      }
      select.disabled = true;
      select.innerHTML = '<option value="">' + t('deployment_default') + '</option>';
      select.value = '';
    }
    function templatePaletteItems() {
      return [
        {type: 'text', label: t('template_block_text')},
        {type: 'field', label: t('template_block_field')},
        {type: 'table', label: t('template_block_table')},
        {type: 'totals', label: t('template_block_totals')},
        {type: 'divider', label: t('template_block_divider')},
        {type: 'image', label: t('template_block_image')},
        {type: 'barcode', label: t('template_block_barcode')},
        {type: 'signature', label: t('template_block_signature')}
      ];
    }
    function templateDefaultLayout(current) {
      const title = pickText(current, 'title') || current.key || 'Template';
      const bodyBlock = current.target_kind === 'report'
        ? {id: 'body-main', type: 'table', rows_path: 'report.rows', columns: [{label: 'Label', path: 'label'}, {label: 'Total', path: 'total'}]}
        : {id: 'body-main', type: 'field', label: 'Document Number', path: 'document.header.number'};
      return {
        schema_version: 'visual-grid/v1',
        title: title,
        settings: {paper_preset: 'a4', orientation: 'portrait', density: 'comfortable'},
        sections: [
          {id: 'header', title: t('template_section_header'), kind: 'header', rows: [{id: 'header-row-1', columns: [{id: 'header-row-1-cell-1', span: 12, blocks: [{id: 'header-title', type: 'text', text: title, font_size: 'xl', emphasis: 'strong'}]}]}]},
          {id: 'body', title: t('template_section_body'), kind: 'body', rows: [{id: 'body-row-1', columns: [{id: 'body-row-1-cell-1', span: 12, blocks: [bodyBlock]}]}]},
          {id: 'footer', title: t('template_section_footer'), kind: 'footer', rows: [{id: 'footer-row-1', columns: [{id: 'footer-row-1-cell-1', span: 12, blocks: [{id: 'footer-note', type: 'text', text: 'Prepared by Orbyte', align: 'right', emphasis: 'muted'}]}]}]}
        ]
      };
    }
    function normalizeTemplateLayout(layout, current) {
      const base = layout && typeof layout === 'object' ? layout : templateDefaultLayout(current);
      const sections = Array.isArray(base.sections) && base.sections.length ? base.sections : templateDefaultLayout(current).sections;
      return {
        schema_version: base.schema_version || 'visual-grid/v1',
        title: base.title || pickText(current, 'title') || current.key || 'Template',
        settings: Object.assign({paper_preset: 'a4', orientation: 'portrait', density: 'comfortable'}, base.settings || {}),
        sections: sections.map((section, sectionIndex) => ({
          id: section.id || ['header', 'body', 'footer'][sectionIndex] || ('section-' + (sectionIndex + 1)),
          title: section.title || ['Header', 'Body', 'Footer'][sectionIndex] || ('Section ' + (sectionIndex + 1)),
          kind: section.kind || section.id || 'body',
          rows: (Array.isArray(section.rows) && section.rows.length ? section.rows : [{columns: [{span: 12, blocks: []}]}]).map((row, rowIndex) => ({
            id: row.id || ((section.id || 'section') + '-row-' + (rowIndex + 1)),
            columns: (Array.isArray(row.columns) && row.columns.length ? row.columns : [{span: 12, blocks: []}]).map((column, columnIndex) => ({
              id: column.id || ((row.id || 'row') + '-cell-' + (columnIndex + 1)),
              span: Math.min(12, Math.max(1, parseInt(column.span || 12, 10) || 12)),
              blocks: (Array.isArray(column.blocks) ? column.blocks : []).map((block, blockIndex) => Object.assign({
                id: block.id || ((column.id || 'cell') + '-block-' + (blockIndex + 1)),
                label: '',
                text: '',
                path: '',
                rows_path: '',
                columns: [],
                align: '',
                font_size: '',
                emphasis: '',
                visible_if: ''
              }, block))
            }))
          }))
        }))
      };
    }
    function parseTemplateDesignerBody(current, body) {
      if ((current.renderer_kind || '').toLowerCase() !== 'visual') return null;
      try {
        return normalizeTemplateLayout(JSON.parse(body || '{}'), current);
      } catch (_) {
        return templateDefaultLayout(current);
      }
    }
    function selectedTemplateDefinition() {
      return (adminState.templateDefinitions || []).find((item) => item.key === document.getElementById('template-definition').value) || null;
    }
    function selectedTemplateDraft() {
      return (adminState.templateVersions || []).find((item) => item.status === 'draft') || (adminState.templateVersions || []).slice(-1)[0] || null;
    }
    function templateSectionName(section) {
      if (!section) return t('template_section_body');
      if (section.id === 'header') return t('template_section_header');
      if (section.id === 'footer') return t('template_section_footer');
      return t('template_section_body');
    }
    function findTemplateBlock(blockID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout || !blockID) return null;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          for (const column of row.columns || []) {
            for (const block of column.blocks || []) {
              if (block.id === blockID) return {section, row, column, block};
            }
          }
        }
      }
      return null;
    }
    function nextDesignerID(prefix) {
      return prefix + '-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 7);
    }
    function createTemplateBlock(type) {
      const id = nextDesignerID('block');
      switch (type) {
      case 'field':
        return {id, type, label: 'Field', path: 'document.header.number'};
      case 'table':
        return {id, type, rows_path: 'document.lines', columns: [{label: 'Label', path: 'payload.name'}, {label: 'Amount', path: 'amount'}]};
      case 'totals':
        return {id, type, label: 'Total', rows_path: 'document.lines', path: 'amount'};
      case 'divider':
        return {id, type};
      case 'image':
        return {id, type, label: 'Logo', image_url: ''};
      case 'barcode':
        return {id, type, label: 'Barcode', path: 'document.header.number'};
      case 'signature':
        return {id, type, label: 'Authorized Signature'};
      default:
        return {id, type: 'text', text: 'New text block'};
      }
    }
    function removeTemplateBlock(blockID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout) return null;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          for (const column of row.columns || []) {
            const index = (column.blocks || []).findIndex((item) => item.id === blockID);
            if (index >= 0) {
              return column.blocks.splice(index, 1)[0];
            }
          }
        }
      }
      return null;
    }
    function moveTemplateBlock(blockID, targetCellID, beforeBlockID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout) return;
      const block = removeTemplateBlock(blockID);
      if (!block) return;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          for (const column of row.columns || []) {
            if (column.id !== targetCellID) continue;
            column.blocks = column.blocks || [];
            if (beforeBlockID) {
              const index = column.blocks.findIndex((item) => item.id === beforeBlockID);
              if (index >= 0) {
                column.blocks.splice(index, 0, block);
                return;
              }
            }
            column.blocks.push(block);
            return;
          }
        }
      }
    }
    function moveRow(sectionID, rowID, direction) {
      const section = (adminState.templateDesigner.layout && adminState.templateDesigner.layout.sections || []).find((item) => item.id === sectionID);
      if (!section || !section.rows) return;
      const index = section.rows.findIndex((item) => item.id === rowID);
      const nextIndex = index + direction;
      if (index < 0 || nextIndex < 0 || nextIndex >= section.rows.length) return;
      const row = section.rows.splice(index, 1)[0];
      section.rows.splice(nextIndex, 0, row);
    }
    function addColumnToActiveSection() {
      const section = (adminState.templateDesigner.layout && adminState.templateDesigner.layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID);
      if (!section || !section.rows || !section.rows.length) return;
      const row = section.rows[section.rows.length - 1];
      row.columns = row.columns || [];
      const nextCount = row.columns.length + 1;
      const nextSpan = Math.max(2, Math.floor(12 / nextCount));
      row.columns = row.columns.map((column) => Object.assign({}, column, {span: nextSpan}));
      row.columns.push({id: nextDesignerID('cell'), span: nextSpan, blocks: []});
    }
    function removeColumn(cellID) {
      const layout = adminState.templateDesigner.layout;
      if (!layout) return;
      for (const section of layout.sections || []) {
        for (const row of section.rows || []) {
          const index = (row.columns || []).findIndex((column) => column.id === cellID);
          if (index >= 0) {
            if ((row.columns || []).length === 1) return;
            row.columns.splice(index, 1);
            const nextSpan = Math.max(2, Math.floor(12 / row.columns.length));
            row.columns = row.columns.map((column) => Object.assign({}, column, {span: nextSpan}));
            return;
          }
        }
      }
    }
    function blockTypeFields(type) {
      switch ((type || '').toLowerCase()) {
      case 'text':
        return ['text', 'align', 'font_size', 'emphasis', 'visible_if'];
      case 'field':
        return ['label', 'path', 'format', 'align', 'font_size', 'emphasis', 'visible_if'];
      case 'table':
        return ['label', 'rows_path', 'columns', 'visible_if'];
      case 'totals':
        return ['label', 'rows_path', 'path', 'format', 'visible_if'];
      case 'image':
        return ['label', 'image_url', 'alt', 'align', 'visible_if'];
      case 'barcode':
        return ['label', 'path', 'value', 'format', 'visible_if'];
      case 'signature':
        return ['label', 'align', 'visible_if'];
      case 'divider':
        return ['visible_if'];
      default:
        return ['label', 'text', 'path', 'rows_path', 'columns', 'align', 'font_size', 'emphasis', 'visible_if'];
      }
    }
    function renderTemplateBindings() {
      const container = document.getElementById('template-bindings');
      if (!container) return;
      const current = selectedTemplateDefinition();
      const bindings = (adminState.templateBindings || []).slice().sort((left, right) => {
        const weight = function(scopeType) {
          if (scopeType === 'location') return 3;
          if (scopeType === 'organization') return 2;
          return 1;
        };
        return weight(right.scope_type) - weight(left.scope_type);
      });
      if (!bindings.length) {
        container.innerHTML = current
          ? '<article class="card"><strong>' + escapeHTML(t('template_module_default')) + '</strong><div class="muted">' + escapeHTML(current.target_kind + ' · ' + current.target_key + ' · ' + (current.purpose || '-') + ' · ' + (current.channel || '-')) + '</div><div class="status">' + escapeHTML(t('template_module_default_help')) + '</div></article>'
          : '<p class="status">-</p>';
        return;
      }
      container.innerHTML = bindings.map((item, index) => {
        const flags = [item.is_default ? t('template_binding_default') : '', item.is_official ? t('template_binding_official') : ''].filter(Boolean);
        const priority = index === 0 ? t('template_binding_effective') : (item.scope_type === 'location' ? t('template_binding_overrides_broader') : item.scope_type === 'organization' ? t('template_binding_overrides_deployment') : t('template_binding_fallback'));
        return '<article class="card"><strong>' + escapeHTML(item.scope_type + (item.scope_id ? ':' + item.scope_id : '')) + '</strong><div class="muted">' + escapeHTML(item.target_kind + ' · ' + item.target_key + ' · ' + (item.purpose || '-') + ' · ' + (item.channel || '-')) + '</div><div class="status">' + escapeHTML([priority].concat(flags).join(' · ')) + '</div></article>';
      }).join('');
    }
    function syncTemplateDesignerBody() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      if ((current.renderer_kind || '').toLowerCase() === 'visual' && adminState.templateDesigner.layout) {
        document.getElementById('template-body').value = JSON.stringify(adminState.templateDesigner.layout, null, 2);
        const settings = adminState.templateDesigner.layout.settings || {};
        let preset = settings.paper_preset || 'a4';
        if (preset === 'a4' && settings.orientation === 'landscape') preset = 'a4-landscape';
        document.getElementById('template-paper-preset').value = preset;
      }
    }
    function renderTemplateSectionTabs() {
      const container = document.getElementById('template-section-tabs');
      const layout = adminState.templateDesigner.layout;
      if (!container || !layout) return;
      container.innerHTML = (layout.sections || []).map((section) => {
        const active = adminState.templateDesigner.sectionID === section.id ? 'template-section-tab active' : 'template-section-tab';
        return '<button type="button" class="' + active + '" data-template-section="' + escapeHTML(section.id) + '">' + escapeHTML(templateSectionName(section)) + '</button>';
      }).join('');
      container.querySelectorAll('[data-template-section]').forEach((node) => {
        node.onclick = () => {
          adminState.templateDesigner.sectionID = node.getAttribute('data-template-section') || 'body';
          renderTemplateDesigner();
        };
      });
    }
    function renderTemplatePalette() {
      const palette = document.getElementById('template-block-palette');
      if (!palette) return;
      palette.innerHTML = templatePaletteItems().map((item) => '<button type="button" class="secondary" draggable="true" data-template-palette="' + item.type + '">' + escapeHTML(item.label) + '</button>').join('');
      palette.querySelectorAll('[data-template-palette]').forEach((node) => {
        node.addEventListener('dragstart', (event) => {
          event.dataTransfer.setData('text/plain', JSON.stringify({kind: 'palette', type: node.getAttribute('data-template-palette')}));
        });
        node.onclick = () => {
          const section = (adminState.templateDesigner.layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID);
          if (!section || !section.rows || !section.rows[0] || !section.rows[0].columns || !section.rows[0].columns[0]) return;
          section.rows[0].columns[0].blocks.push(createTemplateBlock(node.getAttribute('data-template-palette') || 'text'));
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
    }
    function renderTemplateCanvas() {
      const canvas = document.getElementById('template-canvas');
      const layout = adminState.templateDesigner.layout;
      if (!canvas || !layout) return;
      const section = (layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID) || (layout.sections || [])[0];
      if (!section) {
        canvas.innerHTML = '<p class="status">-</p>';
        return;
      }
      document.getElementById('template-active-section').textContent = templateSectionName(section);
      const preset = (() => {
        const settings = layout.settings || {};
        if (settings.paper_preset === 'receipt-80' || settings.paper_preset === 'receipt-58') return settings.paper_preset;
        if (settings.paper_preset === 'a4' && settings.orientation === 'landscape') return 'a4-landscape';
        return settings.paper_preset || 'a4';
      })();
      canvas.innerHTML = '<div class="template-paper ' + escapeHTML('paper-' + preset + ' density-' + ((layout.settings && layout.settings.density) || 'comfortable')) + '">' +
        '<div class="template-designer-section">' +
        (section.rows || []).map((row, rowIndex) => '<div class="template-designer-row-wrap"><div class="page-actions compact template-row-actions"><span class="status">Row ' + (rowIndex + 1) + '</span><button type="button" class="secondary" data-template-row-move="' + escapeHTML(section.id + ':up:' + row.id) + '">' + escapeHTML(t('template_move_up')) + '</button><button type="button" class="secondary" data-template-row-move="' + escapeHTML(section.id + ':down:' + row.id) + '">' + escapeHTML(t('template_move_down')) + '</button><button type="button" class="secondary" data-template-row-delete="' + escapeHTML(section.id + ':' + row.id) + '">' + escapeHTML(t('template_delete_row')) + '</button></div><div class="template-designer-row">' + (row.columns || []).map((column) => {
          const span = Math.min(12, Math.max(1, parseInt(column.span || 12, 10) || 12));
          return '<div class="template-cell-drop" data-template-cell="' + escapeHTML(column.id) + '" style="grid-column: span ' + span + ' / span ' + span + ';">' +
            '<div class="template-cell-toolbar"><span class="muted">Span ' + span + '/12</span>' + ((row.columns || []).length > 1 ? '<button type="button" class="secondary" data-template-remove-column="' + escapeHTML(column.id) + '">' + escapeHTML(t('template_remove_column')) + '</button>' : '') + '</div>' +
            ((column.blocks || []).map((block) => '<div class="template-designer-block' + (adminState.templateDesigner.selectedBlockID === block.id ? ' is-selected' : '') + '" draggable="true" data-template-block="' + escapeHTML(block.id) + '">' +
            '<div class="template-block-title">' + escapeHTML(block.label || block.text || block.type) + '</div>' +
            '<div class="template-block-meta">' + escapeHTML(block.type + (block.path ? ' · ' + block.path : block.rows_path ? ' · ' + block.rows_path : '')) + '</div>' +
            '</div>').join('') || '<div class="template-block-meta">Drop block here</div>') +
            '</div>';
        }).join('') + '</div></div>').join('') +
        '</div></div>';
      canvas.querySelectorAll('[data-template-row-move]').forEach((node) => {
        node.onclick = () => {
          const parts = (node.getAttribute('data-template-row-move') || '').split(':');
          if (parts.length !== 3) return;
          moveRow(parts[0], parts[2], parts[1] === 'up' ? -1 : 1);
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      canvas.querySelectorAll('[data-template-row-delete]').forEach((node) => {
        node.onclick = () => {
          const parts = (node.getAttribute('data-template-row-delete') || '').split(':');
          if (parts.length !== 2) return;
          const targetSection = (layout.sections || []).find((item) => item.id === parts[0]);
          if (!targetSection || !targetSection.rows || targetSection.rows.length <= 1) return;
          targetSection.rows = targetSection.rows.filter((item) => item.id !== parts[1]);
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      canvas.querySelectorAll('[data-template-remove-column]').forEach((node) => {
        node.onclick = () => {
          removeColumn(node.getAttribute('data-template-remove-column') || '');
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      canvas.querySelectorAll('[data-template-cell]').forEach((node) => {
        node.addEventListener('dragover', (event) => {
          event.preventDefault();
          node.classList.add('dragover');
        });
        node.addEventListener('dragleave', () => node.classList.remove('dragover'));
        node.addEventListener('drop', (event) => {
          event.preventDefault();
          node.classList.remove('dragover');
          let payload = null;
          try {
            payload = JSON.parse(event.dataTransfer.getData('text/plain') || '{}');
          } catch (_) {}
          if (!payload) return;
          const cellID = node.getAttribute('data-template-cell');
          let targetCell = null;
          for (const sectionItem of layout.sections || []) {
            for (const row of sectionItem.rows || []) {
              for (const column of row.columns || []) {
                if (column.id === cellID) targetCell = column;
              }
            }
          }
          if (!targetCell) return;
          const beforeNode = event.target && event.target.closest ? event.target.closest('[data-template-block]') : null;
          const beforeBlockID = beforeNode ? beforeNode.getAttribute('data-template-block') : '';
          if (payload.kind === 'palette') {
            const block = createTemplateBlock(payload.type || 'text');
            if (beforeBlockID) {
              const index = targetCell.blocks.findIndex((item) => item.id === beforeBlockID);
              if (index >= 0) targetCell.blocks.splice(index, 0, block);
              else targetCell.blocks.push(block);
            } else {
              targetCell.blocks.push(block);
            }
          }
          if (payload.kind === 'block' && payload.block_id) {
            moveTemplateBlock(payload.block_id, cellID, beforeBlockID || '');
          }
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        });
      });
      canvas.querySelectorAll('[data-template-block]').forEach((node) => {
        node.addEventListener('dragstart', (event) => {
          event.dataTransfer.setData('text/plain', JSON.stringify({kind: 'block', block_id: node.getAttribute('data-template-block')}));
        });
        node.onclick = () => {
          adminState.templateDesigner.selectedBlockID = node.getAttribute('data-template-block') || '';
          renderTemplateDesigner();
        };
      });
    }
    function renderTemplateInspector() {
      const inspector = document.getElementById('template-inspector');
      const layout = adminState.templateDesigner.layout;
      if (!inspector || !layout) return;
      const current = selectedTemplateDefinition();
      const selected = findTemplateBlock(adminState.templateDesigner.selectedBlockID);
      if ((current && current.renderer_kind || '').toLowerCase() !== 'visual') {
        inspector.innerHTML = '<p class="status">' + escapeHTML(t('template_inspector_empty')) + '</p>';
        return;
      }
      if (!selected) {
        inspector.innerHTML = '<label class="field"><span>' + escapeHTML(t('template_paper_preset')) + '</span><select id="template-inspector-paper-preset"><option value="a4">A4 Portrait</option><option value="a4-landscape">A4 Landscape</option><option value="receipt-80">Receipt 80mm</option><option value="receipt-58">Receipt 58mm</option></select></label>' +
          '<label class="field"><span>Density</span><select id="template-inspector-density"><option value="comfortable">comfortable</option><option value="compact">compact</option></select></label>' +
          '<p class="status">' + escapeHTML(t('template_inspector_empty')) + '</p>';
        const presetNode = document.getElementById('template-inspector-paper-preset');
        const densityNode = document.getElementById('template-inspector-density');
        let preset = (layout.settings && layout.settings.paper_preset) || 'a4';
        if (preset === 'a4' && (layout.settings && layout.settings.orientation) === 'landscape') preset = 'a4-landscape';
        presetNode.value = preset;
        densityNode.value = (layout.settings && layout.settings.density) || 'comfortable';
        presetNode.onchange = () => {
          if (!layout.settings) layout.settings = {};
          if (presetNode.value === 'a4-landscape') {
            layout.settings.paper_preset = 'a4';
            layout.settings.orientation = 'landscape';
          } else {
            layout.settings.paper_preset = presetNode.value;
            layout.settings.orientation = 'portrait';
          }
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
        densityNode.onchange = () => {
          if (!layout.settings) layout.settings = {};
          layout.settings.density = densityNode.value;
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
        return;
      }
      const block = selected.block;
      const supportedFields = blockTypeFields(block.type);
      let content = '<label class="field"><span>' + escapeHTML(t('template_block_label')) + '</span><input id="template-inspector-label" value="' + escapeHTML(block.label || '') + '"></label>';
      if (supportedFields.includes('text')) content += '<label class="field"><span>' + escapeHTML(t('template_block_text_prop')) + '</span><textarea id="template-inspector-text">' + escapeHTML(block.text || '') + '</textarea></label>';
      if (supportedFields.includes('path')) content += '<label class="field"><span>' + escapeHTML(t('template_block_path')) + '</span><input id="template-inspector-path" value="' + escapeHTML(block.path || '') + '"></label>';
      if (supportedFields.includes('rows_path')) content += '<label class="field"><span>' + escapeHTML(t('template_block_rows_path')) + '</span><input id="template-inspector-rows-path" value="' + escapeHTML(block.rows_path || '') + '"></label>';
      if (supportedFields.includes('value')) content += '<label class="field"><span>' + escapeHTML(t('template_block_value')) + '</span><input id="template-inspector-value" value="' + escapeHTML(block.value || '') + '"></label>';
      if (supportedFields.includes('image_url')) content += '<label class="field"><span>' + escapeHTML(t('template_block_image_url')) + '</span><input id="template-inspector-image-url" value="' + escapeHTML(block.image_url || '') + '"></label>';
      if (supportedFields.includes('alt')) content += '<label class="field"><span>' + escapeHTML(t('template_block_alt')) + '</span><input id="template-inspector-alt" value="' + escapeHTML(block.alt || '') + '"></label>';
      if (supportedFields.includes('format')) content += '<label class="field"><span>' + escapeHTML(t('template_block_format')) + '</span><input id="template-inspector-format" value="' + escapeHTML(block.format || '') + '"></label>';
      if (supportedFields.includes('columns')) {
        const columns = Array.isArray(block.columns) ? block.columns : [];
        content += '<div class="field"><span>' + escapeHTML(t('template_block_columns')) + '</span><div id="template-column-editor">' + (columns.map((column, index) => '<div class="form-grid compact"><label class="field"><span>' + escapeHTML(t('template_column_label')) + '</span><input data-template-column-label="' + index + '" value="' + escapeHTML(column.label || '') + '"></label><label class="field"><span>' + escapeHTML(t('template_column_path')) + '</span><input data-template-column-path="' + index + '" value="' + escapeHTML(column.path || '') + '"></label><div class="actions compact"><button type="button" class="secondary" data-template-column-remove="' + index + '">' + escapeHTML(t('template_remove_column_definition')) + '</button></div></div>').join('') || '<p class="status">' + escapeHTML(t('template_no_columns')) + '</p>') + '</div><button type="button" id="template-inspector-add-column" class="secondary">' + escapeHTML(t('template_add_column_definition')) + '</button></div>';
      }
      content +=
        '<label class="field"><span>' + escapeHTML(t('template_block_span')) + '</span><input id="template-inspector-span" type="number" min="1" max="12" value="' + escapeHTML(String(selected.column.span || 12)) + '"></label>' +
        (supportedFields.includes('align') ? '<label class="field"><span>' + escapeHTML(t('template_block_align')) + '</span><select id="template-inspector-align"><option value=\"\">default</option><option value=\"left\">left</option><option value=\"center\">center</option><option value=\"right\">right</option></select></label>' : '') +
        (supportedFields.includes('font_size') ? '<label class="field"><span>' + escapeHTML(t('template_block_size')) + '</span><select id="template-inspector-size"><option value=\"\">default</option><option value=\"sm\">sm</option><option value=\"lg\">lg</option><option value=\"xl\">xl</option></select></label>' : '') +
        (supportedFields.includes('emphasis') ? '<label class="field"><span>' + escapeHTML(t('template_block_emphasis')) + '</span><select id="template-inspector-emphasis"><option value=\"\">default</option><option value=\"strong\">strong</option><option value=\"muted\">muted</option></select></label>' : '') +
        (supportedFields.includes('visible_if') ? '<label class="field"><span>' + escapeHTML(t('template_block_visible_if')) + '</span><input id="template-inspector-visible-if" value="' + escapeHTML(block.visible_if || '') + '"></label>' : '') +
        '<div class="actions"><button id="template-duplicate-block" class="secondary">' + escapeHTML(t('template_duplicate_block')) + '</button><button id="template-delete-block" class="warn">' + escapeHTML(t('template_delete_block')) + '</button></div>';
      inspector.innerHTML = content;
      const bind = (id, key) => {
        const node = document.getElementById(id);
        if (!node) return;
        node.value = key === 'align' || key === 'font_size' || key === 'emphasis' ? (block[key] || '') : node.value;
        node.oninput = () => {
          if (key === 'columns') {
            try {
              block.columns = JSON.parse(node.value || '[]');
            } catch (_) {}
          } else if (key === 'span') {
            selected.column.span = Math.min(12, Math.max(1, parseInt(node.value || '12', 10) || 12));
          } else {
            block[key] = node.value;
          }
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
        node.onchange = node.oninput;
      };
      bind('template-inspector-label', 'label');
      bind('template-inspector-text', 'text');
      bind('template-inspector-path', 'path');
      bind('template-inspector-rows-path', 'rows_path');
      bind('template-inspector-value', 'value');
      bind('template-inspector-image-url', 'image_url');
      bind('template-inspector-alt', 'alt');
      bind('template-inspector-format', 'format');
      bind('template-inspector-span', 'span');
      bind('template-inspector-align', 'align');
      bind('template-inspector-size', 'font_size');
      bind('template-inspector-emphasis', 'emphasis');
      bind('template-inspector-visible-if', 'visible_if');
      const addColumnButton = document.getElementById('template-inspector-add-column');
      if (addColumnButton) {
        addColumnButton.onclick = () => {
          block.columns = Array.isArray(block.columns) ? block.columns : [];
          block.columns.push({label: 'Column', path: ''});
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      }
      inspector.querySelectorAll('[data-template-column-label]').forEach((node) => {
        node.oninput = () => {
          const index = parseInt(node.getAttribute('data-template-column-label') || '-1', 10);
          if (index < 0) return;
          block.columns = Array.isArray(block.columns) ? block.columns : [];
          block.columns[index] = Object.assign({}, block.columns[index] || {}, {label: node.value});
          syncTemplateDesignerBody();
        };
      });
      inspector.querySelectorAll('[data-template-column-path]').forEach((node) => {
        node.oninput = () => {
          const index = parseInt(node.getAttribute('data-template-column-path') || '-1', 10);
          if (index < 0) return;
          block.columns = Array.isArray(block.columns) ? block.columns : [];
          block.columns[index] = Object.assign({}, block.columns[index] || {}, {path: node.value});
          syncTemplateDesignerBody();
        };
      });
      inspector.querySelectorAll('[data-template-column-remove]').forEach((node) => {
        node.onclick = () => {
          const index = parseInt(node.getAttribute('data-template-column-remove') || '-1', 10);
          if (index < 0 || !Array.isArray(block.columns)) return;
          block.columns.splice(index, 1);
          syncTemplateDesignerBody();
          renderTemplateDesigner();
        };
      });
      document.getElementById('template-duplicate-block').onclick = () => {
        selected.column.blocks.push(Object.assign({}, JSON.parse(JSON.stringify(block)), {id: nextDesignerID('block')}));
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      document.getElementById('template-delete-block').onclick = () => {
        removeTemplateBlock(block.id);
        adminState.templateDesigner.selectedBlockID = '';
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
    }
    function renderTemplateDesigner() {
      renderTemplatePalette();
      renderTemplateSectionTabs();
      renderTemplateCanvas();
      renderTemplateInspector();
      syncTemplateDesignerBody();
    }
    async function renderTemplates(loadVersions) {
      const defs = adminState.templateDefinitions || [];
      const select = document.getElementById('template-definition');
      if (!select) return;
      select.innerHTML = defs.map((item) => '<option value="' + item.key + '">' + escapeHTML(pickText(item, 'title') || item.key) + ' (' + escapeHTML(item.key) + ')</option>').join('');
      if (!defs.length) {
        document.getElementById('template-preview').innerHTML = '<p class="status">-</p>';
        document.getElementById('template-versions').innerHTML = '<p class="status">-</p>';
        document.getElementById('template-bindings').innerHTML = '<p class="status">-</p>';
        return;
      }
      if (!select.value) {
        select.value = defs[0].key;
      }
      const current = defs.find((item) => item.key === select.value) || defs[0];
      if (loadVersions !== false) {
        await loadTemplateVersions(current.key);
      }
      await loadTemplateBindings(current.key);
      const currentBinding = (adminState.templateBindings || []).find((item) => item.template_key === current.key) || null;
      const draft = (adminState.templateVersions || []).find((item) => item.status === 'draft') || (adminState.templateVersions || []).slice(-1)[0];
      document.getElementById('template-body').value = (draft && draft.body) || current.default_body || '';
      document.getElementById('template-style').value = (draft && draft.style) || current.default_style || '';
      document.getElementById('template-purpose').value = (currentBinding && currentBinding.purpose) || current.purpose || '';
      document.getElementById('template-channel').value = (currentBinding && currentBinding.channel) || current.channel || '';
      document.getElementById('template-binding-scope').value = (currentBinding && currentBinding.scope_type) || 'deployment';
      document.getElementById('template-binding-default').checked = currentBinding ? !!currentBinding.is_default : true;
      document.getElementById('template-binding-official').checked = currentBinding ? !!currentBinding.is_official : (current.purpose || '') === 'official';
      document.getElementById('template-render-target-key').value = current.target_key || '';
      document.getElementById('template-render-target-id').value = '';
      document.getElementById('template-render-mode').value = 'sample';
      document.getElementById('template-status').textContent = t('loaded_template') + ' · ' + current.key;
      document.getElementById('template-versions').innerHTML = (adminState.templateVersions || []).map((item) => '<article class="card"><strong>v' + item.version + '</strong><div class="muted">' + escapeHTML(item.status) + ' · ' + escapeHTML(item.renderer_kind) + '</div></article>').join('');
      renderTemplateBindingScopeOptions(document.getElementById('template-binding-scope').value, currentBinding && currentBinding.scope_id);
      renderTemplateBindings();
      adminState.templateDesigner.layout = parseTemplateDesignerBody(current, document.getElementById('template-body').value);
      adminState.templateDesigner.sectionID = 'body';
      adminState.templateDesigner.selectedBlockID = '';
      const designerGrid = document.querySelector('[data-admin-route="/admin/templates"] .template-admin-grid');
      if (designerGrid) designerGrid.style.display = ((current.renderer_kind || '').toLowerCase() === 'visual') ? '' : 'none';
      document.getElementById('load-template-definition').onclick = () => { void renderTemplates(true); };
      document.getElementById('template-definition').onchange = () => { void renderTemplates(true); };
      document.getElementById('save-template-draft').onclick = saveTemplateDraft;
      document.getElementById('publish-template-version').onclick = publishTemplateDraft;
      document.getElementById('save-template-binding').onclick = saveTemplateBinding;
      document.getElementById('preview-template-render').onclick = previewTemplateRender;
      document.getElementById('template-binding-scope').onchange = () => {
        renderTemplateBindingScopeOptions(document.getElementById('template-binding-scope').value, '');
      };
      document.getElementById('template-add-row').onclick = () => {
        const layout = adminState.templateDesigner.layout;
        const section = layout && (layout.sections || []).find((item) => item.id === adminState.templateDesigner.sectionID);
        if (!section) return;
        const rowID = nextDesignerID((section.id || 'section') + '-row');
        section.rows.push({id: rowID, columns: [{id: rowID + '-cell-1', span: 12, blocks: []}]});
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      document.getElementById('template-paper-preset').onchange = () => {
        const layout = adminState.templateDesigner.layout;
        if (!layout) return;
        if (!layout.settings) layout.settings = {};
        const preset = document.getElementById('template-paper-preset').value;
        if (preset === 'a4-landscape') {
          layout.settings.paper_preset = 'a4';
          layout.settings.orientation = 'landscape';
        } else {
          layout.settings.paper_preset = preset;
          layout.settings.orientation = 'portrait';
        }
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      document.getElementById('template-add-column').onclick = () => {
        addColumnToActiveSection();
        syncTemplateDesignerBody();
        renderTemplateDesigner();
      };
      renderTemplateDesigner();
    }
    async function saveTemplateDraft() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      const key = current.key;
      if ((current.renderer_kind || '').toLowerCase() === 'visual') syncTemplateDesignerBody();
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/templates/' + encodeURIComponent(key) + '/actions/draft', {
        method:'PUT',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({body: document.getElementById('template-body').value, style: document.getElementById('template-style').value})
      });
      document.getElementById('template-status').textContent = t('saved_template_draft') + ' · ' + key;
      await renderTemplates(true);
    }
    async function publishTemplateDraft() {
      const key = document.getElementById('template-definition').value;
      const draft = (adminState.templateVersions || []).find((item) => item.status === 'draft');
      if (!draft) return;
      const csrf = getCookie('orbyte_csrf');
      await getJSON('/admin/api/templates/' + encodeURIComponent(key) + '/versions/' + draft.version + '/publish', {
        method:'POST',
        headers:{'X-CSRF-Token':csrf}
      });
      document.getElementById('template-status').textContent = t('published_template_version') + ' · ' + key + ' v' + draft.version;
      await renderTemplates(true);
    }
    async function saveTemplateBinding() {
      const current = (adminState.templateDefinitions || []).find((item) => item.key === document.getElementById('template-definition').value);
      if (!current) return;
      const csrf = getCookie('orbyte_csrf');
      const previous = (adminState.templateBindings || []).find((item) =>
        item.scope_type === document.getElementById('template-binding-scope').value &&
        (item.scope_id || '') === document.getElementById('template-binding-scope-id').value &&
        item.target_kind === current.target_kind &&
        item.target_key === current.target_key &&
        (item.purpose || '') === document.getElementById('template-purpose').value &&
        (item.channel || '') === document.getElementById('template-channel').value
      );
      const payload = await getJSON('/admin/api/template-bindings', {
        method:'PUT',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          template_key: current.key,
          scope_type: document.getElementById('template-binding-scope').value,
          scope_id: document.getElementById('template-binding-scope-id').value,
          target_kind: current.target_kind,
          target_key: current.target_key,
          purpose: document.getElementById('template-purpose').value,
          channel: document.getElementById('template-channel').value,
          is_default: !!document.getElementById('template-binding-default').checked,
          is_official: !!document.getElementById('template-binding-official').checked
        })
      });
      adminState.templateBindings = [payload.binding].concat((adminState.templateBindings || []).filter((item) => item.id !== payload.binding.id));
      document.getElementById('template-status').textContent = (previous ? t('updated_template_binding') : t('saved_template_binding')) + ' · ' + current.key;
      await renderTemplates(false);
    }
    async function previewTemplateRender() {
      const current = selectedTemplateDefinition();
      if (!current) return;
      if ((current.renderer_kind || '').toLowerCase() === 'visual') syncTemplateDesignerBody();
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/outputs/render', {
        method:'POST',
        headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          template_key: current.key,
          target_kind: current.target_kind,
          target_key: document.getElementById('template-render-target-key').value || current.target_key,
          target_id: document.getElementById('template-render-target-id').value,
          sample: document.getElementById('template-render-mode').value === 'sample',
          format: 'html',
          purpose: document.getElementById('template-purpose').value,
          channel: document.getElementById('template-channel').value,
          body: document.getElementById('template-body').value,
          style: document.getElementById('template-style').value,
          renderer_kind: current.renderer_kind
        })
      });
      document.getElementById('template-preview').innerHTML = payload.output.html || '';
    }
    function routeOptions(surface) {
      const actions = surface === 'admin'
        ? ((adminState.bootstrap && adminState.bootstrap.actions) || [])
        : ((adminState.bootstrap && adminState.bootstrap.user_actions) || []);
      const seen = new Set();
      return actions.filter((action) => {
        const path = action && action.route_path;
        if (!path || seen.has(path)) return false;
        seen.add(path);
        return true;
      }).map((action) => ({path: action.route_path, label: pickText(action, 'label') || action.route_path}));
    }
    function renderRouteOptionsDatalist(id, surface) {
      const options = routeOptions(surface);
      const node = document.getElementById(id);
      if (!node) return;
      node.innerHTML = options.map((item) => '<option value="' + escapeHTML(item.path) + '">' + escapeHTML(item.label) + '</option>').join('');
    }
    function selectedUser() {
      const select = document.getElementById('navigation-user-id');
      if (!select) return null;
      const value = select.value;
      return adminState.users.find((item) => item.id === value) || null;
    }
    function selectedRole() {
      const select = document.getElementById('navigation-role-id');
      if (!select || !adminState.bootstrap) return null;
      const value = select.value;
      return ((adminState.bootstrap.roles) || []).find((item) => item.id === value) || null;
    }
    function bindingsForSelectedUser() {
      const user = selectedUser();
      if (!user) return [];
      return adminState.bindings.filter((item) => item.user_id === user.id);
    }
    function renderBindingOptions() {
      const select = document.getElementById('navigation-binding-id');
      const status = document.getElementById('navigation-settings-status');
      if (!select) return;
      const bindings = bindingsForSelectedUser();
      if (!bindings.length) {
        select.innerHTML = '<option value="">' + t('no_bindings') + '</option>';
        select.value = '';
        document.getElementById('navigation-binding-priority').value = '0';
        if (status) status.textContent = t('no_bindings');
        return;
      }
      select.innerHTML = bindings.map((binding) => {
        const role = ((adminState.bootstrap && adminState.bootstrap.roles) || []).find((item) => item.id === binding.role_id);
        const label = (role ? role.name : binding.role_id) + ' · ' + binding.scope_type + (binding.scope_id ? ':' + binding.scope_id : '');
        return '<option value="' + binding.id + '">' + escapeHTML(label) + '</option>';
      }).join('');
      const current = bindings.find((binding) => binding.id === select.value) || bindings[0];
      select.value = current.id;
      document.getElementById('navigation-binding-priority').value = String(current.priority || 0);
      if (status) status.textContent = t('loaded_navigation_settings');
    }
    function syncNavigationForms() {
      const user = selectedUser();
      const role = selectedRole();
      document.getElementById('navigation-preferred-user-route').value = (user && user.preferred_user_route) || '';
      document.getElementById('navigation-preferred-admin-route').value = (user && user.preferred_admin_route) || '';
      document.getElementById('navigation-default-user-route').value = (role && role.default_user_route) || '';
      document.getElementById('navigation-default-admin-route').value = (role && role.default_admin_route) || '';
      renderBindingOptions();
    }
    function renderNavigationSettings() {
      const container = document.getElementById('navigation-settings');
      if (!container) return;
      if (!adminState.navigationManageAllowed) {
        container.innerHTML = '<p class="status">' + escapeHTML(t('manage_users_required')) + '</p>';
        return;
      }
      const users = adminState.users || [];
      const roles = (adminState.bootstrap && adminState.bootstrap.roles) || [];
      const currentUserID = (adminState.bootstrap && adminState.bootstrap.current_user_id) || (users[0] && users[0].id) || '';
      const currentRoleID = (roles[0] && roles[0].id) || '';
      container.innerHTML = ''
        + '<p class="status">' + escapeHTML(t('navigation_defaults_help')) + '</p>'
        + '<div class="admin-grid">'
        +   '<section class="card">'
        +     '<h3>' + escapeHTML(t('users')) + '</h3>'
        +     '<label class="field"><span>' + escapeHTML(t('selected_user')) + '</span><select id="navigation-user-id">'
        +       users.map((user) => '<option value="' + user.id + '"' + (user.id === currentUserID ? ' selected' : '') + '>' + escapeHTML(user.username + ' (' + user.id + ')') + '</option>').join('')
        +     '</select></label>'
        +     '<label class="field"><span>' + escapeHTML(t('preferred_user_route')) + '</span><input id="navigation-preferred-user-route" list="user-route-options" placeholder="/documents"></label>'
        +     '<label class="field"><span>' + escapeHTML(t('preferred_admin_route')) + '</span><input id="navigation-preferred-admin-route" list="admin-route-options" placeholder="/admin/modules"></label>'
        +     '<button id="save-user-navigation">' + escapeHTML(t('save_user_preferences')) + '</button>'
        +   '</section>'
        +   '<section class="card">'
        +     '<h3>' + escapeHTML(t('roles_label')) + '</h3>'
        +     '<label class="field"><span>' + escapeHTML(t('selected_role')) + '</span><select id="navigation-role-id">'
        +       roles.map((role) => '<option value="' + role.id + '"' + (role.id === currentRoleID ? ' selected' : '') + '>' + escapeHTML(role.name + ' (' + role.id + ')') + '</option>').join('')
        +     '</select></label>'
        +     '<label class="field"><span>' + escapeHTML(t('default_user_route')) + '</span><input id="navigation-default-user-route" list="user-route-options" placeholder="/documents"></label>'
        +     '<label class="field"><span>' + escapeHTML(t('default_admin_route')) + '</span><input id="navigation-default-admin-route" list="admin-route-options" placeholder="/admin/modules"></label>'
        +     '<button id="save-role-navigation">' + escapeHTML(t('save_role_defaults')) + '</button>'
        +   '</section>'
        +   '<section class="card">'
        +     '<h3>' + escapeHTML(t('role_bindings')) + '</h3>'
        +     '<label class="field"><span>' + escapeHTML(t('selected_binding')) + '</span><select id="navigation-binding-id"></select></label>'
        +     '<label class="field"><span>' + escapeHTML(t('binding_priority')) + '</span><input id="navigation-binding-priority" type="number" min="0" step="1"></label>'
        +     '<button id="save-binding-priority">' + escapeHTML(t('save_binding_priority')) + '</button>'
        +   '</section>'
        + '</div>'
        + '<datalist id="user-route-options"></datalist>'
        + '<datalist id="admin-route-options"></datalist>';
      renderRouteOptionsDatalist('user-route-options', 'user');
      renderRouteOptionsDatalist('admin-route-options', 'admin');
      syncNavigationForms();
      document.getElementById('navigation-user-id').addEventListener('change', syncNavigationForms);
      document.getElementById('navigation-role-id').addEventListener('change', syncNavigationForms);
      document.getElementById('navigation-binding-id').addEventListener('change', () => {
        const current = bindingsForSelectedUser().find((binding) => binding.id === document.getElementById('navigation-binding-id').value);
        document.getElementById('navigation-binding-priority').value = String((current && current.priority) || 0);
      });
      document.getElementById('save-user-navigation').addEventListener('click', saveUserNavigationPreferences);
      document.getElementById('save-role-navigation').addEventListener('click', saveRoleNavigationDefaults);
      document.getElementById('save-binding-priority').addEventListener('click', saveBindingPriority);
    }
    async function saveUserNavigationPreferences() {
      const user = selectedUser();
      if (!user) return;
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/users/' + encodeURIComponent(user.id) + '/preferences/navigation', {
        method: 'PUT',
        headers: {'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          preferred_user_route: document.getElementById('navigation-preferred-user-route').value,
          preferred_admin_route: document.getElementById('navigation-preferred-admin-route').value
        })
      });
      adminState.users = adminState.users.map((item) => item.id === payload.user.id ? payload.user : item);
      document.getElementById('navigation-settings-status').textContent = t('saved_user_preferences') + ' · ' + user.username;
      syncNavigationForms();
    }
    async function saveRoleNavigationDefaults() {
      const role = selectedRole();
      if (!role) return;
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/roles/' + encodeURIComponent(role.id) + '/defaults/navigation', {
        method: 'PUT',
        headers: {'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          default_user_route: document.getElementById('navigation-default-user-route').value,
          default_admin_route: document.getElementById('navigation-default-admin-route').value
        })
      });
      adminState.bootstrap.roles = ((adminState.bootstrap && adminState.bootstrap.roles) || []).map((item) => item.id === payload.role.id ? payload.role : item);
      document.getElementById('navigation-settings-status').textContent = t('saved_role_defaults') + ' · ' + role.name;
      syncNavigationForms();
    }
    async function saveBindingPriority() {
      const bindingID = document.getElementById('navigation-binding-id').value;
      if (!bindingID) return;
      const csrf = getCookie('orbyte_csrf');
      const payload = await getJSON('/role-bindings/' + encodeURIComponent(bindingID) + '/priority', {
        method: 'PUT',
        headers: {'Content-Type':'application/json','X-CSRF-Token':csrf},
        body: JSON.stringify({
          priority: parseInt(document.getElementById('navigation-binding-priority').value || '0', 10)
        })
      });
      adminState.bindings = adminState.bindings.map((item) => item.id === payload.binding.id ? payload.binding : item);
      document.getElementById('navigation-settings-status').textContent = t('saved_binding_priority') + ' · ' + bindingID;
      syncNavigationForms();
    }
    function renderPolicyHooks(items) {
      document.getElementById('policy-hooks').innerHTML = items.map((item) => {
        return '<article class="card"><h3>' + escapeHTML(item.key || '') + '</h3><p class="muted">' + escapeHTML(item.kind || '') + ' · ' + escapeHTML(item.target || '') + '</p>' +
          (pickText(item, 'description') ? '<p class="status">' + pickText(item, 'description') + '</p>' : '') +
          '<p><strong>' + t('module_label') + ':</strong> ' + escapeHTML(item.module_key || '-')
          + '</p><p><strong>' + t('target_label') + ':</strong> ' + escapeHTML(item.target || '-')
          + '</p></article>';
      }).join('');
    }
    function renderObservability(payload) {
      payload = payload || {};
      const renderList = (items, kind, textKey) => {
        if (!items || !items.length) return '<p class="status">-</p>';
        return items.map((item) => '<article class="card"><h3>' + escapeHTML(pickText(item, 'title') || item.key || item.type || '') + '</h3><p class="muted">' + escapeHTML(kind) + ' · ' + escapeHTML(item.key || item.type || '') + '</p>' + (pickText(item, 'description') ? '<p class="status">' + escapeHTML(pickText(item, 'description')) + '</p>' : '') + '</article>').join('');
      };
      document.getElementById('observability-contracts').innerHTML =
        '<section class="list"><h3>' + t('dashboards_label') + '</h3>' + renderList(payload.dashboards, 'dashboard', 'dashboards_label') + '</section>' +
        '<section class="list"><h3>' + t('reports_label') + '</h3>' + renderList(payload.reports, 'report', 'reports_label') + '</section>' +
        '<section class="list"><h3>' + t('metrics_label') + '</h3>' + renderList(payload.metrics, 'metric', 'metrics_label') + '</section>' +
        '<section class="list"><h3>' + t('hooks_label') + '</h3>' + renderList(payload.domain_events, 'domain_event', 'hooks_label') + '</section>';
    }
    function isAuthFailureMessage(message) {
      const value = String(message || '').toLowerCase();
      return value.includes('authentication required') ||
        value.includes('session not found') ||
        value.includes('session not active') ||
        value.includes('session revoked') ||
        value.includes('session expired') ||
        value.includes('invalid token signature') ||
        value.includes('invalid session token');
    }
    function getCookie(name) {
      const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
      return match ? decodeURIComponent(match[2]) : '';
    }
    function escapeHTML(value) {
      return String(value == null ? '' : value).replace(/[&<>"]/g, (char) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[char]));
    }
    window.addEventListener('hashchange', applyAdminRoute);
    document.getElementById('admin-logout-button').addEventListener('click', () => { void logoutAdmin(); });
    boot().catch(err => {
      if (isAuthFailureMessage(err && err.message)) {
        window.location.assign('/ui');
        return;
      }
      document.getElementById('definitions').textContent = String(err);
    });
  </script>
</body>
</html>`
