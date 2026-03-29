package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type PlanningCoreService struct {
	documents   *document.Service
	models      *model.Service
	search      *search.Service
	inventory   *InventoryCoreService
	fulfillment *FulfillmentCoreService
	procurement *ProcurementCoreService
}

type ReplenishmentSummary struct {
	CandidateCount                int              `json:"candidate_count"`
	ShortageItemCount             int              `json:"shortage_item_count"`
	WarehouseCount                int              `json:"warehouse_count"`
	TotalShortageQuantity         float64          `json:"total_shortage_quantity"`
	TotalSuggestedRequestQuantity float64          `json:"total_suggested_request_quantity"`
	TotalForecastDemandQuantity   float64          `json:"total_forecast_demand_quantity"`
	DueSoonCount                  int              `json:"due_soon_count"`
	Items                         []map[string]any `json:"items"`
}

type ReplenishmentSelection struct {
	ItemCode      string  `json:"item_code"`
	WarehouseCode string  `json:"warehouse_code"`
	Quantity      float64 `json:"quantity"`
}

type ReplenishmentGenerationResult struct {
	RecordID      string `json:"record_id"`
	DocumentType  string `json:"document_type"`
	WarehouseCode string `json:"warehouse_code"`
}

type PlanningRunSummary struct {
	RunCount                     int              `json:"run_count"`
	OpenRunCount                 int              `json:"open_run_count"`
	ProposalCount                int              `json:"proposal_count"`
	ConvertedProposalCount       int              `json:"converted_proposal_count"`
	TotalForecastDemandQuantity  float64          `json:"total_forecast_demand_quantity"`
	ProjectedShortageItemCount   int              `json:"projected_shortage_item_count"`
	TimeCriticalProposalCount    int              `json:"time_critical_proposal_count"`
	Items                        []map[string]any `json:"items"`
}

type PlanningProposalSummary struct {
	Run                         map[string]any   `json:"run"`
	ProposalCount               int              `json:"proposal_count"`
	ConvertedProposalCount      int              `json:"converted_proposal_count"`
	TimeCriticalProposalCount   int              `json:"time_critical_proposal_count"`
	TotalForecastDemandQuantity float64          `json:"total_forecast_demand_quantity"`
	TotalNormalizedQuantity     float64          `json:"total_normalized_quantity"`
	Items                       []map[string]any `json:"items"`
}

type PlanningProposalSelection struct {
	ProposalIDs []string `json:"proposal_ids"`
}

type replenishmentPolicy struct {
	ItemCode      string
	Name          string
	UOMCode       string
	CategoryCode  string
	Enabled       bool
	Mode          string
	ReorderPoint  float64
	TargetStock   float64
	DefaultWHCode string
	BasePrice     float64
}

type replenishmentVendorProfile struct {
	VendorID          string
	VendorName        string
	VendorItemCode    string
	PurchaseUOMCode   string
	LeadTimeDays      float64
	Priority          float64
	MinimumOrderQty   float64
	PackSize          float64
	LastPurchasePrice float64
	Preferred         bool
	Status            string
}

type replenishmentReference struct {
	ID     string
	Number string
	Status string
}

type planningInboundEvent struct {
	Date time.Time
	Qty  float64
}

func NewPlanningCoreService(documents *document.Service, models *model.Service, searchSvc *search.Service, inventorySvc *InventoryCoreService, fulfillmentSvc *FulfillmentCoreService, procurementSvc *ProcurementCoreService) *PlanningCoreService {
	return &PlanningCoreService{
		documents:   documents,
		models:      models,
		search:      searchSvc,
		inventory:   inventorySvc,
		fulfillment: fulfillmentSvc,
		procurement: procurementSvc,
	}
}

func (s *PlanningCoreService) ReplenishmentSummaryScoped(organizationID, locationID, warehouseCode, itemCode, categoryCode, coverageStatus string, shortageOnly, hasInboundOnly, hasPreferredVendorOnly bool, now time.Time) ReplenishmentSummary {
	rows := s.replenishmentRowsScoped(organizationID, locationID, warehouseCode, itemCode, categoryCode, coverageStatus, shortageOnly, hasInboundOnly, hasPreferredVendorOnly, now)
	summary := ReplenishmentSummary{
		CandidateCount: len(rows),
		Items:          rows,
	}
	warehouseSeen := map[string]struct{}{}
	for _, row := range rows {
		warehouseSeen[textValue(row["warehouse_code"])] = struct{}{}
		shortage := roundMoney(numberValue(row["shortage_quantity"]))
		suggested := roundMoney(numberValue(row["suggested_request_quantity"]))
		if shortage > 0 {
			summary.ShortageItemCount++
		}
		summary.TotalShortageQuantity = roundMoney(summary.TotalShortageQuantity + shortage)
		summary.TotalSuggestedRequestQuantity = roundMoney(summary.TotalSuggestedRequestQuantity + suggested)
		summary.TotalForecastDemandQuantity = roundMoney(summary.TotalForecastDemandQuantity + roundMoney(numberValue(row["forecast_demand_quantity"])))
		if boolValue(row["time_critical"]) {
			summary.DueSoonCount++
		}
	}
	summary.WarehouseCount = len(warehouseSeen)
	return summary
}

func (s *PlanningCoreService) GeneratePurchaseRequest(organizationID, locationID, actorID string, selections []ReplenishmentSelection) (document.Record, error) {
	records, err := s.GeneratePurchaseRequests(organizationID, locationID, actorID, selections)
	if err != nil {
		return document.Record{}, err
	}
	if len(records) == 0 {
		return document.Record{}, shared.Validation("at least one replenishment line is required")
	}
	return records[0], nil
}

