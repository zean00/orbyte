package application

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

type FulfillmentCoreService struct {
	documents *document.Service
	search    *search.Service
	inventory *InventoryCoreService
}

func NewFulfillmentCoreService(documents *document.Service, searchSvc *search.Service, inventorySvc *InventoryCoreService) *FulfillmentCoreService {
	return &FulfillmentCoreService{documents: documents, search: searchSvc, inventory: inventorySvc}
}

func (s *FulfillmentCoreService) GenerateFulfillmentFromOrder(orderID, actorID string) (document.Record, error) {
	order, err := s.documents.Get(strings.TrimSpace(orderID))
	if err != nil {
		return document.Record{}, err
	}
	if order.Header.Type != "sales_order" {
		return document.Record{}, shared.Validation("source document must be a sales order")
	}
	if order.Header.Status != "confirmed" {
		return document.Record{}, shared.Conflict("fulfillment can only be generated from a confirmed order")
	}
	payload := clonedPayload(order.Body.Payload)
	sourceLines := recordList(payload["lines"])
	existingQuantities := s.existingFulfillmentQuantities(order.Header.ID)
	fulfillmentLines := make([]map[string]any, 0)
	for index, line := range sourceLines {
		itemCode := strings.TrimSpace(textValue(line["item_code"]))
		if itemCode == "" {
			continue
		}
		policy := s.inventory.lookupItemPolicy(itemCode)
		if !policy.Enabled {
			continue
		}
		quantity := roundMoney(numberValue(line["quantity"]) - existingQuantities[index])
		if quantity <= 0 {
			continue
		}
		proposed, allocErr := s.inventory.ProposeFulfillmentLines(order.Header.OrganizationID, order.Header.LocationID, []map[string]any{{
			"source_order_line_index": index,
			"product_code":            textValue(line["product_code"]),
			"variant_signature":       textValue(line["variant_signature"]),
			"item_code":               itemCode,
			"description":             firstNonEmptyString(textValue(line["description"]), policy.Name),
			"uom_code":                firstNonEmptyString(textValue(line["uom_code"]), policy.UOMCode),
			"ordered_quantity":        quantity,
			"quantity":                quantity,
			"note":                    textValue(line["note"]),
		}}, "")
		if allocErr != nil {
			return document.Record{}, allocErr
		}
		fulfillmentLines = append(fulfillmentLines, proposed...)
	}
	if len(fulfillmentLines) == 0 {
		return document.Record{}, shared.Validation("sales order has no inventory-tracked lines to fulfill")
	}
	now := time.Now().UTC()
	fulfillmentPayload := map[string]any{
		"source_order_id":          order.Header.ID,
		"source_order_number":      firstNonEmptyString(order.Header.Number, order.Header.ID),
		"party_id":                 textValue(payload["party_id"]),
		"party_name":               textValue(payload["party_name"]),
		"fulfillment_date":         now.Format("2006-01-02"),
		"fulfillment_status":       "reserved",
		"reserved_quantity_total":  roundMoney(sumInventoryLineQuantity(fulfillmentLines, "quantity")),
		"fulfilled_quantity_total": 0.0,
		"lines":                    fulfillmentLines,
		"notes":                    firstNonEmptyString(textValue(payload["notes"]), fmt.Sprintf("Generated from order %s", firstNonEmptyString(order.Header.Number, order.Header.ID))),
	}
	record, err := s.documents.Create("sales_fulfillment", order.Header.OrganizationID, order.Header.LocationID, actorID, fulfillmentPayload)
	if err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, order.Header.ID, "fulfillment_for", map[string]any{"source_type": "sales_order"}); err != nil {
		return document.Record{}, err
	}
	if _, err := s.documents.AddLink(order.Header.ID, record.Header.ID, "fulfillment_for", map[string]any{"generated_document_type": "sales_fulfillment"}); err != nil {
		return document.Record{}, err
	}
	created, err := s.documents.Get(record.Header.ID)
	if err != nil {
		return document.Record{}, err
	}
	if err := s.refreshDocuments(created, order); err != nil {
		return document.Record{}, err
	}
	return created, nil
}

func (s *FulfillmentCoreService) ValidateApprove(record document.Record) error {
	if record.Header.Type != "sales_fulfillment" {
		return nil
	}
	if len(recordList(record.Body.Payload["lines"])) == 0 {
		return shared.Validation("fulfillment lines are required")
	}
	return s.inventory.ValidateFulfillmentIssue(record)
}

func (s *FulfillmentCoreService) ValidateCancel(record document.Record) error {
	if record.Header.Type != "sales_fulfillment" {
		return nil
	}
	return nil
}

func (s *FulfillmentCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	if record.Header.Type != "sales_fulfillment" {
		return nil
	}
	if err := s.inventory.HandleApprovedFulfillment(record, actorID); err != nil {
		return err
	}
	updated, err := s.documents.Get(record.Header.ID)
	if err == nil {
		record = updated
	}
	return s.refreshDocuments(record)
}

func (s *FulfillmentCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	if record.Header.Type != "sales_fulfillment" {
		return nil
	}
	return s.refreshDocuments(record)
}

func (s *FulfillmentCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	if strings.TrimSpace(documentType) != "sales_fulfillment" {
		return document.NormalizePayload(cloneMap(payload))
	}
	next := document.NormalizePayload(cloneMap(payload))
	lines := s.inventory.normalizeInventoryLines(recordList(next["lines"]), false)
	next["lines"] = lines
	next["reserved_quantity_total"] = roundMoney(sumInventoryLineQuantity(lines, "quantity"))
	next["fulfilled_quantity_total"] = roundMoney(numberValue(next["fulfilled_quantity_total"]))
	next["fulfillment_status"] = firstNonEmptyString(textValue(next["fulfillment_status"]), "reserved")
	return next
}

func (s *FulfillmentCoreService) refreshDocuments(records ...document.Record) error {
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

func (s *FulfillmentCoreService) existingFulfillmentQuantities(orderID string) map[int]float64 {
	quantities := map[int]float64{}
	for _, record := range s.documents.List() {
		if record.Header.Type != "sales_fulfillment" {
			continue
		}
		if record.Header.Status == "cancelled" || record.Header.Status == "rejected" {
			continue
		}
		for _, link := range record.Links {
			if link.LinkType != "fulfillment_for" || link.LinkedDocumentID != orderID {
				continue
			}
			for _, line := range recordList(record.Body.Payload["lines"]) {
				index := int(numberValue(line["source_order_line_index"]))
				quantities[index] = roundMoney(quantities[index] + numberValue(line["quantity"]))
			}
		}
	}
	return quantities
}
