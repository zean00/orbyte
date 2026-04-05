package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/shared"
)

func registerAdminDashboardRoutes(mux *http.ServeMux, org *organization.Service, ident *identity.Service, analyticsSvc *analytics.Service, modules *module.Service) {
	if analyticsSvc == nil {
		return
	}

	mux.HandleFunc("GET /admin/api/dashboards", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		surface := strings.TrimSpace(r.URL.Query().Get("surface"))
		if surface == "" {
			surface = string(module.UISurfaceDashboard)
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items":     analyticsSvc.DashboardsForSurface(surface),
			"widgets":   modules.DashboardWidgetsForSurface(module.UISurface(surface)),
			"roles":     ident.Roles(),
			"locations": org.Locations(),
			"root":      org.Root(),
			"surface":   surface,
		})
	})

	mux.HandleFunc("POST /admin/api/dashboards", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "analytics.author", "", "analytics.author")
		if !ok {
			return
		}
		var item analytics.Dashboard
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		item.Surface = firstNonEmptyString(strings.TrimSpace(item.Surface), string(module.UISurfaceDashboard))
		item.OwnerUserID = principalEffectiveUserID(p)
		if err := validateAdminDashboard(item, org, ident, modules); err != nil {
			respondError(w, err)
			return
		}
		saved, err := analyticsSvc.SaveDashboard(item)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, saved)
	})

	mux.HandleFunc("DELETE /admin/api/dashboards/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.author", "", "analytics.author"); !ok {
			return
		}
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/admin/api/dashboards/"))
		if id == "" {
			respondError(w, shared.NotFound("dashboard not found"))
			return
		}
		if err := analyticsSvc.DeleteDashboard(id); err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
	})
}

func validateAdminDashboard(item analytics.Dashboard, org *organization.Service, ident *identity.Service, modules *module.Service) error {
	surface := module.UISurface(strings.TrimSpace(item.Surface))
	if surface == "" {
		surface = module.UISurfaceDashboard
	}
	switch strings.TrimSpace(item.ScopeType) {
	case "", "deployment":
		item.ScopeID = ""
	case "organization":
		if strings.TrimSpace(item.ScopeID) == "" || org.Root().ID != strings.TrimSpace(item.ScopeID) {
			return shared.Validation("organization scope_id is invalid")
		}
	case "location":
		if !containsLocationID(org.Locations(), item.ScopeID) {
			return shared.Validation("location scope_id is invalid")
		}
	case "role":
		if !containsRoleID(ident.Roles(), item.ScopeID) {
			return shared.Validation("role scope_id is invalid")
		}
	default:
		return shared.Validation("scope_type must be deployment, organization, location, or role")
	}
	for i := range item.Widgets {
		widget := &item.Widgets[i]
		if strings.TrimSpace(widget.WidgetKey) == "" {
			return shared.Validation("widget_key is required")
		}
		def, ok := modules.DashboardWidgetForSurface(widget.WidgetKey, surface)
		if !ok {
			return shared.Validation("dashboard widget is not registered")
		}
		if strings.TrimSpace(widget.Title) == "" {
			widget.Title = def.Title
		}
		if strings.TrimSpace(widget.Kind) == "" {
			widget.Kind = def.RendererKind
		}
		if widget.Width <= 0 {
			if def.DefaultWidth > 0 {
				widget.Width = def.DefaultWidth
			} else {
				widget.Width = 4
			}
		}
		if widget.Height <= 0 {
			if def.DefaultHeight > 0 {
				widget.Height = def.DefaultHeight
			} else {
				widget.Height = 1
			}
		}
	}
	return nil
}

func containsLocationID(items []organization.Location, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == id {
			return true
		}
	}
	return false
}

func containsRoleID(items []identity.Role, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == id {
			return true
		}
	}
	return false
}
