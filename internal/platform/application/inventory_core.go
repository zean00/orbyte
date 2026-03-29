package application

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type InventoryCoreService struct {
	documents *document.Service
	config    *config.Service
	models    *model.Service
	search    *search.Service
	finance   *FinanceReportingCoreService
}

type InventorySummary struct {
	TrackedItemCount    int              `json:"tracked_item_count"`
	BatchCount          int              `json:"batch_count"`
	TotalOnHand         float64          `json:"total_on_hand"`
	TotalReserved       float64          `json:"total_reserved"`
	TotalAvailable      float64          `json:"total_available"`
	ExpiredBatchCount   int              `json:"expired_batch_count"`
	NearExpiryCount     int              `json:"near_expiry_batch_count"`
	ExpiredQuantity     float64          `json:"expired_quantity"`
	NearExpiryQuantity  float64          `json:"near_expiry_quantity"`
	QuarantinedCount    int              `json:"quarantined_batch_count"`
	QuarantinedQuantity float64          `json:"quarantined_quantity"`
	BlockedCount        int              `json:"blocked_batch_count"`
	BlockedQuantity     float64          `json:"blocked_quantity"`
	RecalledCount       int              `json:"recalled_batch_count"`
	RecalledQuantity    float64          `json:"recalled_quantity"`
	WarehouseCount      int              `json:"warehouse_count"`
	Items               []map[string]any `json:"items"`
	WarehouseBreakdown  []map[string]any `json:"warehouse_breakdown"`
	Batches             []map[string]any `json:"batches"`
}

type inventoryPolicy struct {
	Enabled            bool
	TrackingMode       string
	ExpiryTracking     bool
	AllowNegativeStock bool
	DefaultIssue       string
	Name               string
	UOMCode            string
	ItemType           string
	InventoryAccount   string
	COGSAccount        string
	WIPAccount         string
}

type inventoryBalance struct {
	ItemCode       string
	WarehouseCode  string
	BatchCode      string
	ExpirationDate string
	Quantity       float64
}

type inventoryValuationSnapshot struct {
	ID              string
	Version         int
	ItemCode        string
	WarehouseCode   string
	QuantityOnHand  float64
	AverageUnitCost float64
	InventoryValue  float64
}

const defaultInventoryNearExpiryDays = 30

type inventoryBatchState struct {
	Status          string
	IsIssuable      bool
	HoldReason      string
	HoldNotes       string
	RecallReference string
}

func NewInventoryCoreService(documents *document.Service, configSvc *config.Service, models *model.Service, searchSvc *search.Service) *InventoryCoreService {
	return &InventoryCoreService{documents: documents, config: configSvc, models: models, search: searchSvc}
}

func (s *InventoryCoreService) SetFinanceReporting(finance *FinanceReportingCoreService) {
	s.finance = finance
}

func (s *InventoryCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	switch strings.TrimSpace(documentType) {
	case "stock_receipt", "stock_issue", "stock_adjustment":
		next["lines"] = s.normalizeInventoryLines(recordList(next["lines"]), false)
		next["total_quantity"] = roundMoney(sumInventoryLineQuantity(recordList(next["lines"]), "quantity"))
	case "stock_transfer":
		next["lines"] = s.normalizeInventoryLines(recordList(next["lines"]), true)
		next["total_quantity"] = roundMoney(sumInventoryLineQuantity(recordList(next["lines"]), "quantity"))
	}
	return next
}

func (s *InventoryCoreService) ValidateApprove(record document.Record) error {
	switch record.Header.Type {
	case "goods_receipt":
		return s.validateGoodsReceiptForInventory(record)
	case "stock_receipt":
		return s.validateInventoryReceipt(record.Body.Payload)
	case "stock_issue":
		return s.validateStockIssue(record)
	case "stock_adjustment":
		return s.validateStockAdjustment(record)
	case "stock_transfer":
		return s.validateStockTransfer(record)
	default:
		return nil
	}
}

func (s *InventoryCoreService) ValidateCancel(record document.Record) error {
	switch record.Header.Type {
	case "stock_receipt", "stock_issue", "stock_adjustment", "stock_transfer", "goods_receipt":
		return nil
	default:
		return nil
	}
}

func (s *InventoryCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "goods_receipt":
		return s.handleApprovedGoodsReceipt(record, actorID)
	case "stock_receipt":
		return s.handleApprovedStockReceipt(record, actorID)
	case "stock_issue":
		return s.handleApprovedStockIssue(record, actorID)
	case "stock_adjustment":
		return s.handleApprovedStockAdjustment(record, actorID)
	case "stock_transfer":
		return s.handleApprovedStockTransfer(record, actorID)
	default:
		return nil
	}
}

func (s *InventoryCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "goods_receipt":
		return s.reverseMovements(record, actorID, "goods_receipt_inventory", "goods_receipt_inventory_reversal")
	case "stock_receipt":
		return s.reverseMovements(record, actorID, "stock_receipt", "stock_receipt_reversal")
	case "stock_issue":
		return s.reverseMovements(record, actorID, "stock_issue", "stock_issue_reversal")
	case "stock_adjustment":
		return s.reverseMovements(record, actorID, "stock_adjustment", "stock_adjustment_reversal")
	case "stock_transfer":
		return s.reverseMovements(record, actorID, "stock_transfer", "stock_transfer_reversal")
	default:
		return nil
	}
}

func (s *InventoryCoreService) Summary(now time.Time) InventorySummary {
	return s.SummaryScoped("", "", now)
}

func (s *InventoryCoreService) SummaryScoped(organizationID, locationID string, now time.Time) InventorySummary {
	balances := s.currentBalances(organizationID, locationID)
	reserved := s.currentReservedBalances(organizationID, locationID, "")
	batchStates := s.batchStateIndex(organizationID, locationID, now)
	itemSeen := map[string]struct{}{}
	warehouseSeen := map[string]struct{}{}
	warehouseTotals := map[string]float64{}
	warehouseReserved := map[string]float64{}
	items := map[string]float64{}
	itemReserved := map[string]float64{}
	batches := make([]map[string]any, 0)
	summary := InventorySummary{
		Items:              make([]map[string]any, 0),
		WarehouseBreakdown: make([]map[string]any, 0),
		Batches:            make([]map[string]any, 0),
	}
	for _, balance := range balances {
		if balance.Quantity <= 0 {
			continue
		}
		summary.TotalOnHand = roundMoney(summary.TotalOnHand + balance.Quantity)
		reservedQty := roundMoney(s.sumBalance(reserved, balance.ItemCode, balance.WarehouseCode, balance.BatchCode))
		availableQty := roundMoney(maxFloat(balance.Quantity-reservedQty, 0))
		summary.TotalReserved = roundMoney(summary.TotalReserved + reservedQty)
		summary.TotalAvailable = roundMoney(summary.TotalAvailable + availableQty)
		itemSeen[balance.ItemCode] = struct{}{}
		if balance.WarehouseCode != "" {
			warehouseSeen[balance.WarehouseCode] = struct{}{}
			warehouseTotals[balance.WarehouseCode] = roundMoney(warehouseTotals[balance.WarehouseCode] + balance.Quantity)
			warehouseReserved[balance.WarehouseCode] = roundMoney(warehouseReserved[balance.WarehouseCode] + reservedQty)
		}
		items[balance.ItemCode] = roundMoney(items[balance.ItemCode] + balance.Quantity)
		itemReserved[balance.ItemCode] = roundMoney(itemReserved[balance.ItemCode] + reservedQty)
		if balance.BatchCode != "" {
			summary.BatchCount++
			state := batchStates[s.batchKey(balance.ItemCode, balance.WarehouseCode, balance.BatchCode)]
			switch state.Status {
			case "expired":
				summary.ExpiredBatchCount++
				summary.ExpiredQuantity = roundMoney(summary.ExpiredQuantity + balance.Quantity)
			case "near_expiry":
				summary.NearExpiryCount++
				summary.NearExpiryQuantity = roundMoney(summary.NearExpiryQuantity + balance.Quantity)
			case "quarantined":
				summary.QuarantinedCount++
				summary.QuarantinedQuantity = roundMoney(summary.QuarantinedQuantity + balance.Quantity)
			case "blocked":
				summary.BlockedCount++
				summary.BlockedQuantity = roundMoney(summary.BlockedQuantity + balance.Quantity)
			case "recalled":
				summary.RecalledCount++
				summary.RecalledQuantity = roundMoney(summary.RecalledQuantity + balance.Quantity)
			}
			batches = append(batches, map[string]any{
				"item_code":          balance.ItemCode,
				"warehouse_code":     balance.WarehouseCode,
				"batch_code":         balance.BatchCode,
				"expiration_date":    balance.ExpirationDate,
				"on_hand_quantity":   balance.Quantity,
				"reserved_quantity":  reservedQty,
				"available_quantity": availableQty,
				"status":             state.Status,
				"is_issuable":        state.IsIssuable,
				"hold_reason":        state.HoldReason,
				"hold_notes":         state.HoldNotes,
				"recall_reference":   state.RecallReference,
			})
		}
	}
	summary.TrackedItemCount = len(itemSeen)
	summary.WarehouseCount = len(warehouseSeen)
	for itemCode, total := range items {
		reservedQty := roundMoney(itemReserved[itemCode])
		summary.Items = append(summary.Items, map[string]any{
			"item_code":          itemCode,
			"on_hand_quantity":   total,
			"reserved_quantity":  reservedQty,
			"available_quantity": roundMoney(maxFloat(total-reservedQty, 0)),
		})
	}
	for warehouseCode, total := range warehouseTotals {
		reservedQty := roundMoney(warehouseReserved[warehouseCode])
		summary.WarehouseBreakdown = append(summary.WarehouseBreakdown, map[string]any{
			"warehouse_code":     warehouseCode,
			"on_hand_quantity":   total,
			"reserved_quantity":  reservedQty,
			"available_quantity": roundMoney(maxFloat(total-reservedQty, 0)),
		})
	}
	sort.Slice(summary.Items, func(i, j int) bool {
		return textValue(summary.Items[i]["item_code"]) < textValue(summary.Items[j]["item_code"])
	})
	sort.Slice(summary.WarehouseBreakdown, func(i, j int) bool {
		return textValue(summary.WarehouseBreakdown[i]["warehouse_code"]) < textValue(summary.WarehouseBreakdown[j]["warehouse_code"])
	})
	sort.Slice(batches, func(i, j int) bool {
		leftItem := textValue(batches[i]["item_code"])
		rightItem := textValue(batches[j]["item_code"])
		if leftItem != rightItem {
			return leftItem < rightItem
		}
		leftExpiry := textValue(batches[i]["expiration_date"])
		rightExpiry := textValue(batches[j]["expiration_date"])
		if leftExpiry != rightExpiry {
			return leftExpiry < rightExpiry
		}
		return textValue(batches[i]["batch_code"]) < textValue(batches[j]["batch_code"])
	})
	summary.Batches = batches
	return summary
}

