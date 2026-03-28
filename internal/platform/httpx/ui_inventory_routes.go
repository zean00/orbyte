package httpx

import (
	"net/http"
	"strings"
	"time"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUIInventoryRoutes(mux *http.ServeMux, ident *identity.Service, inventorySvc *application.InventoryCoreService) {
	if inventorySvc == nil {
		return
	}

	mux.HandleFunc("GET /ui/data/inventory/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("inventory summary is not allowed"))
			return
		}
		summary := inventorySvc.SummaryScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			time.Now().UTC(),
		)
		respondJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("GET /ui/data/inventory/items/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/stock") {
			http.NotFound(w, r)
			return
		}
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("inventory detail is not allowed"))
			return
		}
		itemCode := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/data/inventory/items/"), "/stock")
		payload := inventorySvc.ItemStockScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			itemCode,
			time.Now().UTC(),
		)
		respondJSON(w, http.StatusOK, payload)
	})
}
