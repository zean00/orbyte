package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type ProductionCostingSummaryRow struct {
	ProductionOrderID       string  `json:"production_order_id"`
	OrderNumber             string  `json:"order_number"`
	Status                  string  `json:"status"`
	FinishedItemCode        string  `json:"finished_item_code"`
	FinishedItemName        string  `json:"finished_item_name"`
	PlannedQuantity         float64 `json:"planned_quantity"`
	ActualOutputQuantity    float64 `json:"actual_output_quantity"`
	StandardMaterialCost    float64 `json:"standard_material_cost_total"`
	StandardLaborCost       float64 `json:"standard_labor_cost_total"`
	StandardOverheadCost    float64 `json:"standard_overhead_cost_total"`
	StandardTotalCost       float64 `json:"standard_total_cost"`
	ActualMaterialCost      float64 `json:"actual_material_cost_total"`
	ActualLaborCost         float64 `json:"actual_labor_cost_total"`
	ActualOverheadCost      float64 `json:"actual_overhead_cost_total"`
	ActualTotalCost         float64 `json:"actual_total_cost"`
	UnitStandardCost        float64 `json:"unit_standard_cost"`
	UnitActualCost          float64 `json:"unit_actual_cost"`
	MaterialVarianceAmount  float64 `json:"material_variance_amount"`
	LaborVarianceAmount     float64 `json:"labor_variance_amount"`
	OverheadVarianceAmount  float64 `json:"overhead_variance_amount"`
	YieldVarianceAmount     float64 `json:"yield_variance_amount"`
	TotalVarianceAmount     float64 `json:"total_variance_amount"`
}

type ProductionCostSummaryReport struct {
	Rows []ProductionCostingSummaryRow `json:"rows"`
}

type ProductionVarianceReport struct {
	Rows []ProductionCostingSummaryRow `json:"rows"`
}

type ProductionCostingCoreService struct {
	documents *document.Service
	models    *model.Service
	inventory *InventoryCoreService
	finance   *FinanceReportingCoreService
}

func NewProductionCostingCoreService(documents *document.Service, models *model.Service, inventory *InventoryCoreService, finance *FinanceReportingCoreService) *ProductionCostingCoreService {
	return &ProductionCostingCoreService{
		documents: documents,
		models:    models,
		inventory: inventory,
		finance:   finance,
	}
}

func (s *ProductionCostingCoreService) SyncProductionOrder(record document.Record, actorID string) error {
	if record.Header.Type != "production_order" {
		return nil
	}
	return s.syncProductionOrder(record, actorID)
}

func (s *ProductionCostingCoreService) BeforeApproveProductionOutput(record document.Record, actorID string) (map[string]any, []map[string]any, error) {
	payload := clonedPayload(record.Body.Payload)
	order, ok := s.productionOrderForOutput(payload)
	if ok {
		if err := s.postApprovedCostCaptures(order, actorID); err != nil {
			return nil, nil, err
		}
		if err := s.syncProductionOrder(order, actorID); err != nil {
			return nil, nil, err
		}
		refreshed, err := s.documents.Get(order.Header.ID)
		if err == nil {
			order = refreshed
		}
		payload["source_production_order_id"] = order.Header.ID
		payload["source_production_order_number"] = firstNonEmptyString(order.Header.Number, order.Header.ID)
		payload["actual_labor_cost_total"] = roundMoney(numberValue(order.Body.Payload["actual_labor_cost_total"]))
		payload["actual_overhead_cost_total"] = roundMoney(numberValue(order.Body.Payload["actual_overhead_cost_total"]))
		payload["actual_total_cost"] = roundMoney(numberValue(order.Body.Payload["actual_total_cost"]))
		if summary, ok := order.Body.Payload["production_variance_summary"].(map[string]any); ok {
			payload["production_variance_summary"] = cloneMap(summary)
		}
	}
	totalCost := s.totalProductionCost(payload)
	outputQty := roundMoney(numberValue(payload["output_quantity"]))
	allocations := s.resolveOutputAllocations(record, payload, totalCost, outputQty)
	return payload, allocations, nil
}