func (s *InventoryCoreService) ItemStockScoped(organizationID, locationID, itemCode string, now time.Time) map[string]any {
	balances := s.currentBalances(organizationID, locationID)
	reserved := s.currentReservedBalances(organizationID, locationID, "")
	batchStates := s.batchStateIndex(organizationID, locationID, now)
	filtered := make([]map[string]any, 0)
	total := 0.0
	totalReserved := 0.0
	for _, balance := range balances {
		if balance.ItemCode != strings.TrimSpace(itemCode) || balance.Quantity <= 0 {
			continue
		}
		total = roundMoney(total + balance.Quantity)
		reservedQty := roundMoney(s.sumBalance(reserved, balance.ItemCode, balance.WarehouseCode, balance.BatchCode))
		totalReserved = roundMoney(totalReserved + reservedQty)
		state := batchStates[s.batchKey(balance.ItemCode, balance.WarehouseCode, balance.BatchCode)]
		filtered = append(filtered, map[string]any{
			"warehouse_code":     balance.WarehouseCode,
			"batch_code":         balance.BatchCode,
			"expiration_date":    balance.ExpirationDate,
			"on_hand_quantity":   balance.Quantity,
			"reserved_quantity":  reservedQty,
			"available_quantity": roundMoney(maxFloat(balance.Quantity-reservedQty, 0)),
			"is_expired":         state.Status == "expired",
			"status":             state.Status,
			"is_issuable":        state.IsIssuable,
			"hold_reason":        state.HoldReason,
			"hold_notes":         state.HoldNotes,
			"recall_reference":   state.RecallReference,
		})
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftWarehouse := textValue(filtered[i]["warehouse_code"])
		rightWarehouse := textValue(filtered[j]["warehouse_code"])
		if leftWarehouse != rightWarehouse {
			return leftWarehouse < rightWarehouse
		}
		leftExpiry := textValue(filtered[i]["expiration_date"])
		rightExpiry := textValue(filtered[j]["expiration_date"])
		if leftExpiry != rightExpiry {
			return leftExpiry < rightExpiry
		}
		return textValue(filtered[i]["batch_code"]) < textValue(filtered[j]["batch_code"])
	})
	return map[string]any{
		"item_code":          itemCode,
		"on_hand_quantity":   total,
		"reserved_quantity":  totalReserved,
		"available_quantity": roundMoney(maxFloat(total-totalReserved, 0)),
		"warehouse_batches":  filtered,
	}
}

func (s *InventoryCoreService) DecorateBatchRecord(record model.Record, organizationID, locationID string, now time.Time) model.Record {
	records := s.DecorateBatchRecords([]model.Record{record}, organizationID, locationID, now)
	if len(records) == 0 {
		return record
	}
	return records[0]
}

func (s *InventoryCoreService) DecorateBatchRecords(records []model.Record, organizationID, locationID string, now time.Time) []model.Record {
	if len(records) == 0 {
		return records
	}
	balances := s.currentBalances(organizationID, locationID)
	reserved := s.currentReservedBalances(organizationID, locationID, "")
	nearDays := s.nearExpiryDays(organizationID, locationID)
	out := make([]model.Record, 0, len(records))
	for _, record := range records {
		next := record
		next.Values = cloneMap(record.Values)
		itemCode := textValue(next.Values["item_code"])
		warehouseCode := textValue(next.Values["warehouse_code"])
		batchCode := textValue(next.Values["batch_code"])
		expirationDate := textValue(next.Values["expiration_date"])
		next.Values["status"] = s.effectiveBatchStatus(textValue(next.Values["status"]), expirationDate, now, nearDays)
		next.Values["is_issuable"] = inventoryBatchStatusIssuable(textValue(next.Values["status"]))
		next.Values["on_hand_quantity"] = roundMoney(s.sumBalanceExact(balances, itemCode, warehouseCode, batchCode, expirationDate))
		next.Values["reserved_quantity"] = roundMoney(s.sumBalanceExact(reserved, itemCode, warehouseCode, batchCode, expirationDate))
		next.Values["available_quantity"] = roundMoney(maxFloat(numberValue(next.Values["on_hand_quantity"])-numberValue(next.Values["reserved_quantity"]), 0))
		out = append(out, next)
	}
	return out
}

func (s *InventoryCoreService) ApplyBatchAction(recordID, action, actorID, reason, notes, recallReference string, now time.Time) (model.Record, error) {
	if s.models == nil {
		return model.Record{}, shared.Validation("inventory batches are unavailable")
	}
	record, err := s.models.Get("inventory_batch", strings.TrimSpace(recordID))
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	switch strings.TrimSpace(action) {
	case "quarantine":
		values["status"] = "quarantined"
		values["hold_reason"] = firstNonEmptyString(strings.TrimSpace(reason), "quarantine")
		values["hold_notes"] = strings.TrimSpace(notes)
	case "release", "unblock":
		values["status"] = s.suggestedBatchStatus(textValue(values["expiration_date"]), now, s.nearExpiryDays("", ""))
		values["hold_reason"] = ""
		values["hold_notes"] = ""
		values["recall_reference"] = ""
	case "block":
		values["status"] = "blocked"
		values["hold_reason"] = firstNonEmptyString(strings.TrimSpace(reason), "blocked")
		values["hold_notes"] = strings.TrimSpace(notes)
	case "recall":
		values["status"] = "recalled"
		values["hold_reason"] = firstNonEmptyString(strings.TrimSpace(reason), "recalled")
		values["hold_notes"] = strings.TrimSpace(notes)
		values["recall_reference"] = strings.TrimSpace(recallReference)
	default:
		return model.Record{}, shared.Validation("unsupported batch action")
	}
	updated, err := s.models.Update("inventory_batch", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	return s.DecorateBatchRecord(updated, "", "", now), nil
}

func (s *InventoryCoreService) ProposeFulfillmentLines(organizationID, locationID string, lines []map[string]any, excludeDocumentID string) ([]map[string]any, error) {
	balances := s.currentBalances(organizationID, locationID)
	reserved := s.currentReservedBalances(organizationID, locationID, excludeDocumentID)
	return s.resolveFulfillmentLinesWithBalances(organizationID, locationID, balances, reserved, lines)
}

func (s *InventoryCoreService) ValidateFulfillmentIssue(record document.Record) error {
	lines := recordList(record.Body.Payload["lines"])
	if len(lines) == 0 {
		return shared.Validation("fulfillment lines are required")
	}
	balances := s.currentBalances(record.Header.OrganizationID, record.Header.LocationID)
	reserved := s.currentReservedBalances(record.Header.OrganizationID, record.Header.LocationID, record.Header.ID)
	today := time.Now().UTC().Format("2006-01-02")
	remaining := map[string]float64{}
	for _, line := range lines {
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			return shared.Validation("item code is required")
		}
		warehouseCode := textValue(line["warehouse_code"])
		if warehouseCode == "" {
			return shared.Validation(fmt.Sprintf("warehouse code is required for item %s", itemCode))
		}
		qty := roundMoney(numberValue(line["quantity"]))
		if qty <= 0 {
			return shared.Validation(fmt.Sprintf("quantity must be greater than zero for item %s", itemCode))
		}
		policy := s.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			return shared.Validation(fmt.Sprintf("item %s is not inventory-tracked", itemCode))
		}
		batchCode := textValue(line["batch_code"])
		if policy.TrackingMode == "batch" && batchCode == "" {
			return shared.Validation(fmt.Sprintf("batch code is required for item %s", itemCode))
		}
		expirationDate := textValue(line["expiration_date"])
		if policy.TrackingMode == "batch" && policy.ExpiryTracking && strings.TrimSpace(expirationDate) == "" {
			return shared.Validation(fmt.Sprintf("expiration date is required for item %s", itemCode))
		}
		if policy.TrackingMode == "batch" && policy.ExpiryTracking && expirationDate < today {
			return shared.Validation(fmt.Sprintf("expired batch %s cannot be fulfilled for item %s", batchCode, itemCode))
		}
		if policy.TrackingMode == "batch" && !s.batchStatusIssuable(record.Header.OrganizationID, record.Header.LocationID, itemCode, warehouseCode, batchCode, expirationDate, time.Now().UTC()) {
			return shared.Validation(fmt.Sprintf("batch %s is not issuable for item %s", batchCode, itemCode))
		}
		key := fmt.Sprintf("%s|%s|%s|%s", itemCode, warehouseCode, batchCode, expirationDate)
		if _, ok := remaining[key]; !ok {
			available := roundMoney(s.availableFulfillmentQuantity(balances, reserved, itemCode, warehouseCode, batchCode, expirationDate, policy))
			remaining[key] = available
		}
		if !policy.AllowNegativeStock && roundMoney(remaining[key]) < qty {
			return shared.Validation(fmt.Sprintf("insufficient available stock for item %s in warehouse %s", itemCode, warehouseCode))
		}
		remaining[key] = roundMoney(remaining[key] - qty)
	}
	return nil
}

