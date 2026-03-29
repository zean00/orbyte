package application

import (
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type TraceabilityCoreService struct {
	documents *document.Service
	models    *model.Service
	inventory *InventoryCoreService
}

func NewTraceabilityCoreService(documents *document.Service, models *model.Service, inventorySvc *InventoryCoreService) *TraceabilityCoreService {
	return &TraceabilityCoreService{
		documents: documents,
		models:    models,
		inventory: inventorySvc,
	}
}

func (s *TraceabilityCoreService) BatchTrace(batchID, organizationID, locationID string, now time.Time) (map[string]any, error) {
	if s.models == nil {
		return nil, shared.Validation("traceability batches are unavailable")
	}
	batch, err := s.models.Get("inventory_batch", strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	if s.inventory != nil {
		batch = s.inventory.DecorateBatchRecord(batch, organizationID, locationID, now)
	}
	itemCode := textValue(batch.Values["item_code"])
	warehouseCode := textValue(batch.Values["warehouse_code"])
	batchCode := textValue(batch.Values["batch_code"])
	expirationDate := textValue(batch.Values["expiration_date"])
	if itemCode == "" || warehouseCode == "" || batchCode == "" {
		return nil, shared.Validation("batch trace requires item, warehouse, and batch")
	}
	documentsByID := map[string]document.Record{}
	for _, record := range s.documents.List() {
		documentsByID[record.Header.ID] = record
	}

	movements := s.batchMovements(documentsByID, organizationID, locationID, itemCode, warehouseCode, batchCode, expirationDate)
	nodes := make(map[string]map[string]any)
	edges := make([]map[string]any, 0)
	docQueue := make([]string, 0)
	enqueued := map[string]struct{}{}

	for _, movement := range movements {
		movementNodeID := "document:" + movement.Header.ID
		nodes[movementNodeID] = traceNodeFromDocument(movement, movement.Body.Payload, "movement")
		sourceDocumentID := textValue(movement.Body.Payload["source_document_id"])
		if sourceDocumentID != "" {
			if source, ok := documentsByID[sourceDocumentID]; ok {
				sourceNodeID := "document:" + source.Header.ID
				nodes[sourceNodeID] = traceNodeFromDocument(source, source.Body.Payload, "document")
				edges = append(edges, map[string]any{
					"from":     movementNodeID,
					"to":       sourceNodeID,
					"kind":     "movement_source",
					"linkType": "movement_for",
				})
				if _, seen := enqueued[source.Header.ID]; !seen {
					enqueued[source.Header.ID] = struct{}{}
					docQueue = append(docQueue, source.Header.ID)
				}
			}
		}
	}

	producedInto := make([]map[string]any, 0)
	consumedFrom := make([]map[string]any, 0)
	visitedDocs := map[string]struct{}{}
	for len(docQueue) > 0 {
		documentID := docQueue[0]
		docQueue = docQueue[1:]
		if _, seen := visitedDocs[documentID]; seen {
			continue
		}
		visitedDocs[documentID] = struct{}{}
		record, ok := documentsByID[documentID]
		if !ok {
			continue
		}
		recordNodeID := "document:" + record.Header.ID
		nodes[recordNodeID] = traceNodeFromDocument(record, record.Body.Payload, "document")
		for _, link := range record.Links {
			linked, ok := documentsByID[link.LinkedDocumentID]
			if !ok {
				continue
			}
			if !matchesInventoryScope(linked, organizationID, locationID) && linked.Header.Type != "sales_order" && linked.Header.Type != "invoice" {
				continue
			}
			linkedNodeID := "document:" + linked.Header.ID
			nodes[linkedNodeID] = traceNodeFromDocument(linked, linked.Body.Payload, "document")
			edges = append(edges, map[string]any{
				"from":     recordNodeID,
				"to":       linkedNodeID,
				"kind":     "document_link",
				"linkType": link.LinkType,
			})
			if shouldExpandTraceLink(link.LinkType, linked.Header.Type) {
				if _, seen := visitedDocs[linked.Header.ID]; !seen {
					docQueue = append(docQueue, linked.Header.ID)
				}
			}
			if record.Header.Type == "production_issue" && linked.Header.Type == "production_order" {
				outputs := s.linkedDocumentsByType(documentsByID, linked, "production_for", "production_output")
				for _, output := range outputs {
					producedInto = append(producedInto, map[string]any{
						"production_order_id":     linked.Header.ID,
						"production_order_number": firstNonEmptyString(linked.Header.Number, linked.Header.ID),
						"production_output_id":    output.Header.ID,
						"production_output_number": firstNonEmptyString(
							output.Header.Number,
							output.Header.ID,
						),
						"item_code":        textValue(output.Body.Payload["finished_item_code"]),
						"batch_code":       firstNonEmptyString(textValue(output.Body.Payload["production_lot_code"]), textValue(output.Body.Payload["batch_code"])),
						"warehouse_code":   textValue(output.Body.Payload["warehouse_code"]),
						"expiration_date":  textValue(output.Body.Payload["expiration_date"]),
						"output_quantity":  roundMoney(numberValue(output.Body.Payload["output_quantity"])),
						"status":           output.Header.Status,
						"source_batch_id":  batch.ID,
						"source_batch_code": batchCode,
					})
				}
			}
			if record.Header.Type == "production_output" && linked.Header.Type == "production_order" {
				issues := s.linkedDocumentsByType(documentsByID, linked, "production_for", "production_issue")
				for _, issue := range issues {
					for _, line := range recordList(issue.Body.Payload["lines"]) {
						if textValue(line["batch_code"]) != batchCode {
							continue
						}
						if warehouseCode != "" && textValue(line["warehouse_code"]) != warehouseCode {
							continue
						}
						consumedFrom = append(consumedFrom, map[string]any{
							"production_order_id":     linked.Header.ID,
							"production_order_number": firstNonEmptyString(linked.Header.Number, linked.Header.ID),
							"production_issue_id":     issue.Header.ID,
							"production_issue_number": firstNonEmptyString(issue.Header.Number, issue.Header.ID),
							"item_code":               textValue(line["item_code"]),
							"batch_code":              textValue(line["batch_code"]),
							"warehouse_code":          textValue(line["warehouse_code"]),
							"expiration_date":         textValue(line["expiration_date"]),
							"quantity":                roundMoney(numberValue(line["quantity"])),
							"status":                  issue.Header.Status,
						})
					}
				}
			}
		}
	}

	nodeList := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		nodeList = append(nodeList, node)
	}
	sort.Slice(nodeList, func(i, j int) bool {
		leftDate := textValue(nodeList[i]["date"])
		rightDate := textValue(nodeList[j]["date"])
		if leftDate != rightDate {
			return leftDate < rightDate
		}
		return textValue(nodeList[i]["number"]) < textValue(nodeList[j]["number"])
	})
	sort.Slice(edges, func(i, j int) bool {
		left := textValue(edges[i]["from"]) + "|" + textValue(edges[i]["to"]) + "|" + textValue(edges[i]["kind"])
		right := textValue(edges[j]["from"]) + "|" + textValue(edges[j]["to"]) + "|" + textValue(edges[j]["kind"])
		return left < right
	})
	sort.Slice(producedInto, func(i, j int) bool {
		left := textValue(producedInto[i]["batch_code"])
		right := textValue(producedInto[j]["batch_code"])
		if left != right {
			return left < right
		}
		return textValue(producedInto[i]["production_output_number"]) < textValue(producedInto[j]["production_output_number"])
	})
	sort.Slice(consumedFrom, func(i, j int) bool {
		left := textValue(consumedFrom[i]["batch_code"])
		right := textValue(consumedFrom[j]["batch_code"])
		if left != right {
			return left < right
		}
		return textValue(consumedFrom[i]["production_issue_number"]) < textValue(consumedFrom[j]["production_issue_number"])
	})

	summary := map[string]any{
		"batch_id":            batch.ID,
		"item_code":           itemCode,
		"warehouse_code":      warehouseCode,
		"batch_code":          batchCode,
		"expiration_date":     expirationDate,
		"status":              textValue(batch.Values["status"]),
		"is_issuable":         boolValue(batch.Values["is_issuable"]),
		"hold_reason":         textValue(batch.Values["hold_reason"]),
		"hold_notes":          textValue(batch.Values["hold_notes"]),
		"recall_reference":    textValue(batch.Values["recall_reference"]),
		"on_hand_quantity":    roundMoney(numberValue(batch.Values["on_hand_quantity"])),
		"reserved_quantity":   roundMoney(numberValue(batch.Values["reserved_quantity"])),
		"available_quantity":  roundMoney(numberValue(batch.Values["available_quantity"])),
		"movement_count":      len(movements),
		"document_node_count": len(nodeList),
	}

	return map[string]any{
		"summary":       summary,
		"nodes":         nodeList,
		"edges":         edges,
		"produced_into": producedInto,
		"consumed_from": consumedFrom,
	}, nil
}

func (s *TraceabilityCoreService) batchMovements(documentsByID map[string]document.Record, organizationID, locationID, itemCode, warehouseCode, batchCode, expirationDate string) []document.Record {
	movements := make([]document.Record, 0)
	for _, record := range documentsByID {
		if record.Header.Type != "stock_movement" || record.Header.Status != "posted" {
			continue
		}
		if !matchesInventoryScope(record, organizationID, locationID) {
			continue
		}
		payload := record.Body.Payload
		if textValue(payload["item_code"]) != itemCode || textValue(payload["warehouse_code"]) != warehouseCode || textValue(payload["batch_code"]) != batchCode {
			continue
		}
		if expirationDate != "" && textValue(payload["expiration_date"]) != expirationDate {
			continue
		}
		movements = append(movements, record)
	}
	sort.Slice(movements, func(i, j int) bool {
		leftDate := textValue(movements[i].Body.Payload["movement_date"])
		rightDate := textValue(movements[j].Body.Payload["movement_date"])
		if leftDate != rightDate {
			return leftDate < rightDate
		}
		return movements[i].Header.ID < movements[j].Header.ID
	})
	return movements
}

func (s *TraceabilityCoreService) linkedDocumentsByType(documentsByID map[string]document.Record, record document.Record, linkType, documentType string) []document.Record {
	out := make([]document.Record, 0)
	seen := map[string]struct{}{}
	for _, link := range record.Links {
		if link.LinkType != linkType {
			continue
		}
		linked, ok := documentsByID[link.LinkedDocumentID]
		if !ok || linked.Header.Type != documentType {
			continue
		}
		if _, exists := seen[linked.Header.ID]; exists {
			continue
		}
		seen[linked.Header.ID] = struct{}{}
		out = append(out, linked)
	}
	return out
}

func shouldExpandTraceLink(linkType, documentType string) bool {
	switch linkType {
	case "movement_for", "production_for", "delivery_for", "return_for", "exchange_for", "credit_for", "fulfillment_for", "receipt_for", "invoice_for":
		return true
	}
	switch documentType {
	case "sales_order", "sales_fulfillment", "delivery_order", "sales_return", "return_receipt", "supplier_return", "goods_receipt", "vendor_bill", "production_order", "production_issue", "production_output":
		return true
	}
	return false
}

func traceNodeFromDocument(record document.Record, payload map[string]any, category string) map[string]any {
	date := firstNonEmptyString(
		textValue(payload["movement_date"]),
		textValue(payload["delivery_date"]),
		textValue(payload["dispatch_date"]),
		textValue(payload["delivered_date"]),
		textValue(payload["return_date"]),
		textValue(payload["receipt_date"]),
		textValue(payload["bill_date"]),
		textValue(payload["order_date"]),
		textValue(payload["output_date"]),
		textValue(payload["issue_date"]),
		textValue(payload["planned_date"]),
		textValue(payload["credit_date"]),
		textValue(payload["request_date"]),
		record.Header.CreatedAt.UTC().Format("2006-01-02"),
	)
	node := map[string]any{
		"id":       "document:" + record.Header.ID,
		"category": category,
		"type":     record.Header.Type,
		"status":   record.Header.Status,
		"number":   firstNonEmptyString(record.Header.Number, record.Header.ID),
		"date":     date,
	}
	if category == "movement" {
		node["movement_reason"] = textValue(payload["movement_reason"])
		node["direction"] = textValue(payload["movement_direction"])
		node["quantity_delta"] = roundMoney(numberValue(payload["quantity_delta"]))
		node["warehouse_code"] = textValue(payload["warehouse_code"])
		node["batch_code"] = textValue(payload["batch_code"])
	}
	return node
}
