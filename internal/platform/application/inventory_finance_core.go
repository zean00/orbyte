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

type InventoryValuationRow struct {
	ItemCode         string  `json:"item_code"`
	ItemName         string  `json:"item_name"`
	WarehouseCode    string  `json:"warehouse_code"`
	AccountCode      string  `json:"account_code"`
	AccountName      string  `json:"account_name"`
	QuantityOnHand   float64 `json:"quantity_on_hand"`
	AverageUnitCost  float64 `json:"average_unit_cost"`
	InventoryValue   float64 `json:"inventory_value"`
	LastCalculatedAt string  `json:"last_calculated_at,omitempty"`
}

type InventoryValuationReport struct {
	OrganizationID string                  `json:"organization_id"`
	LocationID     string                  `json:"location_id,omitempty"`
	AsOfDate       string                  `json:"as_of_date,omitempty"`
	Totals         map[string]float64      `json:"totals"`
	Rows           []InventoryValuationRow `json:"rows"`
}

type InventoryGLReconciliationAccountRow struct {
	AccountCode    string  `json:"account_code"`
	AccountName    string  `json:"account_name"`
	InventoryValue float64 `json:"inventory_value"`
	GLValue        float64 `json:"gl_value"`
	Difference     float64 `json:"difference"`
}

type InventoryGLReconciliationMismatch struct {
	AccountCode    string  `json:"account_code"`
	AccountName    string  `json:"account_name"`
	InventoryValue float64 `json:"inventory_value"`
	GLValue        float64 `json:"gl_value"`
	Difference     float64 `json:"difference"`
	Reason         string  `json:"reason"`
}

type InventoryGLReconciliationReport struct {
	OrganizationID string                                `json:"organization_id"`
	LocationID     string                                `json:"location_id,omitempty"`
	AsOfDate       string                                `json:"as_of_date,omitempty"`
	InventoryTotal float64                               `json:"inventory_total"`
	GLTotal        float64                               `json:"gl_total"`
	Difference     float64                               `json:"difference"`
	Accounts       []InventoryGLReconciliationAccountRow `json:"accounts"`
	Mismatches     []InventoryGLReconciliationMismatch   `json:"mismatches"`
}

type InventoryAdjustmentReviewItem struct {
	DocumentID            string  `json:"document_id"`
	DocumentNumber        string  `json:"document_number"`
	Status                string  `json:"status"`
	WarehouseCode         string  `json:"warehouse_code"`
	LineCount             int     `json:"line_count"`
	QuantityDeltaTotal    float64 `json:"quantity_delta_total"`
	EstimatedValueImpact  float64 `json:"estimated_value_impact"`
	AdjustmentAccountCode string  `json:"adjustment_account_code"`
	FinanceReviewRequired bool    `json:"finance_review_required"`
	CreatedBy             string  `json:"created_by"`
	CountSessionID        string  `json:"count_session_id,omitempty"`
	CountSessionNumber    string  `json:"count_session_number,omitempty"`
}

type InventoryAdjustmentReviewReport struct {
	OrganizationID string                          `json:"organization_id"`
	LocationID     string                          `json:"location_id,omitempty"`
	Totals         map[string]float64              `json:"totals"`
	Items          []InventoryAdjustmentReviewItem `json:"items"`
}

type InventoryFinanceCoreService struct {
	documents *document.Service
	models    *model.Service
	inventory *InventoryCoreService
	finance   *FinanceReportingCoreService
}

func NewInventoryFinanceCoreService(documents *document.Service, models *model.Service, inventory *InventoryCoreService, finance *FinanceReportingCoreService) *InventoryFinanceCoreService {
	return &InventoryFinanceCoreService{
		documents: documents,
		models:    models,
		inventory: inventory,
		finance:   finance,
	}
}