func (s *InventoryCoreService) HandleApprovedFulfillment(record document.Record, actorID string) error {
	if s.hasMovementLink(record, "sales_fulfillment") {
		return nil
	}
	payload := clonedPayload(record.Body.Payload)
	lines := recordList(payload["lines"])
	costedLines := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		costed := s.prepareMovementLineForCost(record, "sales_fulfillment", line, "out")
		if err := s.createMovement(record, actorID, "sales_fulfillment", costed, "out"); err != nil {
			return err
		}
		costedLines = append(costedLines, costed)
	}
	payload["lines"] = costedLines
	payload["fulfillment_status"] = "issued"
	payload["fulfilled_quantity_total"] = roundMoney(sumInventoryLineQuantity(costedLines, "quantity"))
	payload["reserved_quantity_total"] = 0.0
	payload["cost_amount_total"] = roundMoney(sumInventoryLineQuantity(costedLines, "extended_cost"))
	if err := s.updateDocumentPayload(record, actorID, payload); err != nil {
		return err
	}
	return s.createFulfillmentCostPosting(record, actorID, costedLines)
}

func (s *InventoryCoreService) validateGoodsReceiptForInventory(record document.Record) error {
	for _, line := range recordList(record.Body.Payload["lines"]) {
		policy := s.lookupItemPolicy(textValue(line["item_code"]))
		if !policy.Enabled {
			continue
		}
		if policy.TrackingMode == "batch" && strings.TrimSpace(textValue(line["batch_code"])) == "" {
			return shared.Validation(fmt.Sprintf("batch code is required for item %s", textValue(line["item_code"])))
		}
		if policy.TrackingMode == "batch" && policy.ExpiryTracking && strings.TrimSpace(textValue(line["expiration_date"])) == "" {
			return shared.Validation(fmt.Sprintf("expiration date is required for item %s", textValue(line["item_code"])))
		}
		if strings.TrimSpace(textValue(line["warehouse_code"])) == "" {
			return shared.Validation(fmt.Sprintf("warehouse code is required for item %s", textValue(line["item_code"])))
		}
		if roundMoney(firstPositiveNumber(line["receipt_qty"], line["received_qty"], line["quantity"])) <= 0 {
			return shared.Validation(fmt.Sprintf("receipt quantity is required for item %s", textValue(line["item_code"])))
		}
	}
	return nil
}

func (s *InventoryCoreService) validateInventoryReceipt(payload map[string]any) error {
	lines := recordList(payload["lines"])
	if len(lines) == 0 {
		return shared.Validation("stock receipt lines are required")
	}
	for _, line := range lines {
		if err := s.validateInventoryLine(line, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *InventoryCoreService) validateStockIssue(record document.Record) error {
	lines := recordList(record.Body.Payload["lines"])
	if len(lines) == 0 {
		return shared.Validation("stock issue lines are required")
	}
	for _, line := range lines {
		if err := s.validateInventoryLine(line, false); err != nil {
			return err
		}
		policy := s.lookupItemPolicy(textValue(line["item_code"]))
		if !policy.Enabled {
			return shared.Validation(fmt.Sprintf("item %s is not inventory-tracked", textValue(line["item_code"])))
		}
		if policy.TrackingMode == "batch" && policy.DefaultIssue == "manual" && strings.TrimSpace(textValue(line["batch_code"])) == "" {
			return shared.Validation(fmt.Sprintf("batch code is required for item %s", textValue(line["item_code"])))
		}
	}
	_, err := s.resolveIssueMovementLines(record.Header.OrganizationID, record.Header.LocationID, record.Body.Payload)
	return err
}

func (s *InventoryCoreService) validateStockAdjustment(record document.Record) error {
	lines := recordList(record.Body.Payload["lines"])
	if len(lines) == 0 {
		return shared.Validation("stock adjustment lines are required")
	}
	for _, line := range lines {
		if strings.TrimSpace(textValue(line["item_code"])) == "" {
			return shared.Validation("item code is required")
		}
		if strings.TrimSpace(textValue(line["warehouse_code"])) == "" {
			return shared.Validation("warehouse code is required")
		}
		if roundMoney(numberValue(line["quantity"])) == 0 {
			return shared.Validation("adjustment quantity must be non-zero")
		}
		policy := s.lookupItemPolicy(textValue(line["item_code"]))
		if policy.TrackingMode == "batch" && strings.TrimSpace(textValue(line["batch_code"])) == "" {
			return shared.Validation(fmt.Sprintf("batch code is required for item %s", textValue(line["item_code"])))
		}
		if policy.TrackingMode == "batch" && policy.ExpiryTracking && strings.TrimSpace(textValue(line["expiration_date"])) == "" {
			return shared.Validation(fmt.Sprintf("expiration date is required for item %s", textValue(line["item_code"])))
		}
	}
	return nil
}

func (s *InventoryCoreService) validateStockTransfer(record document.Record) error {
	lines := recordList(record.Body.Payload["lines"])
	if len(lines) == 0 {
		return shared.Validation("stock transfer lines are required")
	}
	for _, line := range lines {
		if strings.TrimSpace(textValue(line["item_code"])) == "" {
			return shared.Validation("item code is required")
		}
		if strings.TrimSpace(textValue(line["source_warehouse_code"])) == "" || strings.TrimSpace(textValue(line["target_warehouse_code"])) == "" {
			return shared.Validation("source and target warehouses are required")
		}
		if textValue(line["source_warehouse_code"]) == textValue(line["target_warehouse_code"]) {
			return shared.Validation("source and target warehouses must differ")
		}
		if roundMoney(numberValue(line["quantity"])) <= 0 {
			return shared.Validation("transfer quantity must be greater than zero")
		}
		policy := s.lookupItemPolicy(textValue(line["item_code"]))
		if policy.TrackingMode == "batch" && strings.TrimSpace(textValue(line["batch_code"])) == "" {
			return shared.Validation(fmt.Sprintf("batch code is required for item %s", textValue(line["item_code"])))
		}
	}
	return nil
}

func (s *InventoryCoreService) handleApprovedGoodsReceipt(receipt document.Record, actorID string) error {
	lines := make([]map[string]any, 0)
	for _, line := range recordList(receipt.Body.Payload["lines"]) {
		policy := s.lookupItemPolicy(textValue(line["item_code"]))
		if !policy.Enabled {
			continue
		}
		qty := roundMoney(firstPositiveNumber(line["receipt_qty"], line["received_qty"], line["quantity"]))
		if qty <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"item_code":                    textValue(line["item_code"]),
			"description":                  textValue(line["description"]),
			"warehouse_code":               textValue(line["warehouse_code"]),
			"batch_code":                   textValue(line["batch_code"]),
			"expiration_date":              textValue(line["expiration_date"]),
			"quantity":                     qty,
			"uom_code":                     firstNonEmptyString(textValue(line["uom_code"]), policy.UOMCode),
			"unit_cost":                    firstPositiveNumber(line["unit_cost"], line["unit_price"]),
			"currency_code":                firstNonEmptyString(textValue(line["currency_code"]), textValue(receipt.Body.Payload["currency_code"]), "IDR"),
			"inventory_asset_account_code": firstNonEmptyString(textValue(line["inventory_asset_account_code"]), policy.InventoryAccount),
			"cogs_account_code":            firstNonEmptyString(textValue(line["cogs_account_code"]), policy.COGSAccount),
			"wip_account_code":             firstNonEmptyString(textValue(line["wip_account_code"]), policy.WIPAccount),
			"note":                         firstNonEmptyString(textValue(line["note"]), "Auto-received from procurement"),
		})
	}
	if len(lines) == 0 || s.hasMovementLink(receipt, "goods_receipt_inventory") {
		return nil
	}
	for _, line := range lines {
		costed := s.prepareMovementLineForCost(receipt, "goods_receipt_inventory", line, "in")
		if err := s.createMovement(receipt, actorID, "goods_receipt_inventory", costed, "in"); err != nil {
			return err
		}
	}
	return nil
}

