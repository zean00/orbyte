package httpx

import (
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUICommercialRoutes(mux *http.ServeMux, ident *identity.Service, commercialSvc *application.CommercialCoreService) {
	if commercialSvc == nil {
		return
	}
	mux.HandleFunc("GET /ui/data/commercial/receivables/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("commercial receivables are not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, commercialSvc.ReceivablesSummaryScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			time.Now().UTC(),
		))
	})
	mux.HandleFunc("GET /ui/data/commercial/parties/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"party.read", "document.list"}) {
			respondError(w, shared.Forbidden("party commercial summary is not allowed"))
			return
		}
		const prefix = "/ui/data/commercial/parties/"
		path := r.URL.Path
		if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/summary") {
			http.NotFound(w, r)
			return
		}
		partyID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/summary")
		partyID = strings.TrimSpace(strings.Trim(partyID, "/"))
		if partyID == "" {
			respondError(w, shared.NotFound("party not found"))
			return
		}
		respondJSON(w, http.StatusOK, commercialSvc.PartyCommercialSummaryScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			partyID,
			strings.TrimSpace(r.URL.Query().Get("from")),
			strings.TrimSpace(r.URL.Query().Get("to")),
		))
	})
}