func (s *InventoryFinanceCoreService) InventoryValuation(organizationID, locationID, warehouseCode, asOfDate string) InventoryValuationReport {
	asOfDate = strings.TrimSpace(asOfDate)
	var rows []InventoryValuationRow
	if asOfDate == "" {
		rows = s.currentValuationRows(organizationID, locationID, warehouseCode)
	} else {
		rows = s.valuationRowsAsOf(organizationID, locationID, warehouseCode, asOfDate)
	}
	report := InventoryValuationReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		AsOfDate:       asOfDate,
		Totals:         map[string]float64{"quantity_on_hand": 0, "inventory_value": 0},
		Rows:           rows,
	}
	for _, row := range rows {
		report.Totals["quantity_on_hand"] = roundMoney(report.Totals["quantity_on_hand"] + row.QuantityOnHand)
		report.Totals["inventory_value"] = roundMoney(report.Totals["inventory_value"] + row.InventoryValue)
	}
	return report
}

func (s *InventoryFinanceCoreService) InventoryGLReconciliation(organizationID, locationID, asOfDate, accountCode string) InventoryGLReconciliationReport {
	valuation := s.InventoryValuation(organizationID, locationID, "", asOfDate)
	accountIndex := map[string]financeAccountMeta{}
	if s.finance != nil {
		accountIndex = s.finance.accountMetaIndex()
	}
	subledgerByAccount := map[string]float64{}
	for _, row := range valuation.Rows {
		if accountCode != "" && row.AccountCode != accountCode {
			continue
		}
		subledgerByAccount[row.AccountCode] = roundMoney(subledgerByAccount[row.AccountCode] + row.InventoryValue)
	}
	relevantAccounts := map[string]struct{}{}
	for code := range subledgerByAccount {
		if code != "" {
			relevantAccounts[code] = struct{}{}
		}
	}
	if accountCode != "" {
		relevantAccounts[accountCode] = struct{}{}
	} else {
		for code, meta := range accountIndex {
			if strings.TrimSpace(code) == "" {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(meta.ReportGroup), "inventory") {
				relevantAccounts[code] = struct{}{}
			}
		}
	}
	glByAccount := s.inventoryGLBalances(organizationID, locationID, firstNonEmptyString(asOfDate, time.Now().UTC().Format("2006-01-02")), accountCode, relevantAccounts)
	report := InventoryGLReconciliationReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		AsOfDate:       firstNonEmptyString(asOfDate, time.Now().UTC().Format("2006-01-02")),
		Accounts:       make([]InventoryGLReconciliationAccountRow, 0),
		Mismatches:     make([]InventoryGLReconciliationMismatch, 0),
	}
	accountCodes := map[string]struct{}{}
	for code := range subledgerByAccount {
		accountCodes[code] = struct{}{}
	}
	for code := range glByAccount {
		accountCodes[code] = struct{}{}
	}
	for code := range accountCodes {
		if code == "" {
			continue
		}
		inventoryValue := roundMoney(subledgerByAccount[code])
		glValue := roundMoney(glByAccount[code])
		diff := roundMoney(inventoryValue - glValue)
		report.InventoryTotal = roundMoney(report.InventoryTotal + inventoryValue)
		report.GLTotal = roundMoney(report.GLTotal + glValue)
		row := InventoryGLReconciliationAccountRow{
			AccountCode:    code,
			AccountName:    s.financeAccountName(accountIndex, code),
			InventoryValue: inventoryValue,
			GLValue:        glValue,
			Difference:     diff,
		}
		report.Accounts = append(report.Accounts, row)
		if diff != 0 {
			reason := "inventory valuation and ledger balances differ"
			switch {
			case inventoryValue == 0 && glValue != 0:
				reason = "ledger balance exists without matching inventory valuation"
			case inventoryValue != 0 && glValue == 0:
				reason = "inventory valuation exists without matching ledger balance"
			}
			report.Mismatches = append(report.Mismatches, InventoryGLReconciliationMismatch{
				AccountCode:    code,
				AccountName:    row.AccountName,
				InventoryValue: inventoryValue,
				GLValue:        glValue,
				Difference:     diff,
				Reason:         reason,
			})
		}
	}
	sort.Slice(report.Accounts, func(i, j int) bool { return report.Accounts[i].AccountCode < report.Accounts[j].AccountCode })
	sort.Slice(report.Mismatches, func(i, j int) bool { return report.Mismatches[i].AccountCode < report.Mismatches[j].AccountCode })
	report.Difference = roundMoney(report.InventoryTotal - report.GLTotal)
	return report
}