func (s *InventoryCoreService) handleApprovedStockReceipt(record document.Record, actorID string) error {
	if s.hasMovementLink(record, "stock_receipt") {
		return nil
	}
	for _, line := range recordList(record.Body.Payload["lines"]) {
		costed := s.prepareMovementLineForCost(record, "stock_receipt", line, "in")
		if err := s.createMovement(record, actorID, "stock_receipt", costed, "in"); err != nil {
			return err
		}
	}
	return nil
}

func (s *InventoryCoreService) handleApprovedStockIssue(record document.Record, actorID string) error {
	if s.hasMovementLink(record, "stock_issue") {
		return nil
	}
	movementLines, err := s.resolveIssueMovementLines(record.Header.OrganizationID, record.Header.LocationID, record.Body.Payload)
	if err != nil {
		return err
	}
	payload := clonedPayload(record.Body.Payload)
	payload["lines"] = movementLines
	payload["total_quantity"] = roundMoney(sumInventoryLineQuantity(movementLines, "quantity"))
	if err := s.updateDocumentPayload(record, actorID, payload); err != nil {
		return err
	}
	updated, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = updated
	}
	costedLines := make([]map[string]any, 0, len(movementLines))
	for _, line := range movementLines {
		costed := s.prepareMovementLineForCost(record, "stock_issue", line, "out")
		if err := s.createMovement(record, actorID, "stock_issue", costed, "out"); err != nil {
			return err
		}
		costedLines = append(costedLines, costed)
	}
	payload["lines"] = costedLines
	payload["cost_amount_total"] = roundMoney(sumInventoryLineQuantity(costedLines, "extended_cost"))
	if err := s.updateDocumentPayload(record, actorID, payload); err != nil {
		return err
	}
	return nil
}

func (s *InventoryCoreService) handleApprovedStockAdjustment(record document.Record, actorID string) error {
	if s.hasMovementLink(record, "stock_adjustment") {
		return nil
	}
	for _, line := range recordList(record.Body.Payload["lines"]) {
		direction := "in"
		if roundMoney(numberValue(line["quantity"])) < 0 {
			direction = "out"
			line = cloneMap(line)
			line["quantity"] = roundMoney(-numberValue(line["quantity"]))
		}
		costed := s.prepareMovementLineForCost(record, "stock_adjustment", line, direction)
		if err := s.createMovement(record, actorID, "stock_adjustment", costed, direction); err != nil {
			return err
		}
	}
	return nil
}

func (s *InventoryCoreService) handleApprovedStockTransfer(record document.Record, actorID string) error {
	if s.hasMovementLink(record, "stock_transfer") {
		return nil
	}
	for _, line := range recordList(record.Body.Payload["lines"]) {
		outLine := cloneMap(line)
		outLine["warehouse_code"] = textValue(line["source_warehouse_code"])
		outLine = s.prepareMovementLineForCost(record, "stock_transfer", outLine, "out")
		if err := s.createMovement(record, actorID, "stock_transfer", outLine, "out"); err != nil {
			return err
		}
		inLine := cloneMap(line)
		inLine["warehouse_code"] = textValue(line["target_warehouse_code"])
		inLine["unit_cost"] = numberValue(outLine["unit_cost"])
		inLine["inventory_asset_account_code"] = textValue(outLine["inventory_asset_account_code"])
		inLine["wip_account_code"] = textValue(outLine["wip_account_code"])
		inLine = s.prepareMovementLineForCost(record, "stock_transfer", inLine, "in")
		if err := s.createMovement(record, actorID, "stock_transfer", inLine, "in"); err != nil {
			return err
		}
	}
	return nil
}

func (s *InventoryCoreService) reverseMovements(source document.Record, actorID, originalReason, reversalReason string) error {
	movements := s.findMovementsForReason(source, originalReason)
	for _, original := range movements {
		payload := clonedPayload(original.Body.Payload)
		payload["movement_reason"] = reversalReason
		payload["movement_date"] = time.Now().UTC().Format("2006-01-02")
		payload["quantity_delta"] = roundMoney(-numberValue(payload["quantity_delta"]))
		payload["total_cost"] = roundMoney(-numberValue(payload["total_cost"]))
		payload["notes"] = fmt.Sprintf("Reversal of %s", firstNonEmptyString(original.Header.Number, original.Header.ID))
		reversal, err := s.documents.Create("stock_movement", source.Header.OrganizationID, source.Header.LocationID, actorID, payload)
		if err != nil {
			return err
		}
		if err := s.finalizeSystemMovement(reversal, actorID, "posted"); err != nil {
			return err
		}
		if _, err := s.documents.AddLink(reversal.Header.ID, source.Header.ID, "movement_for", map[string]any{
			"movement_reason": reversalReason,
			"reversal_of":     original.Header.ID,
		}); err != nil {
			return err
		}
		if _, err := s.documents.AddLink(source.Header.ID, reversal.Header.ID, "movement_for", map[string]any{
			"movement_reason": reversalReason,
			"reversal_of":     original.Header.ID,
		}); err != nil {
			return err
		}
		s.ensureBatchRecord(source.Header.OrganizationID, source.Header.LocationID, payload)
		if err := s.recordCostImpact(reversal, actorID); err != nil {
			return err
		}
	}
	return nil
}

func (s *InventoryCoreService) normalizeInventoryLines(lines []map[string]any, transfer bool) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		next := cloneMap(line)
		qty := roundMoney(numberValue(next["quantity"]))
		next["quantity"] = qty
		if !transfer {
			next["warehouse_code"] = textValue(next["warehouse_code"])
		} else {
			next["source_warehouse_code"] = textValue(next["source_warehouse_code"])
			next["target_warehouse_code"] = textValue(next["target_warehouse_code"])
		}
		next["item_code"] = textValue(next["item_code"])
		next["batch_code"] = textValue(next["batch_code"])
		next["expiration_date"] = textValue(next["expiration_date"])
		next["uom_code"] = textValue(next["uom_code"])
		next["available_quantity"] = roundMoney(numberValue(next["available_quantity"]))
		normalized = append(normalized, next)
	}
	return normalized
}

func (s *InventoryCoreService) validateInventoryLine(line map[string]any, inbound bool) error {
	itemCode := textValue(line["item_code"])
	if itemCode == "" {
		return shared.Validation("item code is required")
	}
	if textValue(line["warehouse_code"]) == "" {
		return shared.Validation(fmt.Sprintf("warehouse code is required for item %s", itemCode))
	}
	qty := roundMoney(numberValue(line["quantity"]))
	if qty <= 0 {
		return shared.Validation(fmt.Sprintf("quantity must be greater than zero for item %s", itemCode))
	}
	policy := s.lookupItemPolicy(itemCode)
	if !policy.Enabled {
		return shared.Validation(fmt.Sprintf("item %s is not inventory-tracked", itemCode))
	}
	if policy.TrackingMode == "batch" && strings.TrimSpace(textValue(line["batch_code"])) == "" {
		return shared.Validation(fmt.Sprintf("batch code is required for item %s", itemCode))
	}
	if inbound && policy.TrackingMode == "batch" && policy.ExpiryTracking && strings.TrimSpace(textValue(line["expiration_date"])) == "" {
		return shared.Validation(fmt.Sprintf("expiration date is required for item %s", itemCode))
	}
	return nil
}

