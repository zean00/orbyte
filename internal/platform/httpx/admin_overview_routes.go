package httpx

import (
	"net/http"
	"strings"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/workflow"
)

func registerAdminOverviewRoutes(mux *http.ServeMux, cfg *config.Service, org *organization.Service, ident *identity.Service, modules *module.Service, workflowSvc *workflow.Service, policySvc *policy.Service, acpSvc *acp.Service) {
	mux.HandleFunc("GET /admin/api/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read")
		if !ok {
			return
		}
		respondJSON(w, http.StatusOK, buildAdminBootstrapPayload(r, org, ident, modules, p, acpSvc))
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
		respondJSON(w, http.StatusOK, map[string]any{
			"items": modules.ListForScope(
				strings.TrimSpace(r.URL.Query().Get("organization_id")),
				strings.TrimSpace(r.URL.Query().Get("location_id")),
				strings.TrimSpace(r.URL.Query().Get("operating_unit_id")),
			),
		})
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

	mux.HandleFunc("GET /admin/api/workflows", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": workflowSvc.ListDefinitions()})
	})

}
