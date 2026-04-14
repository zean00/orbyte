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

type ReturnsCoreService struct {
	documents   *document.Service
	search      *search.Service
	inventory   *InventoryCoreService
	commercial  *CommercialCoreService
	fulfillment *FulfillmentCoreService
}

func NewReturnsCoreService(documents *document.Service, searchSvc *search.Service, inventorySvc *InventoryCoreService, commercialSvc *CommercialCoreService, fulfillmentSvc *FulfillmentCoreService) *ReturnsCoreService {
	return &ReturnsCoreService{
		documents:   documents,
		search:      searchSvc,
		inventory:   inventorySvc,
		commercial:  commercialSvc,
		fulfillment: fulfillmentSvc,
	}
}

func (s *ReturnsCoreService) GenerateReturnFromFulfillment(fulfillmentID, actorID string) (document.Record, error) {
	fulfillment, err := s.documents.Get(strings.TrimSpace(fulfillmentID))
	if err != nil {
		return document.Record{}, err
	}
	if fulfillment.Header.Type != "sales_fulfillment" {
		return document.Record{}, shared.Validation("source document must be a sales fulfillment")
	}
	if fulfillment.Header.Status != "issued" {
		return document.Record{}, shared.Conflict("returns can only be registered from issued fulfillments")
	}
	payload := clonedPayload(fulfillment.Body.Payload)
	sourceOrderID := textValue(payload["source_order_id"])
	sourceOrderNumber := textValue(payload["source_order_number"])
	sourceInvoiceID, sourceInvoiceNumber := s.findInvoiceForOrder(sourceOrderID)
	existingReturned := s.existingReturnQuantities(fulfillment.Header.ID)
	lines := make([]map[string]any, 0)
	for _, line := range recordList(payload["lines"]) {
		lineIndex := int(numberValue(line["source_order_line_index"]))
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			continue
		}
		fulfilledQty := roundMoney(numberValue(line["quantity"]))
		returnableQty := roundMoney(fulfilledQty - existingReturned[lineIndex])
		if returnableQty <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"source_order_line_index":       lineIndex,
			"source_fulfillment_line_index": lineIndex,
			"product_code":                  textValue(line["product_code"]),
			"variant_signature":             textValue(line["variant_signature"]),
			"item_code":                     itemCode,
			"description":                   textValue(line["description"]),
			"warehouse_code":                textValue(line["warehouse_code"]),
			"batch_code":                    textValue(line["batch_code"]),
			"expiration_date":               textValue(line["expiration_date"]),
			"uom_code":                      textValue(line["uom_code"]),
			"fulfilled_quantity":            fulfilledQty,
			"quantity":                      returnableQty,
			"disposition":                   "restock",
			"note":                          "",
		})
	}
	if len(lines) == 0 {
		return document.Record{}, shared.Validation("fulfillment has no remaining returnable lines")
	}
	now := time.Now().UTC()
	returnPayload := map[string]any{
		"source_sales_order_id":     sourceOrderID,
		"source_sales_order_number": sourceOrderNumber,
		"source_invoice_id":         sourceInvoiceID,
		"source_invoice_number":     sourceInvoiceNumber,
		"source_fulfillment_id":     fulfillment.Header.ID,
		"source_fulfillment_number": firstNonEmptyString(fulfillment.Header.Number, fulfillment.Header.ID),
		"party_id":                  textValue(payload["party_id"]),
		"party_name":                textValue(payload["party_name"]),
		"return_date":               now.Format("2006-01-02"),
		"return_status":             "approved",
		"resolution_type":           "refund",
		"credit_note_status":        "",
		"refund_status":             "",
		"replacement_order_status":  "",
		"credited_amount":           0.0,
		"refunded_amount":           0.0,
		"total_quantity":            roundMoney(sumInventoryLineQuantity(lines, "quantity")),
		"reason":                    fmt.Sprintf("Return for fulfillment %s", firstNonEmptyString(fulfillment.Header.Number, fulfillment.Header.ID)),
		"lines":                     lines,
	}
	record, err := s.documents.Create("sales_return", fulfillment.Header.OrganizationID, fulfillment.Header.LocationID, actorID, returnPayload)
	if err != nil {
		return document.Record{}, err
	}
	for _, link := range []struct {
		sourceID string
		meta     map[string]any
	}{
		{sourceID: fulfillment.Header.ID, meta: map[string]any{"source_type": "sales_fulfillment"}},
		{sourceID: sourceOrderID, meta: map[string]any{"source_type": "sales_order"}},
		{sourceID: sourceInvoiceID, meta: map[string]any{"source_type": "invoice"}},
	} {
		if strings.TrimSpace(link.sourceID) == "" {
			continue
		}
		if _, err := s.documents.AddLink(record.Header.ID, link.sourceID, "return_for", link.meta); err != nil && !isConflict(err) {
			return document.Record{}, err
		}
		if _, err := s.documents.AddLink(link.sourceID, record.Header.ID, "return_for", map[string]any{"generated_document_type": "sales_return"}); err != nil && !isConflict(err) {
			return document.Record{}, err
		}
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, fulfillment)
	return created, nil
}

