package httpx

import (
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/shared"
)

func registerUIDashboardRoutes(mux *http.ServeMux, ident *identity.Service, analyticsSvc *analytics.Service, modules *module.Service) {
	if analyticsSvc == nil {
		return
	}

	mux.HandleFunc("GET /ui/data/dashboard/boards/effective", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("dashboard is not allowed"))
			return
		}
		surface := strings.TrimSpace(r.URL.Query().Get("surface"))
		if surface == "" {
			surface = string(module.UISurfaceDashboard)
		}
		roleIDs := ident.ActiveRoleIDsForUser(
			principalEffectiveUserID(p),
			organizationIDForPrincipal(p),
			p.currentLocationID,
			"",
			time.Now().UTC(),
		)
		board, found := analyticsSvc.EffectiveDashboard(surface, organizationIDForPrincipal(p), p.currentLocationID, roleIDs)
		if !found {
			respondJSON(w, http.StatusOK, map[string]any{
				"surface": surface,
				"board":   nil,
				"widgets": []map[string]any{},
			})
			return
		}
		items := make([]map[string]any, 0, len(board.Widgets))
		for _, placement := range board.Widgets {
			def, ok := modules.DashboardWidgetForSurface(placement.WidgetKey, module.UISurface(surface))
			if !ok {
				continue
			}
			if !principalAllowsAll(ident, p, def.RequiredPermissions) {
				continue
			}
			items = append(items, map[string]any{
				"id":               placement.ID,
				"title":            firstNonEmptyString(strings.TrimSpace(placement.Title), def.Title),
				"kind":             firstNonEmptyString(strings.TrimSpace(placement.Kind), def.RendererKind),
				"widget_key":       placement.WidgetKey,
				"width":            placement.Width,
				"height":           placement.Height,
				"order":            placement.Order,
				"refresh_override": placement.RefreshOverride,
				"filters":          placement.Filters,
				"definition":       def,
			})
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"surface": surface,
			"board":   board,
			"widgets": items,
		})
	})
}