func (s *InventoryCoreService) resolveIssueMovementLines(organizationID, locationID string, payload map[string]any) ([]map[string]any, error) {
	balances := s.currentBalances(organizationID, locationID)
	lines := make([]map[string]any, 0)
	today := time.Now().UTC().Format("2006-01-02")
	for _, row := range recordList(payload["lines"]) {
		itemCode := textValue(row["item_code"])
		policy := s.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			return nil, shared.Validation(fmt.Sprintf("item %s is not inventory-tracked", itemCode))
		}
		warehouseCode := textValue(row["warehouse_code"])
		qty := roundMoney(numberValue(row["quantity"]))
		if qty <= 0 {
			continue
		}
		if policy.TrackingMode != "batch" {
			available := s.sumBalance(balances, itemCode, warehouseCode, "")
			if !policy.AllowNegativeStock && available < qty {
				return nil, shared.Validation(fmt.Sprintf("insufficient stock for item %s in warehouse %s", itemCode, warehouseCode))
			}
			line := cloneMap(row)
			line["available_quantity"] = available
			lines = append(lines, line)
			continue
		}
		explicitBatch := textValue(row["batch_code"])
		explicitExpiration := textValue(row["expiration_date"])
		if explicitBatch != "" {
			if policy.ExpiryTracking && isExpiredInventoryBatch(explicitExpiration, today) {
				return nil, shared.Validation(fmt.Sprintf("expired batch %s cannot be issued for item %s", explicitBatch, itemCode))
			}
			if !s.batchStatusIssuable(organizationID, locationID, itemCode, warehouseCode, explicitBatch, explicitExpiration, time.Now().UTC()) {
				return nil, shared.Validation(fmt.Sprintf("batch %s cannot be issued for item %s", explicitBatch, itemCode))
			}
			available := s.availableFulfillmentQuantity(balances, nil, itemCode, warehouseCode, explicitBatch, explicitExpiration, policy)
			if !policy.AllowNegativeStock && available < qty {
				return nil, shared.Validation(fmt.Sprintf("insufficient stock for item %s batch %s", itemCode, explicitBatch))
			}
			line := cloneMap(row)
			line["available_quantity"] = available
			lines = append(lines, line)
			continue
		}
		candidates := s.fefoCandidates(organizationID, locationID, balances, itemCode, warehouseCode, policy.ExpiryTracking, today, time.Now().UTC())
		remaining := qty
		for _, candidate := range candidates {
			if remaining <= 0 {
				break
			}
			allocate := roundMoney(minFloat(candidate.Quantity, remaining))
			if allocate <= 0 {
				continue
			}
			line := cloneMap(row)
			line["batch_code"] = candidate.BatchCode
			line["expiration_date"] = candidate.ExpirationDate
			line["quantity"] = allocate
			line["available_quantity"] = candidate.Quantity
			lines = append(lines, line)
			remaining = roundMoney(remaining - allocate)
		}
		if !policy.AllowNegativeStock && remaining > 0 {
			return nil, shared.Validation(fmt.Sprintf("insufficient FEFO stock for item %s in warehouse %s", itemCode, warehouseCode))
		}
		if remaining > 0 {
			line := cloneMap(row)
			line["quantity"] = remaining
			line["available_quantity"] = 0.0
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func (s *InventoryCoreService) createMovement(source document.Record, actorID, reason string, line map[string]any, direction string) error {
	qty := roundMoney(numberValue(line["quantity"]))
	if qty <= 0 {
		return nil
	}
	if direction == "out" {
		qty = -qty
	}
	payload := map[string]any{
		"source_document_type":         source.Header.Type,
		"source_document_id":           source.Header.ID,
		"movement_date":                time.Now().UTC().Format("2006-01-02"),
		"movement_reason":              reason,
		"movement_direction":           direction,
		"item_code":                    textValue(line["item_code"]),
		"description":                  textValue(line["description"]),
		"warehouse_code":               textValue(line["warehouse_code"]),
		"batch_code":                   textValue(line["batch_code"]),
		"expiration_date":              textValue(line["expiration_date"]),
		"uom_code":                     textValue(line["uom_code"]),
		"quantity_delta":               qty,
		"available_quantity":           roundMoney(numberValue(line["available_quantity"])),
		"note":                         textValue(line["note"]),
		"unit_cost":                    roundMoney(numberValue(line["unit_cost"])),
		"total_cost":                   roundMoney(numberValue(line["total_cost"])),
		"currency_code":                firstNonEmptyString(textValue(line["currency_code"]), "IDR"),
		"inventory_asset_account_code": textValue(line["inventory_asset_account_code"]),
		"cogs_account_code":            textValue(line["cogs_account_code"]),
		"wip_account_code":             textValue(line["wip_account_code"]),
	}
	movement, err := s.documents.Create("stock_movement", source.Header.OrganizationID, source.Header.LocationID, actorID, payload)
	if err != nil {
		return err
	}
	if err := s.finalizeSystemMovement(movement, actorID, "posted"); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(movement.Header.ID, source.Header.ID, "movement_for", map[string]any{
		"movement_reason": reason,
	}); err != nil {
		return err
	}
	if _, err := s.documents.AddLink(source.Header.ID, movement.Header.ID, "movement_for", map[string]any{
		"movement_reason": reason,
	}); err != nil {
		return err
	}
	s.ensureBatchRecord(source.Header.OrganizationID, source.Header.LocationID, payload)
	return s.recordCostImpact(movement, actorID)
}

func (s *InventoryCoreService) ensureBatchRecord(organizationID, locationID string, payload map[string]any) {
	if s.models == nil {
		return
	}
	itemCode := textValue(payload["item_code"])
	batchCode := textValue(payload["batch_code"])
	if itemCode == "" || batchCode == "" {
		return
	}
	warehouseCode := textValue(payload["warehouse_code"])
	existing, _, err := s.models.List("inventory_batch", model.Query{
		Filters: map[string]string{
			"item_code":      itemCode,
			"warehouse_code": warehouseCode,
			"batch_code":     batchCode,
		},
		Page: 1, PageSize: 1,
	})
	if err == nil && len(existing) > 0 {
		return
	}
	_, _ = s.models.Create("inventory_batch", "system", map[string]any{
		"item_code":       itemCode,
		"warehouse_code":  warehouseCode,
		"batch_code":      batchCode,
		"expiration_date": textValue(payload["expiration_date"]),
		"status":          s.suggestedBatchStatus(textValue(payload["expiration_date"]), time.Now().UTC(), s.nearExpiryDays(organizationID, locationID)),
	})
}

func (s *InventoryCoreService) batchKey(itemCode, warehouseCode, batchCode string) string {
	return fmt.Sprintf("%s|%s|%s", strings.TrimSpace(itemCode), strings.TrimSpace(warehouseCode), strings.TrimSpace(batchCode))
}

func (s *InventoryCoreService) batchStateIndex(organizationID, locationID string, now time.Time) map[string]inventoryBatchState {
	index := map[string]inventoryBatchState{}
	if s.models == nil {
		return index
	}
	items, _, err := s.models.List("inventory_batch", model.Query{Page: 1, PageSize: 1000})
	if err != nil {
		return index
	}
	nearDays := s.nearExpiryDays(organizationID, locationID)
	for _, item := range items {
		itemCode := textValue(item.Values["item_code"])
		warehouseCode := textValue(item.Values["warehouse_code"])
		batchCode := textValue(item.Values["batch_code"])
		if itemCode == "" || batchCode == "" {
			continue
		}
		state := inventoryBatchState{
			Status:          s.effectiveBatchStatus(textValue(item.Values["status"]), textValue(item.Values["expiration_date"]), now, nearDays),
			HoldReason:      textValue(item.Values["hold_reason"]),
			HoldNotes:       textValue(item.Values["hold_notes"]),
			RecallReference: textValue(item.Values["recall_reference"]),
		}
		state.IsIssuable = inventoryBatchStatusIssuable(state.Status)
		index[s.batchKey(itemCode, warehouseCode, batchCode)] = state
	}
	return index
}

func (s *InventoryCoreService) batchStatusIssuable(organizationID, locationID, itemCode, warehouseCode, batchCode, expirationDate string, now time.Time) bool {
	if strings.TrimSpace(batchCode) == "" {
		return true
	}
	state, ok := s.batchStateIndex(organizationID, locationID, now)[s.batchKey(itemCode, warehouseCode, batchCode)]
	if ok {
		return state.IsIssuable
	}
	return inventoryBatchStatusIssuable(s.suggestedBatchStatus(expirationDate, now, s.nearExpiryDays(organizationID, locationID)))
}

func inventoryBatchStatusIssuable(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active", "near_expiry":
		return true
	default:
		return false
	}
}

func (s *InventoryCoreService) nearExpiryDays(organizationID, locationID string) int {
	if s.config == nil {
		return defaultInventoryNearExpiryDays
	}
	entry, ok := s.config.Resolve("inventory.policy", organizationID, locationID)
	if !ok {
		return defaultInventoryNearExpiryDays
	}
	days := int(roundMoney(numberValue(entry.Value["near_expiry_days"])))
	if days <= 0 {
		return defaultInventoryNearExpiryDays
	}
	return days
}

func (s *InventoryCoreService) suggestedBatchStatus(expirationDate string, now time.Time, nearDays int) string {
	return s.effectiveBatchStatus("", expirationDate, now, nearDays)
}

func (s *InventoryCoreService) effectiveBatchStatus(rawStatus, expirationDate string, now time.Time, nearDays int) string {
	status := strings.ToLower(strings.TrimSpace(rawStatus))
	switch status {
	case "quarantined", "blocked", "recalled":
		return status
	}
	expirationDate = strings.TrimSpace(expirationDate)
	if expirationDate == "" {
		if status == "near_expiry" {
			return "active"
		}
		if status == "" {
			return "active"
		}
		return status
	}
	today := now.UTC().Format("2006-01-02")
	if expirationDate < today {
		return "expired"
	}
	if nearDays <= 0 {
		nearDays = defaultInventoryNearExpiryDays
	}
	if expiry, err := time.Parse("2006-01-02", expirationDate); err == nil {
		threshold := now.UTC().Truncate(24 * time.Hour).Add(time.Duration(nearDays) * 24 * time.Hour)
		if !expiry.After(threshold) {
			return "near_expiry"
		}
	}
	if status == "" || status == "expired" || status == "near_expiry" {
		return "active"
	}
	return status
}

func (s *InventoryCoreService) lookupItemPolicy(itemCode string) inventoryPolicy {
	policy := inventoryPolicy{
		TrackingMode:     "none",
		DefaultIssue:     "manual",
		InventoryAccount: "1200-INV",
		COGSAccount:      "5000-COGS",
		WIPAccount:       "1300-WIP",
	}
	if s.models == nil || strings.TrimSpace(itemCode) == "" {
		return policy
	}
	items, _, err := s.models.List("commercial_item", model.Query{
		Filters: map[string]string{
			"sku": itemCode,
		},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return policy
	}
	values := items[0].Values
	policy.Enabled = boolValue(values["inventory_enabled"])
	policy.TrackingMode = firstNonEmptyString(textValue(values["inventory_tracking_mode"]), "none")
	policy.ExpiryTracking = boolValue(values["expiry_tracking_enabled"])
	policy.AllowNegativeStock = boolValue(values["allow_negative_stock"])
	policy.DefaultIssue = firstNonEmptyString(textValue(values["default_issue_strategy"]), "manual")
	policy.Name = firstNonEmptyString(textValue(values["name"]), itemCode)
	policy.UOMCode = textValue(values["uom_code"])
	policy.ItemType = textValue(values["item_type"])
	policy.InventoryAccount = firstNonEmptyString(textValue(values["inventory_asset_account_code"]), policy.InventoryAccount)
	policy.COGSAccount = firstNonEmptyString(textValue(values["cogs_account_code"]), policy.COGSAccount)
	policy.WIPAccount = firstNonEmptyString(textValue(values["wip_account_code"]), policy.WIPAccount)
	return policy
}

func (s *InventoryCoreService) CurrentAverageUnitCost(organizationID, locationID, itemCode, warehouseCode string) float64 {
	snapshot, _ := s.currentValuationSnapshot(organizationID, locationID, itemCode, warehouseCode)
	return roundMoney(snapshot.AverageUnitCost)
}

func (s *InventoryCoreService) CostAccounts(itemCode string) (string, string, string) {
	policy := s.lookupItemPolicy(itemCode)
	return policy.InventoryAccount, policy.COGSAccount, policy.WIPAccount
}

func (s *InventoryCoreService) currentBalances(organizationID, locationID string) []inventoryBalance {
	balances := map[string]inventoryBalance{}
	for _, record := range s.documents.List() {
		if record.Header.Type != "stock_movement" || record.Header.Status != "posted" {
			continue
		}
		if !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		payload := record.Body.Payload
		key := fmt.Sprintf("%s|%s|%s|%s", textValue(payload["item_code"]), textValue(payload["warehouse_code"]), textValue(payload["batch_code"]), textValue(payload["expiration_date"]))
		current := balances[key]
		current.ItemCode = textValue(payload["item_code"])
		current.WarehouseCode = textValue(payload["warehouse_code"])
		current.BatchCode = textValue(payload["batch_code"])
		current.ExpirationDate = textValue(payload["expiration_date"])
		current.Quantity = roundMoney(current.Quantity + numberValue(payload["quantity_delta"]))
		balances[key] = current
	}
	results := make([]inventoryBalance, 0, len(balances))
	for _, balance := range balances {
		results = append(results, balance)
	}
	return results
}

func (s *InventoryCoreService) currentReservedBalances(organizationID, locationID, excludeDocumentID string) []inventoryBalance {
	balances := map[string]inventoryBalance{}
	for _, record := range s.documents.List() {
		if record.Header.ID == excludeDocumentID {
			continue
		}
		if !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		switch record.Header.Type {
		case "sales_fulfillment":
			if record.Header.Status != "draft" && record.Header.Status != "submitted" {
				continue
			}
			for _, line := range recordList(record.Body.Payload["lines"]) {
				itemCode := textValue(line["item_code"])
				warehouseCode := textValue(line["warehouse_code"])
				qty := roundMoney(numberValue(line["quantity"]))
				if itemCode == "" || warehouseCode == "" || qty <= 0 {
					continue
				}
				key := fmt.Sprintf("%s|%s|%s|%s", itemCode, warehouseCode, textValue(line["batch_code"]), textValue(line["expiration_date"]))
				current := balances[key]
				current.ItemCode = itemCode
				current.WarehouseCode = warehouseCode
				current.BatchCode = textValue(line["batch_code"])
				current.ExpirationDate = textValue(line["expiration_date"])
				current.Quantity = roundMoney(current.Quantity + qty)
				balances[key] = current
			}
		case "production_order":
			if record.Header.Status != "approved" && record.Header.Status != "in_progress" {
				continue
			}
			for _, line := range recordList(record.Body.Payload["lines"]) {
				itemCode := firstNonEmptyString(textValue(line["actual_item_code"]), textValue(line["component_item_code"]), textValue(line["item_code"]))
				warehouseCode := textValue(line["warehouse_code"])
				qty := roundMoney(numberValue(line["reserved_quantity"]))
				if itemCode == "" || warehouseCode == "" || qty <= 0 {
					continue
				}
				key := fmt.Sprintf("%s|%s|%s|%s", itemCode, warehouseCode, textValue(line["batch_code"]), textValue(line["expiration_date"]))
				current := balances[key]
				current.ItemCode = itemCode
				current.WarehouseCode = warehouseCode
				current.BatchCode = textValue(line["batch_code"])
				current.ExpirationDate = textValue(line["expiration_date"])
				current.Quantity = roundMoney(current.Quantity + qty)
				balances[key] = current
			}
		}
	}
	results := make([]inventoryBalance, 0, len(balances))
	for _, balance := range balances {
		results = append(results, balance)
	}
	return results
}

func (s *InventoryCoreService) sumBalance(balances []inventoryBalance, itemCode, warehouseCode, batchCode string) float64 {
	total := 0.0
	for _, balance := range balances {
		if balance.ItemCode != itemCode || balance.WarehouseCode != warehouseCode {
			continue
		}
		if batchCode != "" && balance.BatchCode != batchCode {
			continue
		}
		total = roundMoney(total + balance.Quantity)
	}
	return total
}

func (s *InventoryCoreService) sumBalanceExact(balances []inventoryBalance, itemCode, warehouseCode, batchCode, expirationDate string) float64 {
	total := 0.0
	for _, balance := range balances {
		if balance.ItemCode != itemCode || balance.WarehouseCode != warehouseCode {
			continue
		}
		if batchCode != "" && balance.BatchCode != batchCode {
			continue
		}
		if expirationDate != "" && balance.ExpirationDate != expirationDate {
			continue
		}
		total = roundMoney(total + balance.Quantity)
	}
	return total
}

func (s *InventoryCoreService) availableFulfillmentQuantity(balances, reserved []inventoryBalance, itemCode, warehouseCode, batchCode, expirationDate string, policy inventoryPolicy) float64 {
	if policy.TrackingMode == "batch" && policy.ExpiryTracking {
		return roundMoney(s.sumBalanceExact(balances, itemCode, warehouseCode, batchCode, expirationDate) - s.sumBalanceExact(reserved, itemCode, warehouseCode, batchCode, expirationDate))
	}
	return roundMoney(s.sumBalance(balances, itemCode, warehouseCode, batchCode) - s.sumBalance(reserved, itemCode, warehouseCode, batchCode))
}

func isExpiredInventoryBatch(expirationDate, today string) bool {
	expirationDate = strings.TrimSpace(expirationDate)
	return expirationDate != "" && expirationDate < today
}

func (s *InventoryCoreService) fefoCandidates(organizationID, locationID string, balances []inventoryBalance, itemCode, warehouseCode string, excludeExpired bool, today string, now time.Time) []inventoryBalance {
	candidates := make([]inventoryBalance, 0)
	for _, balance := range balances {
		if balance.ItemCode != itemCode || balance.WarehouseCode != warehouseCode || balance.Quantity <= 0 {
			continue
		}
		if excludeExpired && isExpiredInventoryBatch(balance.ExpirationDate, today) {
			continue
		}
		if !s.batchStatusIssuable(organizationID, locationID, balance.ItemCode, balance.WarehouseCode, balance.BatchCode, balance.ExpirationDate, now) {
			continue
		}
		candidates = append(candidates, balance)
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftExpiry := candidates[i].ExpirationDate
		rightExpiry := candidates[j].ExpirationDate
		if leftExpiry == "" && rightExpiry != "" {
			return false
		}
		if leftExpiry != "" && rightExpiry == "" {
			return true
		}
		if leftExpiry != rightExpiry {
			return leftExpiry < rightExpiry
		}
		return candidates[i].BatchCode < candidates[j].BatchCode
	})
	return candidates
}

func (s *InventoryCoreService) fulfillmentCandidates(organizationID, locationID string, balances, reserved []inventoryBalance, itemCode, warehouseCode string, policy inventoryPolicy, today string) []inventoryBalance {
	candidates := make([]inventoryBalance, 0)
	for _, balance := range balances {
		if balance.ItemCode != itemCode || balance.Quantity <= 0 {
			continue
		}
		if warehouseCode != "" && balance.WarehouseCode != warehouseCode {
			continue
		}
		if policy.TrackingMode == "batch" && policy.ExpiryTracking && isExpiredInventoryBatch(balance.ExpirationDate, today) {
			continue
		}
		if policy.TrackingMode == "batch" && !s.batchStatusIssuable(organizationID, locationID, balance.ItemCode, balance.WarehouseCode, balance.BatchCode, balance.ExpirationDate, time.Now().UTC()) {
			continue
		}
		available := s.availableFulfillmentQuantity(balances, reserved, itemCode, balance.WarehouseCode, balance.BatchCode, balance.ExpirationDate, policy)
		if available <= 0 {
			continue
		}
		candidate := balance
		candidate.Quantity = available
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftExpiry := candidates[i].ExpirationDate
		rightExpiry := candidates[j].ExpirationDate
		if leftExpiry == "" && rightExpiry != "" {
			return false
		}
		if leftExpiry != "" && rightExpiry == "" {
			return true
		}
		if leftExpiry != rightExpiry {
			return leftExpiry < rightExpiry
		}
		if candidates[i].WarehouseCode != candidates[j].WarehouseCode {
			return candidates[i].WarehouseCode < candidates[j].WarehouseCode
		}
		return candidates[i].BatchCode < candidates[j].BatchCode
	})
	return candidates
}

