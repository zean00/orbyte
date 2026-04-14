package httpx

import (
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUIProcurementRoutes(mux *http.ServeMux, ident *identity.Service, procurementSvc *application.ProcurementCoreService) {
	if procurementSvc == nil {
		return
	}
	mux.HandleFunc("GET /ui/data/procurement/payables/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("procurement payables are not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, procurementSvc.PayablesSummaryScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			time.Now().UTC(),
		))
	})
	mux.HandleFunc("GET /ui/data/procurement/vendors/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"vendor.read", "document.list"}) {
			respondError(w, shared.Forbidden("vendor payables summary is not allowed"))
			return
		}
		const prefix = "/ui/data/procurement/vendors/"
		path := r.URL.Path
		if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/summary") {
			http.NotFound(w, r)
			return
		}
		vendorID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/summary")
		vendorID = strings.TrimSpace(strings.Trim(vendorID, "/"))
		if vendorID == "" {
			respondError(w, shared.NotFound("vendor not found"))
			return
		}
		respondJSON(w, http.StatusOK, procurementSvc.VendorSummaryScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			vendorID,
			strings.TrimSpace(r.URL.Query().Get("from")),
			strings.TrimSpace(r.URL.Query().Get("to")),
		))
	})
}