func (s *InventoryFinanceCoreService) InventoryAdjustmentReview(organizationID, locationID, status string) InventoryAdjustmentReviewReport {
	report := InventoryAdjustmentReviewReport{
		OrganizationID: organizationID,
		LocationID:     locationID,
		Totals:         map[string]float64{"documents": 0, "quantity_delta_total": 0, "estimated_value_impact": 0},
		Items:          make([]InventoryAdjustmentReviewItem, 0),
	}
	if s.documents == nil {
		return report
	}
	for _, record := range s.documents.List() {
		if record.Header.Type != "stock_adjustment" || !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		if status != "" && record.Header.Status != status {
			continue
		}
		item := s.adjustmentReviewItem(record)
		report.Items = append(report.Items, item)
		report.Totals["documents"]++
		report.Totals["quantity_delta_total"] = roundMoney(report.Totals["quantity_delta_total"] + item.QuantityDeltaTotal)
		report.Totals["estimated_value_impact"] = roundMoney(report.Totals["estimated_value_impact"] + item.EstimatedValueImpact)
	}
	if s.models != nil {
		filters := map[string]string{"organization_id": organizationID}
		if locationID != "" {
			filters["location_id"] = locationID
		}
		if status != "" {
			filters["status"] = status
		}
		items, _, err := s.models.List("inventory_count_session", model.Query{Page: 1, PageSize: 500, Filters: filters})
		if err == nil {
			for _, session := range items {
				if textValue(session.Values["generated_adjustment_id"]) != "" {
					continue
				}
				item := s.adjustmentReviewItemFromSession(session)
				report.Items = append(report.Items, item)
				report.Totals["documents"]++
				report.Totals["quantity_delta_total"] = roundMoney(report.Totals["quantity_delta_total"] + item.QuantityDeltaTotal)
				report.Totals["estimated_value_impact"] = roundMoney(report.Totals["estimated_value_impact"] + item.EstimatedValueImpact)
			}
		}
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].Status != report.Items[j].Status {
			return report.Items[i].Status < report.Items[j].Status
		}
		return report.Items[i].DocumentNumber < report.Items[j].DocumentNumber
	})
	return report
}

