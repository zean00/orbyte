package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUIInventoryRoutes(mux *http.ServeMux, ident *identity.Service, inventorySvc *application.InventoryCoreService, traceabilitySvc *application.TraceabilityCoreService) {
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

	mux.HandleFunc("POST /ui/data/inventory/batches/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/actions") {
			http.NotFound(w, r)
			return
		}
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"inventory_batch.update"}) {
			respondError(w, shared.Forbidden("batch update is not allowed"))
			return
		}
		batchID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/data/inventory/batches/"), "/actions")
		var req struct {
			Action          string `json:"action"`
			Reason          string `json:"reason"`
			Notes           string `json:"notes"`
			RecallReference string `json:"recall_reference"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		actorID := strings.TrimSpace(p.effectiveUserID)
		if actorID == "" {
			actorID = firstNonEmptyString(strings.TrimSpace(p.userID), strings.TrimSpace(p.serviceID), "system")
		}
		record, err := inventorySvc.ApplyBatchAction(batchID, req.Action, actorID, req.Reason, req.Notes, req.RecallReference, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": record})
	})

	mux.HandleFunc("GET /ui/data/inventory/batches/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/trace") || traceabilitySvc == nil {
			http.NotFound(w, r)
			return
		}
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"inventory_batch.read", "document.list", "traceability.read"}) {
			respondError(w, shared.Forbidden("batch trace is not allowed"))
			return
		}
		batchID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/data/inventory/batches/"), "/trace")
		trace, err := traceabilitySvc.BatchTrace(batchID, organizationIDForPrincipal(p), p.currentLocationID, time.Now().UTC())
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, trace)
	})
}
