package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUIFinanceRoutes(mux *http.ServeMux, ident *identity.Service, financeSvc *application.FinanceReportingCoreService, reconciliationSvc *application.FinanceReconciliationCoreService, periodEndSvc *application.FinancePeriodEndCoreService, manualJournalSvc *application.FinanceManualJournalCoreService, collectionsSvc *application.FinanceCollectionsCoreService, inventoryFinanceSvc *application.InventoryFinanceCoreService, retailFinanceSvc *application.RetailFinanceCoreService) {
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
	if reconciliationSvc != nil {
		mux.HandleFunc("GET /ui/data/finance/ar-aging", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "document.read"}) {
				respondError(w, shared.Forbidden("ar aging is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, reconciliationSvc.ARAging(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("party_id")),
				strings.TrimSpace(query.Get("aging_bucket")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/ap-aging", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "document.read"}) {
				respondError(w, shared.Forbidden("ap aging is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, reconciliationSvc.APAging(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("vendor_id")),
				strings.TrimSpace(query.Get("aging_bucket")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/ar-reconciliation", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "document.read"}) {
				respondError(w, shared.Forbidden("ar reconciliation is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, reconciliationSvc.ARReconciliation(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("party_id")),
				strings.TrimSpace(query.Get("account_code")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/ap-reconciliation", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "document.read"}) {
				respondError(w, shared.Forbidden("ap reconciliation is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, reconciliationSvc.APReconciliation(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("vendor_id")),
				strings.TrimSpace(query.Get("account_code")),
			))
		})
	}
	if collectionsSvc != nil {
		mux.HandleFunc("GET /ui/data/finance/ar-statements", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "document.read"}) {
				respondError(w, shared.Forbidden("ar statements are not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, collectionsSvc.ARStatement(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("party_id")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/ap-statements", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "document.read"}) {
				respondError(w, shared.Forbidden("ap statements are not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, collectionsSvc.APStatement(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("vendor_id")),
			))
		})
		mux.HandleFunc("POST /ui/data/finance/ar-statements/generate", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.collections.manage", "party_statement_run.create"}) {
				respondError(w, shared.Forbidden("statement generation is not allowed"))
				return
			}
			var req struct {
				PartyID  string `json:"party_id"`
				AsOfDate string `json:"as_of_date"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			record, err := collectionsSvc.GenerateARStatementRun(organizationIDForPrincipal(p), p.currentLocationID, strings.TrimSpace(req.PartyID), strings.TrimSpace(req.AsOfDate), principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, record)
		})
		mux.HandleFunc("POST /ui/data/finance/ap-statements/generate", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.collections.manage", "vendor_statement_run.create"}) {
				respondError(w, shared.Forbidden("statement generation is not allowed"))
				return
			}
			var req struct {
				VendorID string `json:"vendor_id"`
				AsOfDate string `json:"as_of_date"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			record, err := collectionsSvc.GenerateAPStatementRun(organizationIDForPrincipal(p), p.currentLocationID, strings.TrimSpace(req.VendorID), strings.TrimSpace(req.AsOfDate), principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, record)
		})
		mux.HandleFunc("GET /ui/data/finance/collections", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "collection_case.read"}) {
				respondError(w, shared.Forbidden("collections are not allowed"))
				return
			}
			query := r.URL.Query()
			items, err := collectionsSvc.CollectionCases(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("kind")),
				strings.TrimSpace(query.Get("status")),
			)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"items": items})
		})
		mux.HandleFunc("GET /ui/data/finance/settlement-exceptions", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "settlement_exception.read"}) {
				respondError(w, shared.Forbidden("settlement exceptions are not allowed"))
				return
			}
			query := r.URL.Query()
			report, err := collectionsSvc.SettlementExceptionRecords(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("kind")),
			)
			if err != nil {
				respondError(w, err)
				return
			}
			if len(report.Items) == 0 {
				report = collectionsSvc.SettlementExceptions(
					organizationIDForPrincipal(p),
					p.currentLocationID,
					strings.TrimSpace(query.Get("as_of_date")),
					strings.TrimSpace(query.Get("kind")),
				)
			}
			respondJSON(w, http.StatusOK, report)
		})
		mux.HandleFunc("POST /ui/data/finance/settlement-exceptions/sync", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.collections.manage", "settlement_exception.create", "settlement_exception.update"}) {
				respondError(w, shared.Forbidden("settlement exception sync is not allowed"))
				return
			}
			var req struct {
				AsOfDate string `json:"as_of_date"`
				Kind     string `json:"kind"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			report, err := collectionsSvc.SyncSettlementExceptions(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(req.AsOfDate),
				strings.TrimSpace(req.Kind),
				principalEffectiveUserID(p),
			)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, report)
		})
		mux.HandleFunc("POST /ui/data/finance/settlement-exceptions/", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/ui/data/finance/settlement-exceptions/")
			switch {
			case strings.HasSuffix(path, "/open-case"):
				if !principalAllowsAll(ident, p, []string{"finance.collections.manage", "collection_case.create", "settlement_exception.update"}) {
					respondError(w, shared.Forbidden("open collection case is not allowed"))
					return
				}
				exceptionID := strings.TrimSuffix(path, "/open-case")
				record, err := collectionsSvc.OpenCollectionCaseFromException(exceptionID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusCreated, record)
				return
			case strings.HasSuffix(path, "/apply"):
				if !principalAllowsAll(ident, p, []string{"finance.collections.manage", "settlement_exception.update", "document.update_draft"}) {
					respondError(w, shared.Forbidden("apply settlement exception is not allowed"))
					return
				}
				var req struct {
					TargetDocumentID string  `json:"target_document_id"`
					Amount           float64 `json:"amount"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				exceptionID := strings.TrimSuffix(path, "/apply")
				record, err := collectionsSvc.ApplySettlementException(exceptionID, strings.TrimSpace(req.TargetDocumentID), req.Amount, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, record)
				return
			case strings.HasSuffix(path, "/write-off"):
				if !principalAllowsAll(ident, p, []string{"finance.writeoff", "settlement_exception.update", "document.create"}) {
					respondError(w, shared.Forbidden("write-off is not allowed"))
					return
				}
				var req struct {
					PostingDate string  `json:"posting_date"`
					Amount      float64 `json:"amount"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				exceptionID := strings.TrimSuffix(path, "/write-off")
				record, err := collectionsSvc.WriteOffSettlementException(exceptionID, strings.TrimSpace(req.PostingDate), req.Amount, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusCreated, record)
				return
			default:
				http.NotFound(w, r)
				return
			}
		})
		mux.HandleFunc("POST /ui/data/finance/collections/", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.collections.manage", "collection_case.update"}) {
				respondError(w, shared.Forbidden("collection case refresh is not allowed"))
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/ui/data/finance/collections/")
			if !strings.HasSuffix(path, "/refresh") {
				http.NotFound(w, r)
				return
			}
			caseID := strings.TrimSuffix(path, "/refresh")
			record, err := collectionsSvc.RefreshCollectionCase(caseID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, record)
		})
	}
	if inventoryFinanceSvc != nil {
		mux.HandleFunc("GET /ui/data/finance/inventory-valuation", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read"}) {
				respondError(w, shared.Forbidden("inventory valuation is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, inventoryFinanceSvc.InventoryValuation(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("warehouse_code")),
				"",
			))
		})
		mux.HandleFunc("GET /ui/data/finance/inventory-valuation-as-of", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read"}) {
				respondError(w, shared.Forbidden("inventory valuation as of is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, inventoryFinanceSvc.InventoryValuation(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("warehouse_code")),
				strings.TrimSpace(query.Get("as_of_date")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/inventory-gl-reconciliation", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read"}) {
				respondError(w, shared.Forbidden("inventory GL reconciliation is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, inventoryFinanceSvc.InventoryGLReconciliation(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("account_code")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/inventory-adjustment-review", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"inventory.finance.review"}) {
				respondError(w, shared.Forbidden("inventory adjustment review is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, inventoryFinanceSvc.InventoryAdjustmentReview(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("status")),
			))
		})
		mux.HandleFunc("POST /ui/data/inventory/count-sessions/", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"inventory_count_session.read", "inventory_count_session.update", "document.create"}) {
				respondError(w, shared.Forbidden("count session adjustment generation is not allowed"))
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/ui/data/inventory/count-sessions/")
			if !strings.HasSuffix(path, "/generate-adjustment") {
				http.NotFound(w, r)
				return
			}
			sessionID := strings.TrimSuffix(path, "/generate-adjustment")
			record, err := inventoryFinanceSvc.GenerateAdjustmentFromCountSession(sessionID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, record)
		})
		mux.HandleFunc("POST /ui/data/finance/inventory-reconciliation-cases/open", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"inventory_reconciliation_case.create", "inventory.finance.review"}) {
				respondError(w, shared.Forbidden("inventory reconciliation case creation is not allowed"))
				return
			}
			var req struct {
				AsOfDate       string  `json:"as_of_date"`
				AccountCode    string  `json:"account_code"`
				Reason         string  `json:"reason"`
				InventoryValue float64 `json:"inventory_value"`
				GLValue        float64 `json:"gl_value"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			record, err := inventoryFinanceSvc.OpenReconciliationCase(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(req.AsOfDate),
				strings.TrimSpace(req.AccountCode),
				strings.TrimSpace(req.Reason),
				req.InventoryValue,
				req.GLValue,
				principalEffectiveUserID(p),
			)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, record)
		})
	}
	if periodEndSvc != nil {
		mux.HandleFunc("GET /ui/data/finance/periods/", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read", "accounting_period.read"}) {
				respondError(w, shared.Forbidden("period close pack is not allowed"))
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/ui/data/finance/periods/")
			if !strings.HasSuffix(path, "/close-pack") {
				http.NotFound(w, r)
				return
			}
			periodID := strings.TrimSuffix(path, "/close-pack")
			pack, err := periodEndSvc.ReadClosePack(periodID, organizationIDForPrincipal(p), p.currentLocationID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, pack)
		})
		mux.HandleFunc("POST /ui/data/finance/periods/", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/ui/data/finance/periods/")
			switch {
			case strings.HasSuffix(path, "/generate-journals"):
				if !principalAllowsAll(ident, p, []string{
					"finance.period_end.manage",
					"accounting_period.read",
					"document.create",
					"journal_run.create",
					"journal_run.update",
					"accounting_period_task.create",
					"accounting_period_task.update",
				}) {
					respondError(w, shared.Forbidden("period-end journal generation is not allowed"))
					return
				}
				periodID := strings.TrimSuffix(path, "/generate-journals")
				pack, err := periodEndSvc.GenerateJournalRuns(periodID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, pack)
				return
			case strings.HasSuffix(path, "/close"):
				if !principalAllowsAll(ident, p, []string{"finance.close", "accounting_period.update"}) {
					respondError(w, shared.Forbidden("accounting period transition is not allowed"))
					return
				}
				periodID := strings.TrimSuffix(path, "/close")
				record, err := periodEndSvc.CloseAccountingPeriod(periodID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, record)
				return
			case strings.HasSuffix(path, "/reopen"):
				if !principalAllowsAll(ident, p, []string{"finance.close", "accounting_period.update"}) {
					respondError(w, shared.Forbidden("accounting period transition is not allowed"))
					return
				}
				periodID := strings.TrimSuffix(path, "/reopen")
				record, err := periodEndSvc.ReopenAccountingPeriod(periodID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, record)
				return
			default:
				http.NotFound(w, r)
				return
			}
		})
		mux.HandleFunc("POST /ui/data/finance/period-tasks/", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.period_end.manage", "accounting_period_task.update"}) {
				respondError(w, shared.Forbidden("period close task update is not allowed"))
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/ui/data/finance/period-tasks/")
			switch {
			case strings.HasSuffix(path, "/complete"):
				taskID := strings.TrimSuffix(path, "/complete")
				record, err := periodEndSvc.CompleteTask(taskID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, record)
			case strings.HasSuffix(path, "/waive"):
				taskID := strings.TrimSuffix(path, "/waive")
				record, err := periodEndSvc.WaiveTask(taskID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, record)
			default:
				http.NotFound(w, r)
			}
		})
		mux.HandleFunc("POST /ui/data/finance/journals/", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/ui/data/finance/journals/")
			switch {
			case strings.HasSuffix(path, "/reverse"):
				if !principalAllowsAll(ident, p, []string{"finance.reverse", "document.create"}) {
					respondError(w, shared.Forbidden("journal reversal is not allowed"))
					return
				}
				var req struct {
					ReversalDate string `json:"reversal_date"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				postingID := strings.TrimSuffix(path, "/reverse")
				record, err := periodEndSvc.ReverseJournalPosting(postingID, strings.TrimSpace(req.ReversalDate), principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusCreated, record)
			case strings.HasSuffix(path, "/correction"):
				if manualJournalSvc == nil {
					http.NotFound(w, r)
					return
				}
				if !principalAllowsAll(ident, p, []string{"finance.journal.read", "finance.journal.create", "document.create"}) {
					respondError(w, shared.Forbidden("journal correction is not allowed"))
					return
				}
				postingID := strings.TrimSuffix(path, "/correction")
				record, err := manualJournalSvc.CreateCorrectionJournal(postingID, principalEffectiveUserID(p), organizationIDForPrincipal(p), p.currentLocationID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusCreated, record)
			default:
				http.NotFound(w, r)
			}
		})
	}
	if periodEndSvc == nil {
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
	if retailFinanceSvc != nil {
		mux.HandleFunc("GET /ui/data/finance/pos-shift-reconciliation", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read"}) {
				respondError(w, shared.Forbidden("pos shift reconciliation is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, retailFinanceSvc.ShiftReconciliationReport(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("store_code")),
				strings.TrimSpace(query.Get("register_code")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/pos-tender-settlements", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read"}) {
				respondError(w, shared.Forbidden("pos tender settlements are not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, retailFinanceSvc.TenderSettlementReport(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				strings.TrimSpace(query.Get("as_of_date")),
				strings.TrimSpace(query.Get("store_code")),
				strings.TrimSpace(query.Get("register_code")),
				strings.TrimSpace(query.Get("status")),
			))
		})
		mux.HandleFunc("GET /ui/data/finance/cash-over-short", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"finance.read"}) {
				respondError(w, shared.Forbidden("cash over short report is not allowed"))
				return
			}
			query := r.URL.Query()
			respondJSON(w, http.StatusOK, retailFinanceSvc.CashOverShortReport(
				organizationIDForPrincipal(p),
				p.currentLocationID,
				firstNonEmptyString(strings.TrimSpace(query.Get("as_of_date")), strings.TrimSpace(query.Get("to_date"))),
				strings.TrimSpace(query.Get("store_code")),
				strings.TrimSpace(query.Get("register_code")),
			))
		})
		mux.HandleFunc("POST /ui/data/finance/pos-shift-reconciliation/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/approve") {
				http.NotFound(w, r)
				return
			}
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"retail.finance.manage", "pos_tender_reconciliation.update", "document.create", "document.submit", "document.approve"}) {
				respondError(w, shared.Forbidden("pos shift reconciliation approval is not allowed"))
				return
			}
			reconciliationID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/data/finance/pos-shift-reconciliation/"), "/approve")
			record, err := retailFinanceSvc.ApproveShiftReconciliation(reconciliationID, principalEffectiveUserID(p))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"record": record})
		})
		mux.HandleFunc("POST /ui/data/finance/pos-tender-settlements/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/settle") {
				http.NotFound(w, r)
				return
			}
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"retail.finance.manage", "pos_tender_settlement.update"}) {
				respondError(w, shared.Forbidden("pos tender settlement is not allowed"))
				return
			}
			settlementID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/data/finance/pos-tender-settlements/"), "/settle")
			var req struct {
				SettledAmount       float64 `json:"settled_amount"`
				SettlementReference string  `json:"settlement_reference"`
				SettlementDate      string  `json:"settlement_date"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid request body"))
				return
			}
			record, err := retailFinanceSvc.SettleTenderSettlement(settlementID, principalEffectiveUserID(p), req.SettledAmount, strings.TrimSpace(req.SettlementDate), strings.TrimSpace(req.SettlementReference), "")
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"record": record})
		})
		mux.HandleFunc("POST /ui/data/finance/gift-cards/issue", func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			if !principalAllowsAll(ident, p, []string{"retail.finance.manage", "gift_card.create", "gift_card_transaction.create", "document.create", "document.submit", "document.approve"}) {
				respondError(w, shared.Forbidden("gift card issue is not allowed"))
				return
			}
			var req struct {
				Code               string  `json:"code"`
				PartyID            string  `json:"party_id"`
				StoreCode          string  `json:"store_code"`
				OriginalAmount     float64 `json:"original_amount"`
				ExpiryDate         string  `json:"expiry_date"`
				PaymentAccountCode string  `json:"payment_account_code"`
				Notes              string  `json:"notes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid request body"))
				return
			}
			record, err := retailFinanceSvc.IssueGiftCard(organizationIDForPrincipal(p), p.currentLocationID, principalEffectiveUserID(p), map[string]any{
				"code":                 strings.TrimSpace(req.Code),
				"party_id":             strings.TrimSpace(req.PartyID),
				"store_code":           strings.TrimSpace(req.StoreCode),
				"amount":               req.OriginalAmount,
				"original_amount":      req.OriginalAmount,
				"remaining_balance":    req.OriginalAmount,
				"expiry_date":          strings.TrimSpace(req.ExpiryDate),
				"payment_account_code": strings.TrimSpace(req.PaymentAccountCode),
				"notes":                strings.TrimSpace(req.Notes),
			})
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, map[string]any{"record": record})
		})
	}
}