func (s *InventoryFinanceCoreService) GenerateAdjustmentFromCountSession(sessionID, actorID, organizationID, locationID string) (document.Record, error) {
	if s.models == nil || s.documents == nil || s.inventory == nil {
		return document.Record{}, shared.NotFound("inventory finance service is not available")
	}
	session, err := s.models.Get("inventory_count_session", strings.TrimSpace(sessionID))
	if err != nil {
		return document.Record{}, err
	}
	if err := s.validateScopedModelRecord(session, organizationID, locationID); err != nil {
		return document.Record{}, err
	}
	if status := textValue(session.Values["status"]); status != "" && status != "open" && status != "draft" {
		return document.Record{}, shared.Validation("count session must be open before generating adjustment")
	}
	if existingID := textValue(session.Values["generated_adjustment_id"]); existingID != "" {
		return s.documents.Get(existingID)
	}
	lines := recordList(session.Values["lines"])
	adjustmentLines := make([]map[string]any, 0)
	warehouseCode := textValue(session.Values["warehouse_code"])
	for _, row := range lines {
		itemCode := textValue(row["item_code"])
		if itemCode == "" {
			continue
		}
		lineWarehouse := firstNonEmptyString(textValue(row["warehouse_code"]), warehouseCode)
		batchCode := textValue(row["batch_code"])
		systemQty := roundMoney(firstPositiveOrZero(row["system_quantity"], s.currentCountSessionSystemQuantity(organizationID, locationID, itemCode, lineWarehouse, batchCode)))
		countedQty := roundMoney(numberValue(row["counted_quantity"]))
		delta := roundMoney(countedQty - systemQty)
		if delta == 0 {
			continue
		}
		unitCost := roundMoney(firstPositiveNumber(row["unit_cost"], s.inventory.CurrentAverageUnitCost(organizationID, locationID, itemCode, lineWarehouse)))
		inventoryAccount, _, _ := s.inventory.CostAccounts(itemCode)
		adjustmentLine := map[string]any{
			"item_code":                    itemCode,
			"warehouse_code":               lineWarehouse,
			"batch_code":                   batchCode,
			"expiration_date":              textValue(row["expiration_date"]),
			"quantity":                     delta,
			"uom_code":                     textValue(row["uom_code"]),
			"system_quantity":              systemQty,
			"counted_quantity":             countedQty,
			"unit_cost":                    unitCost,
			"inventory_asset_account_code": inventoryAccount,
			"adjustment_account_code":      firstNonEmptyString(textValue(row["adjustment_account_code"]), textValue(session.Values["adjustment_account_code"]), "5800-INV-ADJ"),
		}
		adjustmentLines = append(adjustmentLines, adjustmentLine)
	}
	if len(adjustmentLines) == 0 {
		return document.Record{}, shared.Validation("count session has no adjustment delta to post")
	}
	payload := s.inventory.NormalizePayload("stock_adjustment", map[string]any{
		"warehouse_code":          warehouseCode,
		"count_session_id":        session.ID,
		"finance_review_required": true,
		"adjustment_account_code": firstNonEmptyString(textValue(session.Values["adjustment_account_code"]), "5800-INV-ADJ"),
		"adjustment_reason":       firstNonEmptyString(textValue(session.Values["adjustment_reason"]), "cycle_count"),
		"lines":                   adjustmentLines,
		"notes":                   fmt.Sprintf("Generated from count session %s", firstNonEmptyString(textValue(session.Values["session_code"]), session.ID)),
	})
	payload = s.previewStockAdjustmentFinancials(organizationID, locationID, payload)
	valueImpact := roundMoney(s.countSessionValueImpact(organizationID, locationID, adjustmentLines))
	if previewed := roundMoney(numberValue(payload["estimated_value_impact"])); previewed != 0 {
		valueImpact = previewed
	}
	payload["estimated_value_impact"] = valueImpact
	record, err := s.documents.Create("stock_adjustment", organizationID, locationID, actorID, payload)
	if err != nil {
		return document.Record{}, err
	}
	updatedValues := mergeModelValues(session.Values, map[string]any{
		"generated_adjustment_id":     record.Header.ID,
		"generated_adjustment_number": firstNonEmptyString(record.Header.Number, record.Header.ID),
		"status":                      "generated",
	})
	if _, err := s.models.Update("inventory_count_session", session.ID, actorID, updatedValues, session.Version); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (s *InventoryFinanceCoreService) OpenReconciliationCase(organizationID, locationID, asOfDate, accountCode, reason string, inventoryValue, glValue float64, actorID string) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.NotFound("models service is not available")
	}
	accountIndex := map[string]financeAccountMeta{}
	if s.finance != nil {
		accountIndex = s.finance.accountMetaIndex()
	}
	return s.models.Create("inventory_reconciliation_case", actorID, map[string]any{
		"organization_id":  organizationID,
		"location_id":      locationID,
		"as_of_date":       normalizeAsOfDate(asOfDate),
		"account_code":     accountCode,
		"account_name":     s.financeAccountName(accountIndex, accountCode),
		"mismatch_type":    "inventory_gl",
		"inventory_value":  roundMoney(inventoryValue),
		"gl_value":         roundMoney(glValue),
		"difference":       roundMoney(inventoryValue - glValue),
		"status":           "pending",
		"assignee_user_id": actorID,
		"note":             reason,
	})
}