func (s *InventoryCoreService) resolveFulfillmentLinesWithBalances(organizationID, locationID string, balances, reserved []inventoryBalance, rows []map[string]any) ([]map[string]any, error) {
	lines := make([]map[string]any, 0)
	today := time.Now().UTC().Format("2006-01-02")
	for _, row := range rows {
		itemCode := textValue(row["item_code"])
		if itemCode == "" {
			return nil, shared.Validation("item code is required")
		}
		policy := s.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			return nil, shared.Validation(fmt.Sprintf("item %s is not inventory-tracked", itemCode))
		}
		qty := roundMoney(numberValue(row["quantity"]))
		if qty <= 0 {
			return nil, shared.Validation(fmt.Sprintf("quantity must be greater than zero for item %s", itemCode))
		}
		warehouseCode := textValue(row["warehouse_code"])
		explicitBatch := textValue(row["batch_code"])
		if policy.TrackingMode != "batch" {
			candidates := s.fulfillmentCandidates(organizationID, locationID, balances, reserved, itemCode, warehouseCode, policy, today)
			if warehouseCode != "" {
				available := roundMoney(s.sumBalance(balances, itemCode, warehouseCode, "") - s.sumBalance(reserved, itemCode, warehouseCode, ""))
				if !policy.AllowNegativeStock && available < qty {
					return nil, shared.Validation(fmt.Sprintf("insufficient available stock for item %s in warehouse %s", itemCode, warehouseCode))
				}
				line := cloneMap(row)
				line["warehouse_code"] = warehouseCode
				line["available_quantity"] = available
				lines = append(lines, line)
				continue
			}
			remaining := qty
			for _, candidate := range candidates {
				if remaining <= 0 {
					break
				}
				allocate := roundMoney(minFloat(candidate.Quantity, remaining))
				if allocate <= 0 {
					continue
				}
				line := cloneMap(row)
				line["warehouse_code"] = candidate.WarehouseCode
				line["quantity"] = allocate
				line["available_quantity"] = candidate.Quantity
				lines = append(lines, line)
				remaining = roundMoney(remaining - allocate)
			}
			if remaining > 0 {
				return nil, shared.Validation(fmt.Sprintf("insufficient available stock for item %s", itemCode))
			}
			continue
		}

		explicitExpiration := textValue(row["expiration_date"])
		if explicitBatch != "" && policy.ExpiryTracking && isExpiredInventoryBatch(explicitExpiration, today) {
			return nil, shared.Validation(fmt.Sprintf("expired batch %s cannot be fulfilled for item %s", explicitBatch, itemCode))
		}
		if explicitBatch != "" && !s.batchStatusIssuable(organizationID, locationID, itemCode, warehouseCode, explicitBatch, explicitExpiration, time.Now().UTC()) {
			return nil, shared.Validation(fmt.Sprintf("batch %s cannot be fulfilled for item %s", explicitBatch, itemCode))
		}
		candidates := s.fulfillmentCandidates(organizationID, locationID, balances, reserved, itemCode, warehouseCode, policy, today)
		if explicitBatch != "" {
			filtered := make([]inventoryBalance, 0, len(candidates))
			for _, candidate := range candidates {
				if candidate.BatchCode != explicitBatch {
					continue
				}
				if explicitExpiration != "" && candidate.ExpirationDate != explicitExpiration {
					continue
				}
				if policy.ExpiryTracking && isExpiredInventoryBatch(candidate.ExpirationDate, today) {
					continue
				}
				if candidate.BatchCode == explicitBatch {
					filtered = append(filtered, candidate)
				}
			}
			candidates = filtered
		}
		remaining := qty
		for _, candidate := range candidates {
			if remaining <= 0 {
				break
			}
			allocate := roundMoney(minFloat(candidate.Quantity, remaining))
			if allocate <= 0 {
				continue
			}
			line := cloneMap(row)
			line["warehouse_code"] = candidate.WarehouseCode
			line["batch_code"] = candidate.BatchCode
			line["expiration_date"] = candidate.ExpirationDate
			line["quantity"] = allocate
			line["available_quantity"] = candidate.Quantity
			lines = append(lines, line)
			remaining = roundMoney(remaining - allocate)
		}
		if remaining > 0 {
			if explicitBatch != "" && warehouseCode != "" {
				return nil, shared.Validation(fmt.Sprintf("insufficient available stock for item %s batch %s in warehouse %s", itemCode, explicitBatch, warehouseCode))
			}
			return nil, shared.Validation(fmt.Sprintf("insufficient FEFO stock for item %s", itemCode))
		}
	}
	return lines, nil
}