func (s *ReturnsCoreService) CreateReturnReceiptFromReturn(returnID, actorID string) (document.Record, error) {
	salesReturn, err := s.documents.Get(strings.TrimSpace(returnID))
	if err != nil {
		return document.Record{}, err
	}
	if salesReturn.Header.Type != "sales_return" {
		return document.Record{}, shared.Validation("source document must be a sales return")
	}
	if salesReturn.Header.Status != "approved" && salesReturn.Header.Status != "received" {
		return document.Record{}, shared.Conflict("return receipt can only be registered from an approved return")
	}
	payload := clonedPayload(salesReturn.Body.Payload)
	existingReceived := s.existingReturnReceiptQuantities(salesReturn.Header.ID)
	lines := make([]map[string]any, 0)
	for _, line := range recordList(payload["lines"]) {
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			continue
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			continue
		}
		lineIndex := int(numberValue(line["source_fulfillment_line_index"]))
		returnQty := roundMoney(numberValue(line["quantity"]))
		receivableQty := roundMoney(returnQty - existingReceived[lineIndex])
		if receivableQty <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"source_return_line_index":      lineIndex,
			"source_fulfillment_line_index": lineIndex,
			"product_code":                  textValue(line["product_code"]),
			"variant_signature":             textValue(line["variant_signature"]),
			"item_code":                     itemCode,
			"description":                   textValue(line["description"]),
			"warehouse_code":                textValue(line["warehouse_code"]),
			"batch_code":                    textValue(line["batch_code"]),
			"expiration_date":               textValue(line["expiration_date"]),
			"uom_code":                      textValue(line["uom_code"]),
			"quantity":                      receivableQty,
			"disposition":                   firstNonEmptyString(textValue(line["disposition"]), "restock"),
			"note":                          textValue(line["note"]),
		})
	}
	if len(lines) == 0 {
		return document.Record{}, shared.Validation("sales return has no receivable inventory lines")
	}
	now := time.Now().UTC()
	receiptPayload := map[string]any{
		"source_return_id":          salesReturn.Header.ID,
		"source_return_number":      firstNonEmptyString(salesReturn.Header.Number, salesReturn.Header.ID),
		"source_fulfillment_id":     textValue(payload["source_fulfillment_id"]),
		"source_fulfillment_number": textValue(payload["source_fulfillment_number"]),
		"party_id":                  textValue(payload["party_id"]),
		"party_name":                textValue(payload["party_name"]),
		"receipt_date":              now.Format("2006-01-02"),
		"total_quantity":            roundMoney(sumInventoryLineQuantity(lines, "quantity")),
		"lines":                     lines,
		"notes":                     fmt.Sprintf("Generated from return %s", firstNonEmptyString(salesReturn.Header.Number, salesReturn.Header.ID)),
	}
	record, err := s.documents.Create("return_receipt", salesReturn.Header.OrganizationID, salesReturn.Header.LocationID, actorID, receiptPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, salesReturn.Header.ID, "return_for", map[string]any{"source_type": "sales_return"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(salesReturn.Header.ID, record.Header.ID, "return_for", map[string]any{"generated_document_type": "return_receipt"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, salesReturn)
	return created, nil
}

func (s *ReturnsCoreService) CreateCreditNoteFromReturn(returnID, actorID string) (document.Record, error) {
	if s.commercial == nil {
		return document.Record{}, shared.Validation("commercial service is unavailable")
	}
	salesReturn, err := s.documents.Get(strings.TrimSpace(returnID))
	if err != nil {
		return document.Record{}, err
	}
	if salesReturn.Header.Type != "sales_return" {
		return document.Record{}, shared.Validation("source document must be a sales return")
	}
	invoiceID := textValue(salesReturn.Body.Payload["source_invoice_id"])
	if invoiceID == "" {
		return document.Record{}, shared.Validation("sales return source invoice is required")
	}
	record, err := s.commercial.CreateCreditNoteFromInvoice(invoiceID, actorID)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, salesReturn.Header.ID, "return_for", map[string]any{"source_type": "sales_return"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(salesReturn.Header.ID, record.Header.ID, "return_for", map[string]any{"generated_document_type": "credit_note"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if err := s.updateReturnCommercialState(salesReturn.Header.ID, actorID, map[string]any{
		"source_credit_note_id":     record.Header.ID,
		"source_credit_note_number": firstNonEmptyString(record.Header.Number, record.Header.ID),
		"credit_note_status":        record.Header.Status,
	}); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (s *ReturnsCoreService) CreateRefundFromReturn(returnID, actorID string) (document.Record, error) {
	if s.commercial == nil {
		return document.Record{}, shared.Validation("commercial service is unavailable")
	}
	salesReturn, err := s.documents.Get(strings.TrimSpace(returnID))
	if err != nil {
		return document.Record{}, err
	}
	if salesReturn.Header.Type != "sales_return" {
		return document.Record{}, shared.Validation("source document must be a sales return")
	}
	payload := clonedPayload(salesReturn.Body.Payload)
	creditNoteID := textValue(payload["source_credit_note_id"])
	if creditNoteID == "" {
		creditNoteID = s.findLinkedDocumentID(salesReturn, "return_for", "credit_note")
	}
	if creditNoteID == "" {
		return document.Record{}, shared.Validation("sales return requires a credit note before refund")
	}
	record, err := s.commercial.CreateRefundFromCreditNote(creditNoteID, actorID)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, salesReturn.Header.ID, "return_for", map[string]any{"source_type": "sales_return"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(salesReturn.Header.ID, record.Header.ID, "return_for", map[string]any{"generated_document_type": "payment_refund"}); err != nil && !isConflict(err) {
		return document.Record{}, err
	}
	if err := s.updateReturnCommercialState(salesReturn.Header.ID, actorID, map[string]any{
		"source_refund_id":     record.Header.ID,
		"source_refund_number": firstNonEmptyString(record.Header.Number, record.Header.ID),
		"refund_status":        record.Header.Status,
	}); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (s *ReturnsCoreService) CreateReplacementOrderFromReturn(returnID, actorID string) (document.Record, error) {
	if s.commercial == nil {
		return document.Record{}, shared.Validation("commercial service is unavailable")
	}
	salesReturn, err := s.documents.Get(strings.TrimSpace(returnID))
	if err != nil {
		return document.Record{}, err
	}
	if salesReturn.Header.Type != "sales_return" {
		return document.Record{}, shared.Validation("source document must be a sales return")
	}
	if salesReturn.Header.Status != "approved" && salesReturn.Header.Status != "received" {
		return document.Record{}, shared.Conflict("replacement order can only be created from an approved or received return")
	}
	if existingID := textValue(salesReturn.Body.Payload["source_replacement_order_id"]); existingID != "" {
		return document.Record{}, shared.Conflict("replacement order already exists for this return")
	}
	returnPayload := clonedPayload(salesReturn.Body.Payload)
	sourceOrderID := textValue(returnPayload["source_sales_order_id"])
	sourceOrder := document.Record{}
	if sourceOrderID != "" {
		sourceOrder, _ = s.documents.Get(sourceOrderID)
	}
	sourceOrderPayload := clonedPayload(sourceOrder.Body.Payload)
	lines := make([]map[string]any, 0)
	for _, line := range recordList(returnPayload["lines"]) {
		itemCode := textValue(line["item_code"])
		qty := roundMoney(numberValue(line["quantity"]))
		if itemCode == "" || qty <= 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"product_code":       textValue(line["product_code"]),
			"variant_signature":  textValue(line["variant_signature"]),
			"item_code":          itemCode,
			"description":        textValue(line["description"]),
			"uom_code":           textValue(line["uom_code"]),
			"quantity":           qty,
			"unit_price":         numberValue(line["unit_price"]),
			"discount_amount":    numberValue(line["discount_amount"]),
			"tax_code":           textValue(line["tax_code"]),
			"tax_rate":           numberValue(line["tax_rate"]),
			"line_subtotal":      numberValue(line["line_subtotal"]),
			"tax_amount":         numberValue(line["tax_amount"]),
			"line_total":         numberValue(line["line_total"]),
			"exchange_source_id": salesReturn.Header.ID,
		})
	}
	if len(lines) == 0 {
		return document.Record{}, shared.Validation("sales return has no eligible replacement lines")
	}
	orderPayload := map[string]any{
		"party_id":                           firstNonEmptyString(textValue(returnPayload["party_id"]), textValue(sourceOrderPayload["party_id"])),
		"party_name":                         firstNonEmptyString(textValue(returnPayload["party_name"]), textValue(sourceOrderPayload["party_name"])),
		"order_date":                         time.Now().UTC().Format("2006-01-02"),
		"currency_code":                      firstNonEmptyString(textValue(sourceOrderPayload["currency_code"]), "IDR"),
		"price_list_code":                    textValue(sourceOrderPayload["price_list_code"]),
		"tax_profile_code":                   textValue(sourceOrderPayload["tax_profile_code"]),
		"default_tax_code":                   textValue(sourceOrderPayload["default_tax_code"]),
		"payment_term_days":                  int(numberValue(sourceOrderPayload["payment_term_days"])),
		"resolution_type":                    "exchange",
		"exchange_source_return_id":          salesReturn.Header.ID,
		"exchange_source_return_number":      firstNonEmptyString(salesReturn.Header.Number, salesReturn.Header.ID),
		"exchange_source_order_id":           sourceOrderID,
		"exchange_source_order_number":       textValue(returnPayload["source_sales_order_number"]),
		"exchange_source_fulfillment_id":     textValue(returnPayload["source_fulfillment_id"]),
		"exchange_source_fulfillment_number": textValue(returnPayload["source_fulfillment_number"]),
		"reference":                          fmt.Sprintf("Exchange for return %s", firstNonEmptyString(salesReturn.Header.Number, salesReturn.Header.ID)),
		"notes":                              fmt.Sprintf("Replacement order for return %s", firstNonEmptyString(salesReturn.Header.Number, salesReturn.Header.ID)),
		"lines":                              lines,
	}
	orderPayload = s.commercial.NormalizePayload("sales_order", orderPayload)
	record, err := s.documents.Create("sales_order", salesReturn.Header.OrganizationID, salesReturn.Header.LocationID, actorID, orderPayload)
	if err != nil {
		return document.Record{}, err
	}
	for _, link := range []struct {
		sourceID string
		meta     map[string]any
	}{
		{sourceID: salesReturn.Header.ID, meta: map[string]any{"source_type": "sales_return"}},
		{sourceID: textValue(returnPayload["source_sales_order_id"]), meta: map[string]any{"source_type": "sales_order"}},
		{sourceID: textValue(returnPayload["source_fulfillment_id"]), meta: map[string]any{"source_type": "sales_fulfillment"}},
	} {
		if strings.TrimSpace(link.sourceID) == "" {
			continue
		}
		if _, err := s.documents.AddLink(record.Header.ID, link.sourceID, "exchange_for", link.meta); err != nil && !isConflict(err) {
			return document.Record{}, err
		}
		if _, err := s.documents.AddLink(link.sourceID, record.Header.ID, "exchange_for", map[string]any{"generated_document_type": "sales_order"}); err != nil && !isConflict(err) {
			return document.Record{}, err
		}
	}
	if err := s.updateReturnCommercialState(salesReturn.Header.ID, actorID, map[string]any{
		"resolution_type":                 "exchange",
		"replacement_order_status":        "draft",
		"source_replacement_order_id":     record.Header.ID,
		"source_replacement_order_number": firstNonEmptyString(record.Header.Number, record.Header.ID),
	}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	s.refreshDocuments(created, salesReturn)
	return created, nil
}

func (s *ReturnsCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	switch strings.TrimSpace(documentType) {
	case "sales_return", "return_receipt":
		lines := s.normalizeReturnLines(recordList(next["lines"]))
		next["lines"] = lines
		next["total_quantity"] = roundMoney(sumInventoryLineQuantity(lines, "quantity"))
		if strings.TrimSpace(documentType) == "sales_return" {
			next["resolution_type"] = firstNonEmptyString(textValue(next["resolution_type"]), "refund")
			next["replacement_order_status"] = textValue(next["replacement_order_status"])
			next["source_replacement_order_id"] = textValue(next["source_replacement_order_id"])
			next["source_replacement_order_number"] = textValue(next["source_replacement_order_number"])
		}
	}
	return next
}

func (s *ReturnsCoreService) ValidateApprove(record document.Record) error {
	switch record.Header.Type {
	case "sales_return":
		return s.validateSalesReturn(record)
	case "return_receipt":
		return s.validateReturnReceipt(record)
	default:
		return nil
	}
}

func (s *ReturnsCoreService) ValidateCancel(record document.Record) error {
	switch record.Header.Type {
	case "return_receipt":
		if record.Header.Status == "received" {
			return nil
		}
	}
	return nil
}

func (s *ReturnsCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "sales_return":
		return s.handleApprovedSalesReturn(record, actorID)
	case "return_receipt":
		return s.handleApprovedReturnReceipt(record, actorID)
	default:
		return nil
	}
}

func (s *ReturnsCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "return_receipt":
		return s.handleCanceledReturnReceipt(record, actorID)
	default:
		return nil
	}
}

func (s *ReturnsCoreService) validateSalesReturn(record document.Record) error {
	payload := clonedPayload(record.Body.Payload)
	fulfillmentID := textValue(payload["source_fulfillment_id"])
	if fulfillmentID == "" {
		return shared.Validation("sales return source fulfillment is required")
	}
	fulfillment, err := s.documents.Get(fulfillmentID)
	if err != nil {
		return err
	}
	if fulfillment.Header.Type != "sales_fulfillment" || fulfillment.Header.Status != "issued" {
		return shared.Conflict("sales return source fulfillment must be issued")
	}
	returnable := s.fulfillmentLineQuantities(fulfillment)
	existing := s.existingReturnQuantitiesExcept(fulfillment.Header.ID, record.Header.ID)
	for _, line := range recordList(payload["lines"]) {
		itemCode := textValue(line["item_code"])
		qty := roundMoney(numberValue(line["quantity"]))
		if itemCode == "" || qty <= 0 {
			return shared.Validation("return lines require item and quantity")
		}
		index := int(numberValue(line["source_fulfillment_line_index"]))
		remaining := roundMoney(returnable[index] - existing[index])
		if remaining < qty {
			return shared.Validation(fmt.Sprintf("return quantity exceeds fulfilled quantity for item %s", itemCode))
		}
	}
	return nil
}

func (s *ReturnsCoreService) validateReturnReceipt(record document.Record) error {
	payload := clonedPayload(record.Body.Payload)
	returnID := textValue(payload["source_return_id"])
	if returnID == "" {
		return shared.Validation("return receipt source return is required")
	}
	salesReturn, err := s.documents.Get(returnID)
	if err != nil {
		return err
	}
	returnLines := recordList(salesReturn.Body.Payload["lines"])
	received := s.existingReturnReceiptQuantitiesExcept(returnID, record.Header.ID)
	limits := map[int]float64{}
	for _, line := range returnLines {
		index := int(numberValue(line["source_fulfillment_line_index"]))
		limits[index] = roundMoney(numberValue(line["quantity"]))
	}
	for _, line := range recordList(payload["lines"]) {
		itemCode := textValue(line["item_code"])
		qty := roundMoney(numberValue(line["quantity"]))
		warehouseCode := textValue(line["warehouse_code"])
		disposition := firstNonEmptyString(textValue(line["disposition"]), "restock")
		if itemCode == "" || qty <= 0 || warehouseCode == "" {
			return shared.Validation("return receipt lines require item, warehouse, and quantity")
		}
		if err := validateCommercialItemCode(s.inventory.models, itemCode); err != nil {
			return err
		}
		if err := validateWarehouseCode(s.inventory.models, warehouseCode); err != nil {
			return err
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			return shared.Validation(fmt.Sprintf("item %s is not inventory-tracked", itemCode))
		}
		if policy.TrackingMode == "batch" && textValue(line["batch_code"]) == "" {
			return shared.Validation(fmt.Sprintf("batch code is required for item %s", itemCode))
		}
		if policy.TrackingMode == "batch" && policy.ExpiryTracking && textValue(line["expiration_date"]) == "" {
			return shared.Validation(fmt.Sprintf("expiration date is required for item %s", itemCode))
		}
		if disposition != "restock" && policy.TrackingMode != "batch" {
			return shared.Validation(fmt.Sprintf("controlled return disposition requires batch-tracked item %s", itemCode))
		}
		index := int(numberValue(line["source_fulfillment_line_index"]))
		if roundMoney(limits[index]-received[index]) < qty {
			return shared.Validation(fmt.Sprintf("return receipt exceeds approved return quantity for item %s", itemCode))
		}
	}
	return nil
}

func (s *ReturnsCoreService) handleApprovedSalesReturn(record document.Record, actorID string) error {
	payload := clonedPayload(record.Body.Payload)
	payload["return_status"] = "approved"
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *ReturnsCoreService) handleApprovedReturnReceipt(record document.Record, actorID string) error {
	if s.inventory == nil {
		return shared.Validation("inventory service is unavailable")
	}
	if s.inventory.hasMovementLink(record, "return_receipt") {
		return nil
	}
	for _, line := range recordList(record.Body.Payload["lines"]) {
		movementLine := cloneMap(line)
		movementLine["note"] = firstNonEmptyString(textValue(line["note"]), "Returned from customer")
		if err := s.inventory.createMovement(record, actorID, "return_receipt", movementLine, "in"); err != nil {
			return err
		}
		switch firstNonEmptyString(textValue(line["disposition"]), "restock") {
		case "quarantine":
			if err := s.applyReturnedBatchState(movementLine, actorID, "quarantine"); err != nil {
				return err
			}
		case "block":
			if err := s.applyReturnedBatchState(movementLine, actorID, "block"); err != nil {
				return err
			}
		}
	}
	if err := s.updateLinkedReturnFromReceipt(record, actorID, "received"); err != nil {
		return err
	}
	s.refreshDocuments(record)
	return nil
}

func (s *ReturnsCoreService) handleCanceledReturnReceipt(record document.Record, actorID string) error {
	if s.inventory == nil {
		return nil
	}
	if err := s.inventory.reverseMovements(record, actorID, "return_receipt", "return_receipt_reversal"); err != nil {
		return err
	}
	return s.updateLinkedReturnFromReceipt(record, actorID, "approved")
}

func (s *ReturnsCoreService) applyReturnedBatchState(line map[string]any, actorID, action string) error {
	if s.inventory == nil || s.inventory.models == nil {
		return nil
	}
	itemCode := textValue(line["item_code"])
	warehouseCode := textValue(line["warehouse_code"])
	batchCode := textValue(line["batch_code"])
	if itemCode == "" || warehouseCode == "" || batchCode == "" {
		return nil
	}
	items, _, err := s.inventory.models.List("inventory_batch", model.Query{
		Filters: map[string]string{
			"item_code":      itemCode,
			"warehouse_code": warehouseCode,
			"batch_code":     batchCode,
		},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(items) == 0 {
		return err
	}
	_, err = s.inventory.ApplyBatchAction(items[0].ID, action, actorID, action, firstNonEmptyString(textValue(line["note"]), action), "", time.Now().UTC())
	return err
}

func (s *ReturnsCoreService) updateLinkedReturnFromReceipt(receipt document.Record, actorID, status string) error {
	returnID := textValue(receipt.Body.Payload["source_return_id"])
	if returnID == "" {
		return nil
	}
	salesReturn, err := s.documents.Get(returnID)
	if err != nil {
		return err
	}
	payload := clonedPayload(salesReturn.Body.Payload)
	payload["return_status"] = status
	payload["received_quantity_total"] = roundMoney(sumInventoryLineQuantity(recordList(receipt.Body.Payload["lines"]), "quantity"))
	if status == "received" {
		salesReturn.Header.Status = "received"
	}
	return s.saveMutatedDocument(salesReturn, actorID, payload)
}

func (s *ReturnsCoreService) updateReturnCommercialState(returnID, actorID string, updates map[string]any) error {
	salesReturn, err := s.documents.Get(returnID)
	if err != nil {
		return err
	}
	payload := clonedPayload(salesReturn.Body.Payload)
	for key, value := range updates {
		payload[key] = value
	}
	return s.saveMutatedDocument(salesReturn, actorID, payload)
}

func (s *ReturnsCoreService) normalizeReturnLines(lines []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		next := cloneMap(line)
		next["product_code"] = textValue(next["product_code"])
		next["variant_signature"] = textValue(next["variant_signature"])
		next["item_code"] = textValue(next["item_code"])
		next["description"] = textValue(next["description"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["batch_code"] = textValue(next["batch_code"])
		next["expiration_date"] = textValue(next["expiration_date"])
		next["uom_code"] = textValue(next["uom_code"])
		next["quantity"] = roundMoney(numberValue(next["quantity"]))
		next["fulfilled_quantity"] = roundMoney(numberValue(next["fulfilled_quantity"]))
		next["disposition"] = firstNonEmptyString(textValue(next["disposition"]), "restock")
		next["note"] = textValue(next["note"])
		normalized = append(normalized, next)
	}
	return normalized
}

func (s *ReturnsCoreService) fulfillmentLineQuantities(fulfillment document.Record) map[int]float64 {
	quantities := map[int]float64{}
	for _, line := range recordList(fulfillment.Body.Payload["lines"]) {
		index := int(numberValue(line["source_fulfillment_line_index"]))
		if index == 0 && numberValue(line["source_order_line_index"]) > 0 {
			index = int(numberValue(line["source_order_line_index"]))
		}
		quantities[index] = roundMoney(quantities[index] + numberValue(line["quantity"]))
	}
	return quantities
}

func (s *ReturnsCoreService) existingReturnQuantities(fulfillmentID string) map[int]float64 {
	return s.existingReturnQuantitiesExcept(fulfillmentID, "")
}

func (s *ReturnsCoreService) existingReturnQuantitiesExcept(fulfillmentID, excludeReturnID string) map[int]float64 {
	quantities := map[int]float64{}
	for _, record := range s.documents.List() {
		if record.Header.Type != "sales_return" || record.Header.ID == excludeReturnID {
			continue
		}
		if record.Header.Status == "cancelled" || record.Header.Status == "rejected" {
			continue
		}
		if textValue(record.Body.Payload["source_fulfillment_id"]) != fulfillmentID {
			continue
		}
		for _, line := range recordList(record.Body.Payload["lines"]) {
			index := int(numberValue(line["source_fulfillment_line_index"]))
			quantities[index] = roundMoney(quantities[index] + numberValue(line["quantity"]))
		}
	}
	return quantities
}

func (s *ReturnsCoreService) existingReturnReceiptQuantities(returnID string) map[int]float64 {
	return s.existingReturnReceiptQuantitiesExcept(returnID, "")
}

func (s *ReturnsCoreService) existingReturnReceiptQuantitiesExcept(returnID, excludeReceiptID string) map[int]float64 {
	quantities := map[int]float64{}
	for _, record := range s.documents.List() {
		if record.Header.Type != "return_receipt" || record.Header.ID == excludeReceiptID {
			continue
		}
		if record.Header.Status == "cancelled" || record.Header.Status == "rejected" {
			continue
		}
		if textValue(record.Body.Payload["source_return_id"]) != returnID {
			continue
		}
		for _, line := range recordList(record.Body.Payload["lines"]) {
			index := int(numberValue(line["source_fulfillment_line_index"]))
			quantities[index] = roundMoney(quantities[index] + numberValue(line["quantity"]))
		}
	}
	return quantities
}

func (s *ReturnsCoreService) findInvoiceForOrder(orderID string) (string, string) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return "", ""
	}
	for _, record := range s.documents.List() {
		if record.Header.Type != "invoice" {
			continue
		}
		if textValue(record.Body.Payload["source_order_id"]) == orderID {
			return record.Header.ID, firstNonEmptyString(record.Header.Number, record.Header.ID)
		}
		for _, link := range record.Links {
			if link.LinkType == "invoice_for" && link.LinkedDocumentID == orderID {
				return record.Header.ID, firstNonEmptyString(record.Header.Number, record.Header.ID)
			}
		}
	}
	return "", ""
}

func (s *ReturnsCoreService) findLinkedDocumentID(record document.Record, linkType, documentType string) string {
	for _, link := range record.Links {
		if link.LinkType != linkType {
			continue
		}
		target, err := s.documents.Get(link.LinkedDocumentID)
		if err == nil && target.Header.Type == documentType {
			return target.Header.ID
		}
	}
	return ""
}

func (s *ReturnsCoreService) refreshDocuments(records ...document.Record) {
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

func (s *ReturnsCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
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