func (s *InventoryFinanceCoreService) currentValuationRows(organizationID, locationID, warehouseCode string) []InventoryValuationRow {
	if s.models == nil || s.inventory == nil {
		return nil
	}
	filters := map[string]string{}
	if organizationID != "" {
		filters["organization_id"] = organizationID
	}
	if locationID != "" {
		filters["location_id"] = locationID
	}
	if warehouseCode != "" {
		filters["warehouse_code"] = warehouseCode
	}
	items, _, err := s.models.List("inventory_valuation_snapshot", model.Query{Page: 1, PageSize: 1000, Filters: filters})
	if err != nil {
		return nil
	}
	accountIndex := map[string]financeAccountMeta{}
	if s.finance != nil {
		accountIndex = s.finance.accountMetaIndex()
	}
	rows := make([]InventoryValuationRow, 0, len(items))
	for _, item := range items {
		itemCode := textValue(item.Values["item_code"])
		inventoryAccount, _, _ := s.inventory.CostAccounts(itemCode)
		rows = append(rows, InventoryValuationRow{
			ItemCode:         itemCode,
			ItemName:         s.inventory.lookupItemPolicy(itemCode).Name,
			WarehouseCode:    textValue(item.Values["warehouse_code"]),
			AccountCode:      inventoryAccount,
			AccountName:      s.financeAccountName(accountIndex, inventoryAccount),
			QuantityOnHand:   roundMoney(numberValue(item.Values["quantity_on_hand"])),
			AverageUnitCost:  roundMoney(numberValue(item.Values["average_unit_cost"])),
			InventoryValue:   roundMoney(numberValue(item.Values["inventory_value"])),
			LastCalculatedAt: textValue(item.Values["last_calculated_at"]),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].WarehouseCode != rows[j].WarehouseCode {
			return rows[i].WarehouseCode < rows[j].WarehouseCode
		}
		return rows[i].ItemCode < rows[j].ItemCode
	})
	return rows
}

func (s *InventoryFinanceCoreService) valuationRowsAsOf(organizationID, locationID, warehouseCode, asOfDate string) []InventoryValuationRow {
	if s.models == nil || s.inventory == nil {
		return nil
	}
	asOfDate = normalizeAsOfDate(asOfDate)
	layers, _, err := s.models.List("inventory_cost_layer", model.Query{Page: 1, PageSize: 5000})
	if err != nil {
		return nil
	}
	type totals struct {
		qty   float64
		value float64
	}
	grouped := map[string]*totals{}
	for _, layer := range layers {
		if textValue(layer.Values["status"]) != "" && textValue(layer.Values["status"]) != "posted" {
			continue
		}
		if organizationID != "" && textValue(layer.Values["organization_id"]) != organizationID {
			continue
		}
		if locationID != "" && textValue(layer.Values["location_id"]) != locationID {
			continue
		}
		if warehouseCode != "" && textValue(layer.Values["warehouse_code"]) != warehouseCode {
			continue
		}
		if !inventoryCostLayerEffectiveByDate(textValue(layer.Values["effective_at"]), asOfDate) {
			continue
		}
		key := fmt.Sprintf("%s|%s", textValue(layer.Values["item_code"]), textValue(layer.Values["warehouse_code"]))
		entry := grouped[key]
		if entry == nil {
			entry = &totals{}
			grouped[key] = entry
		}
		entry.qty = roundMoney(entry.qty + numberValue(layer.Values["quantity_delta"]))
		entry.value = roundMoney(entry.value + numberValue(layer.Values["total_cost"]))
	}
	accountIndex := map[string]financeAccountMeta{}
	if s.finance != nil {
		accountIndex = s.finance.accountMetaIndex()
	}
	rows := make([]InventoryValuationRow, 0, len(grouped))
	for key, totals := range grouped {
		parts := strings.SplitN(key, "|", 2)
		itemCode := parts[0]
		rowWarehouse := ""
		if len(parts) > 1 {
			rowWarehouse = parts[1]
		}
		inventoryAccount, _, _ := s.inventory.CostAccounts(itemCode)
		avg := 0.0
		if roundMoney(totals.qty) != 0 {
			avg = roundMoney(totals.value / totals.qty)
		}
		rows = append(rows, InventoryValuationRow{
			ItemCode:        itemCode,
			ItemName:        s.inventory.lookupItemPolicy(itemCode).Name,
			WarehouseCode:   rowWarehouse,
			AccountCode:     inventoryAccount,
			AccountName:     s.financeAccountName(accountIndex, inventoryAccount),
			QuantityOnHand:  roundMoney(totals.qty),
			AverageUnitCost: avg,
			InventoryValue:  roundMoney(totals.value),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].WarehouseCode != rows[j].WarehouseCode {
			return rows[i].WarehouseCode < rows[j].WarehouseCode
		}
		return rows[i].ItemCode < rows[j].ItemCode
	})
	return rows
}

