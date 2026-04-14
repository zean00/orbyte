package application

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type DeliveryCoreService struct {
	documents *document.Service
	search    *search.Service
}

func NewDeliveryCoreService(documents *document.Service, searchSvc *search.Service) *DeliveryCoreService {
	return &DeliveryCoreService{documents: documents, search: searchSvc}
}

func (s *DeliveryCoreService) GenerateDeliveryFromFulfillment(fulfillmentID, actorID string) (document.Record, error) {
	fulfillment, err := s.documents.Get(strings.TrimSpace(fulfillmentID))
	if err != nil {
		return document.Record{}, err
	}
	if fulfillment.Header.Type != "sales_fulfillment" {
		return document.Record{}, shared.Validation("source document must be a sales fulfillment")
	}
	if fulfillment.Header.Status != "issued" {
		return document.Record{}, shared.Conflict("delivery can only be generated from an issued fulfillment")
	}
	payload := clonedPayload(fulfillment.Body.Payload)
	existingDelivered := s.existingDeliveredQuantities(fulfillment.Header.ID)
	lines := make([]map[string]any, 0)
	for _, line := range recordList(payload["lines"]) {
		itemCode := textValue(line["item_code"])
		if itemCode == "" {
			continue
		}
		lineIndex := int(numberValue(line["source_order_line_index"]))
		fulfilledQty := roundMoney(numberValue(line["quantity"]))
		deliverableQty := roundMoney(fulfilledQty - existingDelivered[lineIndex])
		if deliverableQty <= 0 {
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
			"quantity":                      deliverableQty,
			"tracking_number":               "",
			"note":                          "",
		})
	}
	if len(lines) == 0 {
		return document.Record{}, shared.Validation("sales fulfillment has no remaining deliverable lines")
	}
	now := time.Now().UTC()
	deliveryPayload := map[string]any{
		"source_fulfillment_id":     fulfillment.Header.ID,
		"source_fulfillment_number": firstNonEmptyString(fulfillment.Header.Number, fulfillment.Header.ID),
		"source_sales_order_id":     textValue(payload["source_order_id"]),
		"source_sales_order_number": textValue(payload["source_order_number"]),
		"party_id":                  textValue(payload["party_id"]),
		"party_name":                textValue(payload["party_name"]),
		"delivery_date":             now.Format("2006-01-02"),
		"shipment_status":           "planned",
		"carrier_name":              "",
		"tracking_number":           "",
		"dispatch_date":             "",
		"delivered_date":            "",
		"proof_of_delivery":         "",
		"delivered_quantity_total":  0.0,
		"lines":                     lines,
		"notes":                     fmt.Sprintf("Generated from fulfillment %s", firstNonEmptyString(fulfillment.Header.Number, fulfillment.Header.ID)),
	}
	record, err := s.documents.Create("delivery_order", fulfillment.Header.OrganizationID, fulfillment.Header.LocationID, actorID, deliveryPayload)
	if err != nil {
		return document.Record{}, err
	}
	for _, link := range []struct {
		sourceID string
		meta     map[string]any
	}{
		{sourceID: fulfillment.Header.ID, meta: map[string]any{"source_type": "sales_fulfillment"}},
		{sourceID: textValue(payload["source_order_id"]), meta: map[string]any{"source_type": "sales_order"}},
	} {
		if strings.TrimSpace(link.sourceID) == "" {
			continue
		}
		if _, err := s.documents.AddLink(record.Header.ID, link.sourceID, "delivery_for", link.meta); err != nil && !isConflict(err) {
			return document.Record{}, err
		}
		if _, err := s.documents.AddLink(link.sourceID, record.Header.ID, "delivery_for", map[string]any{"generated_document_type": "delivery_order"}); err != nil && !isConflict(err) {
			return document.Record{}, err
		}
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	if err := s.refreshLinkedDocuments(created, actorID); err != nil {
		return document.Record{}, err
	}
	return created, nil
}

