package httpx

import (
	"encoding/json"
	"image/png"
	"net/http"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

const posTerminalMetadataKey = "pos_terminal"

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
		terminalContext, shift, err := loadValidPOSTerminalContext(ident, posSvc, p)
		if err != nil {
			respondError(w, err)
			return
		}
		storeCode := strings.TrimSpace(r.URL.Query().Get("store_code"))
		registerCode := strings.TrimSpace(r.URL.Query().Get("register_code"))
		if terminalContext != nil {
			if storeCode == "" {
				storeCode = terminalContext.StoreCode
			}
			if registerCode == "" {
				registerCode = terminalContext.RegisterCode
			}
		}
		payload, err := posSvc.Bootstrap(
			p.currentLocationID,
			storeCode,
			registerCode,
			principalEffectiveUserID(p),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		if user, found := ident.FindUser(principalEffectiveUserID(p)); found {
			payload.CurrentCashier = &application.POSCashierSummary{
				UserID:   user.ID,
				Username: user.Username,
				Name:     user.Username,
			}
		}
		if pinState, pinErr := ident.CashierPINState(principalEffectiveUserID(p)); pinErr == nil {
			if configured, ok := pinState["configured"].(bool); ok {
				payload.CashierPINConfigured = configured
			}
		}
		if terminalContext != nil {
			payload.TerminalContext = terminalContext
			if shift != nil {
				payload.OpenShift = shift
			}
		}
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /ui/data/pos/receipt/qr", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create"}) {
			respondError(w, shared.Forbidden("pos receipt qr is not allowed"))
			return
		}
		value := strings.TrimSpace(r.URL.Query().Get("value"))
		if value == "" {
			respondError(w, shared.Validation("receipt qr value is required"))
			return
		}
		if len(value) > 256 {
			respondError(w, shared.Validation("receipt qr value is too long"))
			return
		}
		code, err := qrcode.New(value, qrcode.Medium)
		if err != nil {
			respondError(w, shared.Validation("receipt qr value is invalid"))
			return
		}
		img := code.Image(144)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, no-store")
		_ = png.Encode(w, img)
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

	mux.HandleFunc("GET /ui/data/pos/customers/search", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create", "party.list"}) {
			respondError(w, shared.Forbidden("pos customer lookup is not allowed"))
			return
		}
		items, err := posSvc.SearchCustomers(strings.TrimSpace(r.URL.Query().Get("q")))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /ui/data/pos/promotions/validate", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create"}) {
			respondError(w, shared.Forbidden("pos promotion validation is not allowed"))
			return
		}
		var req struct {
			StoreCode      string                         `json:"store_code"`
			PartyID        string                         `json:"party_id,omitempty"`
			PartyName      string                         `json:"party_name,omitempty"`
			Lines          []application.POSCartLineInput `json:"lines"`
			PromotionCodes []string                       `json:"promotion_codes,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		result, err := posSvc.ValidatePromotionCodes(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			req.StoreCode,
			req.PartyID,
			req.PartyName,
			req.PromotionCodes,
			req.Lines,
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /ui/data/pos/terminal/enter", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create"}) {
			respondError(w, shared.Forbidden("pos terminal is not allowed"))
			return
		}
		var req struct {
			StoreCode    string  `json:"store_code"`
			RegisterCode string  `json:"register_code"`
			ShiftID      string  `json:"shift_id,omitempty"`
			PIN          string  `json:"pin"`
			OpeningCash  float64 `json:"opening_cash_amount"`
			Notes        string  `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		policy := config.NewService().AuthPolicy()
		limiter := newLoginRateLimiter(ident, policy.LoginRateLimitAttempts, policy.LoginRateLimitWindow)
		limiterKey := loginLimitKey(r, "cashier-pin:"+principalEffectiveUserID(p))
		if !limiter.Allow(limiterKey) {
			respondError(w, shared.Forbidden("cashier PIN rate limit exceeded"))
			return
		}
		if err := ident.VerifyCashierPIN(principalEffectiveUserID(p), req.PIN); err != nil {
			limiter.AddFailure(limiterKey)
			respondError(w, err)
			return
		}
		limiter.Reset(limiterKey)
		var (
			shift model.Record
			err   error
		)
		if strings.TrimSpace(req.ShiftID) != "" {
			record, resumeErr := posSvc.ResumeShift(req.StoreCode, req.RegisterCode, req.ShiftID, principalEffectiveUserID(p))
			err = resumeErr
			shift = record
		} else {
			record, openErr := posSvc.OpenShift(organizationIDForPrincipal(p), p.currentLocationID, req.StoreCode, req.RegisterCode, principalEffectiveUserID(p), principalEffectiveUserID(p), req.OpeningCash, req.Notes)
			err = openErr
			shift = record
		}
		if err != nil {
			respondError(w, err)
			return
		}
		context := &application.POSTerminalContext{
			CashierUserID: principalEffectiveUserID(p),
			StoreCode:     strings.TrimSpace(req.StoreCode),
			RegisterCode:  strings.TrimSpace(req.RegisterCode),
			ShiftID:       strings.TrimSpace(shift.ID),
			VerifiedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		if err := savePOSTerminalContext(ident, p.sessionID, context); err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"terminal_context": context,
			"shift":            shift,
		})
	})

	mux.HandleFunc("POST /ui/data/pos/terminal/lock", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if err := clearPOSTerminalContext(ident, p.sessionID); err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"status": "locked"})
	})

	mux.HandleFunc("GET /ui/data/pos/stored-value/lookup", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"pos_sale.create"}) {
			respondError(w, shared.Forbidden("pos stored value lookup is not allowed"))
			return
		}
		result, err := posSvc.LookupStoredValue(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			strings.TrimSpace(r.URL.Query().Get("kind")),
			strings.TrimSpace(r.URL.Query().Get("reference")),
			strings.TrimSpace(r.URL.Query().Get("party_id")),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"item": result})
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
		record, err := posSvc.OpenShift(organizationIDForPrincipal(p), p.currentLocationID, req.StoreCode, req.RegisterCode, cashierUserID, principalEffectiveUserID(p), req.OpeningCash, req.Notes)
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
		record, err := posSvc.CloseShift(organizationIDForPrincipal(p), p.currentLocationID, shiftID, principalEffectiveUserID(p), req.ActualCash, req.Notes)
		if err != nil {
			respondError(w, err)
			return
		}
		if context, _, contextErr := loadValidPOSTerminalContext(ident, posSvc, p); contextErr == nil && context != nil && strings.TrimSpace(context.ShiftID) == strings.TrimSpace(shiftID) {
			_ = clearPOSTerminalContext(ident, p.sessionID)
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
		context, _, err := requirePOSTerminalContext(ident, posSvc, p)
		if err != nil {
			respondError(w, err)
			return
		}
		items, err := posSvc.HeldSales(principalEffectiveUserID(p), context.RegisterCode, context.ShiftID)
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
		context, _, err := requirePOSTerminalContext(ident, posSvc, p)
		if err != nil {
			respondError(w, err)
			return
		}
		if mismatchPOSTerminalContext(context, req.StoreCode, req.RegisterCode, req.ShiftID) {
			respondError(w, shared.Validation("sale context does not match the active POS terminal"))
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
		context, _, err := requirePOSTerminalContext(ident, posSvc, p)
		if err != nil {
			respondError(w, err)
			return
		}
		if mismatchPOSTerminalContext(context, req.StoreCode, req.RegisterCode, req.ShiftID) {
			respondError(w, shared.Validation("sale context does not match the active POS terminal"))
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
		context, _, err := requirePOSTerminalContext(ident, posSvc, p)
		if err != nil {
			respondError(w, err)
			return
		}
		items, err := posSvc.TransactionLookup(strings.TrimSpace(r.URL.Query().Get("q")), principalEffectiveUserID(p), context.StoreCode, context.RegisterCode)
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
		if _, _, err := requirePOSTerminalContext(ident, posSvc, p); err != nil {
			respondError(w, err)
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
		case strings.HasSuffix(base, "/refund-store-credit"):
			if !principalAllowsAll(ident, p, []string{"pos_sale.update", "document.create", "document.submit", "document.approve", "store_credit_account.create", "store_credit_transaction.create"}) {
				respondError(w, shared.Forbidden("pos refund to store credit is not allowed"))
				return
			}
			saleID := strings.TrimSuffix(base, "/refund-store-credit")
			payload, err := posSvc.RefundSaleToStoreCredit(organizationIDForPrincipal(p), p.currentLocationID, saleID, principalEffectiveUserID(p))
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

func loadValidPOSTerminalContext(ident *identity.Service, posSvc *application.POSCoreService, p principal) (*application.POSTerminalContext, *model.Record, error) {
	if p.kind != userPrincipal || strings.TrimSpace(p.sessionID) == "" {
		return nil, nil, nil
	}
	session, ok := ident.FindSession(p.sessionID)
	if !ok {
		return nil, nil, nil
	}
	context := readPOSTerminalContext(session.ClientMetadata)
	if context == nil {
		return nil, nil, nil
	}
	if strings.TrimSpace(context.CashierUserID) != strings.TrimSpace(principalEffectiveUserID(p)) {
		_ = clearPOSTerminalContext(ident, p.sessionID)
		return nil, nil, nil
	}
	shift, err := posSvc.ValidateTerminalContext(context.StoreCode, context.RegisterCode, context.ShiftID, principalEffectiveUserID(p))
	if err != nil {
		_ = clearPOSTerminalContext(ident, p.sessionID)
		return nil, nil, nil
	}
	return context, &shift, nil
}

func requirePOSTerminalContext(ident *identity.Service, posSvc *application.POSCoreService, p principal) (*application.POSTerminalContext, *model.Record, error) {
	context, shift, err := loadValidPOSTerminalContext(ident, posSvc, p)
	if err != nil {
		return nil, nil, err
	}
	if context == nil {
		return nil, nil, shared.Forbidden("pos terminal is locked")
	}
	return context, shift, nil
}

func readPOSTerminalContext(metadata map[string]any) *application.POSTerminalContext {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[posTerminalMetadataKey]
	if !ok {
		return nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	context := &application.POSTerminalContext{
		CashierUserID: strings.TrimSpace(posStringValue(values["cashier_user_id"])),
		StoreCode:     strings.TrimSpace(posStringValue(values["store_code"])),
		RegisterCode:  strings.TrimSpace(posStringValue(values["register_code"])),
		ShiftID:       strings.TrimSpace(posStringValue(values["shift_id"])),
		VerifiedAt:    strings.TrimSpace(posStringValue(values["verified_at"])),
	}
	if context.CashierUserID == "" || context.StoreCode == "" || context.RegisterCode == "" || context.ShiftID == "" {
		return nil
	}
	return context
}

func savePOSTerminalContext(ident *identity.Service, sessionID string, context *application.POSTerminalContext) error {
	session, ok := ident.FindSession(strings.TrimSpace(sessionID))
	if !ok {
		return shared.NotFound("session not found")
	}
	if session.ClientMetadata == nil {
		session.ClientMetadata = map[string]any{}
	}
	session.ClientMetadata[posTerminalMetadataKey] = map[string]any{
		"cashier_user_id": strings.TrimSpace(context.CashierUserID),
		"store_code":      strings.TrimSpace(context.StoreCode),
		"register_code":   strings.TrimSpace(context.RegisterCode),
		"shift_id":        strings.TrimSpace(context.ShiftID),
		"verified_at":     strings.TrimSpace(context.VerifiedAt),
	}
	return ident.SaveSession(session)
}

func clearPOSTerminalContext(ident *identity.Service, sessionID string) error {
	session, ok := ident.FindSession(strings.TrimSpace(sessionID))
	if !ok {
		return shared.NotFound("session not found")
	}
	if session.ClientMetadata != nil {
		delete(session.ClientMetadata, posTerminalMetadataKey)
	}
	return ident.SaveSession(session)
}

func mismatchPOSTerminalContext(context *application.POSTerminalContext, storeCode, registerCode, shiftID string) bool {
	return strings.TrimSpace(context.StoreCode) != strings.TrimSpace(storeCode) ||
		strings.TrimSpace(context.RegisterCode) != strings.TrimSpace(registerCode) ||
		strings.TrimSpace(context.ShiftID) != strings.TrimSpace(shiftID)
}

func posStringValue(input any) string {
	if text, ok := input.(string); ok {
		return text
	}
	return ""
}