func (s *InventoryFinanceCoreService) inventoryGLBalances(organizationID, locationID, asOfDate, accountCode string, relevantAccounts map[string]struct{}) map[string]float64 {
	result := map[string]float64{}
	if s.documents == nil {
		return result
	}
	asOfDate = normalizeAsOfDate(asOfDate)
	for _, record := range s.documents.List() {
		if record.Header.Type != "ledger_posting" || record.Header.Status != "posted" {
			continue
		}
		if !matchesCommercialScope(record, organizationID, locationID) {
			continue
		}
		postingDate := textValue(record.Body.Payload["posting_date"])
		if postingDate == "" || postingDate > asOfDate {
			continue
		}
		for _, line := range recordList(record.Body.Payload["journal_lines"]) {
			code := textValue(line["account_code"])
			if code == "" {
				continue
			}
			if accountCode != "" && code != accountCode {
				continue
			}
			if len(relevantAccounts) > 0 {
				if _, ok := relevantAccounts[code]; !ok {
					continue
				}
			}
			result[code] = roundMoney(result[code] + numberValue(line["debit"]) - numberValue(line["credit"]))
		}
	}
	return result
}

func (s *InventoryFinanceCoreService) adjustmentReviewItem(record document.Record) InventoryAdjustmentReviewItem {
	payload := clonedPayload(record.Body.Payload)
	warehouseCode := firstNonEmptyString(textValue(payload["warehouse_code"]), firstWarehouseCode(recordList(payload["lines"])))
	item := InventoryAdjustmentReviewItem{
		DocumentID:            record.Header.ID,
		DocumentNumber:        firstNonEmptyString(record.Header.Number, record.Header.ID),
		Status:                record.Header.Status,
		WarehouseCode:         warehouseCode,
		LineCount:             len(recordList(payload["lines"])),
		AdjustmentAccountCode: firstNonEmptyString(textValue(payload["adjustment_account_code"]), "5800-INV-ADJ"),
		FinanceReviewRequired: boolValue(payload["finance_review_required"]) || textValue(payload["count_session_id"]) != "",
		CreatedBy:             record.Header.CreatedBy,
		CountSessionID:        textValue(payload["count_session_id"]),
	}
	if item.CountSessionID != "" && s.models != nil {
		if session, err := s.models.Get("inventory_count_session", item.CountSessionID); err == nil {
			item.CountSessionNumber = firstNonEmptyString(textValue(session.Values["session_code"]), session.ID)
		}
	}
	for _, line := range recordList(payload["lines"]) {
		qty := roundMoney(numberValue(line["quantity"]))
		item.QuantityDeltaTotal = roundMoney(item.QuantityDeltaTotal + qty)
		unitCost := roundMoney(firstPositiveNumber(line["unit_cost"], s.inventory.CurrentAverageUnitCost(record.Header.OrganizationID, record.Header.LocationID, textValue(line["item_code"]), textValue(line["warehouse_code"]))))
		item.EstimatedValueImpact = roundMoney(item.EstimatedValueImpact + qty*unitCost)
	}
	return item
}

