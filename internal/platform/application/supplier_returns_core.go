package application

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type SupplierReturnsCoreService struct {
	documents   *document.Service
	search      *search.Service
	inventory   *InventoryCoreService
	procurement *ProcurementCoreService
}

func NewSupplierReturnsCoreService(documents *document.Service, searchSvc *search.Service, inventorySvc *InventoryCoreService, procurementSvc *ProcurementCoreService) *SupplierReturnsCoreService {
	return &SupplierReturnsCoreService{
		documents:   documents,
		search:      searchSvc,
		inventory:   inventorySvc,
		procurement: procurementSvc,
	}
}

func (s *SupplierReturnsCoreService) GenerateSupplierReturnFromReceipt(receiptID, actorID string) (document.Record, error) {
	receipt, err := s.documents.Get(strings.TrimSpace(receiptID))
	if err != nil {
		return document.Record{}, err
	}
	if receipt.Header.Type != "goods_receipt" {
		return document.Record{}, shared.Validation("source document must be a goods receipt")
	}
	if receipt.Header.Status != "received" {
		return document.Record{}, shared.Conflict("supplier return can only be registered from a received goods receipt")
	}
	payload := clonedPayload(receipt.Body.Payload)
	now := time.Now().UTC()
	lines := make([]map[string]any, 0)
	for index, line := range recordList(payload["lines"]) {
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			continue
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			continue
		}
		qty := roundMoney(firstPositiveNumber(line["receipt_qty"], line["received_qty"], line["quantity"]))
		warehouseCode := textValue(line["warehouse_code"])
		batchCode := textValue(line["batch_code"])
		expirationDate := textValue(line["expiration_date"])
		returnableQty := s.availableReturnableQuantity(receipt.Header.OrganizationID, receipt.Header.LocationID, itemCode, warehouseCode, batchCode, expirationDate, policy)
		qty = roundMoney(minFloat(qty, returnableQty))
		if qty <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"source_document_line_index": index,
			"item_code":                  itemCode,
			"description":                textValue(line["description"]),
			"warehouse_code":             warehouseCode,
			"batch_code":                 batchCode,
			"expiration_date":            expirationDate,
			"uom_code":                   firstNonEmptyString(textValue(line["uom_code"]), policy.UOMCode),
			"received_quantity":          roundMoney(firstPositiveNumber(line["receipt_qty"], line["received_qty"], line["quantity"])),
			"quantity":                   qty,
			"note":                       "",
		})
	}
	if len(lines) == 0 {
		return document.Record{}, shared.Validation("goods receipt has no returnable inventory lines")
	}
	returnPayload := map[string]any{
		"vendor_id":                    textValue(payload["vendor_id"]),
		"vendor_name":                  textValue(payload["vendor_name"]),
		"source_purchase_order_id":     textValue(payload["source_purchase_order_id"]),
		"source_purchase_order_number": textValue(payload["source_purchase_order_number"]),
		"source_goods_receipt_id":      receipt.Header.ID,
		"source_goods_receipt_number":  firstNonEmptyString(receipt.Header.Number, receipt.Header.ID),
		"return_date":                  now.Format("2006-01-02"),
		"warehouse_code":               firstNonEmptyString(textValue(lines[0]["warehouse_code"]), textValue(payload["warehouse_code"])),
		"reason":                       fmt.Sprintf("Return to vendor for receipt %s", firstNonEmptyString(receipt.Header.Number, receipt.Header.ID)),
		"return_status":                "draft",
		"credit_note_status":           "",
		"total_quantity":               roundMoney(sumInventoryLineQuantity(lines, "quantity")),
		"lines":                        lines,
	}
	record, err := s.documents.Create("supplier_return", receipt.Header.OrganizationID, receipt.Header.LocationID, actorID, returnPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, receipt.Header.ID, "return_for", map[string]any{"source_type": "goods_receipt"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(receipt.Header.ID, record.Header.ID, "return_for", map[string]any{"generated_document_type": "supplier_return"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, receipt)
	return created, nil
}

func (s *SupplierReturnsCoreService) GenerateSupplierReturnFromBill(billID, actorID string) (document.Record, error) {
	bill, err := s.documents.Get(strings.TrimSpace(billID))
	if err != nil {
		return document.Record{}, err
	}
	if bill.Header.Type != "vendor_bill" {
		return document.Record{}, shared.Validation("source document must be a vendor bill")
	}
	if bill.Header.Status != "issued" && bill.Header.Status != "partially_paid" && bill.Header.Status != "paid" {
		return document.Record{}, shared.Conflict("supplier return can only be registered from an issued vendor bill")
	}
	payload := clonedPayload(bill.Body.Payload)
	now := time.Now().UTC()
	lines := make([]map[string]any, 0)
	for index, line := range recordList(payload["lines"]) {
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			continue
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			continue
		}
		warehouseCode := firstNonEmptyString(textValue(line["warehouse_code"]), textValue(payload["warehouse_code"]), s.defaultWarehouseForItem(itemCode))
		batchCode := textValue(line["batch_code"])
		expirationDate := textValue(line["expiration_date"])
		if policy.TrackingMode == "batch" && batchCode == "" {
			balance := s.firstAvailableReturnableBatch(bill.Header.OrganizationID, bill.Header.LocationID, itemCode, warehouseCode, policy)
			if warehouseCode == "" {
				warehouseCode = balance.WarehouseCode
			}
			if batchCode == "" {
				batchCode = balance.BatchCode
			}
			if expirationDate == "" {
				expirationDate = balance.ExpirationDate
			}
		}
		lineQty := roundMoney(firstPositiveNumber(line["billed_qty"], line["quantity"]))
		returnableQty := s.availableReturnableQuantity(bill.Header.OrganizationID, bill.Header.LocationID, itemCode, warehouseCode, batchCode, expirationDate, policy)
		lineQty = roundMoney(minFloat(lineQty, returnableQty))
		if lineQty <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"source_document_line_index": index,
			"item_code":                  itemCode,
			"description":                textValue(line["description"]),
			"warehouse_code":             warehouseCode,
			"batch_code":                 batchCode,
			"expiration_date":            expirationDate,
			"uom_code":                   firstNonEmptyString(textValue(line["uom_code"]), policy.UOMCode),
			"received_quantity":          roundMoney(firstPositiveNumber(line["billed_qty"], line["quantity"])),
			"quantity":                   lineQty,
			"note":                       "",
		})
	}
	if len(lines) == 0 {
		return document.Record{}, shared.Validation("vendor bill has no returnable inventory lines")
	}
	returnPayload := map[string]any{
		"vendor_id":                    textValue(payload["vendor_id"]),
		"vendor_name":                  textValue(payload["vendor_name"]),
		"source_purchase_order_id":     textValue(payload["source_purchase_order_id"]),
		"source_purchase_order_number": textValue(payload["source_purchase_order_number"]),
		"source_goods_receipt_id":      textValue(payload["source_goods_receipt_id"]),
		"source_goods_receipt_number":  textValue(payload["source_goods_receipt_number"]),
		"source_vendor_bill_id":        bill.Header.ID,
		"source_vendor_bill_number":    firstNonEmptyString(bill.Header.Number, bill.Header.ID),
		"return_date":                  now.Format("2006-01-02"),
		"warehouse_code":               firstNonEmptyString(textValue(lines[0]["warehouse_code"]), textValue(payload["warehouse_code"])),
		"reason":                       fmt.Sprintf("Return to vendor for bill %s", firstNonEmptyString(bill.Header.Number, bill.Header.ID)),
		"return_status":                "draft",
		"credit_note_status":           "",
		"total_quantity":               roundMoney(sumInventoryLineQuantity(lines, "quantity")),
		"lines":                        lines,
	}
	record, err := s.documents.Create("supplier_return", bill.Header.OrganizationID, bill.Header.LocationID, actorID, returnPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, bill.Header.ID, "return_for", map[string]any{"source_type": "vendor_bill"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(bill.Header.ID, record.Header.ID, "return_for", map[string]any{"generated_document_type": "supplier_return"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, bill)
	return created, nil
}

func (s *SupplierReturnsCoreService) CreateVendorCreditFromReturn(returnID, actorID string) (document.Record, error) {
	if s.procurement == nil {
		return document.Record{}, shared.Validation("procurement service is unavailable")
	}
	supplierReturn, err := s.documents.Get(strings.TrimSpace(returnID))
	if err != nil {
		return document.Record{}, err
	}
	if supplierReturn.Header.Type != "supplier_return" {
		return document.Record{}, shared.Validation("source document must be a supplier return")
	}
	payload := clonedPayload(supplierReturn.Body.Payload)
	billID := textValue(payload["source_vendor_bill_id"])
	if billID == "" {
		return document.Record{}, shared.Validation("supplier return source vendor bill is required")
	}
	bill, err := s.documents.Get(billID)
	if err != nil {
		return document.Record{}, err
	}
	billPayload := clonedPayload(bill.Body.Payload)
	billLines := recordList(billPayload["lines"])
	returnLines := recordList(payload["lines"])
	creditLines := make([]map[string]any, 0, len(returnLines))
	for _, line := range returnLines {
		itemCode := textValue(line["item_code"])
		qty := roundMoney(numberValue(line["quantity"]))
		if itemCode == "" || qty <= 0 {
			continue
		}
		sourceLine := procurementLineForItem(billLines, itemCode)
		unitPrice := roundMoney(numberValue(sourceLine["unit_price"]))
		taxRate := roundMoney(numberValue(sourceLine["tax_rate"]))
		lineSubtotal := roundMoney(qty * unitPrice)
		lineTax := roundMoney(lineSubtotal * taxRate / 100)
		lineTotal := roundMoney(lineSubtotal + lineTax)
		creditLines = append(creditLines, map[string]any{
			"item_code":   itemCode,
			"description": firstNonEmptyString(textValue(line["description"]), textValue(sourceLine["description"])),
			"uom_code":    firstNonEmptyString(textValue(line["uom_code"]), textValue(sourceLine["uom_code"])),
			"quantity":    qty,
			"billed_qty":  qty,
			"unit_price":  unitPrice,
			"tax_code":    textValue(sourceLine["tax_code"]),
			"tax_rate":    taxRate,
			"line_total":  lineTotal,
		})
	}
	if len(creditLines) == 0 {
		return document.Record{}, shared.Validation("supplier return has no creditable lines")
	}
	creditPayload := map[string]any{
		"vendor_id":                 textValue(payload["vendor_id"]),
		"vendor_name":               textValue(payload["vendor_name"]),
		"credit_date":               time.Now().UTC().Format("2006-01-02"),
		"currency_code":             firstNonEmptyString(textValue(billPayload["currency_code"]), bill.Header.TotalAmount.Currency, "IDR"),
		"source_vendor_bill_id":     bill.Header.ID,
		"source_vendor_bill_number": firstNonEmptyString(bill.Header.Number, bill.Header.ID),
		"payable_account_code":      textValue(billPayload["payable_account_code"]),
		"default_tax_code":          textValue(billPayload["default_tax_code"]),
		"tax_profile_code":          textValue(billPayload["tax_profile_code"]),
		"reason":                    fmt.Sprintf("Vendor credit for supplier return %s", firstNonEmptyString(supplierReturn.Header.Number, supplierReturn.Header.ID)),
		"lines":                     creditLines,
		"notes":                     textValue(payload["reason"]),
	}
	creditPayload = s.procurement.normalizeProcurementLines(creditPayload)
	record, err := s.documents.Create("vendor_credit_note", supplierReturn.Header.OrganizationID, supplierReturn.Header.LocationID, actorID, creditPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, supplierReturn.Header.ID, "credit_for", map[string]any{"source_type": "supplier_return"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(supplierReturn.Header.ID, record.Header.ID, "credit_for", map[string]any{"generated_document_type": "vendor_credit_note"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if err := s.updateReturnCreditState(supplierReturn.Header.ID, actorID, map[string]any{
		"source_vendor_credit_id":     record.Header.ID,
		"source_vendor_credit_number": firstNonEmptyString(record.Header.Number, record.Header.ID),
		"credit_note_status":          record.Header.Status,
	}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, supplierReturn, bill)
	return created, nil
}

func (s *SupplierReturnsCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	switch strings.TrimSpace(documentType) {
	case "supplier_return":
		lines := s.normalizeSupplierReturnLines(recordList(next["lines"]))
		next["lines"] = lines
		next["total_quantity"] = roundMoney(sumInventoryLineQuantity(lines, "quantity"))
	}
	return next
}

func (s *SupplierReturnsCoreService) ValidateApprove(record document.Record) error {
	if record.Header.Type != "supplier_return" {
		return nil
	}
	return s.validateSupplierReturn(record)
}

func (s *SupplierReturnsCoreService) ValidateCancel(record document.Record) error {
	if record.Header.Type == "supplier_return" && record.Header.Status == "approved" {
		return nil
	}
	return nil
}

func (s *SupplierReturnsCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	if record.Header.Type != "supplier_return" {
		return nil
	}
	if s.inventory == nil {
		return shared.Validation("inventory service is unavailable")
	}
	if s.inventory.hasMovementLink(record, "supplier_return") {
		return nil
	}
	for _, line := range recordList(record.Body.Payload["lines"]) {
		movementLine := cloneMap(line)
		movementLine["note"] = firstNonEmptyString(textValue(line["note"]), "Returned to vendor")
		if err := s.inventory.createMovement(record, actorID, "supplier_return", movementLine, "out"); err != nil {
			return err
		}
	}
	payload := clonedPayload(record.Body.Payload)
	payload["return_status"] = "approved"
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *SupplierReturnsCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	if record.Header.Type != "supplier_return" || s.inventory == nil {
		return nil
	}
	if err := s.inventory.reverseMovements(record, actorID, "supplier_return", "supplier_return_reversal"); err != nil {
		return err
	}
	payload := clonedPayload(record.Body.Payload)
	payload["return_status"] = "cancelled"
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *SupplierReturnsCoreService) validateSupplierReturn(record document.Record) error {
	lines := recordList(record.Body.Payload["lines"])
	if len(lines) == 0 {
		return shared.Validation("supplier return lines are required")
	}
	for _, line := range lines {
		itemCode := textValue(line["item_code"])
		warehouseCode := textValue(line["warehouse_code"])
		qty := roundMoney(numberValue(line["quantity"]))
		if itemCode == "" || warehouseCode == "" || qty <= 0 {
			return shared.Validation("supplier return lines require item, warehouse, and quantity")
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			return shared.Validation(fmt.Sprintf("item %s is not inventory-tracked", itemCode))
		}
		batchCode := textValue(line["batch_code"])
		expirationDate := textValue(line["expiration_date"])
		if policy.TrackingMode == "batch" && batchCode == "" {
			return shared.Validation(fmt.Sprintf("batch code is required for item %s", itemCode))
		}
		if policy.TrackingMode == "batch" && policy.ExpiryTracking && expirationDate == "" {
			return shared.Validation(fmt.Sprintf("expiration date is required for item %s", itemCode))
		}
		available := s.availableReturnableQuantity(record.Header.OrganizationID, record.Header.LocationID, itemCode, warehouseCode, batchCode, expirationDate, policy)
		if available < qty && !policy.AllowNegativeStock {
			return shared.Validation(fmt.Sprintf("insufficient available stock for supplier return item %s in warehouse %s", itemCode, warehouseCode))
		}
	}
	return nil
}

func (s *SupplierReturnsCoreService) normalizeSupplierReturnLines(lines []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		next := cloneMap(line)
		next["item_code"] = textValue(next["item_code"])
		next["description"] = textValue(next["description"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["batch_code"] = textValue(next["batch_code"])
		next["expiration_date"] = textValue(next["expiration_date"])
		next["uom_code"] = textValue(next["uom_code"])
		next["received_quantity"] = roundMoney(numberValue(next["received_quantity"]))
		next["quantity"] = roundMoney(numberValue(next["quantity"]))
		next["note"] = textValue(next["note"])
		normalized = append(normalized, next)
	}
	return normalized
}

func (s *SupplierReturnsCoreService) availableReturnableQuantity(organizationID, locationID, itemCode, warehouseCode, batchCode, expirationDate string, policy inventoryPolicy) float64 {
	if s.inventory == nil {
		return 0
	}
	balances := s.inventory.currentBalances(organizationID, locationID)
	reserved := s.inventory.currentReservedBalances(organizationID, locationID, "")
	if policy.TrackingMode == "batch" && policy.ExpiryTracking {
		return roundMoney(maxFloat(s.inventory.sumBalanceExact(balances, itemCode, warehouseCode, batchCode, expirationDate)-s.inventory.sumBalanceExact(reserved, itemCode, warehouseCode, batchCode, expirationDate), 0))
	}
	return roundMoney(maxFloat(s.inventory.sumBalance(balances, itemCode, warehouseCode, batchCode)-s.inventory.sumBalance(reserved, itemCode, warehouseCode, batchCode), 0))
}

func (s *SupplierReturnsCoreService) firstAvailableReturnableBatch(organizationID, locationID, itemCode, warehouseCode string, policy inventoryPolicy) inventoryBalance {
	if s.inventory == nil {
		return inventoryBalance{}
	}
	balances := s.inventory.currentBalances(organizationID, locationID)
	reserved := s.inventory.currentReservedBalances(organizationID, locationID, "")
	candidates := make([]inventoryBalance, 0)
	for _, balance := range balances {
		if balance.ItemCode != itemCode || balance.Quantity <= 0 {
			continue
		}
		if warehouseCode != "" && balance.WarehouseCode != warehouseCode {
			continue
		}
		available := s.availableReturnableQuantity(organizationID, locationID, itemCode, balance.WarehouseCode, balance.BatchCode, balance.ExpirationDate, policy)
		if available <= 0 {
			continue
		}
		if policy.TrackingMode == "batch" {
			if policy.ExpiryTracking {
				if s.inventory.sumBalanceExact(reserved, itemCode, balance.WarehouseCode, balance.BatchCode, balance.ExpirationDate) >= s.inventory.sumBalanceExact(balances, itemCode, balance.WarehouseCode, balance.BatchCode, balance.ExpirationDate) {
					continue
				}
			} else if s.inventory.sumBalance(reserved, itemCode, balance.WarehouseCode, balance.BatchCode) >= s.inventory.sumBalance(balances, itemCode, balance.WarehouseCode, balance.BatchCode) {
				continue
			}
		}
		candidates = append(candidates, balance)
	}
	if len(candidates) == 0 {
		return inventoryBalance{}
	}
	return candidates[0]
}

func (s *SupplierReturnsCoreService) defaultWarehouseForItem(itemCode string) string {
	if s.inventory == nil || s.inventory.models == nil || itemCode == "" {
		return ""
	}
	items, _, err := s.inventory.models.List("commercial_item", model.Query{
		Filters:  map[string]string{"sku": itemCode},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return ""
	}
	return textValue(items[0].Values["default_replenishment_warehouse_code"])
}

func (s *SupplierReturnsCoreService) updateReturnCreditState(returnID, actorID string, updates map[string]any) error {
	supplierReturn, err := s.documents.Get(returnID)
	if err != nil {
		return err
	}
	payload := clonedPayload(supplierReturn.Body.Payload)
	for key, value := range updates {
		payload[key] = value
	}
	return s.saveMutatedDocument(supplierReturn, actorID, payload)
}

func procurementLineForItem(lines []map[string]any, itemCode string) map[string]any {
	for _, line := range lines {
		if textValue(line["item_code"]) == itemCode {
			return cloneMap(line)
		}
	}
	return map[string]any{}
}

func (s *SupplierReturnsCoreService) refreshDocuments(records ...document.Record) {
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

func (s *SupplierReturnsCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
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
