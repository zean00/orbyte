package httpx

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerUIPlanningRoutes(mux *http.ServeMux, ident *identity.Service, planningSvc *application.PlanningCoreService) {
	if planningSvc == nil {
		return
	}
	mux.HandleFunc("GET /ui/data/planning/runs", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"planning_run.list"}) {
			respondError(w, shared.Forbidden("planning runs are not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, planningSvc.PlanningRunsSummaryScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
		))
	})

	mux.HandleFunc("POST /ui/data/planning/runs", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"planning_run.create", "document.list"}) {
			respondError(w, shared.Forbidden("planning run creation is not allowed"))
			return
		}
		var req struct {
			WarehouseCode         string `json:"warehouse_code"`
			ItemCode              string `json:"item_code"`
			CategoryCode          string `json:"category_code"`
			CoverageStatus        string `json:"coverage_status"`
			ShortageOnly          bool   `json:"shortage_only"`
			HasInboundOnly        bool   `json:"has_inbound"`
			HasPreferredVendorOnly bool  `json:"has_preferred_vendor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		record, err := planningSvc.CreatePlanningRun(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			principalEffectiveUserID(p),
			strings.TrimSpace(req.WarehouseCode),
			strings.TrimSpace(req.ItemCode),
			strings.TrimSpace(req.CategoryCode),
			strings.TrimSpace(req.CoverageStatus),
			req.ShortageOnly,
			req.HasInboundOnly,
			req.HasPreferredVendorOnly,
			time.Now().UTC(),
		)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{
			"run_id":  record.ID,
			"status":  record.Values["status"],
			"created": true,
		})
	})

	mux.HandleFunc("GET /ui/data/planning/runs/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/proposals") {
			http.NotFound(w, r)
			return
		}
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"planning_run.read"}) {
			respondError(w, shared.Forbidden("planning proposals are not allowed"))
			return
		}
		runID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ui/data/planning/runs/"), "/proposals")
		summary, err := planningSvc.PlanningRunProposalsScoped(runID, organizationIDForPrincipal(p), p.currentLocationID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("GET /ui/data/planning/replenishment/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("planning summary is not allowed"))
			return
		}
		query := r.URL.Query()
		summary := planningSvc.ReplenishmentSummaryScoped(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			strings.TrimSpace(query.Get("warehouse_code")),
			strings.TrimSpace(query.Get("item_code")),
			strings.TrimSpace(query.Get("category_code")),
			strings.TrimSpace(query.Get("coverage_status")),
			query.Get("shortage_only") == "1" || strings.EqualFold(query.Get("shortage_only"), "true"),
			query.Get("has_inbound") == "1" || strings.EqualFold(query.Get("has_inbound"), "true"),
			query.Get("has_preferred_vendor") == "1" || strings.EqualFold(query.Get("has_preferred_vendor"), "true"),
			time.Now().UTC(),
		)
		respondJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("POST /ui/data/planning/replenishment/generate-purchase-request", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.create", "document.list"}) {
			respondError(w, shared.Forbidden("purchase request generation is not allowed"))
			return
		}
		var req struct {
			ItemCode      string                               `json:"item_code"`
			WarehouseCode string                               `json:"warehouse_code"`
			Quantity      float64                              `json:"quantity"`
			Items         []application.ReplenishmentSelection `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		selections := req.Items
		if len(selections) == 0 && strings.TrimSpace(req.ItemCode) != "" && strings.TrimSpace(req.WarehouseCode) != "" && req.Quantity > 0 {
			selections = []application.ReplenishmentSelection{{
				ItemCode:      req.ItemCode,
				WarehouseCode: req.WarehouseCode,
				Quantity:      req.Quantity,
			}}
		}
		if len(selections) == 0 {
			respondError(w, shared.Validation("at least one replenishment selection is required"))
			return
		}
		records, err := planningSvc.GeneratePurchaseRequests(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			principalEffectiveUserID(p),
			selections,
		)
		if err != nil {
			respondError(w, err)
			return
		}
		results := planningSvc.GenerationResults(records)
		if len(results) == 1 {
			respondJSON(w, http.StatusCreated, map[string]any{
				"record_id":      results[0].RecordID,
				"document_type":  results[0].DocumentType,
				"warehouse_code": results[0].WarehouseCode,
				"quantity":       strconv.FormatFloat(selections[0].Quantity, 'f', -1, 64),
				"records":        results,
			})
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{
			"record_count": len(results),
			"records":      results,
		})
	})

	mux.HandleFunc("POST /ui/data/planning/proposals/convert-purchase-request", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"planning_proposal.update", "document.create", "document.list"}) {
			respondError(w, shared.Forbidden("proposal conversion is not allowed"))
			return
		}
		var req application.PlanningProposalSelection
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		records, err := planningSvc.ConvertPlanningProposals(
			organizationIDForPrincipal(p),
			p.currentLocationID,
			principalEffectiveUserID(p),
			req.ProposalIDs,
		)
		if err != nil {
			respondError(w, err)
			return
		}
		results := planningSvc.GenerationResults(records)
		respondJSON(w, http.StatusCreated, map[string]any{
			"record_count": len(results),
			"records":      results,
		})
	})
}
