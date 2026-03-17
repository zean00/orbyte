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
		defaultPath := ""
		if len(menus) > 0 {
			for _, action := range actions {
				if action.Key == menus[0].ActionKey {
					defaultPath = action.RoutePath
					break
				}
			}
		}
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
			"views":             views,
			"custom_entries":    entries,
			"default_path":      defaultPath,
			"ui_access":         len(uiMenus) > 0,
			"ui_path":           uiPath,
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
    <section class="card" data-admin-route="/admin/definitions">
      <h2 id="definitions-heading">Definitions</h2>
      <div id="definitions" class="list"></div>
    </section>
    <div class="admin-grid" data-admin-route="/admin/security">
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
        definitions: 'Definitions',
        role_templates: 'Role Templates',
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
        definitions: 'Definisi',
        role_templates: 'Template Peran',
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
    const adminState = { bootstrap: null, locale: 'en', supportedLocales: ['en', 'id'] };
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
        'definitions-heading': 'definitions',
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
    async function boot() {
      if (!adminState.bootstrap) {
        adminState.locale = normalizeLocale(navigator.language || 'en');
      }
      const [bootstrap, modules, definitions, roleTemplates, policyHooks, observability, authSettings] = await Promise.all([
        getJSON('/admin/api/bootstrap'),
        getJSON('/admin/api/modules'),
        getJSON('/admin/api/config/definitions'),
        getJSON('/admin/api/security/role-templates'),
        getJSON('/admin/api/security/policy-hooks'),
        getJSON('/admin/api/observability/contracts'),
        getJSON('/admin/api/auth/settings')
      ]);
      adminState.bootstrap = bootstrap;
      adminState.supportedLocales = bootstrap.supported_locales || ['en', 'id'];
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
      renderAuthSettings(authSettings.entry.value);
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