func (s *InventoryFinanceCoreService) adjustmentReviewItemFromSession(session model.Record) InventoryAdjustmentReviewItem {
	item := InventoryAdjustmentReviewItem{
		DocumentID:            "",
		DocumentNumber:        firstNonEmptyString(textValue(session.Values["session_code"]), session.ID),
		Status:                textValue(session.Values["status"]),
		WarehouseCode:         textValue(session.Values["warehouse_code"]),
		AdjustmentAccountCode: firstNonEmptyString(textValue(session.Values["adjustment_account_code"]), "5800-INV-ADJ"),
		FinanceReviewRequired: true,
		CreatedBy:             session.UpdatedBy,
		CountSessionID:        session.ID,
		CountSessionNumber:    firstNonEmptyString(textValue(session.Values["session_code"]), session.ID),
	}
	for _, line := range recordList(session.Values["lines"]) {
		item.LineCount++
		systemQty := roundMoney(firstPositiveOrZero(line["system_quantity"], s.currentCountSessionSystemQuantity(textValue(session.Values["organization_id"]), textValue(session.Values["location_id"]), textValue(line["item_code"]), firstNonEmptyString(textValue(line["warehouse_code"]), item.WarehouseCode), textValue(line["batch_code"]))))
		countedQty := roundMoney(numberValue(line["counted_quantity"]))
		delta := roundMoney(countedQty - systemQty)
		item.QuantityDeltaTotal = roundMoney(item.QuantityDeltaTotal + delta)
		unitCost := roundMoney(firstPositiveNumber(line["unit_cost"], s.inventory.CurrentAverageUnitCost(textValue(session.Values["organization_id"]), textValue(session.Values["location_id"]), textValue(line["item_code"]), firstNonEmptyString(textValue(line["warehouse_code"]), item.WarehouseCode))))
		item.EstimatedValueImpact = roundMoney(item.EstimatedValueImpact + delta*unitCost)
	}
	return item
}

func (s *InventoryFinanceCoreService) countSessionValueImpact(organizationID, locationID string, lines []map[string]any) float64 {
	total := 0.0
	for _, line := range lines {
		qty := roundMoney(numberValue(line["quantity"]))
		unitCost := roundMoney(firstPositiveNumber(line["unit_cost"], s.inventory.CurrentAverageUnitCost(organizationID, locationID, textValue(line["item_code"]), textValue(line["warehouse_code"]))))
		total = roundMoney(total + qty*unitCost)
	}
	return total
}