func (s *ProductionCostingCoreService) HandleApprovedProductionOutput(record document.Record, payload map[string]any, allocations []map[string]any, actorID string) error {
	order, ok := s.productionOrderForOutput(payload)
	if !ok {
		return nil
	}
	outputDate := firstNonEmptyString(textValue(payload["output_date"]), time.Now().UTC().Format("2006-01-02"))
	for _, allocation := range allocations {
		values := map[string]any{
			"organization_id":         record.Header.OrganizationID,
			"location_id":             record.Header.LocationID,
			"source_production_output_id": record.Header.ID,
			"production_order_id":     order.Header.ID,
			"output_item_code":        textValue(allocation["output_item_code"]),
			"output_item_name":        textValue(allocation["output_item_name"]),
			"warehouse_code":          textValue(allocation["warehouse_code"]),
			"output_quantity":         roundMoney(numberValue(allocation["output_quantity"])),
			"allocation_basis":        firstNonEmptyString(textValue(allocation["allocation_basis"]), "quantity_share"),
			"allocation_share_percent": roundMoney(numberValue(allocation["allocation_share_percent"])),
			"allocated_total_cost":    roundMoney(numberValue(allocation["allocated_total_cost"])),
			"allocated_unit_cost":     roundMoney(numberValue(allocation["allocated_unit_cost"])),
			"output_date":             outputDate,
			"status":                  "posted",
		}
		if _, err := s.models.Create("production_output_allocation", actorID, values); err != nil && !isConflict(err) {
			return err
		}
	}
	return s.syncProductionOrder(order, actorID)
}

func (s *ProductionCostingCoreService) ProductionCostSummary(organizationID, locationID string) ProductionCostSummaryReport {
	rows := s.productionSummaryRows(organizationID, locationID)
	return ProductionCostSummaryReport{Rows: rows}
}

func (s *ProductionCostingCoreService) ProductionVarianceReport(organizationID, locationID string) ProductionVarianceReport {
	rows := make([]ProductionCostingSummaryRow, 0)
	for _, row := range s.productionSummaryRows(organizationID, locationID) {
		if row.TotalVarianceAmount != 0 || row.MaterialVarianceAmount != 0 || row.LaborVarianceAmount != 0 || row.OverheadVarianceAmount != 0 || row.YieldVarianceAmount != 0 {
			rows = append(rows, row)
		}
	}
	return ProductionVarianceReport{Rows: rows}
}

func (s *ProductionCostingCoreService) totalProductionCost(payload map[string]any) float64 {
	if total := roundMoney(numberValue(payload["actual_total_cost"])); total > 0 {
		return total
	}
	if total := roundMoney(numberValue(payload["total_production_cost"])); total > 0 {
		return total
	}
	material := roundMoney(firstPositiveNumber(payload["issued_material_cost_total"], payload["actual_material_cost_total"]))
	labor := roundMoney(numberValue(payload["actual_labor_cost_total"]))
	overhead := roundMoney(numberValue(payload["actual_overhead_cost_total"]))
	return roundMoney(material + labor + overhead)
}

