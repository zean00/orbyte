package httpx

import (
	"encoding/json"
	"net/http"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/idempotency"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/shared"
)

func registerAdminIntegrationRoutes(mux *http.ServeMux, ident *identity.Service, auditSvc *audit.Service, integrationSvc *integration.Service, idempotencySvc *idempotency.Service) {
	mux.HandleFunc("GET /admin/api/integrations/systems", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListSystems()})
	})

	mux.HandleFunc("GET /admin/api/integrations/endpoints", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListEndpoints()})
	})

	mux.HandleFunc("GET /admin/api/integrations/contracts", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListContracts()})
	})

	mux.HandleFunc("GET /admin/api/integrations/mappings", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListMappings()})
	})

	mux.HandleFunc("GET /admin/api/integrations/submissions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListSubmissions()})
	})

	mux.HandleFunc("GET /admin/api/integrations/adapters", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListAdapterDescriptors()})
	})

	mux.HandleFunc("GET /admin/api/integrations/systems/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		systemKey, action, ok := adminIntegrationSystemDetailPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("integration system route not found"))
			return
		}
		switch action {
		case "config":
			view, err := integrationSvc.ValidateSystemConfig(systemKey)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, view)
		case "health":
			health, err := integrationSvc.HealthForSystem(systemKey)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, health)
		default:
			respondError(w, shared.NotFound("integration system route not found"))
		}
	})

	mux.HandleFunc("PUT /admin/api/integrations/systems/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.manage", "", "module.manage"); !ok {
			return
		}
		systemKey, action, ok := adminIntegrationSystemDetailPath(r.URL.Path)
		if !ok || action != "config" {
			respondError(w, shared.NotFound("integration system route not found"))
			return
		}
		var req struct {
			Settings map[string]any `json:"settings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		record, view, err := integrationSvc.UpdateSystemSettings(systemKey, req.Settings)
		if err != nil {
			respondIntegrationError(w, err, view)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": record, "config": view})
	})

	mux.HandleFunc("GET /admin/api/integrations/endpoints/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		endpointKey, action, ok := adminIntegrationEndpointDetailPath(r.URL.Path)
		if !ok || action != "config" {
			respondError(w, shared.NotFound("integration endpoint route not found"))
			return
		}
		view, err := integrationSvc.ValidateEndpointConfig(endpointKey)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("PUT /admin/api/integrations/endpoints/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.manage", "", "module.manage"); !ok {
			return
		}
		endpointKey, action, ok := adminIntegrationEndpointDetailPath(r.URL.Path)
		if !ok || action != "config" {
			respondError(w, shared.NotFound("integration endpoint route not found"))
			return
		}
		var req struct {
			Settings map[string]any `json:"settings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		record, view, err := integrationSvc.UpdateEndpointSettings(endpointKey, req.Settings)
		if err != nil {
			respondIntegrationError(w, err, view)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": record, "config": view})
	})

	mux.HandleFunc("GET /admin/api/integrations/submissions/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		submissionID, detail, ok := adminIntegrationSubmissionDetailPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("integration submission route not found"))
			return
		}
		switch detail {
		case "":
			record, ok := integrationSvc.GetSubmission(submissionID)
			if !ok {
				respondError(w, shared.NotFound("integration submission not found"))
				return
			}
			respondJSON(w, http.StatusOK, record)
		case "attempts":
			respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListSubmissionAttempts(submissionID)})
		default:
			respondError(w, shared.NotFound("integration submission route not found"))
		}
	})

}