func (s *PlanningCoreService) GeneratePurchaseRequests(organizationID, locationID, actorID string, selections []ReplenishmentSelection) ([]document.Record, error) {
	if s.documents == nil {
		return nil, shared.Validation("planning is unavailable")
	}
	grouped := map[string][]ReplenishmentSelection{}
	currentRows := map[string]map[string]any{}
	for _, row := range s.replenishmentRowsScoped(organizationID, locationID, "", "", "", "", false, false, false, time.Now().UTC()) {
		currentRows[planningKey(textValue(row["item_code"]), textValue(row["warehouse_code"]))] = row
	}
	for _, selection := range selections {
		itemCode := strings.TrimSpace(selection.ItemCode)
		warehouseCode := strings.TrimSpace(selection.WarehouseCode)
		quantity := roundMoney(selection.Quantity)
		if itemCode == "" || warehouseCode == "" || quantity <= 0 {
			return nil, shared.Validation("item_code, warehouse_code, and quantity are required")
		}
		row := currentRows[planningKey(itemCode, warehouseCode)]
		if row == nil {
			return nil, shared.Validation(fmt.Sprintf("no replenishment shortage exists for %s in %s", itemCode, warehouseCode))
		}
		normalizedQty := roundMoney(numberValue(row["normalized_request_quantity"]))
		if normalizedQty <= 0 {
			return nil, shared.Validation(fmt.Sprintf("item %s in %s is already covered", itemCode, warehouseCode))
		}
		if quantity > normalizedQty {
			return nil, shared.Validation(fmt.Sprintf("item %s in %s exceeds uncovered replenishment quantity", itemCode, warehouseCode))
		}
		grouped[warehouseCode] = append(grouped[warehouseCode], ReplenishmentSelection{
			ItemCode:      itemCode,
			WarehouseCode: warehouseCode,
			Quantity:      quantity,
		})
	}
	if len(grouped) == 0 {
		return nil, shared.Validation("at least one replenishment line is required")
	}

	vendors := s.preferredVendorProfiles()
	warehouseCodes := make([]string, 0, len(grouped))
	for warehouseCode := range grouped {
		warehouseCodes = append(warehouseCodes, warehouseCode)
	}
	sort.Strings(warehouseCodes)

	now := time.Now().UTC()
	records := make([]document.Record, 0, len(warehouseCodes))
	for _, warehouseCode := range warehouseCodes {
		lines := make([]map[string]any, 0, len(grouped[warehouseCode]))
		vendorID := ""
		vendorName := ""
		singleVendor := true
		for _, selection := range grouped[warehouseCode] {
			policy := s.lookupReplenishmentPolicy(selection.ItemCode)
			if !policy.Enabled {
				return nil, shared.Validation(fmt.Sprintf("item %s is not replenishment-enabled", selection.ItemCode))
			}
			vendor := vendors[selection.ItemCode]
			normalizedQty, quantityRule := normalizePlanningQuantity(selection.Quantity, vendor.MinimumOrderQty, vendor.PackSize)
			lines = append(lines, map[string]any{
				"item_code":                    selection.ItemCode,
				"description":                  firstNonEmptyString(policy.Name, selection.ItemCode),
				"uom_code":                     firstNonEmptyString(vendor.PurchaseUOMCode, policy.UOMCode),
				"quantity":                     normalizedQty,
				"unit_price":                   firstPositivePlanningNumber(vendor.LastPurchasePrice, policy.BasePrice),
				"warehouse_code":               warehouseCode,
				"note":                         "Generated from replenishment planning",
				"preferred_vendor_id":          vendor.VendorID,
				"preferred_vendor_name":        vendor.VendorName,
				"vendor_item_code":             vendor.VendorItemCode,
				"minimum_order_quantity":       vendor.MinimumOrderQty,
				"pack_size":                    vendor.PackSize,
				"lead_time_days":               vendor.LeadTimeDays,
				"planning_generated":           true,
				"planning_suggested_quantity":  selection.Quantity,
				"planning_normalized_quantity": normalizedQty,
				"planning_quantity_rule":       quantityRule,
			})
			if vendor.VendorID == "" {
				singleVendor = false
				continue
			}
			if vendorID == "" {
				vendorID = vendor.VendorID
				vendorName = vendor.VendorName
				continue
			}
			if vendorID != vendor.VendorID {
				singleVendor = false
			}
		}
		payload := map[string]any{
			"request_date":                         now.Format("2006-01-02"),
			"currency_code":                        "IDR",
			"notes":                                fmt.Sprintf("Generated from replenishment planning for warehouse %s", warehouseCode),
			"planning_generated":                   true,
			"planning_source":                      "replenishment",
			"planning_warehouse_code":              warehouseCode,
			"planning_generated_at":                now.Format(time.RFC3339),
			"planning_generated_by_item":           len(lines),
			"needed_by_date":                       now.Format("2006-01-02"),
			"default_replenishment_warehouse_code": warehouseCode,
			"lines":                                lines,
		}
		if singleVendor && vendorID != "" {
			payload["vendor_id"] = vendorID
			payload["vendor_name"] = vendorName
		}
		if s.procurement != nil {
			payload = s.procurement.NormalizePayload("purchase_request", payload)
		}
		record, err := s.documents.Create("purchase_request", organizationID, locationID, actorID, payload)
		if err != nil {
			return nil, err
		}
		if s.search != nil {
			s.search.RefreshDocument(record)
		}
		created, err := s.documents.Get(record.Header.ID)
		if err != nil {
			return nil, err
		}
		records = append(records, created)
	}
	return records, nil
}

func (s *PlanningCoreService) GenerationResults(records []document.Record) []ReplenishmentGenerationResult {
	results := make([]ReplenishmentGenerationResult, 0, len(records))
	for _, record := range records {
		results = append(results, ReplenishmentGenerationResult{
			RecordID:      record.Header.ID,
			DocumentType:  record.Header.Type,
			WarehouseCode: textValue(record.Body.Payload["planning_warehouse_code"]),
		})
	}
	return results
}