func (s *ProductionCostingCoreService) syncProductionOrder(record document.Record, actorID string) error {
	if s.documents == nil {
		return shared.Validation("documents are unavailable")
	}
	payload := clonedPayload(record.Body.Payload)
	routing, steps := s.resolveRouting(record)
	standardMaterialCost := s.standardMaterialCost(record)
	standardLaborCost, standardOverheadCost := s.standardStepCosts(record, steps)
	actualMaterialCost := roundMoney(firstPositiveNumber(payload["issued_material_cost_total"], payload["actual_material_cost_total"]))
	actualLaborCost, actualOverheadCost := s.actualCaptureCosts(record.Header.ID)
	plannedQty := roundMoney(firstPositiveNumber(payload["planned_quantity"], payload["expected_output_quantity"], 1))
	actualOutputQty := roundMoney(firstPositiveNumber(payload["actual_output_quantity"], payload["expected_output_quantity"], payload["planned_quantity"]))
	standardTotal := roundMoney(standardMaterialCost + standardLaborCost + standardOverheadCost)
	actualTotal := roundMoney(actualMaterialCost + actualLaborCost + actualOverheadCost)
	unitStandard := 0.0
	unitActual := 0.0
	if plannedQty > 0 {
		unitStandard = roundMoney(standardTotal / plannedQty)
	}
	if actualOutputQty > 0 {
		unitActual = roundMoney(actualTotal / actualOutputQty)
	}
	yieldVariance := 0.0
	if actualOutputQty > 0 && unitStandard > 0 {
		yieldVariance = roundMoney(actualTotal - roundMoney(unitStandard*actualOutputQty))
	}
	payload["production_routing_id"] = routing.ID
	payload["production_routing_code"] = textValue(routing.Values["code"])
	payload["standard_material_cost_total"] = standardMaterialCost
	payload["standard_labor_cost_total"] = standardLaborCost
	payload["standard_overhead_cost_total"] = standardOverheadCost
	payload["standard_total_cost"] = standardTotal
	payload["actual_material_cost_total"] = actualMaterialCost
	payload["actual_labor_cost_total"] = actualLaborCost
	payload["actual_overhead_cost_total"] = actualOverheadCost
	payload["actual_total_cost"] = actualTotal
	payload["unit_standard_cost"] = unitStandard
	payload["unit_actual_cost"] = unitActual
	payload["material_variance_amount"] = roundMoney(actualMaterialCost - standardMaterialCost)
	payload["labor_variance_amount"] = roundMoney(actualLaborCost - standardLaborCost)
	payload["overhead_variance_amount"] = roundMoney(actualOverheadCost - standardOverheadCost)
	payload["yield_variance_amount"] = yieldVariance
	payload["total_variance_amount"] = roundMoney(actualTotal - standardTotal)
	payload["production_variance_summary"] = map[string]any{
		"material_variance_amount": roundMoney(actualMaterialCost - standardMaterialCost),
		"labor_variance_amount":    roundMoney(actualLaborCost - standardLaborCost),
		"overhead_variance_amount": roundMoney(actualOverheadCost - standardOverheadCost),
		"yield_variance_amount":    yieldVariance,
		"total_variance_amount":    roundMoney(actualTotal - standardTotal),
	}
	if err := s.updateDocumentPayload(record, actorID, payload); err != nil {
		return err
	}
	return s.syncVarianceCase(record, actorID, payload)
}

