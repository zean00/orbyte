package httpx

import (
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUIFinanceRoutes(mux *http.ServeMux, ident *identity.Service, financeSvc *application.FinanceReportingCoreService) {
	if financeSvc == nil {
		return
	}
	mux.HandleFunc("GET /ui/data/finance/trial-balance", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"finance.read"}) {
			respondError(w, shared.Forbidden("trial balance is not allowed"))
			return
		}
		query := r.URL.Query()
		respondJSON(w, http.StatusOK, financeSvc.TrialBalance(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			strings.TrimSpace(query.Get("from_date")),
			strings.TrimSpace(query.Get("to_date")),
		))
	})
	mux.HandleFunc("GET /ui/data/finance/profit-and-loss", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"finance.read"}) {
			respondError(w, shared.Forbidden("profit and loss is not allowed"))
			return
		}
		query := r.URL.Query()
		respondJSON(w, http.StatusOK, financeSvc.ProfitAndLoss(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			strings.TrimSpace(query.Get("from_date")),
			strings.TrimSpace(query.Get("to_date")),
		))
	})
	mux.HandleFunc("GET /ui/data/finance/balance-sheet", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"finance.read"}) {
			respondError(w, shared.Forbidden("balance sheet is not allowed"))
			return
		}
		asOfDate := strings.TrimSpace(r.URL.Query().Get("as_of_date"))
		if asOfDate == "" {
			asOfDate = time.Now().UTC().Format("2006-01-02")
		}
		respondJSON(w, http.StatusOK, financeSvc.BalanceSheet(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			asOfDate,
		))
	})
	mux.HandleFunc("GET /ui/data/finance/tax-summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"finance.read"}) {
			respondError(w, shared.Forbidden("tax summary is not allowed"))
			return
		}
		query := r.URL.Query()
		respondJSON(w, http.StatusOK, financeSvc.TaxSummary(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			strings.TrimSpace(query.Get("from_date")),
			strings.TrimSpace(query.Get("to_date")),
		))
	})
	mux.HandleFunc("GET /ui/data/finance/journal-ledger", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"finance.read", "document.read"}) {
			respondError(w, shared.Forbidden("journal ledger is not allowed"))
			return
		}
		query := r.URL.Query()
		respondJSON(w, http.StatusOK, financeSvc.JournalLedger(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			strings.TrimSpace(query.Get("from_date")),
			strings.TrimSpace(query.Get("to_date")),
		))
	})
	mux.HandleFunc("POST /ui/data/finance/periods/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"finance.close", "accounting_period.update"}) {
			respondError(w, shared.Forbidden("accounting period transition is not allowed"))
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/ui/data/finance/periods/")
		switch {
		case strings.HasSuffix(path, "/close"):
			periodID := strings.TrimSuffix(path, "/close")
			record, err := financeSvc.CloseAccountingPeriod(periodID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, record)
		case strings.HasSuffix(path, "/reopen"):
			periodID := strings.TrimSuffix(path, "/reopen")
			record, err := financeSvc.ReopenAccountingPeriod(periodID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, record)
		default:
			http.NotFound(w, r)
		}
	})
}
