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

type ProductionCoreService struct {
	documents *document.Service
	models    *model.Service
	search    *search.Service
	inventory *InventoryCoreService
	finance   *FinanceReportingCoreService
}

func NewProductionCoreService(documents *document.Service, models *model.Service, searchSvc *search.Service, inventorySvc *InventoryCoreService) *ProductionCoreService {
	return &ProductionCoreService{
		documents: documents,
		models:    models,
		search:    searchSvc,
		inventory: inventorySvc,
	}
}

func (s *ProductionCoreService) SetFinanceReporting(finance *FinanceReportingCoreService) {
	s.finance = finance
}

func (s *ProductionCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	switch strings.TrimSpace(documentType) {
	case "production_order":
		next["production_pattern"] = firstNonEmptyString(textValue(next["production_pattern"]), "make_to_stock")
		next["planned_quantity"] = roundMoney(numberValue(next["planned_quantity"]))
		if roundMoney(numberValue(next["expected_output_quantity"])) <= 0 {
			next["expected_output_quantity"] = roundMoney(numberValue(next["planned_quantity"]))
		} else {
			next["expected_output_quantity"] = roundMoney(numberValue(next["expected_output_quantity"]))
		}
		next["actual_output_quantity"] = roundMoney(numberValue(next["actual_output_quantity"]))
		next["reserved_quantity_total"] = roundMoney(numberValue(next["reserved_quantity_total"]))
		next["shortage_quantity_total"] = roundMoney(numberValue(next["shortage_quantity_total"]))
		next["waste_quantity"] = roundMoney(numberValue(next["waste_quantity"]))
		next["status_summary"] = firstNonEmptyString(textValue(next["status_summary"]), "planned")
		lines := s.normalizeProductionComponentLines(recordList(next["lines"]))
		if len(lines) == 0 {
			lines = s.componentPlanLines(next)
		}
		next["lines"] = lines
		stages := s.normalizeProductionStages(recordList(next["stages"]))
		if len(stages) == 0 {
			stages = s.stagePlanLines(next)
		}
		next["stages"] = stages
	case "production_issue":
		next["total_quantity"] = roundMoney(numberValue(next["total_quantity"]))
		next["lines"] = s.normalizeProductionIssueRecordLines(recordList(next["lines"]))
		next["total_quantity"] = roundMoney(sumInventoryLineQuantity(recordList(next["lines"]), "quantity"))
	case "production_output":
		next["output_quantity"] = roundMoney(numberValue(next["output_quantity"]))
		next["waste_quantity"] = roundMoney(numberValue(next["waste_quantity"]))
		next["production_lot_code"] = textValue(next["production_lot_code"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["finished_item_code"] = textValue(next["finished_item_code"])
		next["uom_code"] = textValue(next["uom_code"])
		next["expiration_date"] = textValue(next["expiration_date"])
	}
	return next
}

func (s *ProductionCoreService) GenerateProductionOrdersFromSalesOrder(orderID, actorID string) ([]document.Record, error) {
	order, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return nil, err
	}
	if order.Header.Type != "sales_order" {
		return nil, shared.Validation("source document must be a sales order")
	}
	if order.Header.Status != "confirmed" && order.Header.Status != "approved" {
		return nil, shared.Conflict("production can only be generated from an approved sales order")
	}
	payload := clonedPayload(order.Body.Payload)
	lines := recordList(payload["lines"])
	created := make([]document.Record, 0)
	now := time.Now().UTC()
	for index, line := range lines {
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			continue
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			continue
		}
		bom, version, ok := s.resolveDefaultBOMForItem(itemCode)
		if !ok {
			continue
		}
		quantity := roundMoney(numberValue(line["quantity"]))
		if quantity <= 0 {
			continue
		}
		componentLines := s.componentPlanLines(map[string]any{
			"bom_id":             bom.ID,
			"bom_version_id":     version.ID,
			"planned_quantity":   quantity,
			"warehouse_code":     "",
			"finished_item_code": itemCode,
		})
		warehouseCode := firstNonEmptyString(
			textValue(line["warehouse_code"]),
			textValue(s.lookupItemValues(itemCode)["default_replenishment_warehouse_code"]),
		)
		if warehouseCode == "" && len(componentLines) > 0 {
			warehouseCode = textValue(componentLines[0]["warehouse_code"])
		}
		orderPayload := map[string]any{
			"production_pattern":       "make_to_order",
			"finished_item_code":       itemCode,
			"finished_item_name":       firstNonEmptyString(textValue(line["description"]), policy.Name),
			"product_code":             textValue(line["product_code"]),
			"variant_signature":        textValue(line["variant_signature"]),
			"bom_id":                   bom.ID,
			"bom_code":                 textValue(bom.Values["code"]),
			"bom_version_id":           version.ID,
			"bom_version_code":         textValue(version.Values["version_code"]),
			"planned_quantity":         quantity,
			"expected_output_quantity": quantity,
			"actual_output_quantity":   0.0,
			"waste_quantity":           0.0,
			"warehouse_code":           warehouseCode,
			"source_sales_order_id":    order.Header.ID,
			"source_sales_order_number": firstNonEmptyString(
				order.Header.Number,
				order.Header.ID,
			),
			"source_sales_order_line_index": index,
			"planned_date":                  now.Format("2006-01-02"),
			"status_summary":                "planned",
			"notes":                         fmt.Sprintf("Generated from sales order %s", firstNonEmptyString(order.Header.Number, order.Header.ID)),
		}
		if len(componentLines) == 0 {
			componentLines = s.componentPlanLines(orderPayload)
		}
		orderPayload["lines"] = componentLines
		record, createErr := s.documents.Create("production_order", order.Header.OrganizationID, order.Header.LocationID, actorID, orderPayload)
		if createErr != nil {
			return nil, createErr
		}
		if _, createErr = s.documents.AddLink(record.Header.ID, order.Header.ID, "production_for", map[string]any{"source_type": "sales_order", "source_line_index": index}); createErr != nil && !isConflict(createErr) {
			return nil, createErr
		}
		if _, createErr = s.documents.AddLink(order.Header.ID, record.Header.ID, "production_for", map[string]any{"generated_document_type": "production_order", "source_line_index": index}); createErr != nil && !isConflict(createErr) {
			return nil, createErr
		}
		linkedRecord, getErr := s.documents.Get(record.Header.ID)
		if getErr == nil {
			record = linkedRecord
		}
		created = append(created, record)
	}
	if len(created) == 0 {
		return nil, shared.Validation("sales order has no inventory-tracked lines with an active BOM")
	}
	if err := s.refreshDocuments(append(created, order)...); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *ProductionCoreService) CreateProductionIssueFromOrder(orderID, actorID string) (document.Record, error) {
	record, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return document.Record{}, err
	}
	if record.Header.Type != "production_order" {
		return document.Record{}, shared.Validation("source document must be a production order")
	}
	if record.Header.Status != "approved" && record.Header.Status != "in_progress" {
		return document.Record{}, shared.Conflict("production issue can only be created from an approved or in-progress production order")
	}
	if s.hasLinkedDocument(record.Header.ID, "production_issue", "production_for") {
		return document.Record{}, shared.Conflict("production issue already exists for this order")
	}
	payload := clonedPayload(record.Body.Payload)
	lines := s.normalizeProductionIssueLines(recordList(payload["lines"]), textValue(payload["warehouse_code"]))
	if len(lines) == 0 {
		return document.Record{}, shared.Validation("production order has no component lines to issue")
	}
	issuePayload := map[string]any{
		"source_production_order_id":     record.Header.ID,
		"source_production_order_number": firstNonEmptyString(record.Header.Number, record.Header.ID),
		"finished_item_code":             textValue(payload["finished_item_code"]),
		"finished_item_name":             textValue(payload["finished_item_name"]),
		"warehouse_code":                 textValue(payload["warehouse_code"]),
		"issue_date":                     time.Now().UTC().Format("2006-01-02"),
		"total_quantity":                 roundMoney(sumInventoryLineQuantity(lines, "quantity")),
		"lines":                          lines,
		"notes":                          fmt.Sprintf("Generated from production order %s", firstNonEmptyString(record.Header.Number, record.Header.ID)),
	}
	issue, err := s.documents.Create("production_issue", record.Header.OrganizationID, record.Header.LocationID, actorID, issuePayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(issue.Header.ID, record.Header.ID, "production_for", map[string]any{"source_type": "production_order"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, issue.Header.ID, "production_for", map[string]any{"generated_document_type": "production_issue"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	created, err := s.documents.Get(issue.Header.ID)
	if err == nil {
		issue = created
	}
	s.refreshDocuments(issue, record)
	return issue, nil
}

func (s *ProductionCoreService) CreateProductionOutputFromOrder(orderID, actorID string) (document.Record, error) {
	record, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return document.Record{}, err
	}
	if record.Header.Type != "production_order" {
		return document.Record{}, shared.Validation("source document must be a production order")
	}
	if record.Header.Status != "approved" && record.Header.Status != "in_progress" {
		return document.Record{}, shared.Conflict("production output can only be created from an approved or in-progress production order")
	}
	if s.hasLinkedDocument(record.Header.ID, "production_output", "production_for") {
		return document.Record{}, shared.Conflict("production output already exists for this order")
	}
	payload := clonedPayload(record.Body.Payload)
	outputPayload := map[string]any{
		"source_production_order_id":     record.Header.ID,
		"source_production_order_number": firstNonEmptyString(record.Header.Number, record.Header.ID),
		"finished_item_code":             textValue(payload["finished_item_code"]),
		"finished_item_name":             textValue(payload["finished_item_name"]),
		"warehouse_code":                 textValue(payload["warehouse_code"]),
		"output_date":                    time.Now().UTC().Format("2006-01-02"),
		"output_quantity":                roundMoney(firstPositiveNumber(payload["expected_output_quantity"], payload["planned_quantity"])),
		"waste_quantity":                 roundMoney(numberValue(payload["waste_quantity"])),
		"uom_code":                       s.inventory.lookupItemPolicy(textValue(payload["finished_item_code"])).UOMCode,
		"production_lot_code":            "",
		"expiration_date":                "",
		"notes":                          fmt.Sprintf("Generated from production order %s", firstNonEmptyString(record.Header.Number, record.Header.ID)),
	}
	output, err := s.documents.Create("production_output", record.Header.OrganizationID, record.Header.LocationID, actorID, outputPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(output.Header.ID, record.Header.ID, "production_for", map[string]any{"source_type": "production_order"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, output.Header.ID, "production_for", map[string]any{"generated_document_type": "production_output"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	created, err := s.documents.Get(output.Header.ID)
	if err == nil {
		output = created
	}
	s.refreshDocuments(output, record)
	return output, nil
}

func (s *ProductionCoreService) ValidateApprove(record document.Record) error {
	switch record.Header.Type {
	case "production_order":
		return s.validateProductionOrder(record)
	case "production_issue":
		return s.validateProductionIssue(record)
	case "production_output":
		return s.validateProductionOutput(record)
	default:
		return nil
	}
}

func (s *ProductionCoreService) ValidateCancel(record document.Record) error {
	switch record.Header.Type {
	case "production_order", "production_issue", "production_output":
		return nil
	default:
		return nil
	}
}

func (s *ProductionCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "production_issue":
		return s.handleApprovedProductionIssue(record, actorID)
	case "production_output":
		return s.handleApprovedProductionOutput(record, actorID)
	case "production_order":
		if err := s.applyProductionReservations(record, actorID); err != nil {
			return err
		}
		return s.refreshDocuments(record)
	default:
		return nil
	}
}

func (s *ProductionCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	current, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = current
	}
	switch record.Header.Type {
	case "production_issue":
		if err := s.inventory.reverseMovements(record, actorID, "production_issue", "production_issue_reversal"); err != nil {
			return err
		}
		if err := s.createReversalPosting(record, actorID, "production_issue_wip", "production_issue_wip_reversal", "production_issue_wip_reversal_default"); err != nil {
			return err
		}
		return s.refreshLinkedProductionOrder(record, actorID)
	case "production_output":
		if err := s.inventory.reverseMovements(record, actorID, "production_output", "production_output_reversal"); err != nil {
			return err
		}
		if err := s.createReversalPosting(record, actorID, "production_output_wip_clear", "production_output_wip_clear_reversal", "production_output_wip_clear_reversal_default"); err != nil {
			return err
		}
		return s.refreshLinkedProductionOrder(record, actorID)
	default:
		return nil
	}
}

func (s *ProductionCoreService) validateProductionOrder(record document.Record) error {
	payload := clonedPayload(record.Body.Payload)
	if textValue(payload["finished_item_code"]) == "" {
		return shared.Validation("finished item is required")
	}
	finishedPolicy := s.inventory.lookupItemPolicy(textValue(payload["finished_item_code"]))
	if !finishedPolicy.Enabled {
		return shared.Validation("finished item must be inventory-tracked")
	}
	if roundMoney(numberValue(payload["planned_quantity"])) <= 0 {
		return shared.Validation("planned quantity must be greater than zero")
	}
	if len(recordList(payload["lines"])) == 0 {
		return shared.Validation("production order lines are required")
	}
	if textValue(payload["production_pattern"]) == "make_to_order" && textValue(payload["source_sales_order_id"]) == "" {
		return shared.Validation("make-to-order production requires a source sales order")
	}
	return nil
}

func (s *ProductionCoreService) validateProductionIssue(record document.Record) error {
	for _, line := range recordList(record.Body.Payload["lines"]) {
		plannedItemCode := firstNonEmptyString(textValue(line["planned_item_code"]), textValue(line["component_item_code"]))
		actualItemCode := firstNonEmptyString(textValue(line["actual_item_code"]), textValue(line["item_code"]), plannedItemCode)
		allowedSubstitutes := normalizeStringList(line["allowed_substitute_item_codes"])
		if actualItemCode != "" && plannedItemCode != "" && actualItemCode != plannedItemCode && !containsString(allowedSubstitutes, actualItemCode) {
			return shared.Validation(fmt.Sprintf("item %s is not an approved substitute for component %s", actualItemCode, plannedItemCode))
		}
	}
	lines, err := s.inventory.resolveIssueMovementLines(record.Header.OrganizationID, record.Header.LocationID, record.Body.Payload)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return shared.Validation("production issue lines are required")
	}
	return nil
}

func (s *ProductionCoreService) validateProductionOutput(record document.Record) error {
	payload := clonedPayload(record.Body.Payload)
	itemCode := textValue(payload["finished_item_code"])
	if itemCode == "" {
		return shared.Validation("finished item is required")
	}
	if textValue(payload["warehouse_code"]) == "" {
		return shared.Validation("warehouse code is required")
	}
	if roundMoney(numberValue(payload["output_quantity"])) <= 0 {
		return shared.Validation("output quantity must be greater than zero")
	}
	if textValue(payload["production_lot_code"]) == "" {
		return shared.Validation("production lot code is required")
	}
	policy := s.inventory.lookupItemPolicy(itemCode)
	if !policy.Enabled {
		return shared.Validation("finished item must be inventory-tracked")
	}
	stageRows := recordList(payload["stages"])
	if len(stageRows) == 0 {
		if sourceOrderID := textValue(payload["source_production_order_id"]); sourceOrderID != "" {
			if sourceOrder, err := s.documents.Get(sourceOrderID); err == nil {
				stageRows = recordList(sourceOrder.Body.Payload["stages"])
			}
		}
	}
	for _, stage := range stageRows {
		if !boolValue(stage["required"]) {
			continue
		}
		status := textValue(stage["status"])
		if status != "" && status != "completed" && status != "skipped" {
			return shared.Validation("all required production stages must be completed before output")
		}
	}
	if policy.ExpiryTracking && textValue(payload["expiration_date"]) == "" {
		return shared.Validation("expiration date is required for the finished item")
	}
	return nil
}

func (s *ProductionCoreService) handleApprovedProductionIssue(record document.Record, actorID string) error {
	if s.inventory.hasMovementLink(record, "production_issue") {
		return s.refreshLinkedProductionOrder(record, actorID)
	}
	movementLines, err := s.inventory.resolveIssueMovementLines(record.Header.OrganizationID, record.Header.LocationID, record.Body.Payload)
	if err != nil {
		return err
	}
	payload := clonedPayload(record.Body.Payload)
	costedLines := make([]map[string]any, 0, len(movementLines))
	issueCostTotal := 0.0
	for _, line := range movementLines {
		costed := s.inventory.prepareMovementLineForCost(record, "production_issue", line, "out")
		if err := s.inventory.createMovement(record, actorID, "production_issue", costed, "out"); err != nil {
			return err
		}
		costedLines = append(costedLines, costed)
		issueCostTotal = roundMoney(issueCostTotal + numberValue(costed["extended_cost"]))
	}
	payload["lines"] = costedLines
	payload["total_quantity"] = roundMoney(sumInventoryLineQuantity(costedLines, "quantity"))
	payload["issued_quantity_total"] = roundMoney(sumInventoryLineQuantity(costedLines, "quantity"))
	payload["issued_material_cost_total"] = issueCostTotal
	if err := s.updateDocumentPayload(record, actorID, payload); err != nil {
		return err
	}
	if err := s.createProductionIssueCostPosting(record, actorID, costedLines); err != nil {
		return err
	}
	return s.refreshLinkedProductionOrder(record, actorID)
}

func (s *ProductionCoreService) handleApprovedProductionOutput(record document.Record, actorID string) error {
	if s.inventory.hasMovementLink(record, "production_output") {
		return s.refreshLinkedProductionOrder(record, actorID)
	}
	payload := clonedPayload(record.Body.Payload)
	itemPolicy := s.inventory.lookupItemPolicy(textValue(payload["finished_item_code"]))
	line := map[string]any{
		"item_code":       textValue(payload["finished_item_code"]),
		"description":     textValue(payload["finished_item_name"]),
		"warehouse_code":  textValue(payload["warehouse_code"]),
		"expiration_date": textValue(payload["expiration_date"]),
		"quantity":        roundMoney(numberValue(payload["output_quantity"])),
		"uom_code":        textValue(payload["uom_code"]),
		"note":            firstNonEmptyString(textValue(payload["notes"]), textValue(payload["production_lot_code"])),
	}
	totalProductionCost := s.productionOutputTotalCost(payload)
	outputQty := roundMoney(numberValue(payload["output_quantity"]))
	if outputQty > 0 && totalProductionCost > 0 {
		line["unit_cost"] = roundMoney(totalProductionCost / outputQty)
		line["total_cost"] = totalProductionCost
	}
	if strings.EqualFold(itemPolicy.TrackingMode, "batch") {
		line["batch_code"] = textValue(payload["production_lot_code"])
	}
	if err := s.inventory.validateInventoryLine(line, true); err != nil {
		return err
	}
	line = s.inventory.prepareMovementLineForCost(record, "production_output", line, "in")
	if err := s.inventory.createMovement(record, actorID, "production_output", line, "in"); err != nil {
		return err
	}
	payload["output_unit_cost"] = roundMoney(numberValue(line["unit_cost"]))
	payload["total_production_cost"] = totalProductionCost
	if err := s.updateDocumentPayload(record, actorID, payload); err != nil {
		return err
	}
	if err := s.createProductionOutputCostPosting(record, actorID, payload, line); err != nil {
		return err
	}
	return s.refreshLinkedProductionOrder(record, actorID)
}

func (s *ProductionCoreService) normalizeProductionComponentLines(lines []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		next := cloneMap(line)
		next["component_item_code"] = textValue(next["component_item_code"])
		next["actual_item_code"] = firstNonEmptyString(textValue(next["actual_item_code"]), textValue(next["component_item_code"]))
		next["description"] = textValue(next["description"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["uom_code"] = textValue(next["uom_code"])
		next["quantity_per_unit"] = roundMoney(numberValue(next["quantity_per_unit"]))
		next["quantity"] = roundMoney(numberValue(next["quantity"]))
		next["issued_quantity"] = roundMoney(numberValue(next["issued_quantity"]))
		next["reserved_quantity"] = roundMoney(numberValue(next["reserved_quantity"]))
		next["shortage_quantity"] = roundMoney(numberValue(next["shortage_quantity"]))
		next["available_quantity"] = roundMoney(numberValue(next["available_quantity"]))
		next["allowed_substitute_item_codes"] = normalizeStringList(next["allowed_substitute_item_codes"])
		next["substitution_status"] = firstNonEmptyString(textValue(next["substitution_status"]), "planned")
		next["reservation_status"] = firstNonEmptyString(textValue(next["reservation_status"]), "unreserved")
		normalized = append(normalized, next)
	}
	return normalized
}

func (s *ProductionCoreService) normalizeProductionIssueLines(lines []map[string]any, warehouseCode string) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for index, line := range lines {
		itemCode := firstNonEmptyString(textValue(line["actual_item_code"]), textValue(line["item_code"]), textValue(line["component_item_code"]))
		if itemCode == "" {
			continue
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		qty := roundMoney(numberValue(line["quantity"]))
		if qty <= 0 {
			continue
		}
		normalized = append(normalized, map[string]any{
			"source_component_line_index":   index,
			"planned_item_code":             textValue(line["component_item_code"]),
			"item_code":                     itemCode,
			"actual_item_code":              itemCode,
			"description":                   firstNonEmptyString(textValue(line["description"]), policy.Name),
			"warehouse_code":                firstNonEmptyString(textValue(line["warehouse_code"]), warehouseCode),
			"batch_code":                    textValue(line["batch_code"]),
			"expiration_date":               textValue(line["expiration_date"]),
			"quantity":                      qty,
			"uom_code":                      firstNonEmptyString(textValue(line["uom_code"]), policy.UOMCode),
			"note":                          textValue(line["note"]),
			"available_quantity":            roundMoney(numberValue(line["available_quantity"])),
			"reserved_quantity":             roundMoney(numberValue(line["reserved_quantity"])),
			"allowed_substitute_item_codes": normalizeStringList(line["allowed_substitute_item_codes"]),
			"substitution_status":           textValue(line["substitution_status"]),
		})
	}
	return normalized
}

func (s *ProductionCoreService) normalizeProductionIssueRecordLines(lines []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		next := cloneMap(line)
		next["source_component_line_index"] = int(numberValue(next["source_component_line_index"]))
		next["planned_item_code"] = textValue(next["planned_item_code"])
		next["item_code"] = firstNonEmptyString(textValue(next["item_code"]), textValue(next["actual_item_code"]), textValue(next["planned_item_code"]))
		next["actual_item_code"] = firstNonEmptyString(textValue(next["actual_item_code"]), textValue(next["item_code"]), textValue(next["planned_item_code"]))
		next["description"] = textValue(next["description"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["batch_code"] = textValue(next["batch_code"])
		next["expiration_date"] = textValue(next["expiration_date"])
		next["quantity"] = roundMoney(numberValue(next["quantity"]))
		next["uom_code"] = textValue(next["uom_code"])
		next["note"] = textValue(next["note"])
		next["available_quantity"] = roundMoney(numberValue(next["available_quantity"]))
		next["reserved_quantity"] = roundMoney(numberValue(next["reserved_quantity"]))
		next["allowed_substitute_item_codes"] = normalizeStringList(next["allowed_substitute_item_codes"])
		next["substitution_status"] = firstNonEmptyString(textValue(next["substitution_status"]), "planned")
		normalized = append(normalized, next)
	}
	return normalized
}

func (s *ProductionCoreService) componentPlanLines(payload map[string]any) []map[string]any {
	version, ok := s.resolveBOMVersion(payload)
	if !ok {
		return s.normalizeProductionComponentLines(recordList(payload["lines"]))
	}
	plannedQty := roundMoney(firstPositiveNumber(payload["planned_quantity"], payload["expected_output_quantity"]))
	yieldQty := roundMoney(firstPositiveNumber(version.Values["yield_quantity"], 1))
	scale := 1.0
	if yieldQty > 0 {
		scale = plannedQty / yieldQty
	}
	sourceLines := recordList(version.Values["lines"])
	lines := make([]map[string]any, 0, len(sourceLines))
	for _, line := range sourceLines {
		itemCode := firstNonEmptyString(textValue(line["component_item_code"]), textValue(line["item_code"]))
		if itemCode == "" {
			continue
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		qtyPerUnit := roundMoney(firstPositiveNumber(line["quantity_per_unit"], line["quantity"]))
		lines = append(lines, map[string]any{
			"component_item_code":           itemCode,
			"actual_item_code":              itemCode,
			"description":                   firstNonEmptyString(textValue(line["description"]), policy.Name),
			"warehouse_code":                textValue(line["warehouse_code"]),
			"uom_code":                      firstNonEmptyString(textValue(line["uom_code"]), policy.UOMCode),
			"quantity_per_unit":             qtyPerUnit,
			"quantity":                      roundMoney(qtyPerUnit * scale),
			"issued_quantity":               0.0,
			"reserved_quantity":             0.0,
			"shortage_quantity":             0.0,
			"available_quantity":            0.0,
			"allowed_substitute_item_codes": normalizeStringList(line["allowed_substitute_item_codes"]),
			"substitution_status":           "planned",
			"reservation_status":            "unreserved",
		})
	}
	return s.normalizeProductionComponentLines(lines)
}

func (s *ProductionCoreService) normalizeProductionStages(lines []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for index, line := range lines {
		next := cloneMap(line)
		next["stage_code"] = textValue(next["stage_code"])
		next["stage_name"] = firstNonEmptyString(textValue(next["stage_name"]), textValue(next["stage_code"]))
		sequence := int(numberValue(next["sequence"]))
		if sequence <= 0 {
			sequence = index + 1
		}
		next["sequence"] = sequence
		next["work_center_code"] = textValue(next["work_center_code"])
		next["status"] = firstNonEmptyString(textValue(next["status"]), "pending")
		required, ok := next["required"].(bool)
		if !ok {
			required = true
		}
		next["required"] = required
		next["note"] = textValue(next["note"])
		normalized = append(normalized, next)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return int(numberValue(normalized[i]["sequence"])) < int(numberValue(normalized[j]["sequence"]))
	})
	return normalized
}

func (s *ProductionCoreService) stagePlanLines(payload map[string]any) []map[string]any {
	version, ok := s.resolveBOMVersion(payload)
	if !ok {
		return defaultProductionStages()
	}
	stages := recordList(version.Values["stages"])
	if len(stages) == 0 {
		return defaultProductionStages()
	}
	return s.normalizeProductionStages(stages)
}

func (s *ProductionCoreService) resolveBOMVersion(payload map[string]any) (model.Record, bool) {
	if s.models == nil {
		return model.Record{}, false
	}
	if versionID := textValue(payload["bom_version_id"]); versionID != "" {
		record, err := s.models.Get("production_bom_version", versionID)
		if err == nil {
			return record, true
		}
	}
	filters := map[string]string{}
	if bomID := textValue(payload["bom_id"]); bomID != "" {
		filters["bom_id"] = bomID
	}
	if len(filters) == 0 && textValue(payload["finished_item_code"]) != "" {
		if bom, _, ok := s.resolveDefaultBOMForItem(textValue(payload["finished_item_code"])); ok {
			filters["bom_id"] = bom.ID
		}
	}
	if len(filters) == 0 {
		return model.Record{}, false
	}
	items, _, err := s.models.List("production_bom_version", model.Query{Filters: filters, Page: 1, PageSize: 1000})
	if err != nil || len(items) == 0 {
		return model.Record{}, false
	}
	sort.Slice(items, func(i, j int) bool {
		leftActive := boolValue(items[i].Values["is_active"])
		rightActive := boolValue(items[j].Values["is_active"])
		if leftActive != rightActive {
			return leftActive
		}
		leftStatus := textValue(items[i].Values["status"])
		rightStatus := textValue(items[j].Values["status"])
		if leftStatus != rightStatus {
			return leftStatus == "active"
		}
		return textValue(items[i].Values["version_code"]) > textValue(items[j].Values["version_code"])
	})
	for _, item := range items {
		if strings.EqualFold(textValue(item.Values["status"]), "active") {
			return item, true
		}
	}
	return items[0], true
}

func (s *ProductionCoreService) resolveDefaultBOMForItem(itemCode string) (model.Record, model.Record, bool) {
	if s.models == nil || strings.TrimSpace(itemCode) == "" {
		return model.Record{}, model.Record{}, false
	}
	normalizedItemCode := strings.TrimSpace(itemCode)
	items, _, err := s.models.List("production_bom", model.Query{
		Filters: map[string]string{"finished_item_code": normalizedItemCode},
		Page:    1, PageSize: 1000,
	})
	if err != nil {
		return model.Record{}, model.Record{}, false
	}
	if len(items) == 0 {
		allItems, _, listErr := s.models.List("production_bom", model.Query{Page: 1, PageSize: 1000})
		if listErr != nil {
			return model.Record{}, model.Record{}, false
		}
		for _, item := range allItems {
			if strings.EqualFold(strings.TrimSpace(textValue(item.Values["finished_item_code"])), normalizedItemCode) {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			return model.Record{}, model.Record{}, false
		}
	}
	sort.Slice(items, func(i, j int) bool {
		leftStatus := textValue(items[i].Values["status"])
		rightStatus := textValue(items[j].Values["status"])
		if leftStatus != rightStatus {
			return leftStatus == "active"
		}
		return textValue(items[i].Values["code"]) < textValue(items[j].Values["code"])
	})
	for _, item := range items {
		if !strings.EqualFold(textValue(item.Values["status"]), "active") {
			continue
		}
		version, ok := s.resolveBOMVersion(map[string]any{"bom_id": item.ID})
		if ok {
			return item, version, true
		}
	}
	return model.Record{}, model.Record{}, false
}

func (s *ProductionCoreService) lookupItemValues(itemCode string) map[string]any {
	if s.models == nil || strings.TrimSpace(itemCode) == "" {
		return map[string]any{}
	}
	items, _, err := s.models.List("commercial_item", model.Query{
		Filters:  map[string]string{"sku": strings.TrimSpace(itemCode)},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return map[string]any{}
	}
	return cloneMap(items[0].Values)
}

func (s *ProductionCoreService) hasLinkedDocument(sourceID, documentType, linkType string) bool {
	for _, item := range s.documents.List() {
		if item.Header.Type != documentType || item.Header.Status == "cancelled" || item.Header.Status == "rejected" {
			continue
		}
		for _, link := range item.Links {
			if link.LinkType == linkType && link.LinkedDocumentID == sourceID {
				return true
			}
		}
	}
	return false
}

func (s *ProductionCoreService) refreshLinkedProductionOrder(record document.Record, actorID string) error {
	order, ok := s.findLinkedProductionOrder(record)
	if !ok && strings.TrimSpace(record.Header.ID) != "" {
		fresh, err := s.documents.Get(record.Header.ID)
		if err == nil {
			record = fresh
			order, ok = s.findLinkedProductionOrder(record)
		}
	}
	if !ok {
		return s.refreshDocuments(record)
	}
	payload := clonedPayload(order.Body.Payload)
	issueQty := 0.0
	outputQty := 0.0
	wasteQty := 0.0
	issuedMaterialCostTotal := 0.0
	for _, item := range s.documents.List() {
		if item.Header.Status == "cancelled" || item.Header.Status == "rejected" {
			continue
		}
		linked := false
		for _, link := range item.Links {
			if link.LinkType == "production_for" && link.LinkedDocumentID == order.Header.ID {
				linked = true
				break
			}
		}
		if !linked {
			continue
		}
		switch item.Header.Type {
		case "production_issue":
			issueQty = roundMoney(issueQty + sumInventoryLineQuantity(recordList(item.Body.Payload["lines"]), "quantity"))
			issuedMaterialCostTotal = roundMoney(issuedMaterialCostTotal + numberValue(item.Body.Payload["issued_material_cost_total"]))
		case "production_output":
			outputQty = roundMoney(outputQty + numberValue(item.Body.Payload["output_quantity"]))
			wasteQty = roundMoney(wasteQty + numberValue(item.Body.Payload["waste_quantity"]))
		}
	}
	lines := s.normalizeProductionComponentLines(recordList(payload["lines"]))
	issuedByIndex := map[int]float64{}
	for i := range lines {
		lines[i]["issued_quantity"] = 0.0
	}
	stages := s.normalizeProductionStages(recordList(payload["stages"]))
	payload["lines"] = lines
	payload["actual_output_quantity"] = outputQty
	payload["waste_quantity"] = wasteQty
	payload["issued_material_cost_total"] = issuedMaterialCostTotal
	switch {
	case outputQty > 0:
		stages = completeRemainingStages(stages)
		payload["status_summary"] = "completed"
		order.Header.Status = "completed"
	case issueQty > 0:
		stages = advanceProductionStages(stages, "issue")
		payload["status_summary"] = "in_progress"
		order.Header.Status = "in_progress"
	default:
		payload["status_summary"] = "planned"
		order.Header.Status = "approved"
	}
	for _, item := range s.documents.List() {
		if item.Header.Status == "cancelled" || item.Header.Status == "rejected" || item.Header.Type != "production_issue" {
			continue
		}
		linked := false
		for _, link := range item.Links {
			if link.LinkType == "production_for" && link.LinkedDocumentID == order.Header.ID {
				linked = true
				break
			}
		}
		if !linked {
			continue
		}
		for _, line := range recordList(item.Body.Payload["lines"]) {
			index := int(numberValue(line["source_component_line_index"]))
			issuedByIndex[index] = roundMoney(issuedByIndex[index] + numberValue(line["quantity"]))
		}
	}
	reservedTotal := 0.0
	shortageTotal := 0.0
	issuedTotal := 0.0
	for i := range lines {
		issuedQty := roundMoney(issuedByIndex[i])
		lines[i]["issued_quantity"] = issuedQty
		reservedQty := roundMoney(maxFloat(numberValue(lines[i]["reserved_quantity"])-issuedQty, 0))
		lines[i]["reserved_quantity"] = reservedQty
		lines[i]["shortage_quantity"] = roundMoney(maxFloat(numberValue(lines[i]["quantity"])-issuedQty-reservedQty, 0))
		if issuedQty > 0 {
			lines[i]["reservation_status"] = "issued"
		}
		issuedTotal = roundMoney(issuedTotal + issuedQty)
		reservedTotal = roundMoney(reservedTotal + numberValue(lines[i]["reserved_quantity"]))
		shortageTotal = roundMoney(shortageTotal + numberValue(lines[i]["shortage_quantity"]))
	}
	payload["lines"] = lines
	payload["stages"] = stages
	payload["issued_quantity_total"] = issuedTotal
	payload["reserved_quantity_total"] = reservedTotal
	payload["shortage_quantity_total"] = shortageTotal
	if strings.TrimSpace(actorID) == "" {
		actorID = "user_admin"
	}
	if err := s.updateDocumentPayload(order, actorID, payload); err != nil {
		return err
	}
	updated, err := s.documents.Get(order.Header.ID)
	if err == nil {
		order = updated
	}
	return s.refreshDocuments(record, order)
}

func (s *ProductionCoreService) productionOutputTotalCost(payload map[string]any) float64 {
	if total := roundMoney(numberValue(payload["total_production_cost"])); total > 0 {
		return total
	}
	sourceOrderID := textValue(payload["source_production_order_id"])
	if sourceOrderID == "" {
		return 0
	}
	order, err := s.documents.Get(sourceOrderID)
	if err != nil {
		return 0
	}
	return roundMoney(numberValue(order.Body.Payload["issued_material_cost_total"]))
}

func (s *ProductionCoreService) createProductionIssueCostPosting(record document.Record, actorID string, lines []map[string]any) error {
	if s.hasPostingLink(record, "production_issue_wip") {
		return nil
	}
	debitByAccount := map[string]float64{}
	creditByAccount := map[string]float64{}
	totalCost := 0.0
	for _, line := range lines {
		extendedCost := roundMoney(numberValue(line["extended_cost"]))
		if extendedCost <= 0 {
			continue
		}
		totalCost = roundMoney(totalCost + extendedCost)
		debitByAccount[firstNonEmptyString(textValue(line["wip_account_code"]), "1300-WIP")] = roundMoney(debitByAccount[firstNonEmptyString(textValue(line["wip_account_code"]), "1300-WIP")] + extendedCost)
		creditByAccount[firstNonEmptyString(textValue(line["inventory_asset_account_code"]), "1200-INV")] = roundMoney(creditByAccount[firstNonEmptyString(textValue(line["inventory_asset_account_code"]), "1200-INV")] + extendedCost)
	}
	if totalCost <= 0 {
		return nil
	}
	return s.createProductionPosting(record, actorID, "production_issue_wip_default", "production_issue_wip", totalCost, debitByAccount, creditByAccount, "Auto-posted WIP from production issue")
}

func (s *ProductionCoreService) createProductionOutputCostPosting(record document.Record, actorID string, payload map[string]any, line map[string]any) error {
	if s.hasPostingLink(record, "production_output_wip_clear") {
		return nil
	}
	totalCost := roundMoney(numberValue(payload["total_production_cost"]))
	if totalCost <= 0 {
		return nil
	}
	debitByAccount := map[string]float64{
		firstNonEmptyString(textValue(line["inventory_asset_account_code"]), "1200-INV"): totalCost,
	}
	creditByAccount := map[string]float64{
		firstNonEmptyString(textValue(line["wip_account_code"]), "1300-WIP"): totalCost,
	}
	return s.createProductionPosting(record, actorID, "production_output_wip_clear_default", "production_output_wip_clear", totalCost, debitByAccount, creditByAccount, "Auto-posted finished goods from production output")
}

func (s *ProductionCoreService) createProductionPosting(record document.Record, actorID, postingRuleKey, postingReason string, totalCost float64, debitByAccount, creditByAccount map[string]float64, notePrefix string) error {
	journalLines := make([]map[string]any, 0, len(debitByAccount)+len(creditByAccount))
	for account, amount := range debitByAccount {
		journalLines = append(journalLines, map[string]any{"account_code": account, "description": "Inventory / WIP", "debit": amount, "credit": 0.0})
	}
	for account, amount := range creditByAccount {
		journalLines = append(journalLines, map[string]any{"account_code": account, "description": "Inventory / WIP", "debit": 0.0, "credit": amount})
	}
	postingPayload := map[string]any{
		"source_document_type": record.Header.Type,
		"source_document_id":   record.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        "IDR",
		"posting_rule_key":     postingRuleKey,
		"total_amount":         totalCost,
		"journal_lines":        journalLines,
		"notes":                fmt.Sprintf("%s %s", notePrefix, firstNonEmptyString(record.Header.Number, record.Header.ID)),
	}
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(record.Header.OrganizationID, record.Header.LocationID, textValue(postingPayload["posting_date"])); err != nil {
			return err
		}
	}
	posting, err := s.documents.Create("ledger_posting", record.Header.OrganizationID, record.Header.LocationID, actorID, postingPayload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(posting, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(posting.Header.ID, record.Header.ID, "posting_for", map[string]any{"posting_reason": postingReason}); err != nil {
		return err
	}
	_, err = s.documents.AddLink(record.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": postingReason})
	return err
}

func (s *ProductionCoreService) hasPostingLink(record document.Record, reason string) bool {
	for _, link := range record.Links {
		if link.LinkType != "posting_for" {
			continue
		}
		if textValue(link.Metadata["posting_reason"]) == reason {
			return true
		}
	}
	return false
}

func (s *ProductionCoreService) createReversalPosting(source document.Record, actorID, originalReason, reversalReason, postingRuleKey string) error {
	originalPosting, ok := s.findPostingForReason(source, originalReason)
	if !ok {
		return nil
	}
	if s.hasPostingLink(source, reversalReason) {
		return nil
	}
	lines := reverseJournalLines(recordList(originalPosting.Body.Payload["journal_lines"]))
	payload := clonedPayload(originalPosting.Body.Payload)
	payload["source_document_type"] = source.Header.Type
	payload["source_document_id"] = source.Header.ID
	payload["posting_date"] = time.Now().UTC().Format("2006-01-02")
	payload["posting_rule_key"] = postingRuleKey
	payload["journal_lines"] = lines
	payload["notes"] = fmt.Sprintf("Reversal of %s", firstNonEmptyString(originalPosting.Header.Number, originalPosting.Header.ID))
	payload["total_amount"] = roundMoney(numberValue(originalPosting.Body.Payload["total_amount"]))
	if s.finance != nil {
		if err := s.finance.ValidatePostingDateOpen(source.Header.OrganizationID, source.Header.LocationID, textValue(payload["posting_date"])); err != nil {
			return err
		}
	}
	reversal, err := s.documents.Create("ledger_posting", source.Header.OrganizationID, source.Header.LocationID, actorID, payload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemPosting(reversal, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(reversal.Header.ID, source.Header.ID, "posting_for", map[string]any{
		"posting_reason": reversalReason,
		"reversal_of":    originalPosting.Header.ID,
	}); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(source.Header.ID, reversal.Header.ID, "posting_for", map[string]any{
		"posting_reason": reversalReason,
		"reversal_of":    originalPosting.Header.ID,
	}); err != nil {
		return err
	}
	return nil
}

func (s *ProductionCoreService) findPostingForReason(record document.Record, reason string) (document.Record, bool) {
	for _, link := range record.Links {
		if link.LinkType != "posting_for" {
			continue
		}
		if textValue(link.Metadata["posting_reason"]) != reason {
			continue
		}
		posting, err := s.documents.Get(link.LinkedDocumentID)
		if err == nil && posting.Header.Type == "ledger_posting" {
			return posting, true
		}
	}
	return document.Record{}, false
}

func (s *ProductionCoreService) applyProductionReservations(record document.Record, actorID string) error {
	payload := clonedPayload(record.Body.Payload)
	lines := s.normalizeProductionComponentLines(recordList(payload["lines"]))
	balances := s.inventory.currentBalances(record.Header.OrganizationID, record.Header.LocationID)
	reserved := s.inventory.currentReservedBalances(record.Header.OrganizationID, record.Header.LocationID, record.Header.ID)
	reservedTotal := 0.0
	shortageTotal := 0.0
	for index := range lines {
		itemCode := firstNonEmptyString(textValue(lines[index]["actual_item_code"]), textValue(lines[index]["component_item_code"]))
		warehouseCode := textValue(lines[index]["warehouse_code"])
		requiredQty := roundMoney(numberValue(lines[index]["quantity"]) - numberValue(lines[index]["issued_quantity"]))
		availableQty := roundMoney(s.inventory.sumBalance(balances, itemCode, warehouseCode, textValue(lines[index]["batch_code"])) - s.inventory.sumBalance(reserved, itemCode, warehouseCode, textValue(lines[index]["batch_code"])))
		if availableQty < 0 {
			availableQty = 0
		}
		reserveQty := roundMoney(minFloat(requiredQty, availableQty))
		shortageQty := roundMoney(maxFloat(requiredQty-reserveQty, 0))
		lines[index]["available_quantity"] = availableQty
		lines[index]["reserved_quantity"] = reserveQty
		lines[index]["shortage_quantity"] = shortageQty
		switch {
		case reserveQty <= 0 && shortageQty > 0:
			lines[index]["reservation_status"] = "short"
		case shortageQty > 0:
			lines[index]["reservation_status"] = "partial"
		default:
			lines[index]["reservation_status"] = "reserved"
		}
		if itemCode != textValue(lines[index]["component_item_code"]) {
			lines[index]["substitution_status"] = "substituted"
		} else {
			lines[index]["substitution_status"] = "planned"
		}
		reservedTotal = roundMoney(reservedTotal + reserveQty)
		shortageTotal = roundMoney(shortageTotal + shortageQty)
	}
	payload["lines"] = lines
	payload["reserved_quantity_total"] = reservedTotal
	payload["shortage_quantity_total"] = shortageTotal
	payload["stages"] = advanceProductionStages(s.normalizeProductionStages(recordList(payload["stages"])), "approve")
	return s.updateDocumentPayload(record, actorID, payload)
}

func normalizeStringList(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		seen := map[string]struct{}{}
		for _, item := range typed {
			value := strings.TrimSpace(item)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		seen := map[string]struct{}{}
		for _, item := range typed {
			value := strings.TrimSpace(textValue(item))
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
		return out
	case string:
		parts := strings.Split(typed, ",")
		out := make([]string, 0, len(parts))
		seen := map[string]struct{}{}
		for _, item := range parts {
			value := strings.TrimSpace(item)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
		return out
	default:
		return []string{}
	}
}

func defaultProductionStages() []map[string]any {
	return []map[string]any{
		{"stage_code": "prep", "stage_name": "Prep", "sequence": 1, "work_center_code": "", "status": "pending", "required": true, "note": ""},
		{"stage_code": "process", "stage_name": "Process", "sequence": 2, "work_center_code": "", "status": "pending", "required": true, "note": ""},
		{"stage_code": "pack", "stage_name": "Pack", "sequence": 3, "work_center_code": "", "status": "pending", "required": true, "note": ""},
	}
}

func advanceProductionStages(stages []map[string]any, event string) []map[string]any {
	if len(stages) == 0 {
		return stages
	}
	next := make([]map[string]any, 0, len(stages))
	progressed := false
	for index, stage := range stages {
		row := cloneMap(stage)
		status := textValue(row["status"])
		if event == "approve" {
			if index == 0 && status == "pending" {
				row["status"] = "ready"
			}
			next = append(next, row)
			continue
		}
		if event == "issue" {
			if status == "ready" || status == "pending" {
				row["status"] = "completed"
				progressed = true
				next = append(next, row)
				continue
			}
			if progressed && status == "pending" {
				row["status"] = "ready"
				progressed = false
			}
			next = append(next, row)
			continue
		}
		next = append(next, row)
	}
	return next
}

func completeRemainingStages(stages []map[string]any) []map[string]any {
	next := make([]map[string]any, 0, len(stages))
	for _, stage := range stages {
		row := cloneMap(stage)
		if textValue(row["status"]) != "skipped" {
			row["status"] = "completed"
		}
		next = append(next, row)
	}
	return next
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func (s *ProductionCoreService) findLinkedProductionOrder(record document.Record) (document.Record, bool) {
	for _, link := range record.Links {
		if link.LinkType != "production_for" {
			continue
		}
		linked, err := s.documents.Get(link.LinkedDocumentID)
		if err == nil && linked.Header.Type == "production_order" {
			return linked, true
		}
	}
	return document.Record{}, false
}

func (s *ProductionCoreService) updateDocumentPayload(record document.Record, actorID string, payload map[string]any) error {
	record.Body.Payload = document.NormalizePayload(payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	return s.documents.Save(record)
}

func (s *ProductionCoreService) finalizeSystemPosting(record document.Record, actorID, status string) error {
	record.Header.Status = status
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(numberValue(record.Body.Payload["total_amount"])),
	}
	return s.documents.Save(record)
}

func (s *ProductionCoreService) refreshDocuments(records ...document.Record) error {
	if s.search == nil {
		return nil
	}
	for _, record := range records {
		if strings.TrimSpace(record.Header.ID) == "" {
			continue
		}
		s.search.RefreshDocument(record)
	}
	return nil
}