func (s *InventoryCoreService) hasMovementLink(record document.Record, reason string) bool {
	for _, link := range record.Links {
		if link.LinkType != "movement_for" {
			continue
		}
		if textValue(link.Metadata["movement_reason"]) == reason {
			return true
		}
	}
	return false
}

func (s *InventoryCoreService) findMovementsForReason(record document.Record, reason string) []document.Record {
	results := make([]document.Record, 0)
	for _, link := range record.Links {
		if link.LinkType != "movement_for" || textValue(link.Metadata["movement_reason"]) != reason {
			continue
		}
		movement, err := s.documents.Get(link.LinkedDocumentID)
		if err == nil && movement.Header.Type == "stock_movement" {
			results = append(results, movement)
		}
	}
	return results
}

func (s *InventoryCoreService) updateDocumentPayload(record document.Record, actorID string, payload map[string]any) error {
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *InventoryCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
	record.Body.Payload = document.NormalizePayload(payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "EA"),
		AmountMinor: 0,
	}
	if err := s.documents.Save(record); err != nil {
		return err
	}
	s.refreshDocuments(record)
	return nil
}

func (s *InventoryCoreService) finalizeSystemMovement(record document.Record, actorID, status string) error {
	record.Header.Status = status
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.TotalAmount = shared.Money{Currency: "EA", AmountMinor: 0}
	if err := s.documents.Save(record); err != nil {
		return err
	}
	s.refreshDocuments(record)
	return nil
}

func (s *InventoryCoreService) refreshDocuments(records ...document.Record) {
	if s.search == nil {
		return
	}
	for _, record := range records {
		if strings.TrimSpace(record.Header.ID) == "" {
			continue
		}
		s.search.RefreshDocument(record)
	}
}

func matchesInventoryScope(record document.Record, organizationID, locationID string) bool {
	if organizationID != "" && record.Header.OrganizationID != "" && record.Header.OrganizationID != organizationID {
		return false
	}
	if locationID != "" && record.Header.LocationID != "" && record.Header.LocationID != locationID {
		return false
	}
	return true
}

func isMissingModelDefinitionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "model definition not found") || strings.Contains(message, "model not found")
}

func (s *InventoryCoreService) prepareMovementLineForCost(source document.Record, reason string, line map[string]any, direction string) map[string]any {
	next := cloneMap(line)
	itemCode := textValue(next["item_code"])
	warehouseCode := textValue(next["warehouse_code"])
	quantity := roundMoney(numberValue(next["quantity"]))
	if quantity <= 0 {
		return next
	}
	inventoryAccount, cogsAccount, wipAccount := s.CostAccounts(itemCode)
	next["inventory_asset_account_code"] = firstNonEmptyString(textValue(next["inventory_asset_account_code"]), inventoryAccount)
	next["cogs_account_code"] = firstNonEmptyString(textValue(next["cogs_account_code"]), cogsAccount)
	next["wip_account_code"] = firstNonEmptyString(textValue(next["wip_account_code"]), wipAccount)
	next["currency_code"] = firstNonEmptyString(textValue(next["currency_code"]), "IDR")
	unitCost := roundMoney(numberValue(next["unit_cost"]))
	if unitCost <= 0 {
		switch direction {
		case "out":
			unitCost = s.CurrentAverageUnitCost(source.Header.OrganizationID, source.Header.LocationID, itemCode, warehouseCode)
		default:
			unitCost = firstPositiveNumber(
				next["received_unit_cost"],
				next["receipt_unit_cost"],
				next["unit_price"],
				next["base_unit_cost"],
				next["average_unit_cost"],
			)
		}
	}
	next["unit_cost"] = roundMoney(unitCost)
	totalCost := roundMoney(numberValue(next["total_cost"]))
	if totalCost == 0 && unitCost != 0 {
		totalCost = roundMoney(quantity * unitCost)
	}
	next["extended_cost"] = roundMoney(maxFloat(totalCost, 0))
	if direction == "out" {
		next["total_cost"] = roundMoney(-maxFloat(totalCost, 0))
	} else {
		next["total_cost"] = roundMoney(maxFloat(totalCost, 0))
	}
	return next
}