func (s *PlanningCoreService) CreatePlanningRun(organizationID, locationID, actorID, warehouseCode, itemCode, categoryCode, coverageStatus string, shortageOnly, hasInboundOnly, hasPreferredVendorOnly bool, now time.Time) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.Validation("planning run storage is unavailable")
	}
	rows := s.replenishmentRowsScoped(organizationID, locationID, warehouseCode, itemCode, categoryCode, coverageStatus, shortageOnly, hasInboundOnly, hasPreferredVendorOnly, now)
	summary := ReplenishmentSummary{
		CandidateCount: len(rows),
		Items:          rows,
	}
	for _, row := range rows {
		shortage := roundMoney(numberValue(row["shortage_quantity"]))
		suggested := roundMoney(numberValue(row["normalized_request_quantity"]))
		if shortage > 0 {
			summary.ShortageItemCount++
		}
		if boolValue(row["time_critical"]) {
			summary.DueSoonCount++
		}
		summary.TotalShortageQuantity = roundMoney(summary.TotalShortageQuantity + shortage)
		summary.TotalSuggestedRequestQuantity = roundMoney(summary.TotalSuggestedRequestQuantity + suggested)
		summary.TotalForecastDemandQuantity = roundMoney(summary.TotalForecastDemandQuantity + roundMoney(numberValue(row["forecast_demand_quantity"])))
	}
	run, err := s.models.Create("planning_run", actorID, map[string]any{
		"organization_id":                  organizationID,
		"location_id":                      locationID,
		"run_date":                         now.Format("2006-01-02"),
		"warehouse_code":                   warehouseCode,
		"item_code":                        itemCode,
		"category_code":                    categoryCode,
		"coverage_status":                  coverageStatus,
		"shortage_only":                    shortageOnly,
		"has_inbound_only":                 hasInboundOnly,
		"has_preferred_vendor_only":        hasPreferredVendorOnly,
		"forecast_method":                  "seasonal_buckets",
		"forecast_window_days":             planningForecastWindowDays(),
		"seasonal_history_weeks":           planningForecastHistoryWeeks(),
		"proposal_count":                   len(rows),
		"projected_shortage_item_count":    summary.ShortageItemCount,
		"total_shortage_quantity":          summary.TotalShortageQuantity,
		"total_forecast_demand_quantity":   summary.TotalForecastDemandQuantity,
		"total_normalized_request_quantity": summary.TotalSuggestedRequestQuantity,
		"due_soon_count":                   summary.DueSoonCount,
		"status":                           "completed",
	})
	if err != nil {
		return model.Record{}, err
	}
	for _, row := range rows {
		values := cloneMap(row)
		values["planning_run_id"] = run.ID
		values["organization_id"] = organizationID
		values["location_id"] = locationID
		values["forecast_method"] = "seasonal_buckets"
		values["forecast_window_days"] = planningForecastWindowDays()
		values["conversion_status"] = "open"
		if roundMoney(numberValue(row["normalized_request_quantity"])) <= 0 {
			values["conversion_status"] = "covered"
		}
		if _, err := s.models.Create("planning_proposal", actorID, values); err != nil {
			return model.Record{}, err
		}
	}
	return run, nil
}

func (s *PlanningCoreService) PlanningRunsSummaryScoped(organizationID, locationID string) PlanningRunSummary {
	summary := PlanningRunSummary{}
	if s.models == nil {
		return summary
	}
	runs, _, err := s.models.List("planning_run", model.Query{Page: 1, PageSize: 500})
	if err != nil {
		return summary
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].UpdatedAt.After(runs[j].UpdatedAt)
	})
	for _, run := range runs {
		if !matchesPlanningModelScope(run, organizationID, locationID) {
			continue
		}
		item := map[string]any{
			"id":                              run.ID,
			"status":                          firstNonEmptyString(textValue(run.Values["status"]), "completed"),
			"run_date":                        textValue(run.Values["run_date"]),
			"warehouse_code":                  textValue(run.Values["warehouse_code"]),
			"item_code":                       textValue(run.Values["item_code"]),
			"category_code":                   textValue(run.Values["category_code"]),
			"proposal_count":                  numberValue(run.Values["proposal_count"]),
			"projected_shortage_item_count":   numberValue(run.Values["projected_shortage_item_count"]),
			"total_shortage_quantity":         numberValue(run.Values["total_shortage_quantity"]),
			"total_forecast_demand_quantity":  numberValue(run.Values["total_forecast_demand_quantity"]),
			"total_normalized_request_quantity": numberValue(run.Values["total_normalized_request_quantity"]),
			"due_soon_count":                  numberValue(run.Values["due_soon_count"]),
			"created_at":                      run.CreatedAt.Format(time.RFC3339),
			"updated_at":                      run.UpdatedAt.Format(time.RFC3339),
		}
		summary.Items = append(summary.Items, item)
		summary.RunCount++
		if !strings.EqualFold(textValue(run.Values["status"]), "converted") {
			summary.OpenRunCount++
		}
		summary.ProposalCount += int(numberValue(run.Values["proposal_count"]))
		summary.TotalForecastDemandQuantity = roundMoney(summary.TotalForecastDemandQuantity + numberValue(run.Values["total_forecast_demand_quantity"]))
		summary.ProjectedShortageItemCount += int(numberValue(run.Values["projected_shortage_item_count"]))
		summary.TimeCriticalProposalCount += int(numberValue(run.Values["due_soon_count"]))
	}
	proposals, _, err := s.models.List("planning_proposal", model.Query{Page: 1, PageSize: 2000})
	if err == nil {
		for _, proposal := range proposals {
			if !matchesPlanningModelScope(proposal, organizationID, locationID) {
				continue
			}
			if strings.EqualFold(textValue(proposal.Values["conversion_status"]), "converted") {
				summary.ConvertedProposalCount++
			}
		}
	}
	return summary
}

func (s *PlanningCoreService) PlanningRunProposalsScoped(runID, organizationID, locationID string) (PlanningProposalSummary, error) {
	summary := PlanningProposalSummary{}
	if s.models == nil {
		return summary, shared.Validation("planning proposal storage is unavailable")
	}
	run, err := s.models.Get("planning_run", runID)
	if err != nil {
		return summary, err
	}
	if !matchesPlanningModelScope(run, organizationID, locationID) {
		return summary, shared.Forbidden("planning run is not allowed")
	}
	summary.Run = map[string]any{
		"id":                              run.ID,
		"status":                          textValue(run.Values["status"]),
		"run_date":                        textValue(run.Values["run_date"]),
		"warehouse_code":                  textValue(run.Values["warehouse_code"]),
		"item_code":                       textValue(run.Values["item_code"]),
		"category_code":                   textValue(run.Values["category_code"]),
		"coverage_status":                 textValue(run.Values["coverage_status"]),
		"shortage_only":                   boolValue(run.Values["shortage_only"]),
		"has_inbound_only":                boolValue(run.Values["has_inbound_only"]),
		"has_preferred_vendor_only":       boolValue(run.Values["has_preferred_vendor_only"]),
		"forecast_method":                 textValue(run.Values["forecast_method"]),
		"forecast_window_days":            numberValue(run.Values["forecast_window_days"]),
		"seasonal_history_weeks":          numberValue(run.Values["seasonal_history_weeks"]),
		"proposal_count":                  numberValue(run.Values["proposal_count"]),
		"projected_shortage_item_count":   numberValue(run.Values["projected_shortage_item_count"]),
		"total_shortage_quantity":         numberValue(run.Values["total_shortage_quantity"]),
		"total_forecast_demand_quantity":  numberValue(run.Values["total_forecast_demand_quantity"]),
		"total_normalized_request_quantity": numberValue(run.Values["total_normalized_request_quantity"]),
		"due_soon_count":                  numberValue(run.Values["due_soon_count"]),
	}
	proposals, _, err := s.models.List("planning_proposal", model.Query{Page: 1, PageSize: 2000, Filters: map[string]string{"planning_run_id": runID}})
	if err != nil {
		return summary, err
	}
	sort.Slice(proposals, func(i, j int) bool {
		left := roundMoney(numberValue(proposals[i].Values["normalized_request_quantity"]))
		right := roundMoney(numberValue(proposals[j].Values["normalized_request_quantity"]))
		if left != right {
			return left > right
		}
		leftDate := textValue(proposals[i].Values["projected_shortage_date"])
		rightDate := textValue(proposals[j].Values["projected_shortage_date"])
		if leftDate != rightDate {
			return leftDate < rightDate
		}
		return textValue(proposals[i].Values["item_code"]) < textValue(proposals[j].Values["item_code"])
	})
	for _, proposal := range proposals {
		if !matchesPlanningModelScope(proposal, organizationID, locationID) {
			continue
		}
		item := cloneMap(proposal.Values)
		item["id"] = proposal.ID
		item["created_at"] = proposal.CreatedAt.Format(time.RFC3339)
		item["updated_at"] = proposal.UpdatedAt.Format(time.RFC3339)
		summary.Items = append(summary.Items, item)
		summary.ProposalCount++
		summary.TotalForecastDemandQuantity = roundMoney(summary.TotalForecastDemandQuantity + numberValue(proposal.Values["forecast_demand_quantity"]))
		summary.TotalNormalizedQuantity = roundMoney(summary.TotalNormalizedQuantity + numberValue(proposal.Values["normalized_request_quantity"]))
		if boolValue(proposal.Values["time_critical"]) {
			summary.TimeCriticalProposalCount++
		}
		if strings.EqualFold(textValue(proposal.Values["conversion_status"]), "converted") {
			summary.ConvertedProposalCount++
		}
	}
	return summary, nil
}