func (s *DeliveryCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	if strings.TrimSpace(documentType) != "delivery_order" {
		return document.NormalizePayload(cloneMap(payload))
	}
	next := document.NormalizePayload(cloneMap(payload))
	lines := s.normalizeDeliveryLines(recordList(next["lines"]))
	next["lines"] = lines
	next["delivered_quantity_total"] = roundMoney(sumInventoryLineQuantity(lines, "quantity"))
	next["shipment_status"] = firstNonEmptyString(textValue(next["shipment_status"]), "planned")
	next["carrier_name"] = textValue(next["carrier_name"])
	next["tracking_number"] = textValue(next["tracking_number"])
	next["dispatch_date"] = textValue(next["dispatch_date"])
	next["delivered_date"] = textValue(next["delivered_date"])
	next["proof_of_delivery"] = textValue(next["proof_of_delivery"])
	return next
}

func (s *DeliveryCoreService) ValidateApprove(record document.Record) error {
	if record.Header.Type != "delivery_order" {
		return nil
	}
	if len(recordList(record.Body.Payload["lines"])) == 0 {
		return shared.Validation("delivery lines are required")
	}
	for _, line := range recordList(record.Body.Payload["lines"]) {
		if textValue(line["item_code"]) == "" || numberValue(line["quantity"]) <= 0 {
			return shared.Validation("delivery lines require item and quantity")
		}
	}
	return s.validateDeliveryQuantities(record)
}

func (s *DeliveryCoreService) ValidateCancel(record document.Record) error {
	if record.Header.Type != "delivery_order" {
		return nil
	}
	if record.Header.Status == "delivered" {
		return shared.Validation("delivered delivery orders cannot be cancelled")
	}
	return nil
}

func (s *DeliveryCoreService) ValidateMarkDelivered(record document.Record) error {
	if record.Header.Type != "delivery_order" {
		return nil
	}
	if record.Header.Status != "dispatched" {
		return shared.Validation("only dispatched delivery orders can be marked delivered")
	}
	return s.validateDeliveryQuantities(record)
}

func (s *DeliveryCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	if record.Header.Type != "delivery_order" {
		return nil
	}
	payload := clonedPayload(record.Body.Payload)
	payload["shipment_status"] = "dispatched"
	if textValue(payload["dispatch_date"]) == "" {
		payload["dispatch_date"] = time.Now().UTC().Format("2006-01-02")
	}
	payload["delivered_quantity_total"] = 0.0
	if err := s.saveMutatedDocument(record, actorID, payload); err != nil {
		return err
	}
	updated, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = updated
	}
	return s.refreshLinkedDocuments(record, actorID)
}

func (s *DeliveryCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	if record.Header.Type != "delivery_order" {
		return nil
	}
	return s.refreshLinkedDocuments(record, actorID)
}

func (s *DeliveryCoreService) HandleMarkedDelivered(record document.Record, actorID string) error {
	if record.Header.Type != "delivery_order" {
		return nil
	}
	payload := clonedPayload(record.Body.Payload)
	payload["shipment_status"] = "delivered"
	if textValue(payload["dispatch_date"]) == "" {
		payload["dispatch_date"] = time.Now().UTC().Format("2006-01-02")
	}
	if textValue(payload["delivered_date"]) == "" {
		payload["delivered_date"] = time.Now().UTC().Format("2006-01-02")
	}
	payload["delivered_quantity_total"] = roundMoney(sumInventoryLineQuantity(recordList(payload["lines"]), "quantity"))
	if err := s.saveMutatedDocument(record, actorID, payload); err != nil {
		return err
	}
	updated, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = updated
	}
	return s.refreshLinkedDocuments(record, actorID)
}

