package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func registerAdminWorkflowRoutes(mux *http.ServeMux, ident *identity.Service, workflowSvc *workflow.Service, auditSvc *audit.Service, policySvc *policy.Service, obsSvc *observability.Service) {
	mux.HandleFunc("GET /admin/api/workflows/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		key, version, action, ok := adminWorkflowPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("workflow route not found"))
			return
		}
		switch {
		case version == 0 && action == "versions":
			respondJSON(w, http.StatusOK, map[string]any{"items": workflowSvc.ListVersions(key)})
		case version > 0 && action == "":
			item, err := workflowSvc.GetVersion(key, version)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, item)
		default:
			respondError(w, shared.NotFound("workflow route not found"))
		}
	})

	mux.HandleFunc("POST /admin/api/workflows/", func(w http.ResponseWriter, r *http.Request) {
		key, version, action, ok := adminWorkflowPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("workflow route not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		switch {
		case version == 0 && action == "drafts":
			item, err := workflowSvc.CreateDraft(key, principalActorID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, item)
		case version > 0 && action == "validate":
			item, err := workflowSvc.GetVersion(key, version)
			if err != nil {
				respondError(w, err)
				return
			}
			validation := workflowSvc.Validate(item)
			policyIssues := workflowPolicyRuntimeIssues(policySvc, item, strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")))
			issues := append([]string{}, validation.Issues...)
			issues = append(issues, policyIssues...)
			respondJSON(w, http.StatusOK, map[string]any{
				"valid":                 validation.Valid && len(policyIssues) == 0,
				"issues":                issues,
				"workflow_issues":       validation.Issues,
				"policy_runtime_issues": policyIssues,
			})
		case version > 0 && action == "simulate":
			item, err := workflowSvc.GetVersion(key, version)
			if err != nil {
				respondError(w, err)
				return
			}
			var req workflow.SimulationInput
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid workflow simulation request"))
				return
			}
			result := workflowSvc.Simulate(item, req)
			respondJSON(w, http.StatusOK, map[string]any{
				"simulation":      result,
				"routing_preview": workflowRoutingPreview(ident, item, req),
			})
		case version > 0 && action == "publish":
			item, err := workflowSvc.GetVersion(key, version)
			if err != nil {
				respondError(w, err)
				return
			}
			if issues := workflowPolicyRuntimeIssues(policySvc, item, strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id"))); len(issues) > 0 {
				respondError(w, shared.Validation(strings.Join(issues, "; ")))
				return
			}
			item, err = workflowSvc.Publish(key, version, principalActorID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, item)
		default:
			respondError(w, shared.NotFound("workflow route not found"))
		}
	})

	mux.HandleFunc("PUT /admin/api/workflows/", func(w http.ResponseWriter, r *http.Request) {
		key, version, action, ok := adminWorkflowPath(r.URL.Path)
		if !ok || version <= 0 || action != "" {
			respondError(w, shared.NotFound("workflow route not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		var req workflow.Definition
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid workflow draft request"))
			return
		}
		req.Key = key
		req.Version = version
		req.Status = "draft"
		item, err := workflowSvc.SaveDraft(req, principalActorID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, item)
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

}
