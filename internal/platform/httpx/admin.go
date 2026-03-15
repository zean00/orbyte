package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
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
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(adminConsoleHTML))
	})

	mux.HandleFunc("GET /admin/api/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"organization": org.Root(),
			"locations":    org.Locations(),
			"roles":        ident.Roles(),
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
  <title>Clinic Admin</title>
  <style>
    :root { --bg:#f5f1e8; --ink:#1f2a21; --muted:#58645a; --card:#fffdf8; --line:#d8cfbf; --accent:#1f6f5f; --danger:#8f2d1f; }
    * { box-sizing:border-box; } body { margin:0; font-family: Georgia, serif; background:linear-gradient(180deg,#efe6d5,#f8f5ee); color:var(--ink); }
    header { padding:24px 32px; border-bottom:1px solid var(--line); background:rgba(255,253,248,.85); backdrop-filter: blur(8px); position:sticky; top:0; }
    main { padding:24px 32px 48px; display:grid; gap:24px; }
    .grid { display:grid; gap:24px; grid-template-columns: 1.2fr 1fr; }
    .card { background:var(--card); border:1px solid var(--line); border-radius:18px; padding:20px; box-shadow:0 10px 30px rgba(31,42,33,.06); }
    h1,h2,h3 { margin:0 0 12px; font-weight:600; }
    p, label, input, select, textarea, button { font: inherit; }
    table { width:100%; border-collapse:collapse; } th, td { text-align:left; padding:10px 8px; border-bottom:1px solid var(--line); vertical-align:top; }
    .muted { color:var(--muted); } .pill { display:inline-block; padding:4px 8px; border-radius:999px; background:#dfeee8; color:#185446; font-size:12px; }
    .pill.off { background:#f7d8cf; color:var(--danger); } .actions { display:flex; gap:8px; flex-wrap:wrap; margin-top:12px; }
    button { border:0; border-radius:12px; padding:10px 14px; background:var(--accent); color:#fff; cursor:pointer; }
    button.secondary { background:#e5ddd0; color:var(--ink); } button.warn { background:var(--danger); }
    input, select, textarea { width:100%; padding:10px 12px; border-radius:12px; border:1px solid var(--line); background:#fff; }
    textarea { min-height:160px; font-family: ui-monospace, monospace; }
    .field { display:grid; gap:6px; margin-bottom:12px; }
    .row { display:grid; grid-template-columns: repeat(2,1fr); gap:12px; }
    pre { background:#f4efe7; padding:12px; border-radius:12px; overflow:auto; }
    @media (max-width: 960px) { .grid, .row { grid-template-columns: 1fr; } header, main { padding:16px; } }
  </style>
</head>
<body>
  <header>
    <h1>Platform Admin</h1>
    <p class="muted">Modules, scoped configuration, and effective runtime settings.</p>
  </header>
  <main>
    <div class="grid">
      <section class="card">
        <h2>Modules</h2>
        <div id="modules"></div>
      </section>
      <section class="card">
        <h2>Authentication Settings</h2>
        <div class="row">
          <label class="field"><span>Password Login</span><select id="auth-password-enabled"><option value="true">enabled</option><option value="false">disabled</option></select></label>
          <label class="field"><span>Google Login</span><select id="auth-google-enabled"><option value="true">enabled</option><option value="false">disabled</option></select></label>
        </div>
        <div class="row">
          <label class="field"><span>Login Title</span><input id="auth-login-title"></label>
          <label class="field"><span>Google Button Label</span><input id="auth-google-button-label"></label>
        </div>
        <label class="field"><span>Login Subtitle</span><input id="auth-login-subtitle"></label>
        <div class="row">
          <label class="field"><span>Google Client ID</span><input id="auth-google-client-id"></label>
          <label class="field"><span>Google Client Secret</span><input id="auth-google-client-secret" placeholder="[redacted]"></label>
        </div>
        <div class="row">
          <label class="field"><span>Google Redirect URL</span><input id="auth-google-redirect-url"></label>
          <label class="field"><span>Google Hosted Domain</span><input id="auth-google-hosted-domain"></label>
        </div>
        <div class="row">
          <label class="field"><span>Google Auth URL</span><input id="auth-google-auth-url"></label>
          <label class="field"><span>Google Token URL</span><input id="auth-google-token-url"></label>
        </div>
        <div class="row">
          <label class="field"><span>Google JWKS URL</span><input id="auth-google-jwks-url"></label>
          <label class="field"><span>Google Issuer</span><input id="auth-google-issuer"></label>
        </div>
        <div class="row">
          <label class="field"><span>Google Timeout Seconds</span><input id="auth-google-timeout-seconds" type="number"></label>
          <label class="field"><span>Provision New Users</span><select id="auth-google-auto-provision-enabled"><option value="true">enabled</option><option value="false">disabled</option></select></label>
        </div>
        <div class="row">
          <label class="field"><span>Provision Role</span><select id="auth-google-auto-provision-role-id"></select></label>
          <label class="field"><span>Provision Default Location</span><select id="auth-google-auto-provision-default-location-id"></select></label>
        </div>
        <div class="row">
          <label class="field"><span>Provision Scope Type</span><select id="auth-google-auto-provision-scope-type"><option value="deployment">deployment</option><option value="organization">organization</option><option value="location">location</option></select></label>
          <label class="field"><span>Provision Scope ID</span><select id="auth-google-auto-provision-scope-id"></select></label>
        </div>
        <label class="field"><span>Provision Allowed Domains</span><input id="auth-google-auto-provision-allowed-domains" placeholder="example.com, example.org"></label>
        <div class="actions">
          <button id="load-auth-settings" class="secondary">Load Auth Settings</button>
          <button id="save-auth-settings">Save Auth Settings</button>
        </div>
        <p id="auth-settings-status" class="muted"></p>
        <pre id="auth-settings-validation"></pre>
      </section>
    </div>
    <div class="grid">
      <section class="card">
        <h2>Config Editor</h2>
        <div class="row">
          <div class="field"><label>Config Key</label><select id="config-key"></select></div>
          <div class="field"><label>Scope</label><select id="config-scope"><option value="deployment">deployment</option><option value="organization">organization</option><option value="location">location</option></select></div>
        </div>
        <div class="row">
          <div class="field"><label>Organization</label><select id="organization-id"></select></div>
          <div class="field"><label>Location</label><select id="location-id"></select></div>
        </div>
        <div class="field"><label>Value JSON</label><textarea id="config-value"></textarea></div>
        <div class="actions">
          <button id="load-effective" class="secondary">Load Effective</button>
          <button id="save-config">Save Entry</button>
        </div>
        <p id="config-status" class="muted"></p>
      </section>
    </div>
    <section class="card">
      <h2>Definitions</h2>
      <pre id="definitions"></pre>
    </section>
    <div class="grid">
      <section class="card">
        <h2>Role Templates</h2>
        <pre id="role-templates"></pre>
      </section>
      <section class="card">
        <h2>Policy Hooks</h2>
        <pre id="policy-hooks"></pre>
      </section>
    </div>
    <section class="card">
      <h2>Observability Contracts</h2>
      <pre id="observability-contracts"></pre>
    </section>
  </main>
  <script>
    const adminState = { bootstrap: null };
    async function getJSON(url, options) {
      const resp = await fetch(url, Object.assign({credentials:'include'}, options || {}));
      if (!resp.ok) {
        const text = await resp.text();
        throw new Error(text || ('HTTP ' + resp.status));
      }
      return resp.json();
    }
    async function boot() {
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
      document.getElementById('organization-id').innerHTML = '<option value="">default</option><option value="' + bootstrap.organization.id + '">' + bootstrap.organization.name + '</option>';
      document.getElementById('location-id').innerHTML = '<option value="">default</option>' + bootstrap.locations.map(loc => '<option value="' + loc.id + '">' + loc.name + '</option>').join('');
      renderModules(modules.items);
      renderDefinitions(definitions.items);
      renderAuthSettings(authSettings.entry.value);
      document.getElementById('role-templates').textContent = JSON.stringify(roleTemplates.items, null, 2);
      document.getElementById('policy-hooks').textContent = JSON.stringify(policyHooks.items, null, 2);
      document.getElementById('observability-contracts').textContent = JSON.stringify(observability, null, 2);
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
        target.innerHTML = '<option value="">Deployment default</option>';
        target.value = '';
        target.disabled = true;
        return;
      }
      if (scopeType === 'organization') {
        const org = adminState.bootstrap && adminState.bootstrap.organization;
        target.innerHTML = '<option value="">Select organization</option>' + (org ? '<option value="' + org.id + '">' + org.name + ' (' + org.id + ')</option>' : '');
        target.disabled = false;
        target.value = selectedValue || '';
        return;
      }
      const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
      target.innerHTML = '<option value="">Select location</option>' + locations.map(loc => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
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
        : 'No authentication validation issues.';
    }
    function renderAuthSettings(value) {
      value = value || {};
      const roles = (adminState.bootstrap && adminState.bootstrap.roles) || [];
      const locations = (adminState.bootstrap && adminState.bootstrap.locations) || [];
      document.getElementById('auth-google-auto-provision-role-id').innerHTML = '<option value="">Select role</option>' + roles.map(role => '<option value="' + role.id + '">' + role.name + ' (' + role.id + ')</option>').join('');
      document.getElementById('auth-google-auto-provision-default-location-id').innerHTML = '<option value="">Default location</option>' + locations.map(loc => '<option value="' + loc.id + '">' + loc.name + ' (' + loc.id + ')</option>').join('');
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
      document.getElementById('auth-settings-status').textContent = 'Loaded authentication settings from ' + payload.entry.source_scope + (payload.entry.source_scope_id ? ':' + payload.entry.source_scope_id : '');
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
      document.getElementById('auth-settings-status').textContent = 'Saved authentication settings at ' + scope + (scopeID ? ':' + scopeID : '');
      await loadAuthSettingsValidation();
    }
    function renderModules(items) {
      document.getElementById('modules').innerHTML = '<table><thead><tr><th>Module</th><th>Status</th><th>Dependencies</th><th></th></tr></thead><tbody>' + items.map(item => {
        const enabled = item.installed.enabled;
        const deps = (item.dependency_diagnostics || []).map(dep => dep.module_key + ':' + (dep.compatible ? 'ok' : dep.reason || 'blocked')).join(', ');
        return '<tr><td><strong>' + item.manifest.name + '</strong><div class="muted">' + item.manifest.key + ' · ' + item.manifest.version + '</div></td><td><span class="pill ' + (enabled ? '' : 'off') + '">' + (enabled ? 'enabled' : 'disabled') + '</span></td><td class="muted">' + (deps || 'none') + '</td><td><button data-key="' + item.manifest.key + '" data-action="' + (enabled ? 'disable' : 'enable') + '" class="' + (enabled ? 'warn' : '') + '">' + (enabled ? 'Disable' : 'Enable') + '</button></td></tr>';
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
      document.getElementById('definitions').textContent = JSON.stringify(items, null, 2);
      document.getElementById('config-key').innerHTML = items.map(item => '<option value="' + item.key + '">' + item.key + '</option>').join('');
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
          document.getElementById('config-status').textContent = 'Loaded effective value from ' + match.source_scope + (match.source_scope_id ? ':' + match.source_scope_id : '');
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
        document.getElementById('config-status').textContent = 'Saved ' + key + ' at ' + scope + (scopeID ? ':' + scopeID : '');
      };
    }
    function getCookie(name) {
      const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
      return match ? decodeURIComponent(match[2]) : '';
    }
    boot().catch(err => { document.getElementById('definitions').textContent = String(err); });
  </script>
</body>
</html>`