func (s *ProductionCostingCoreService) resolveRouting(record document.Record) (model.Record, []model.Record) {
	if s.models == nil {
		return model.Record{}, nil
	}
	payload := clonedPayload(record.Body.Payload)
	itemCode := strings.TrimSpace(textValue(payload["finished_item_code"]))
	if itemCode == "" {
		return model.Record{}, nil
	}
	items, _, err := s.models.List("production_routing", model.Query{
		Filters: map[string]string{
			"organization_id": record.Header.OrganizationID,
			"status":          "active",
			"produced_item_code": itemCode,
		},
		Page:     1,
		PageSize: 100,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return model.Record{}, nil
	}
	if len(items) == 0 {
		return model.Record{}, nil
	}
	sort.Slice(items, func(i, j int) bool {
		return textValue(items[i].Values["code"]) < textValue(items[j].Values["code"])
	})
	selected := items[0]
	steps, _, err := s.models.List("production_routing_step", model.Query{
		Filters: map[string]string{
			"organization_id": record.Header.OrganizationID,
			"routing_id":      selected.ID,
		},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return selected, nil
	}
	sort.Slice(steps, func(i, j int) bool {
		return int(numberValue(steps[i].Values["sequence"])) < int(numberValue(steps[j].Values["sequence"]))
	})
	return selected, steps
}

func (s *ProductionCostingCoreService) standardMaterialCost(record document.Record) float64 {
	total := 0.0
	for _, line := range recordList(record.Body.Payload["lines"]) {
		itemCode := firstNonEmptyString(textValue(line["actual_item_code"]), textValue(line["component_item_code"]))
		if itemCode == "" {
			continue
		}
		unitCost := s.currentAverageCost(record.Header.OrganizationID, record.Header.LocationID, itemCode, textValue(line["warehouse_code"]))
		total = roundMoney(total + roundMoney(numberValue(line["quantity"])*unitCost))
	}
	return total
}

func (s *ProductionCostingCoreService) standardStepCosts(record document.Record, steps []model.Record) (float64, float64) {
	payload := clonedPayload(record.Body.Payload)
	plannedQty := roundMoney(firstPositiveNumber(payload["planned_quantity"], payload["expected_output_quantity"], 1))
	labor := 0.0
	overhead := 0.0
	for _, step := range steps {
		driver := strings.ToLower(strings.TrimSpace(textValue(step.Values["cost_driver"])))
		if driver == "" {
			driver = strings.ToLower(strings.TrimSpace(textValue(step.Values["rate_type"])))
		}
		standardQty := roundMoney(firstPositiveNumber(step.Values["standard_quantity"], step.Values["standard_time_quantity"], 1))
		rate := roundMoney(firstPositiveNumber(step.Values["standard_rate"], s.lookupCostRate(record.Header.OrganizationID, record.Header.LocationID, textValue(step.Values["work_center_code"]), driver)))
		if rate <= 0 {
			continue
		}
		cost := roundMoney(plannedQty * standardQty * rate)
		if driver == "labor" {
			labor = roundMoney(labor + cost)
		} else {
			overhead = roundMoney(overhead + cost)
		}
	}
	return labor, overhead
}

func (s *ProductionCostingCoreService) lookupCostRate(organizationID, locationID, workCenterCode, rateType string) float64 {
	if s.models == nil {
		return 0
	}
	items, _, err := s.models.List("production_cost_rate", model.Query{
		Filters: map[string]string{
			"organization_id": organizationID,
			"work_center_code": strings.TrimSpace(workCenterCode),
			"rate_type":       strings.TrimSpace(rateType),
			"status":          "active",
		},
		Page:     1,
		PageSize: 100,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return 0
	}
	if len(items) == 0 {
		return 0
	}
	sort.Slice(items, func(i, j int) bool {
		leftLocation := strings.TrimSpace(textValue(items[i].Values["location_id"]))
		rightLocation := strings.TrimSpace(textValue(items[j].Values["location_id"]))
		if leftLocation != rightLocation {
			return leftLocation == strings.TrimSpace(locationID)
		}
		return textValue(items[i].Values["effective_start_date"]) > textValue(items[j].Values["effective_start_date"])
	})
	return roundMoney(numberValue(items[0].Values["standard_rate"]))
}

func (s *ProductionCostingCoreService) actualCaptureCosts(orderID string) (float64, float64) {
	if s.models == nil {
		return 0, 0
	}
	items, _, err := s.models.List("production_cost_capture", model.Query{
		Filters: map[string]string{
			"production_order_id": strings.TrimSpace(orderID),
		},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return 0, 0
	}
	labor := 0.0
	overhead := 0.0
	for _, item := range items {
		status := strings.TrimSpace(textValue(item.Values["status"]))
		if status != "approved" && status != "posted" {
			continue
		}
		cost := roundMoney(firstPositiveNumber(item.Values["actual_cost"], roundMoney(numberValue(item.Values["quantity"])*numberValue(item.Values["actual_rate"]))))
		switch strings.ToLower(strings.TrimSpace(textValue(item.Values["capture_type"]))) {
		case "labor":
			labor = roundMoney(labor + cost)
		default:
			overhead = roundMoney(overhead + cost)
		}
	}
	return labor, overhead
}

func (s *ProductionCostingCoreService) postApprovedCostCaptures(order document.Record, actorID string) error {
	if s.models == nil || s.documents == nil {
		return nil
	}
	items, _, err := s.models.List("production_cost_capture", model.Query{
		Filters: map[string]string{
			"production_order_id": order.Header.ID,
		},
		Page:     1,
		PageSize: 1000,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	for _, capture := range items {
		if strings.TrimSpace(textValue(capture.Values["status"])) != "approved" || strings.TrimSpace(textValue(capture.Values["posting_id"])) != "" {
			continue
		}
		cost := roundMoney(firstPositiveNumber(capture.Values["actual_cost"], roundMoney(numberValue(capture.Values["quantity"])*numberValue(capture.Values["actual_rate"]))))
		if cost <= 0 {
			continue
		}
		postingDate := firstNonEmptyString(textValue(capture.Values["capture_date"]), time.Now().UTC().Format("2006-01-02"))
		if s.finance != nil {
			if err := s.finance.ValidatePostingDateOpen(order.Header.OrganizationID, order.Header.LocationID, postingDate); err != nil {
				return err
			}
		}
		captureType := strings.ToLower(strings.TrimSpace(textValue(capture.Values["capture_type"])))
		wipAccount := s.orderWIPAccount(order)
		creditAccount := firstNonEmptyString(textValue(capture.Values["credit_account_code"]), defaultCaptureCreditAccount(captureType))
		payload := map[string]any{
			"source_document_type": "production_cost_capture",
			"source_document_id":   capture.ID,
			"posting_date":         postingDate,
			"currency_code":        "IDR",
			"posting_rule_key":     "production_capture_" + firstNonEmptyString(captureType, "overhead"),
			"total_amount":         cost,
			"journal_source_kind":  "system",
			"journal_lines": []map[string]any{
				{"account_code": wipAccount, "description": "Production WIP", "debit": cost, "credit": 0.0},
				{"account_code": creditAccount, "description": "Production capture accrual", "debit": 0.0, "credit": cost},
			},
			"notes": fmt.Sprintf("Production %s capture %s", firstNonEmptyString(captureType, "overhead"), capture.ID),
		}
		posting, err := s.documents.Create("ledger_posting", order.Header.OrganizationID, order.Header.LocationID, actorID, payload)
		if err != nil {
			return err
		}
		if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
			return err
		}
		values := cloneMap(capture.Values)
		values["actual_cost"] = cost
		values["posting_id"] = posting.Header.ID
		values["status"] = "posted"
		if _, err := s.models.Update("production_cost_capture", capture.ID, actorID, values, capture.Version); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProductionCostingCoreService) resolveOutputAllocations(record document.Record, payload map[string]any, totalCost, outputQty float64) []map[string]any {
	allocations := recordList(payload["output_allocations"])
	if len(allocations) == 0 {
		allocations = []map[string]any{{
			"output_item_code":         textValue(payload["finished_item_code"]),
			"output_item_name":         textValue(payload["finished_item_name"]),
			"warehouse_code":           textValue(payload["warehouse_code"]),
			"output_quantity":          outputQty,
			"allocation_basis":         "quantity_share",
			"allocation_share_percent": 100.0,
		}}
	}
	totalShare := 0.0
	totalAllocationQty := 0.0
	for _, allocation := range allocations {
		totalShare = roundMoney(totalShare + numberValue(allocation["allocation_share_percent"]))
		totalAllocationQty = roundMoney(totalAllocationQty + numberValue(allocation["output_quantity"]))
	}
	resolved := make([]map[string]any, 0, len(allocations))
	remaining := totalCost
	for index, allocation := range allocations {
		next := cloneMap(allocation)
		qty := roundMoney(numberValue(next["output_quantity"]))
		share := roundMoney(numberValue(next["allocation_share_percent"]))
		if share <= 0 {
			if totalAllocationQty > 0 {
				share = roundMoney((qty / totalAllocationQty) * 100)
			} else if totalShare > 0 {
				share = roundMoney((numberValue(next["allocation_share_percent"]) / totalShare) * 100)
			} else if len(allocations) > 0 {
				share = roundMoney(100.0 / float64(len(allocations)))
			}
		}
		allocated := 0.0
		if index == len(allocations)-1 {
			allocated = roundMoney(remaining)
		} else {
			allocated = roundMoney((share / 100.0) * totalCost)
			remaining = roundMoney(remaining - allocated)
		}
		unitCost := 0.0
		if qty > 0 {
			unitCost = roundMoney(allocated / qty)
		}
		next["allocation_share_percent"] = share
		next["allocated_total_cost"] = allocated
		next["allocated_unit_cost"] = unitCost
		next["output_item_name"] = firstNonEmptyString(textValue(next["output_item_name"]), textValue(next["description"]), s.itemName(textValue(next["output_item_code"])))
		next["warehouse_code"] = firstNonEmptyString(textValue(next["warehouse_code"]), textValue(payload["warehouse_code"]))
		resolved = append(resolved, next)
	}
	return resolved
}

func (s *ProductionCostingCoreService) productionOrderForOutput(payload map[string]any) (document.Record, bool) {
	orderID := strings.TrimSpace(textValue(payload["source_production_order_id"]))
	if orderID == "" || s.documents == nil {
		return document.Record{}, false
	}
	record, err := s.documents.Get(orderID)
	if err != nil || record.Header.Type != "production_order" {
		return document.Record{}, false
	}
	return record, true
}

func (s *ProductionCostingCoreService) orderWIPAccount(order document.Record) string {
	for _, line := range recordList(order.Body.Payload["lines"]) {
		if account := strings.TrimSpace(textValue(line["wip_account_code"])); account != "" {
			return account
		}
	}
	itemCode := strings.TrimSpace(textValue(order.Body.Payload["finished_item_code"]))
	if itemCode != "" && s.models != nil {
		items, _, err := s.models.List("commercial_item", model.Query{
			Filters: map[string]string{"sku": itemCode},
			Page:    1, PageSize: 1,
		})
		if err == nil && len(items) > 0 {
			return firstNonEmptyString(textValue(items[0].Values["wip_account_code"]), "1300-WIP")
		}
	}
	return "1300-WIP"
}

func defaultCaptureCreditAccount(captureType string) string {
	switch strings.TrimSpace(strings.ToLower(captureType)) {
	case "labor":
		return "2200-LABOR-ACCRUAL"
	case "machine":
		return "2300-MACHINE-CLEARING"
	default:
		return "2310-OH-CLEARING"
	}
}

func (s *ProductionCostingCoreService) currentAverageCost(organizationID, locationID, itemCode, warehouseCode string) float64 {
	if s.models == nil || strings.TrimSpace(itemCode) == "" {
		return 0
	}
	items, _, err := s.models.List("inventory_valuation_snapshot", model.Query{
		Filters: map[string]string{
			"organization_id": organizationID,
			"location_id":     locationID,
			"item_code":       strings.TrimSpace(itemCode),
			"warehouse_code":  strings.TrimSpace(warehouseCode),
		},
		Page:     1,
		PageSize: 1,
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return 0
	}
	if len(items) == 0 {
		return 0
	}
	return roundMoney(numberValue(items[0].Values["average_unit_cost"]))
}

func (s *ProductionCostingCoreService) itemName(itemCode string) string {
	if s.models == nil || strings.TrimSpace(itemCode) == "" {
		return ""
	}
	items, _, err := s.models.List("commercial_item", model.Query{
		Filters: map[string]string{"sku": strings.TrimSpace(itemCode)},
		Page:    1, PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(textValue(items[0].Values["name"]))
}

func (s *ProductionCostingCoreService) syncVarianceCase(record document.Record, actorID string, payload map[string]any) error {
	if s.models == nil {
		return nil
	}
	totalVariance := roundMoney(numberValue(payload["total_variance_amount"]))
	existing, _ := s.findVarianceCase(record.Header.ID)
	if totalVariance == 0 {
		if existing.ID == "" {
			return nil
		}
		values := cloneMap(existing.Values)
		values["status"] = "closed"
		values["amount"] = 0.0
		_, err := s.models.Update("production_variance_case", existing.ID, actorID, values, existing.Version)
		return err
	}
	values := map[string]any{
		"organization_id":     record.Header.OrganizationID,
		"location_id":         record.Header.LocationID,
		"production_order_id": record.Header.ID,
		"order_number":        firstNonEmptyString(record.Header.Number, record.Header.ID),
		"finished_item_code":  textValue(payload["finished_item_code"]),
		"variance_type":       "total",
		"amount":              totalVariance,
		"status":              "open",
		"notes":               "Auto-synced from production costing",
	}
	if existing.ID != "" {
		_, err := s.models.Update("production_variance_case", existing.ID, actorID, mergeRecordValues(existing.Values, values), existing.Version)
		return err
	}
	_, err := s.models.Create("production_variance_case", actorID, values)
	return err
}

func (s *ProductionCostingCoreService) findVarianceCase(orderID string) (model.Record, bool) {
	if s.models == nil {
		return model.Record{}, false
	}
	items, _, err := s.models.List("production_variance_case", model.Query{
		Filters: map[string]string{"production_order_id": strings.TrimSpace(orderID)},
		Page:    1, PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return model.Record{}, false
	}
	return items[0], true
}

func (s *ProductionCostingCoreService) productionSummaryRows(organizationID, locationID string) []ProductionCostingSummaryRow {
	if s.documents == nil {
		return nil
	}
	rows := make([]ProductionCostingSummaryRow, 0)
	for _, record := range s.documents.List() {
		if record.Header.Type != "production_order" || strings.TrimSpace(record.Header.OrganizationID) != strings.TrimSpace(organizationID) {
			continue
		}
		if strings.TrimSpace(locationID) != "" && strings.TrimSpace(record.Header.LocationID) != strings.TrimSpace(locationID) {
			continue
		}
		payload := clonedPayload(record.Body.Payload)
		rows = append(rows, ProductionCostingSummaryRow{
			ProductionOrderID:      record.Header.ID,
			OrderNumber:            firstNonEmptyString(record.Header.Number, record.Header.ID),
			Status:                 record.Header.Status,
			FinishedItemCode:       textValue(payload["finished_item_code"]),
			FinishedItemName:       textValue(payload["finished_item_name"]),
			PlannedQuantity:        roundMoney(numberValue(payload["planned_quantity"])),
			ActualOutputQuantity:   roundMoney(numberValue(payload["actual_output_quantity"])),
			StandardMaterialCost:   roundMoney(numberValue(payload["standard_material_cost_total"])),
			StandardLaborCost:      roundMoney(numberValue(payload["standard_labor_cost_total"])),
			StandardOverheadCost:   roundMoney(numberValue(payload["standard_overhead_cost_total"])),
			StandardTotalCost:      roundMoney(numberValue(payload["standard_total_cost"])),
			ActualMaterialCost:     roundMoney(numberValue(payload["actual_material_cost_total"])),
			ActualLaborCost:        roundMoney(numberValue(payload["actual_labor_cost_total"])),
			ActualOverheadCost:     roundMoney(numberValue(payload["actual_overhead_cost_total"])),
			ActualTotalCost:        roundMoney(numberValue(payload["actual_total_cost"])),
			UnitStandardCost:       roundMoney(numberValue(payload["unit_standard_cost"])),
			UnitActualCost:         roundMoney(numberValue(payload["unit_actual_cost"])),
			MaterialVarianceAmount: roundMoney(numberValue(payload["material_variance_amount"])),
			LaborVarianceAmount:    roundMoney(numberValue(payload["labor_variance_amount"])),
			OverheadVarianceAmount: roundMoney(numberValue(payload["overhead_variance_amount"])),
			YieldVarianceAmount:    roundMoney(numberValue(payload["yield_variance_amount"])),
			TotalVarianceAmount:    roundMoney(numberValue(payload["total_variance_amount"])),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].OrderNumber < rows[j].OrderNumber
	})
	return rows
}

func (s *ProductionCostingCoreService) updateDocumentPayload(record document.Record, actorID string, payload map[string]any) error {
	current, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = current
	}
	record.Body.Payload = clonedPayload(payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.UpdatedBy = actorID
	return s.documents.Save(record)
}

func (s *ProductionCostingCoreService) finalizeSystemPosting(record document.Record, actorID, status string) error {
	next, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = next
	}
	record.Header.Status = strings.TrimSpace(status)
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.UpdatedBy = actorID
	return s.documents.Save(record)
}