func (s *DeliveryCoreService) validateDeliveryQuantities(record document.Record) error {
	payload := clonedPayload(record.Body.Payload)
	fulfillmentID := textValue(payload["source_fulfillment_id"])
	if fulfillmentID == "" {
		return shared.Validation("delivery source fulfillment is required")
	}
	fulfillment, err := s.documents.Get(fulfillmentID)
	if err != nil {
		return err
	}
	if fulfillment.Header.Type != "sales_fulfillment" || fulfillment.Header.Status != "issued" {
		return shared.Conflict("delivery source fulfillment must be issued")
	}
	limits := s.fulfillmentLineQuantities(fulfillment)
	existing := s.existingDeliveredQuantitiesExcept(fulfillmentID, record.Header.ID)
	for _, line := range recordList(payload["lines"]) {
		itemCode := textValue(line["item_code"])
		qty := roundMoney(numberValue(line["quantity"]))
		if itemCode == "" || qty <= 0 {
			return shared.Validation("delivery lines require item and quantity")
		}
		index := int(numberValue(line["source_fulfillment_line_index"]))
		if roundMoney(limits[index]-existing[index]) < qty {
			return shared.Validation(fmt.Sprintf("delivery quantity exceeds fulfilled quantity for item %s", itemCode))
		}
	}
	return nil
}

