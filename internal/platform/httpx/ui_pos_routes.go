package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUIPosRoutes(mux *http.ServeMux, ident *identity.Service, posSvc *application.POSCoreService) {
	if posSvc == nil {
		return
	}

	mux.HandleFunc("GET /ui/data/pos/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create"}) {
			respondError(w, shared.Forbidden("pos terminal is not allowed"))
			return
		}
		payload, err := posSvc.Bootstrap(
			p.currentLocationID,
			strings.TrimSpace(r.URL.Query().Get("store_code")),
			strings.TrimSpace(r.URL.Query().Get("register_code")),
			principalEffectiveUserID(p),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /ui/data/pos/catalog/search", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create", "item.list"}) {
			respondError(w, shared.Forbidden("pos catalog search is not allowed"))
			return
		}
		items, err := posSvc.SearchCatalog(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			strings.TrimSpace(r.URL.Query().Get("store_code")),
			strings.TrimSpace(r.URL.Query().Get("q")),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /ui/data/pos/shifts/open", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_shift.create"}) {
			respondError(w, shared.Forbidden("pos shift open is not allowed"))
			return
		}
		var req struct {
			StoreCode     string  `json:"store_code"`
			RegisterCode  string  `json:"register_code"`
			OpeningCash   float64 `json:"opening_cash_amount"`
			Notes         string  `json:"notes"`
			CashierUserID string  `json:"cashier_user_id,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		cashierUserID := principalEffectiveUserID(p)
		record, err := posSvc.OpenShift(req.StoreCode, req.RegisterCode, cashierUserID, principalEffectiveUserID(p), req.OpeningCash, req.Notes)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{"record": record})
	})

	mux.HandleFunc("POST /ui/data/pos/shifts/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/close") {
			http.NotFound(w, r)
			return
		}
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_shift.update"}) {
			respondError(w, shared.Forbidden("pos shift close is not allowed"))
			return
		}
		shiftID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/data/pos/shifts/"), "/close")
		var req struct {
			ActualCash float64 `json:"actual_cash_amount"`
			Notes      string  `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		record, err := posSvc.CloseShift(shiftID, principalEffectiveUserID(p), req.ActualCash, req.Notes)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": record})
	})

	mux.HandleFunc("GET /ui/data/pos/sales/held", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.list"}) {
			respondError(w, shared.Forbidden("held sales are not allowed"))
			return
		}
		items, err := posSvc.HeldSales(
			principalEffectiveUserID(p),
			strings.TrimSpace(r.URL.Query().Get("register_code")),
			strings.TrimSpace(r.URL.Query().Get("shift_id")),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /ui/data/pos/sales/hold", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create"}) {
			respondError(w, shared.Forbidden("hold sale is not allowed"))
			return
		}
		var req application.POSHoldSaleInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		record, err := posSvc.HoldSale(req, principalEffectiveUserID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{"record": record})
	})

	mux.HandleFunc("POST /ui/data/pos/checkout", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create", "document.create", "document.submit", "document.approve"}) {
			respondError(w, shared.Forbidden("pos checkout is not allowed"))
			return
		}
		var req application.POSCheckoutInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		result, err := posSvc.Checkout(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			req,
			principalEffectiveUserID(p),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, result)
	})

	mux.HandleFunc("GET /ui/data/pos/transactions/search", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.list"}) {
			respondError(w, shared.Forbidden("pos transaction lookup is not allowed"))
			return
		}
		items, err := posSvc.TransactionLookup(
			strings.TrimSpace(r.URL.Query().Get("q")),
			principalEffectiveUserID(p),
			strings.TrimSpace(r.URL.Query().Get("store_code")),
			strings.TrimSpace(r.URL.Query().Get("register_code")),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /ui/data/pos/transactions/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		base := strings.TrimPrefix(r.URL.Path, "/ui/data/pos/transactions/")
		switch {
		case strings.HasSuffix(base, "/refund"):
			if !principalAllowsAll(ident, p, []string{"pos_sale.update", "document.create", "document.submit", "document.approve"}) {
				respondError(w, shared.Forbidden("pos refund is not allowed"))
				return
			}
			saleID := strings.TrimSuffix(base, "/refund")
			payload, err := posSvc.RefundSale(saleID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, payload)
		case strings.HasSuffix(base, "/exchange"):
			if !principalAllowsAll(ident, p, []string{"pos_sale.update", "document.create", "document.submit", "document.approve"}) {
				respondError(w, shared.Forbidden("pos exchange is not allowed"))
				return
			}
			saleID := strings.TrimSuffix(base, "/exchange")
			payload, err := posSvc.ExchangeSale(saleID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, payload)
		default:
			http.NotFound(w, r)
		}
	})
}
