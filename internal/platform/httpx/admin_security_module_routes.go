package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/shared"
)

func registerAdminSecurityModuleRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, auditSvc *audit.Service, policySvc *policy.Service, integrationSvc *integration.Service, referenceSvc *reference.Service, idempotencySvc *idempotency.Service) {
	mux.HandleFunc("GET /admin/api/idempotency/records", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		items := idempotencySvc.List()
		if operation := strings.TrimSpace(r.URL.Query().Get("operation")); operation != "" {
			filtered := make([]idempotency.Record, 0, len(items))
			for _, item := range items {
				if item.Operation == operation {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
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
			IdempotencyKey  string         `json:"idempotency_key"`
			SystemKey       string         `json:"system_key"`
			EndpointKey     string         `json:"endpoint_key"`
			ContractKey     string         `json:"contract_key"`
			ContractVersion int            `json:"contract_version"`
			Intent          string         `json:"intent"`
			Mode            string         `json:"mode"`
			OperationType   string         `json:"operation_type"`
			DocumentID      string         `json:"document_id"`
			CorrelationID   string         `json:"correlation_id"`
			Payload         map[string]any `json:"payload"`
			ProcessNow      bool           `json:"process_now"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		outcome, err := idempotencySvc.Execute("integration.submission.create", req.IdempotencyKey, principalActorID(p), req, func() (idempotency.Outcome, error) {
			record, err := integrationSvc.CreateDelivery(integration.SubmissionRecord{
				ExternalSystemKey: req.SystemKey,
				EndpointKey:       req.EndpointKey,
				ContractKey:       req.ContractKey,
				ContractVersion:   req.ContractVersion,
				Intent:            req.Intent,
				Mode:              req.Mode,
				OperationType:     req.OperationType,
				DocumentID:        req.DocumentID,
				CorrelationID:     req.CorrelationID,
				IdempotencyKey:    req.IdempotencyKey,
				Payload:           req.Payload,
			})
			if err != nil {
				return idempotency.Outcome{}, err
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
						return idempotency.Outcome{}, err
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
			return idempotency.Outcome{StatusCode: status, Response: response}, nil
		})
		if err != nil {
			respondIntegrationError(w, err, nil)
			return
		}
		respondJSON(w, outcome.StatusCode, outcome.Response)
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

}