func (s *PlanningCoreService) ConvertPlanningProposals(organizationID, locationID, actorID string, proposalIDs []string) ([]document.Record, error) {
	if s.models == nil {
		return nil, shared.Validation("planning proposal storage is unavailable")
	}
	if len(proposalIDs) == 0 {
		return nil, shared.Validation("at least one planning proposal is required")
	}
	selections := make([]ReplenishmentSelection, 0, len(proposalIDs))
	proposals := make([]model.Record, 0, len(proposalIDs))
	runIDs := map[string]struct{}{}
	for _, proposalID := range proposalIDs {
		proposalID = strings.TrimSpace(proposalID)
		if proposalID == "" {
			continue
		}
		record, err := s.models.Get("planning_proposal", proposalID)
		if err != nil {
			return nil, err
		}
		if !matchesPlanningModelScope(record, organizationID, locationID) {
			return nil, shared.Forbidden("planning proposal is not allowed")
		}
		if strings.EqualFold(textValue(record.Values["conversion_status"]), "converted") {
			return nil, shared.Validation(fmt.Sprintf("planning proposal %s is already converted", proposalID))
		}
		quantity := roundMoney(numberValue(record.Values["normalized_request_quantity"]))
		if quantity <= 0 {
			return nil, shared.Validation(fmt.Sprintf("planning proposal %s is already covered", proposalID))
		}
		selections = append(selections, ReplenishmentSelection{
			ItemCode:      textValue(record.Values["item_code"]),
			WarehouseCode: textValue(record.Values["warehouse_code"]),
			Quantity:      quantity,
		})
		proposals = append(proposals, record)
		runIDs[textValue(record.Values["planning_run_id"])] = struct{}{}
	}
	if len(selections) == 0 {
		return nil, shared.Validation("at least one planning proposal is required")
	}
	records, err := s.GeneratePurchaseRequests(organizationID, locationID, actorID, selections)
	if err != nil {
		return nil, err
	}
	resultsByWarehouse := map[string][]replenishmentReference{}
	for _, record := range records {
		ref := replenishmentReference{ID: record.Header.ID, Number: record.Header.Number, Status: record.Header.Status}
		resultsByWarehouse[textValue(record.Body.Payload["planning_warehouse_code"])] = append(resultsByWarehouse[textValue(record.Body.Payload["planning_warehouse_code"])], ref)
	}
	for _, proposal := range proposals {
		values := cloneMap(proposal.Values)
		values["conversion_status"] = "converted"
		values["purchase_request_refs"] = refsToMaps(resultsByWarehouse[textValue(values["warehouse_code"])])
		if _, err := s.models.Update("planning_proposal", proposal.ID, actorID, values, proposal.Version); err != nil {
			return nil, err
		}
	}
	for runID := range runIDs {
		if strings.TrimSpace(runID) == "" {
			continue
		}
		if err := s.refreshPlanningRunStatus(runID, actorID); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (s *PlanningCoreService) replenishmentRowsScoped(organizationID, locationID, warehouseFilter, itemFilter, categoryFilter, coverageFilter string, shortageOnly, hasInboundOnly, hasPreferredVendorOnly bool, now time.Time) []map[string]any {
	policies := s.replenishmentPolicies()
	demand := s.salesDemandByWarehouse(organizationID, locationID, policies)
	historicalAverage, seasonalBuckets := s.historicalDemandByWarehouse(organizationID, locationID, policies, now)
	orderedInbound, orderedReceived, orderRefs, inboundDates, inboundEvents := s.inboundSupplyByWarehouse(organizationID, locationID, policies)
	requested, requestRefs := s.requestCoverageByWarehouse(organizationID, locationID, policies)
	vendors := s.preferredVendorProfiles()

	onHand := map[string]float64{}
	reserved := map[string]float64{}
	if s.inventory != nil {
		for _, balance := range s.inventory.currentBalances(organizationID, locationID) {
			key := planningKey(balance.ItemCode, balance.WarehouseCode)
			onHand[key] = roundMoney(onHand[key] + balance.Quantity)
		}
		for _, balance := range s.inventory.currentReservedBalances(organizationID, locationID, "") {
			key := planningKey(balance.ItemCode, balance.WarehouseCode)
			reserved[key] = roundMoney(reserved[key] + balance.Quantity)
		}
	}

	keys := map[string]struct{}{}
	for _, policy := range policies {
		warehouseCode := firstNonEmptyString(policy.DefaultWHCode, "MAIN")
		keys[planningKey(policy.ItemCode, warehouseCode)] = struct{}{}
	}
	for key := range demand {
		keys[key] = struct{}{}
	}
	for key := range orderedInbound {
		keys[key] = struct{}{}
	}
	for key := range orderedReceived {
		keys[key] = struct{}{}
	}
	for key := range requested {
		keys[key] = struct{}{}
	}
	for key := range onHand {
		keys[key] = struct{}{}
	}
	for key := range reserved {
		keys[key] = struct{}{}
	}

	rows := make([]map[string]any, 0, len(keys))
	for key := range keys {
		itemCode, warehouseCode := splitPlanningKey(key)
		if warehouseFilter != "" && warehouseCode != warehouseFilter {
			continue
		}
		if itemFilter != "" && itemCode != itemFilter {
			continue
		}
		policy, ok := policies[itemCode]
		if !ok || !policy.Enabled {
			continue
		}
		if categoryFilter != "" && policy.CategoryCode != categoryFilter {
			continue
		}
		vendor := vendors[itemCode]
		if hasPreferredVendorOnly && vendor.VendorID == "" {
			continue
		}

		onHandQty := roundMoney(onHand[key])
		reservedQty := roundMoney(reserved[key])
		orderedQty := roundMoney(orderedInbound[key])
		receivedQty := roundMoney(orderedReceived[key])
		requestedQty := roundMoney(requested[key])
		demandQty := roundMoney(demand[key])
		historicalAverageQty := roundMoney(historicalAverage[key])
		forecastQty := roundMoney(historicalAverageQty * planningForecastWindowMultiplier())
		netPosition := roundMoney(onHandQty - reservedQty + orderedQty)
		projectedNetPosition := roundMoney(netPosition - forecastQty)
		unmetDemand := roundMoney(maxFloat((demandQty+forecastQty)-netPosition, 0))
		reorderShortage := roundMoney(maxFloat(policy.ReorderPoint-netPosition, 0))
		targetNeed := roundMoney(maxFloat(policy.TargetStock-netPosition, 0))
		shortageQty := roundMoney(maxFloat(unmetDemand, reorderShortage))
		uncoveredDemand := roundMoney(maxFloat(unmetDemand-requestedQty, 0))
		uncoveredReorder := roundMoney(maxFloat(reorderShortage-requestedQty, 0))
		uncoveredTargetNeed := roundMoney(maxFloat(targetNeed-requestedQty, 0))
		uncoveredShortage := roundMoney(maxFloat(uncoveredDemand, uncoveredReorder))
		suggestedQty := roundMoney(maxFloat(uncoveredDemand, uncoveredTargetNeed))
		if policy.TargetStock <= 0 && policy.ReorderPoint <= 0 {
			shortageQty = roundMoney(unmetDemand)
			uncoveredShortage = roundMoney(uncoveredDemand)
			suggestedQty = roundMoney(uncoveredDemand)
		}
		normalizedQty, quantityRule := normalizePlanningQuantity(suggestedQty, vendor.MinimumOrderQty, vendor.PackSize)
		projectedShortageDate, daysUntilShortage := projectShortageDate(now, onHandQty-reservedQty, demandQty, forecastQty, inboundEvents[key])
		recommendedOrderByDate := ""
		timeCritical := false
		if !projectedShortageDate.IsZero() {
			recommendedOrderByDate = projectedShortageDate.AddDate(0, 0, -int(vendor.LeadTimeDays)).Format("2006-01-02")
			timeCritical = vendor.LeadTimeDays > 0 && float64(daysUntilShortage) <= vendor.LeadTimeDays
			if recommendedOrderByDate != "" && !projectedShortageDate.IsZero() && recommendedOrderByDate <= now.Format("2006-01-02") {
				timeCritical = true
			}
		}

		coverage := "uncovered"
		if shortageQty <= 0 && requestedQty <= 0 && orderedQty <= 0 && receivedQty <= 0 {
			coverage = "healthy"
		} else if receivedQty > 0 {
			if orderedQty > 0 || uncoveredShortage > 0 {
				coverage = "partially_received"
			} else {
				coverage = "received"
			}
		} else if orderedQty > 0 {
			coverage = "ordered"
		} else if uncoveredShortage <= 0 && requestedQty > 0 {
			coverage = "requested"
		} else if requestedQty > 0 {
			coverage = "requested"
		}
		if coverageFilter != "" && coverage != coverageFilter {
			continue
		}
		if hasInboundOnly && orderedQty <= 0 {
			continue
		}
		if shortageOnly && suggestedQty <= 0 {
			continue
		}

		rows = append(rows, map[string]any{
			"item_code":                   itemCode,
			"item_name":                   policy.Name,
			"category_code":               policy.CategoryCode,
			"warehouse_code":              warehouseCode,
			"uom_code":                    policy.UOMCode,
			"on_hand_quantity":            onHandQty,
			"reserved_quantity":           reservedQty,
			"available_quantity":          roundMoney(maxFloat(onHandQty-reservedQty, 0)),
			"inbound_quantity":            orderedQty,
			"net_available_quantity":      netPosition,
			"projected_net_position":      projectedNetPosition,
			"reorder_point_quantity":      policy.ReorderPoint,
			"target_stock_quantity":       policy.TargetStock,
			"sales_demand_quantity":       demandQty,
			"forecast_demand_quantity":    forecastQty,
			"historical_average_quantity": historicalAverageQty,
			"forecast_window_days":        planningForecastWindowDays(),
			"seasonal_bucket_key":         seasonalBuckets[key],
			"shortage_quantity":           shortageQty,
			"requested_quantity":          requestedQty,
			"ordered_quantity":            orderedQty,
			"received_quantity":           receivedQty,
			"uncovered_shortage_quantity": uncoveredShortage,
			"suggested_request_quantity":  suggestedQty,
			"normalized_request_quantity": normalizedQty,
			"planning_quantity_rule":      quantityRule,
			"default_replenishment_mode":  policy.Mode,
			"default_warehouse_code":      policy.DefaultWHCode,
			"preferred_vendor_id":         vendor.VendorID,
			"preferred_vendor_name":       vendor.VendorName,
			"vendor_item_code":            vendor.VendorItemCode,
			"purchase_uom_code":           vendor.PurchaseUOMCode,
			"lead_time_days":              vendor.LeadTimeDays,
			"preferred_vendor_lead_time_days": vendor.LeadTimeDays,
			"vendor_priority":             vendor.Priority,
			"pack_size":                   vendor.PackSize,
			"minimum_order_quantity":      vendor.MinimumOrderQty,
			"coverage_status":             coverage,
			"next_inbound_date":           inboundDates[key],
			"projected_shortage_date":     formatPlanningDate(projectedShortageDate),
			"days_until_shortage":         daysUntilShortage,
			"recommended_order_by_date":   recommendedOrderByDate,
			"time_critical":               timeCritical,
			"purchase_request_refs":       refsToMaps(requestRefs[key]),
			"purchase_order_refs":         refsToMaps(orderRefs[key]),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		leftWarehouse := textValue(rows[i]["warehouse_code"])
		rightWarehouse := textValue(rows[j]["warehouse_code"])
		if leftWarehouse != rightWarehouse {
			return leftWarehouse < rightWarehouse
		}
		leftShortage := numberValue(rows[i]["suggested_request_quantity"])
		rightShortage := numberValue(rows[j]["suggested_request_quantity"])
		if leftShortage != rightShortage {
			return leftShortage > rightShortage
		}
		return textValue(rows[i]["item_code"]) < textValue(rows[j]["item_code"])
	})
	return rows
}

func (s *PlanningCoreService) salesDemandByWarehouse(organizationID, locationID string, policies map[string]replenishmentPolicy) map[string]float64 {
	demand := map[string]float64{}
	if s.documents == nil {
		return demand
	}
	for _, record := range s.documents.List() {
		if record.Header.Type != "sales_order" || record.Header.Status != "confirmed" {
			continue
		}
		if !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		existingFulfillment := map[int]float64{}
		if s.fulfillment != nil {
			existingFulfillment = s.fulfillment.existingFulfillmentQuantities(record.Header.ID)
		}
		for index, line := range recordList(record.Body.Payload["lines"]) {
			itemCode := strings.TrimSpace(textValue(line["item_code"]))
			policy, ok := policies[itemCode]
			if !ok || !policy.Enabled {
				continue
			}
			remainingQty := roundMoney(numberValue(line["quantity"]) - existingFulfillment[index])
			if remainingQty <= 0 {
				continue
			}
			warehouseCode := firstNonEmptyString(textValue(line["warehouse_code"]), policy.DefaultWHCode, "MAIN")
			key := planningKey(itemCode, warehouseCode)
			demand[key] = roundMoney(demand[key] + remainingQty)
		}
	}
	return demand
}

func (s *PlanningCoreService) inboundSupplyByWarehouse(organizationID, locationID string, policies map[string]replenishmentPolicy) (map[string]float64, map[string]float64, map[string][]replenishmentReference, map[string]string, map[string][]planningInboundEvent) {
	inbound := map[string]float64{}
	received := map[string]float64{}
	refs := map[string][]replenishmentReference{}
	nextInbound := map[string]string{}
	events := map[string][]planningInboundEvent{}
	if s.documents == nil {
		return inbound, received, refs, nextInbound, events
	}
	for _, record := range s.documents.List() {
		if record.Header.Type != "purchase_order" {
			continue
		}
		if record.Header.Status != "approved" && record.Header.Status != "partially_received" && record.Header.Status != "received" {
			continue
		}
		if !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		for _, line := range recordList(record.Body.Payload["lines"]) {
			itemCode := strings.TrimSpace(textValue(line["item_code"]))
			policy, ok := policies[itemCode]
			if !ok || !policy.Enabled {
				continue
			}
			orderedQty := roundMoney(maxFloat(numberValue(line["ordered_qty"]), numberValue(line["quantity"])))
			receivedQty := roundMoney(numberValue(line["received_qty"]))
			remainingQty := roundMoney(maxFloat(orderedQty-receivedQty, 0))
			warehouseCode := firstNonEmptyString(textValue(line["warehouse_code"]), policy.DefaultWHCode, "MAIN")
			key := planningKey(itemCode, warehouseCode)
			if remainingQty > 0 {
				inbound[key] = roundMoney(inbound[key] + remainingQty)
				eventDate := firstPlanningDate(
					textValue(line["expected_receipt_date"]),
					textValue(record.Body.Payload["expected_receipt_date"]),
					textValue(line["needed_by_date"]),
					textValue(record.Body.Payload["needed_by_date"]),
					textValue(line["order_date"]),
					textValue(record.Body.Payload["order_date"]),
				)
				if !eventDate.IsZero() {
					events[key] = append(events[key], planningInboundEvent{Date: eventDate, Qty: remainingQty})
					dateValue := eventDate.Format("2006-01-02")
					if nextInbound[key] == "" || dateValue < nextInbound[key] {
						nextInbound[key] = dateValue
					}
				}
			}
			if receivedQty > 0 {
				received[key] = roundMoney(received[key] + receivedQty)
			}
			refs[key] = appendUniqueReference(refs[key], replenishmentReference{
				ID:     record.Header.ID,
				Number: record.Header.Number,
				Status: record.Header.Status,
			})
		}
	}
	for key := range events {
		sort.Slice(events[key], func(i, j int) bool { return events[key][i].Date.Before(events[key][j].Date) })
	}
	return inbound, received, refs, nextInbound, events
}

func (s *PlanningCoreService) historicalDemandByWarehouse(organizationID, locationID string, policies map[string]replenishmentPolicy, now time.Time) (map[string]float64, map[string]string) {
	deliveredTotals := map[string]float64{}
	deliveredCounts := map[string]int{}
	issuedTotals := map[string]float64{}
	issuedCounts := map[string]int{}
	buckets := map[string]string{}
	if s.documents == nil {
		return map[string]float64{}, buckets
	}
	targetWeekday := now.Weekday()
	windowStart := now.AddDate(0, 0, -(planningForecastHistoryWeeks() * 7))
	for _, record := range s.documents.List() {
		if !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		switch record.Header.Type {
		case "delivery_order":
			if record.Header.Status != "delivered" {
				continue
			}
			recordDate := firstPlanningDate(textValue(record.Body.Payload["delivered_date"]), textValue(record.Body.Payload["delivery_date"]))
			if recordDate.IsZero() || recordDate.Before(windowStart) || recordDate.After(now) || recordDate.Weekday() != targetWeekday {
				continue
			}
			for _, line := range recordList(record.Body.Payload["lines"]) {
				itemCode := strings.TrimSpace(textValue(line["item_code"]))
				policy, ok := policies[itemCode]
				if !ok || !policy.Enabled {
					continue
				}
				warehouseCode := firstNonEmptyString(textValue(line["warehouse_code"]), policy.DefaultWHCode, "MAIN")
					qty := roundMoney(maxFloat(numberValue(line["delivered_quantity"]), maxFloat(numberValue(line["fulfilled_quantity"]), numberValue(line["quantity"]))))
				if qty <= 0 {
					continue
				}
				key := planningKey(itemCode, warehouseCode)
				deliveredTotals[key] = roundMoney(deliveredTotals[key] + qty)
				deliveredCounts[key]++
				buckets[key] = strings.ToLower(targetWeekday.String())
			}
		case "sales_fulfillment":
			if record.Header.Status != "issued" {
				continue
			}
			recordDate := firstPlanningDate(textValue(record.Body.Payload["fulfillment_date"]))
			if recordDate.IsZero() || recordDate.Before(windowStart) || recordDate.After(now) || recordDate.Weekday() != targetWeekday {
				continue
			}
			for _, line := range recordList(record.Body.Payload["lines"]) {
				itemCode := strings.TrimSpace(textValue(line["item_code"]))
				policy, ok := policies[itemCode]
				if !ok || !policy.Enabled {
					continue
				}
				warehouseCode := firstNonEmptyString(textValue(line["warehouse_code"]), policy.DefaultWHCode, "MAIN")
				qty := roundMoney(maxFloat(numberValue(line["issued_quantity"]), numberValue(line["quantity"])))
				if qty <= 0 {
					continue
				}
				key := planningKey(itemCode, warehouseCode)
				issuedTotals[key] = roundMoney(issuedTotals[key] + qty)
				issuedCounts[key]++
				buckets[key] = strings.ToLower(targetWeekday.String())
			}
		}
	}
	averages := map[string]float64{}
	keys := map[string]struct{}{}
	for key := range deliveredTotals {
		keys[key] = struct{}{}
	}
	for key := range issuedTotals {
		keys[key] = struct{}{}
	}
	for key := range keys {
		switch {
		case deliveredCounts[key] > 0:
			averages[key] = roundMoney(deliveredTotals[key] / float64(deliveredCounts[key]))
		case issuedCounts[key] > 0:
			averages[key] = roundMoney(issuedTotals[key] / float64(issuedCounts[key]))
		default:
			averages[key] = 0
		}
		if buckets[key] == "" {
			buckets[key] = strings.ToLower(targetWeekday.String())
		}
	}
	return averages, buckets
}

func (s *PlanningCoreService) requestCoverageByWarehouse(organizationID, locationID string, policies map[string]replenishmentPolicy) (map[string]float64, map[string][]replenishmentReference) {
	requested := map[string]float64{}
	refs := map[string][]replenishmentReference{}
	if s.documents == nil {
		return requested, refs
	}
	for _, record := range s.documents.List() {
		if record.Header.Type != "purchase_request" {
			continue
		}
		if record.Header.Status == "rejected" || record.Header.Status == "cancelled" {
			continue
		}
		if !boolValue(record.Body.Payload["planning_generated"]) {
			continue
		}
		if !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		for _, line := range recordList(record.Body.Payload["lines"]) {
			itemCode := strings.TrimSpace(textValue(line["item_code"]))
			policy, ok := policies[itemCode]
			if !ok || !policy.Enabled {
				continue
			}
			quantity := roundMoney(numberValue(line["quantity"]))
			if quantity <= 0 {
				continue
			}
			warehouseCode := firstNonEmptyString(textValue(line["warehouse_code"]), textValue(record.Body.Payload["planning_warehouse_code"]), policy.DefaultWHCode, "MAIN")
			key := planningKey(itemCode, warehouseCode)
			requested[key] = roundMoney(requested[key] + quantity)
			refs[key] = appendUniqueReference(refs[key], replenishmentReference{
				ID:     record.Header.ID,
				Number: record.Header.Number,
				Status: record.Header.Status,
			})
		}
	}
	return requested, refs
}

func (s *PlanningCoreService) replenishmentPolicies() map[string]replenishmentPolicy {
	policies := map[string]replenishmentPolicy{}
	if s.models == nil {
		return policies
	}
	items, _, err := s.models.List("commercial_item", model.Query{Page: 1, PageSize: 2000})
	if err != nil {
		return policies
	}
	for _, item := range items {
		itemCode := strings.TrimSpace(textValue(item.Values["sku"]))
		if itemCode == "" {
			continue
		}
		policies[itemCode] = replenishmentPolicy{
			ItemCode:      itemCode,
			Name:          firstNonEmptyString(textValue(item.Values["name"]), itemCode),
			UOMCode:       textValue(item.Values["uom_code"]),
			CategoryCode:  textValue(item.Values["category_code"]),
			Enabled:       boolValue(item.Values["inventory_enabled"]) && boolValue(item.Values["replenishment_enabled"]),
			Mode:          firstNonEmptyString(textValue(item.Values["replenishment_mode"]), "manual"),
			ReorderPoint:  roundMoney(numberValue(item.Values["reorder_point_quantity"])),
			TargetStock:   roundMoney(numberValue(item.Values["target_stock_quantity"])),
			DefaultWHCode: textValue(item.Values["default_replenishment_warehouse_code"]),
			BasePrice:     roundMoney(maxFloat(numberValue(item.Values["base_price"]), numberValue(item.Values["unit_price"]))),
		}
	}
	return policies
}

func (s *PlanningCoreService) lookupReplenishmentPolicy(itemCode string) replenishmentPolicy {
	if policy, ok := s.replenishmentPolicies()[strings.TrimSpace(itemCode)]; ok {
		return policy
	}
	return replenishmentPolicy{}
}

func (s *PlanningCoreService) preferredVendorProfiles() map[string]replenishmentVendorProfile {
	profiles := map[string]replenishmentVendorProfile{}
	if s.models == nil {
		return profiles
	}
	records, _, err := s.models.List("vendor_item_profile", model.Query{Page: 1, PageSize: 2000})
	if err != nil {
		return profiles
	}
	vendorNames := s.vendorNamesByID()
	for _, record := range records {
		itemCode := strings.TrimSpace(textValue(record.Values["item_code"]))
		vendorID := strings.TrimSpace(textValue(record.Values["vendor_id"]))
		if itemCode == "" || vendorID == "" || strings.EqualFold(textValue(record.Values["status"]), "inactive") {
			continue
		}
		profile := replenishmentVendorProfile{
			VendorID:          vendorID,
			VendorName:        firstNonEmptyString(textValue(record.Values["vendor_name"]), vendorNames[vendorID], vendorID),
			VendorItemCode:    textValue(record.Values["vendor_item_code"]),
			PurchaseUOMCode:   textValue(record.Values["purchase_uom_code"]),
			LeadTimeDays:      roundMoney(numberValue(record.Values["lead_time_days"])),
			Priority:          roundMoney(numberValue(record.Values["priority"])),
			MinimumOrderQty:   roundMoney(numberValue(record.Values["minimum_order_quantity"])),
			PackSize:          roundMoney(numberValue(record.Values["pack_size"])),
			LastPurchasePrice: roundMoney(numberValue(record.Values["last_purchase_price"])),
			Preferred:         boolValue(record.Values["preferred"]),
			Status:            textValue(record.Values["status"]),
		}
		if existing, ok := profiles[itemCode]; !ok || betterVendorProfile(profile, existing) {
			profiles[itemCode] = profile
		}
	}
	return profiles
}

func (s *PlanningCoreService) vendorNamesByID() map[string]string {
	names := map[string]string{}
	if s.models == nil {
		return names
	}
	records, _, err := s.models.List("vendor_profile", model.Query{Page: 1, PageSize: 2000})
	if err != nil {
		return names
	}
	for _, record := range records {
		names[record.ID] = firstNonEmptyString(textValue(record.Values["vendor_name"]), textValue(record.Values["party_id"]))
	}
	return names
}

func planningKey(itemCode, warehouseCode string) string {
	return strings.TrimSpace(itemCode) + "|" + strings.TrimSpace(warehouseCode)
}

func splitPlanningKey(key string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(key), "|", 2)
	if len(parts) != 2 {
		return strings.TrimSpace(key), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func appendUniqueReference(refs []replenishmentReference, ref replenishmentReference) []replenishmentReference {
	for _, existing := range refs {
		if existing.ID == ref.ID {
			return refs
		}
	}
	return append(refs, ref)
}

func refsToMaps(refs []replenishmentReference) []map[string]any {
	items := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		items = append(items, map[string]any{
			"id":     ref.ID,
			"number": ref.Number,
			"status": ref.Status,
		})
	}
	return items
}

func firstPositivePlanningNumber(values ...float64) float64 {
	for _, value := range values {
		value = roundMoney(value)
		if value > 0 {
			return value
		}
	}
	return 0
}

func betterVendorProfile(candidate, existing replenishmentVendorProfile) bool {
	candidatePriority := candidate.Priority
	existingPriority := existing.Priority
	if candidatePriority <= 0 {
		candidatePriority = 999999
	}
	if existingPriority <= 0 {
		existingPriority = 999999
	}
	if candidatePriority != existingPriority {
		return candidatePriority < existingPriority
	}
	if candidate.Preferred != existing.Preferred {
		return candidate.Preferred
	}
	candidatePrice := candidate.LastPurchasePrice
	existingPrice := existing.LastPurchasePrice
	if candidatePrice > 0 && existingPrice > 0 && candidatePrice != existingPrice {
		return candidatePrice < existingPrice
	}
	return candidate.VendorID < existing.VendorID
}

func normalizePlanningQuantity(suggestedQty, minimumOrderQty, packSize float64) (float64, string) {
	normalized := roundMoney(suggestedQty)
	if normalized <= 0 {
		return 0, "none"
	}
	rules := make([]string, 0, 2)
	if minimumOrderQty > 0 && normalized < minimumOrderQty {
		normalized = minimumOrderQty
		rules = append(rules, "moq")
	}
	if packSize > 0 {
		packs := normalized / packSize
		wholePacks := float64(int64(packs))
		if packs > wholePacks {
			wholePacks++
		}
		rounded := roundMoney(wholePacks * packSize)
		if rounded != normalized {
			normalized = rounded
			rules = append(rules, "pack")
		}
	}
	if len(rules) == 0 {
		return roundMoney(normalized), "none"
	}
	return roundMoney(normalized), strings.Join(rules, "+")
}

func (s *PlanningCoreService) refreshPlanningRunStatus(runID, actorID string) error {
	if s.models == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	run, err := s.models.Get("planning_run", runID)
	if err != nil {
		return err
	}
	proposals, _, err := s.models.List("planning_proposal", model.Query{Page: 1, PageSize: 2000, Filters: map[string]string{"planning_run_id": runID}})
	if err != nil {
		return err
	}
	allConverted := len(proposals) > 0
	convertedCount := 0
	for _, proposal := range proposals {
		if strings.EqualFold(textValue(proposal.Values["conversion_status"]), "converted") {
			convertedCount++
			continue
		}
		allConverted = false
	}
	values := cloneMap(run.Values)
	values["status"] = "completed"
	if allConverted {
		values["status"] = "converted"
	}
	values["converted_proposal_count"] = convertedCount
	_, err = s.models.Update("planning_run", run.ID, actorID, values, run.Version)
	return err
}

func matchesPlanningModelScope(record model.Record, organizationID, locationID string) bool {
	if organizationID != "" && !strings.EqualFold(textValue(record.Values["organization_id"]), organizationID) {
		return false
	}
	if locationID != "" && !strings.EqualFold(textValue(record.Values["location_id"]), locationID) {
		return false
	}
	return true
}

func planningForecastWindowDays() int {
	return 14
}

func planningForecastHistoryWeeks() int {
	return 6
}

func planningForecastWindowMultiplier() float64 {
	return float64(planningForecastWindowDays()) / 7.0
}

func firstPlanningDate(values ...string) time.Time {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if parsed, err := time.Parse("2006-01-02", value); err == nil {
			return parsed
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func formatPlanningDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func projectShortageDate(now time.Time, currentAvailable, confirmedDemand, forecastDemand float64, inbound []planningInboundEvent) (time.Time, int) {
	balance := roundMoney(currentAvailable)
	if balance < 0 {
		return now, 0
	}
	dailyRate := roundMoney((confirmedDemand + forecastDemand) / float64(planningForecastWindowDays()))
	if dailyRate <= 0 {
		return time.Time{}, 0
	}
	current := now
	for _, event := range inbound {
		if event.Date.Before(current) {
			balance = roundMoney(balance + event.Qty)
			continue
		}
		days := int(event.Date.Sub(current).Hours() / 24)
		if days < 0 {
			days = 0
		}
		consumption := roundMoney(dailyRate * float64(days))
		if balance-consumption < 0 {
			extraDays := int(balance / dailyRate)
			shortage := current.AddDate(0, 0, extraDays)
			return shortage, maxInt(int(shortage.Sub(now).Hours()/24), 0)
		}
		balance = roundMoney(balance - consumption + event.Qty)
		current = event.Date
	}
	extraDays := int(balance / dailyRate)
	shortage := current.AddDate(0, 0, extraDays)
	return shortage, maxInt(int(shortage.Sub(now).Hours()/24), 0)
}