func (s *DeliveryCoreService) normalizeDeliveryLines(lines []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		next := cloneMap(line)
		next["source_order_line_index"] = int(numberValue(next["source_order_line_index"]))
		next["source_fulfillment_line_index"] = int(numberValue(next["source_fulfillment_line_index"]))
		next["product_code"] = textValue(next["product_code"])
		next["variant_signature"] = textValue(next["variant_signature"])
		next["item_code"] = textValue(next["item_code"])
		next["description"] = textValue(next["description"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["batch_code"] = textValue(next["batch_code"])
		next["expiration_date"] = textValue(next["expiration_date"])
		next["uom_code"] = textValue(next["uom_code"])
		next["fulfilled_quantity"] = roundMoney(numberValue(next["fulfilled_quantity"]))
		next["quantity"] = roundMoney(numberValue(next["quantity"]))
		next["tracking_number"] = textValue(next["tracking_number"])
		next["note"] = textValue(next["note"])
		normalized = append(normalized, next)
	}
	return normalized
}

func (s *DeliveryCoreService) refreshLinkedDocuments(record document.Record, actorID string) error {
	fulfillmentID := textValue(record.Body.Payload["source_fulfillment_id"])
	if fulfillmentID != "" {
		fulfillment, err := s.documents.Get(fulfillmentID)
		if err == nil {
			if err := s.updateFulfillmentDeliveryState(fulfillment, actorID); err != nil {
				return err
			}
		}
	}
	orderID := textValue(record.Body.Payload["source_sales_order_id"])
	if orderID != "" {
		order, err := s.documents.Get(orderID)
		if err == nil {
			if err := s.updateOrderDeliveryState(order, actorID); err != nil {
				return err
			}
		}
	}
	s.refreshDocuments(record)
	return nil
}

func (s *DeliveryCoreService) updateFulfillmentDeliveryState(fulfillment document.Record, actorID string) error {
	payload := clonedPayload(fulfillment.Body.Payload)
	delivered := s.existingDeliveredQuantities(fulfillment.Header.ID)
	totalDelivered := 0.0
	sourceLines := recordList(payload["lines"])
	allDelivered := len(sourceLines) > 0
	hasAnyDelivery := false
	for _, line := range sourceLines {
		index := int(numberValue(line["source_order_line_index"]))
		fulfilledQty := roundMoney(numberValue(line["quantity"]))
		deliveredQty := roundMoney(delivered[index])
		totalDelivered = roundMoney(totalDelivered + deliveredQty)
		if deliveredQty > 0 {
			hasAnyDelivery = true
		}
		if deliveredQty < fulfilledQty {
			allDelivered = false
		}
	}
	switch {
	case allDelivered && len(sourceLines) > 0:
		payload["delivery_status"] = "delivered"
	case hasAnyDelivery:
		payload["delivery_status"] = "partially_delivered"
	case s.hasActiveDelivery(fulfillment.Header.ID):
		payload["delivery_status"] = "planned"
	default:
		payload["delivery_status"] = ""
	}
	payload["delivered_quantity_total"] = roundMoney(totalDelivered)
	return s.saveMutatedDocument(fulfillment, actorID, payload)
}

func (s *DeliveryCoreService) updateOrderDeliveryState(order document.Record, actorID string) error {
	payload := clonedPayload(order.Body.Payload)
	totalDelivered := 0.0
	totalOrdered := 0.0
	hasAnyDelivery := false
	for _, fulfillment := range s.fulfillmentsForOrder(order.Header.ID) {
		totalDelivered = roundMoney(totalDelivered + numberValue(fulfillment.Body.Payload["delivered_quantity_total"]))
		if textValue(fulfillment.Body.Payload["delivery_status"]) != "" {
			hasAnyDelivery = true
		}
		for _, line := range recordList(fulfillment.Body.Payload["lines"]) {
			totalOrdered = roundMoney(totalOrdered + numberValue(line["quantity"]))
		}
	}
	switch {
	case totalOrdered > 0 && totalDelivered >= totalOrdered:
		payload["delivery_status"] = "delivered"
	case totalDelivered > 0:
		payload["delivery_status"] = "partially_delivered"
	case hasAnyDelivery:
		payload["delivery_status"] = "planned"
	default:
		payload["delivery_status"] = ""
	}
	payload["delivered_quantity_total"] = roundMoney(totalDelivered)
	return s.saveMutatedDocument(order, actorID, payload)
}

func (s *DeliveryCoreService) fulfillmentsForOrder(orderID string) []document.Record {
	items := make([]document.Record, 0)
	for _, record := range s.documents.List() {
		if record.Header.Type != "sales_fulfillment" {
			continue
		}
		if record.Header.Status == "cancelled" || record.Header.Status == "rejected" {
			continue
		}
		if textValue(record.Body.Payload["source_order_id"]) == orderID {
			items = append(items, record)
		}
	}
	return items
}

func (s *DeliveryCoreService) hasActiveDelivery(fulfillmentID string) bool {
	for _, record := range s.documents.List() {
		if record.Header.Type != "delivery_order" {
			continue
		}
		if record.Header.Status == "cancelled" || record.Header.Status == "rejected" {
			continue
		}
		if textValue(record.Body.Payload["source_fulfillment_id"]) == fulfillmentID {
			return true
		}
	}
	return false
}

func (s *DeliveryCoreService) existingDeliveredQuantities(fulfillmentID string) map[int]float64 {
	return s.existingDeliveredQuantitiesExcept(fulfillmentID, "")
}

func (s *DeliveryCoreService) existingDeliveredQuantitiesExcept(fulfillmentID, excludeDeliveryID string) map[int]float64 {
	quantities := map[int]float64{}
	for _, record := range s.documents.List() {
		if record.Header.Type != "delivery_order" || record.Header.ID == excludeDeliveryID {
			continue
		}
		if record.Header.Status == "cancelled" || record.Header.Status == "rejected" {
			continue
		}
		if textValue(record.Body.Payload["source_fulfillment_id"]) != fulfillmentID {
			continue
		}
		if record.Header.Status != "delivered" {
			continue
		}
		for _, line := range recordList(record.Body.Payload["lines"]) {
			index := int(numberValue(line["source_fulfillment_line_index"]))
			quantities[index] = roundMoney(quantities[index] + numberValue(line["quantity"]))
		}
	}
	return quantities
}

func (s *DeliveryCoreService) fulfillmentLineQuantities(fulfillment document.Record) map[int]float64 {
	quantities := map[int]float64{}
	for _, line := range recordList(fulfillment.Body.Payload["lines"]) {
		index := int(numberValue(line["source_order_line_index"]))
		quantities[index] = roundMoney(quantities[index] + numberValue(line["quantity"]))
	}
	return quantities
}

func (s *DeliveryCoreService) refreshDocuments(records ...document.Record) {
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

func (s *DeliveryCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
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