func (s *InventoryCoreService) recordCostImpact(movement document.Record, actorID string) error {
	if s.models == nil {
		return nil
	}
	payload := movement.Body.Payload
	itemCode := textValue(payload["item_code"])
	warehouseCode := textValue(payload["warehouse_code"])
	qtyDelta := roundMoney(numberValue(payload["quantity_delta"]))
	totalCost := roundMoney(numberValue(payload["total_cost"]))
	unitCost := roundMoney(numberValue(payload["unit_cost"]))
	if itemCode == "" || warehouseCode == "" {
		return nil
	}
	if err := s.recordCostLayer(movement, actorID, itemCode, warehouseCode, qtyDelta, unitCost, totalCost); err != nil {
		return err
	}
	return s.applyValuationDelta(actorID, movement.Header.OrganizationID, movement.Header.LocationID, itemCode, warehouseCode, qtyDelta, totalCost)
}

func (s *InventoryCoreService) recordCostLayer(movement document.Record, actorID, itemCode, warehouseCode string, qtyDelta, unitCost, totalCost float64) error {
	if s.models == nil {
		return nil
	}
	_, err := s.models.Create("inventory_cost_layer", actorID, map[string]any{
		"organization_id":      movement.Header.OrganizationID,
		"location_id":          movement.Header.LocationID,
		"item_code":            itemCode,
		"warehouse_code":       warehouseCode,
		"batch_code":           textValue(movement.Body.Payload["batch_code"]),
		"source_document_type": textValue(movement.Body.Payload["source_document_type"]),
		"source_document_id":   textValue(movement.Body.Payload["source_document_id"]),
		"movement_document_id": movement.Header.ID,
		"event_type":           textValue(movement.Body.Payload["movement_reason"]),
		"quantity_delta":       qtyDelta,
		"unit_cost":            unitCost,
		"total_cost":           totalCost,
		"currency_code":        firstNonEmptyString(textValue(movement.Body.Payload["currency_code"]), "IDR"),
		"valuation_method":     "weighted_average",
		"effective_at":         time.Now().UTC().Format(time.RFC3339),
		"status":               "posted",
	})
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	return nil
}

func (s *InventoryCoreService) currentValuationSnapshot(organizationID, locationID, itemCode, warehouseCode string) (inventoryValuationSnapshot, bool) {
	if s.models == nil || itemCode == "" || warehouseCode == "" {
		return inventoryValuationSnapshot{}, false
	}
	filters := map[string]string{"item_code": itemCode, "warehouse_code": warehouseCode}
	if strings.TrimSpace(organizationID) != "" {
		filters["organization_id"] = organizationID
	}
	if strings.TrimSpace(locationID) != "" {
		filters["location_id"] = locationID
	}
	items, _, err := s.models.List("inventory_valuation_snapshot", model.Query{
		Filters:  filters,
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return inventoryValuationSnapshot{}, false
	}
	return inventoryValuationSnapshot{
		ID:              items[0].ID,
		Version:         items[0].Version,
		ItemCode:        textValue(items[0].Values["item_code"]),
		WarehouseCode:   textValue(items[0].Values["warehouse_code"]),
		QuantityOnHand:  roundMoney(numberValue(items[0].Values["quantity_on_hand"])),
		AverageUnitCost: roundMoney(numberValue(items[0].Values["average_unit_cost"])),
		InventoryValue:  roundMoney(numberValue(items[0].Values["inventory_value"])),
	}, true
}

func (s *InventoryCoreService) applyValuationDelta(actorID, organizationID, locationID, itemCode, warehouseCode string, qtyDelta, totalCost float64) error {
	snapshot, ok := s.currentValuationSnapshot(organizationID, locationID, itemCode, warehouseCode)
	quantityOnHand := roundMoney(snapshot.QuantityOnHand + qtyDelta)
	inventoryValue := roundMoney(snapshot.InventoryValue + totalCost)
	if roundMoney(quantityOnHand) == 0 {
		quantityOnHand = 0
		inventoryValue = 0
	}
	averageUnitCost := 0.0
	if quantityOnHand != 0 {
		averageUnitCost = roundMoney(inventoryValue / quantityOnHand)
	}
	values := map[string]any{
		"organization_id":    organizationID,
		"location_id":        locationID,
		"item_code":          itemCode,
		"warehouse_code":     warehouseCode,
		"quantity_on_hand":   quantityOnHand,
		"average_unit_cost":  averageUnitCost,
		"inventory_value":    inventoryValue,
		"valuation_method":   "weighted_average",
		"last_calculated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if !ok {
		_, err := s.models.Create("inventory_valuation_snapshot", actorID, values)
		if err != nil && !isMissingModelDefinitionError(err) {
			return err
		}
		return nil
	}
	_, err := s.models.Update("inventory_valuation_snapshot", snapshot.ID, actorID, values, snapshot.Version)
	if err != nil && !isMissingModelDefinitionError(err) {
		return err
	}
	return nil
}

func (s *InventoryCoreService) createFulfillmentCostPosting(record document.Record, actorID string, lines []map[string]any) error {
	if s.hasPostingLink(record, "fulfillment_issue_cogs") {
		return nil
	}
	creditByAccount := map[string]float64{}
	debitByAccount := map[string]float64{}
	totalCost := 0.0
	for _, line := range lines {
		extendedCost := roundMoney(numberValue(line["extended_cost"]))
		if extendedCost <= 0 {
			continue
		}
		totalCost = roundMoney(totalCost + extendedCost)
		debitByAccount[firstNonEmptyString(textValue(line["cogs_account_code"]), "5000-COGS")] = roundMoney(debitByAccount[firstNonEmptyString(textValue(line["cogs_account_code"]), "5000-COGS")] + extendedCost)
		creditByAccount[firstNonEmptyString(textValue(line["inventory_asset_account_code"]), "1200-INV")] = roundMoney(creditByAccount[firstNonEmptyString(textValue(line["inventory_asset_account_code"]), "1200-INV")] + extendedCost)
	}
	if totalCost <= 0 {
		return nil
	}
	journalLines := make([]map[string]any, 0, len(debitByAccount)+len(creditByAccount))
	for account, amount := range debitByAccount {
		journalLines = append(journalLines, map[string]any{"account_code": account, "description": "COGS", "debit": amount, "credit": 0.0})
	}
	for account, amount := range creditByAccount {
		journalLines = append(journalLines, map[string]any{"account_code": account, "description": "Inventory", "debit": 0.0, "credit": amount})
	}
	postingPayload := map[string]any{
		"source_document_type": record.Header.Type,
		"source_document_id":   record.Header.ID,
		"posting_date":         time.Now().UTC().Format("2006-01-02"),
		"currency_code":        "IDR",
		"posting_rule_key":     "fulfillment_issue_cogs_default",
		"total_amount":         totalCost,
		"journal_lines":        journalLines,
		"notes":                fmt.Sprintf("Auto-posted COGS from fulfillment %s", firstNonEmptyString(record.Header.Number, record.Header.ID)),
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
	if _, err := s.documents.AddLink(posting.Header.ID, record.Header.ID, "posting_for", map[string]any{"posting_reason": "fulfillment_issue_cogs"}); err != nil {
		return err
	}
	_, err = s.documents.AddLink(record.Header.ID, posting.Header.ID, "posting_for", map[string]any{"posting_reason": "fulfillment_issue_cogs"})
	return err
}

func (s *InventoryCoreService) hasPostingLink(record document.Record, reason string) bool {
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

func (s *InventoryCoreService) finalizeSystemPosting(record document.Record, actorID, status string) error {
	record.Header.Status = status
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = time.Now().UTC()
	record.Header.TotalAmount = shared.Money{
		Currency:    firstNonEmptyString(textValue(record.Body.Payload["currency_code"]), record.Header.TotalAmount.Currency, "IDR"),
		AmountMinor: moneyMinor(numberValue(record.Body.Payload["total_amount"])),
	}
	if err := s.documents.Save(record); err != nil {
		return err
	}
	s.refreshDocuments(record)
	return nil
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func sumInventoryLineQuantity(lines []map[string]any, key string) float64 {
	total := 0.0
	for _, line := range lines {
		total = roundMoney(total + roundMoney(numberValue(line[key])))
	}
	return total
}

func firstPositiveNumber(values ...any) float64 {
	for _, value := range values {
		if number := roundMoney(numberValue(value)); number > 0 {
			return number
		}
	}
	return 0
}