func (s *InventoryFinanceCoreService) previewStockAdjustmentFinancials(organizationID, locationID string, payload map[string]any) map[string]any {
	next := clonedPayload(payload)
	lines := make([]map[string]any, 0, len(recordList(next["lines"])))
	valueImpact := 0.0
	journalLines := make([]map[string]any, 0)
	debits := map[string]float64{}
	credits := map[string]float64{}
	for _, line := range recordList(next["lines"]) {
		row := cloneMap(line)
		itemCode := textValue(row["item_code"])
		warehouseCode := textValue(row["warehouse_code"])
		unitCost := roundMoney(firstPositiveNumber(row["unit_cost"], s.inventory.CurrentAverageUnitCost(organizationID, locationID, itemCode, warehouseCode)))
		row["unit_cost"] = unitCost
		qty := roundMoney(numberValue(row["quantity"]))
		totalCost := roundMoney(qty * unitCost)
		row["estimated_value_impact"] = totalCost
		if textValue(row["inventory_asset_account_code"]) == "" {
			inventoryAccount, _, _ := s.inventory.CostAccounts(itemCode)
			row["inventory_asset_account_code"] = inventoryAccount
		}
		if textValue(row["adjustment_account_code"]) == "" {
			row["adjustment_account_code"] = firstNonEmptyString(textValue(next["adjustment_account_code"]), "5800-INV-ADJ")
		}
		absCost := roundMoney(absFloat(totalCost))
		if absCost > 0 {
			inventoryAccount := firstNonEmptyString(textValue(row["inventory_asset_account_code"]), "1200-INV")
			adjustmentAccount := firstNonEmptyString(textValue(row["adjustment_account_code"]), "5800-INV-ADJ")
			if totalCost >= 0 {
				debits[inventoryAccount] = roundMoney(debits[inventoryAccount] + absCost)
				credits[adjustmentAccount] = roundMoney(credits[adjustmentAccount] + absCost)
			} else {
				debits[adjustmentAccount] = roundMoney(debits[adjustmentAccount] + absCost)
				credits[inventoryAccount] = roundMoney(credits[inventoryAccount] + absCost)
			}
		}
		valueImpact = roundMoney(valueImpact + totalCost)
		lines = append(lines, row)
	}
	for account, amount := range debits {
		journalLines = append(journalLines, map[string]any{"account_code": account, "description": "Inventory adjustment", "debit": amount, "credit": 0.0})
	}
	for account, amount := range credits {
		journalLines = append(journalLines, map[string]any{"account_code": account, "description": "Inventory adjustment", "debit": 0.0, "credit": amount})
	}
	sort.Slice(journalLines, func(i, j int) bool {
		return textValue(journalLines[i]["account_code"]) < textValue(journalLines[j]["account_code"])
	})
	next["lines"] = lines
	next["estimated_value_impact"] = valueImpact
	next["preview_journal_lines"] = journalLines
	return next
}

func (s *InventoryFinanceCoreService) currentCountSessionSystemQuantity(organizationID, locationID, itemCode, warehouseCode, batchCode string) float64 {
	if s.inventory == nil {
		return 0
	}
	return s.inventory.CurrentOnHandQuantity(organizationID, locationID, itemCode, warehouseCode, batchCode)
}

func (s *InventoryFinanceCoreService) validateScopedModelRecord(record model.Record, organizationID, locationID string) error {
	if organizationID != "" && textValue(record.Values["organization_id"]) != organizationID {
		return shared.Forbidden("record organization is not allowed")
	}
	if locationID != "" && textValue(record.Values["location_id"]) != "" && textValue(record.Values["location_id"]) != locationID {
		return shared.Forbidden("record location is not allowed")
	}
	return nil
}

func (s *InventoryFinanceCoreService) financeAccountName(index map[string]financeAccountMeta, accountCode string) string {
	if meta, ok := index[accountCode]; ok {
		return firstNonEmptyString(meta.Name, accountCode)
	}
	return accountCode
}

func firstWarehouseCode(lines []map[string]any) string {
	for _, line := range lines {
		if warehouse := textValue(line["warehouse_code"]); warehouse != "" {
			return warehouse
		}
	}
	return ""
}

func firstPositiveOrZero(values ...any) float64 {
	for _, value := range values {
		number := roundMoney(numberValue(value))
		if number >= 0 {
			return number
		}
	}
	return 0
}

func inventoryCostLayerEffectiveByDate(effectiveAt, asOfDate string) bool {
	if effectiveAt == "" {
		return true
	}
	if parsed, err := time.Parse(time.RFC3339, effectiveAt); err == nil {
		return parsed.Format("2006-01-02") <= asOfDate
	}
	return effectiveAt <= asOfDate
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
