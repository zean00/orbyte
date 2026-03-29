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

type RecallCoreService struct {
	documents    *document.Service
	models       *model.Service
	search       *search.Service
	inventory    *InventoryCoreService
	traceability *TraceabilityCoreService
}

func NewRecallCoreService(documents *document.Service, models *model.Service, searchSvc *search.Service, inventorySvc *InventoryCoreService, traceabilitySvc *TraceabilityCoreService) *RecallCoreService {
	return &RecallCoreService{
		documents:    documents,
		models:       models,
		search:       searchSvc,
		inventory:    inventorySvc,
		traceability: traceabilitySvc,
	}
}

func (s *RecallCoreService) NormalizePayload(documentType string, payload map[string]any) map[string]any {
	next := document.NormalizePayload(cloneMap(payload))
	switch strings.TrimSpace(documentType) {
	case "recall_case":
		next["title"] = textValue(next["title"])
		next["reason"] = textValue(next["reason"])
		next["severity"] = firstNonEmptyString(textValue(next["severity"]), "medium")
		next["recall_reference"] = textValue(next["recall_reference"])
		next["containment_mode"] = firstNonEmptyString(textValue(next["containment_mode"]), "recalled")
		next["action_generation_mode"] = firstNonEmptyString(textValue(next["action_generation_mode"]), "internal")
		next["affected_batches"] = s.normalizeAffectedBatches(recordList(next["affected_batches"]))
		next["impact_summary"] = normalizeGenericMap(next["impact_summary"])
		next["generated_action_count"] = int(numberValue(next["generated_action_count"]))
	case "recall_action":
		next["action_type"] = textValue(next["action_type"])
		next["action_status"] = firstNonEmptyString(textValue(next["action_status"]), "open")
		next["source_recall_case_id"] = textValue(next["source_recall_case_id"])
		next["source_recall_case_number"] = textValue(next["source_recall_case_number"])
		next["source_document_id"] = textValue(next["source_document_id"])
		next["source_document_type"] = textValue(next["source_document_type"])
		next["source_document_number"] = textValue(next["source_document_number"])
		next["item_code"] = textValue(next["item_code"])
		next["batch_code"] = textValue(next["batch_code"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["quantity"] = roundMoney(numberValue(next["quantity"]))
		next["notes"] = textValue(next["notes"])
	}
	return next
}

func (s *RecallCoreService) ValidateApprove(record document.Record) error {
	switch record.Header.Type {
	case "recall_case":
		batches := recordList(record.Body.Payload["affected_batches"])
		if len(batches) == 0 {
			return shared.Validation("recall case requires at least one affected batch")
		}
		for _, batch := range batches {
			if textValue(batch["batch_id"]) == "" {
				return shared.Validation("affected batch id is required")
			}
		}
	case "recall_action":
		if textValue(record.Body.Payload["action_type"]) == "" {
			return shared.Validation("recall action type is required")
		}
	}
	return nil
}

func (s *RecallCoreService) ValidateCancel(record document.Record) error {
	return nil
}

func (s *RecallCoreService) HandleApprovedDocument(record document.Record, actorID string) error {
	switch record.Header.Type {
	case "recall_case":
		return s.activateRecallCase(record, actorID)
	case "recall_action":
		return s.completeRecallAction(record, actorID)
	default:
		return nil
	}
}

func (s *RecallCoreService) HandleCanceledDocument(record document.Record, actorID string) error {
	if record.Header.Type != "recall_action" {
		return nil
	}
	payload := clonedPayload(record.Body.Payload)
	payload["action_status"] = "cancelled"
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *RecallCoreService) activateRecallCase(record document.Record, actorID string) error {
	if s.inventory == nil || s.traceability == nil {
		return shared.Validation("recall services are unavailable")
	}
	payload := clonedPayload(record.Body.Payload)
	batches := s.normalizeAffectedBatches(recordList(payload["affected_batches"]))
	now := time.Now().UTC()
	impact := map[string]float64{
		"on_hand_quantity":          0,
		"reserved_quantity":         0,
		"available_quantity":        0,
		"delivered_quantity":        0,
		"returned_quantity":         0,
		"supplier_returned_quantity": 0,
		"production_consumed_quantity": 0,
	}
	affectedDeliveries := map[string]document.Record{}
	affectedReturns := map[string]document.Record{}
	affectedSupplierReturns := map[string]document.Record{}
	affectedProduction := map[string]document.Record{}
	producedBatchKeys := map[string]struct{}{}
	createdActions := 0

	for index, batch := range batches {
		batchID := textValue(batch["batch_id"])
		updatedBatch, err := s.inventory.ApplyBatchAction(
			batchID,
			"recall",
			actorID,
			firstNonEmptyString(textValue(payload["reason"]), textValue(payload["title"]), "recall"),
			firstNonEmptyString(textValue(payload["notes"]), textValue(payload["title"])),
			firstNonEmptyString(textValue(payload["recall_reference"]), firstNonEmptyString(record.Header.Number, record.Header.ID)),
			now,
		)
		if err != nil {
			return err
		}
		batches[index] = map[string]any{
			"batch_id":           updatedBatch.ID,
			"item_code":          textValue(updatedBatch.Values["item_code"]),
			"warehouse_code":     textValue(updatedBatch.Values["warehouse_code"]),
			"batch_code":         textValue(updatedBatch.Values["batch_code"]),
			"expiration_date":    textValue(updatedBatch.Values["expiration_date"]),
			"status":             textValue(updatedBatch.Values["status"]),
			"on_hand_quantity":   roundMoney(numberValue(updatedBatch.Values["on_hand_quantity"])),
			"reserved_quantity":  roundMoney(numberValue(updatedBatch.Values["reserved_quantity"])),
			"available_quantity": roundMoney(numberValue(updatedBatch.Values["available_quantity"])),
		}

		trace, err := s.traceability.BatchTrace(batchID, record.Header.OrganizationID, record.Header.LocationID, now)
		if err != nil {
			return err
		}
		summary := normalizeGenericMap(trace["summary"])
		impact["on_hand_quantity"] = roundMoney(impact["on_hand_quantity"] + numberValue(summary["on_hand_quantity"]))
		impact["reserved_quantity"] = roundMoney(impact["reserved_quantity"] + numberValue(summary["reserved_quantity"]))
		impact["available_quantity"] = roundMoney(impact["available_quantity"] + numberValue(summary["available_quantity"]))
		nodes := recordList(trace["nodes"])
		for _, node := range nodes {
			nodeType := textValue(node["type"])
			nodeID := strings.TrimPrefix(textValue(node["id"]), "document:")
			switch nodeType {
			case "delivery_order":
				if doc, ok := s.fetchDocument(nodeID); ok {
					affectedDeliveries[nodeID] = doc
				}
			case "sales_return", "return_receipt":
				if doc, ok := s.fetchDocument(nodeID); ok {
					affectedReturns[nodeID] = doc
				}
			case "supplier_return":
				if doc, ok := s.fetchDocument(nodeID); ok {
					affectedSupplierReturns[nodeID] = doc
				}
			case "production_issue", "production_order", "production_output":
				if doc, ok := s.fetchDocument(nodeID); ok {
					affectedProduction[nodeID] = doc
				}
			}
		}
		for _, produced := range recordList(trace["produced_into"]) {
			key := textValue(produced["item_code"]) + "|" + textValue(produced["batch_code"]) + "|" + textValue(produced["warehouse_code"])
			if key != "||" {
				producedBatchKeys[key] = struct{}{}
			}
		}
		for _, consumed := range recordList(trace["consumed_from"]) {
			impact["production_consumed_quantity"] = roundMoney(impact["production_consumed_quantity"] + numberValue(consumed["quantity"]))
		}
		if created, err := s.ensureRecallAction(record, actorID, "warehouse_hold", "", "", summary); err != nil {
			return err
		} else if created {
			createdActions++
		}
	}

	for _, delivery := range affectedDeliveries {
		qty := s.sumBatchLineQuantity(delivery, "delivery_order", batches)
		impact["delivered_quantity"] = roundMoney(impact["delivered_quantity"] + qty)
		if created, err := s.ensureRecallAction(record, actorID, "delivery_review", delivery.Header.ID, "delivery_order", map[string]any{
			"item_code":      firstBatchValue(batches, "item_code"),
			"batch_code":     firstBatchValue(batches, "batch_code"),
			"warehouse_code": firstBatchValue(batches, "warehouse_code"),
			"quantity":       qty,
			"number":         firstNonEmptyString(delivery.Header.Number, delivery.Header.ID),
		}); err != nil {
			return err
		} else if created {
			createdActions++
		}
	}
	for _, item := range affectedReturns {
		qty := s.sumBatchLineQuantity(item, item.Header.Type, batches)
		impact["returned_quantity"] = roundMoney(impact["returned_quantity"] + qty)
		if created, err := s.ensureRecallAction(record, actorID, "return_review", item.Header.ID, item.Header.Type, map[string]any{
			"item_code":      firstBatchValue(batches, "item_code"),
			"batch_code":     firstBatchValue(batches, "batch_code"),
			"warehouse_code": firstBatchValue(batches, "warehouse_code"),
			"quantity":       qty,
			"number":         firstNonEmptyString(item.Header.Number, item.Header.ID),
		}); err != nil {
			return err
		} else if created {
			createdActions++
		}
	}
	for _, item := range affectedSupplierReturns {
		qty := s.sumBatchLineQuantity(item, item.Header.Type, batches)
		impact["supplier_returned_quantity"] = roundMoney(impact["supplier_returned_quantity"] + qty)
		if created, err := s.ensureRecallAction(record, actorID, "supplier_review", item.Header.ID, item.Header.Type, map[string]any{
			"item_code":      firstBatchValue(batches, "item_code"),
			"batch_code":     firstBatchValue(batches, "batch_code"),
			"warehouse_code": firstBatchValue(batches, "warehouse_code"),
			"quantity":       qty,
			"number":         firstNonEmptyString(item.Header.Number, item.Header.ID),
		}); err != nil {
			return err
		} else if created {
			createdActions++
		}
	}
	for _, item := range affectedProduction {
		qty := s.sumBatchLineQuantity(item, item.Header.Type, batches)
		if qty <= 0 {
			qty = roundMoney(numberValue(item.Body.Payload["output_quantity"]))
		}
		if created, err := s.ensureRecallAction(record, actorID, "production_review", item.Header.ID, item.Header.Type, map[string]any{
			"item_code":      firstBatchValue(batches, "item_code"),
			"batch_code":     firstBatchValue(batches, "batch_code"),
			"warehouse_code": firstBatchValue(batches, "warehouse_code"),
			"quantity":       qty,
			"number":         firstNonEmptyString(item.Header.Number, item.Header.ID),
		}); err != nil {
			return err
		} else if created {
			createdActions++
		}
	}

	impactSummary := map[string]any{
		"on_hand_quantity":            roundMoney(impact["on_hand_quantity"]),
		"reserved_quantity":           roundMoney(impact["reserved_quantity"]),
		"available_quantity":          roundMoney(impact["available_quantity"]),
		"delivered_quantity":          roundMoney(impact["delivered_quantity"]),
		"returned_quantity":           roundMoney(impact["returned_quantity"]),
		"supplier_returned_quantity":  roundMoney(impact["supplier_returned_quantity"]),
		"production_consumed_quantity": roundMoney(impact["production_consumed_quantity"]),
		"affected_delivery_count":     len(affectedDeliveries),
		"affected_return_count":       len(affectedReturns),
		"affected_supplier_return_count": len(affectedSupplierReturns),
		"affected_production_count":   len(affectedProduction),
		"downstream_produced_batches_count": len(producedBatchKeys),
	}
	payload["affected_batches"] = batches
	payload["impact_summary"] = impactSummary
	payload["generated_action_count"] = createdActions
	payload["containment_mode"] = firstNonEmptyString(textValue(payload["containment_mode"]), "recalled")
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *RecallCoreService) completeRecallAction(record document.Record, actorID string) error {
	payload := clonedPayload(record.Body.Payload)
	payload["action_status"] = "completed"
	return s.saveMutatedDocument(record, actorID, payload)
}

func (s *RecallCoreService) ensureRecallAction(caseRecord document.Record, actorID, actionType, sourceDocumentID, sourceDocumentType string, traceSummary map[string]any) (bool, error) {
	if s.documents == nil {
		return false, shared.Validation("documents service is unavailable")
	}
	batchCode := textValue(traceSummary["batch_code"])
	warehouseCode := textValue(traceSummary["warehouse_code"])
	itemCode := textValue(traceSummary["item_code"])
	for _, record := range s.documents.List() {
		if record.Header.Type != "recall_action" {
			continue
		}
		payload := record.Body.Payload
		if textValue(payload["source_recall_case_id"]) != caseRecord.Header.ID {
			continue
		}
		if textValue(payload["action_type"]) != actionType {
			continue
		}
		if textValue(payload["source_document_id"]) != sourceDocumentID {
			continue
		}
		if textValue(payload["batch_code"]) != batchCode || textValue(payload["warehouse_code"]) != warehouseCode || textValue(payload["item_code"]) != itemCode {
			continue
		}
		if record.Header.Status != "cancelled" && record.Header.Status != "rejected" {
			return false, nil
		}
	}
	actionPayload := map[string]any{
		"source_recall_case_id":     caseRecord.Header.ID,
		"source_recall_case_number": firstNonEmptyString(caseRecord.Header.Number, caseRecord.Header.ID),
		"action_type":               actionType,
		"action_status":             "open",
		"source_document_id":        sourceDocumentID,
		"source_document_type":      sourceDocumentType,
		"source_document_number":    textValue(traceSummary["number"]),
		"item_code":                 itemCode,
		"batch_code":                batchCode,
		"warehouse_code":            warehouseCode,
		"quantity":                  roundMoney(numberValue(traceSummary["quantity"])),
		"notes":                     firstNonEmptyString(textValue(traceSummary["notes"]), fmt.Sprintf("%s for batch %s", humanizeActionType(actionType), batchCode)),
	}
	record, err := s.documents.Create("recall_action", caseRecord.Header.OrganizationID, caseRecord.Header.LocationID, actorID, actionPayload)
	if err != nil {
		return false, err
	}
	if _, err := s.documents.AddLink(record.Header.ID, caseRecord.Header.ID, "recall_for", map[string]any{"source_type": "recall_case"}); err != nil && !isConflict(err) {
		return false, err
	}
	if _, err := s.documents.AddLink(caseRecord.Header.ID, record.Header.ID, "recall_for", map[string]any{"generated_document_type": "recall_action"}); err != nil && !isConflict(err) {
		return false, err
	}
	if sourceDocumentID != "" {
		if _, err := s.documents.AddLink(record.Header.ID, sourceDocumentID, "related_to", map[string]any{"source_type": sourceDocumentType}); err != nil && !isConflict(err) {
			return false, err
		}
	}
	if err := s.promoteRecallAction(record, actorID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *RecallCoreService) promoteRecallAction(record document.Record, actorID string) error {
	record.Header.Status = "submitted"
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

func (s *RecallCoreService) normalizeAffectedBatches(rows []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneMap(row)
		next["batch_id"] = textValue(next["batch_id"])
		next["item_code"] = textValue(next["item_code"])
		next["warehouse_code"] = textValue(next["warehouse_code"])
		next["batch_code"] = textValue(next["batch_code"])
		next["expiration_date"] = textValue(next["expiration_date"])
		next["status"] = textValue(next["status"])
		next["on_hand_quantity"] = roundMoney(numberValue(next["on_hand_quantity"]))
		next["reserved_quantity"] = roundMoney(numberValue(next["reserved_quantity"]))
		next["available_quantity"] = roundMoney(numberValue(next["available_quantity"]))
		normalized = append(normalized, next)
	}
	sort.Slice(normalized, func(i, j int) bool {
		leftItem := textValue(normalized[i]["item_code"])
		rightItem := textValue(normalized[j]["item_code"])
		if leftItem != rightItem {
			return leftItem < rightItem
		}
		return textValue(normalized[i]["batch_code"]) < textValue(normalized[j]["batch_code"])
	})
	return normalized
}

func (s *RecallCoreService) sumBatchLineQuantity(record document.Record, documentType string, batches []map[string]any) float64 {
	batchKeys := map[string]struct{}{}
	for _, batch := range batches {
		batchKeys[fmt.Sprintf("%s|%s|%s|%s", textValue(batch["item_code"]), textValue(batch["warehouse_code"]), textValue(batch["batch_code"]), textValue(batch["expiration_date"]))] = struct{}{}
	}
	total := 0.0
	switch documentType {
	case "delivery_order", "sales_return", "return_receipt", "supplier_return", "production_issue":
		for _, line := range recordList(record.Body.Payload["lines"]) {
			key := fmt.Sprintf("%s|%s|%s|%s", textValue(line["item_code"]), textValue(line["warehouse_code"]), textValue(line["batch_code"]), textValue(line["expiration_date"]))
			if _, ok := batchKeys[key]; !ok {
				continue
			}
			total = roundMoney(total + firstPositiveNumber(line["quantity"], line["received_quantity"], line["fulfilled_quantity"]))
		}
	case "production_output":
		key := fmt.Sprintf("%s|%s|%s|%s", textValue(record.Body.Payload["finished_item_code"]), textValue(record.Body.Payload["warehouse_code"]), firstNonEmptyString(textValue(record.Body.Payload["production_lot_code"]), textValue(record.Body.Payload["batch_code"])), textValue(record.Body.Payload["expiration_date"]))
		if _, ok := batchKeys[key]; ok {
			total = roundMoney(numberValue(record.Body.Payload["output_quantity"]))
		}
	}
	return total
}

func (s *RecallCoreService) fetchDocument(documentID string) (document.Record, bool) {
	if strings.TrimSpace(documentID) == "" {
		return document.Record{}, false
	}
	record, err := s.documents.Get(documentID)
	if err != nil {
		return document.Record{}, false
	}
	return record, true
}

func (s *RecallCoreService) saveMutatedDocument(record document.Record, actorID string, payload map[string]any) error {
	record.Body.Payload = document.NormalizePayload(payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)
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

func (s *RecallCoreService) refreshDocuments(records ...document.Record) {
	if s.search == nil {
		return
	}
	for _, record := range records {
		s.search.RefreshDocument(record)
	}
}

func normalizeGenericMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return cloneMap(typed)
	}
	return map[string]any{}
}

func firstBatchValue(batches []map[string]any, key string) string {
	if len(batches) == 0 {
		return ""
	}
	return textValue(batches[0][key])
}

func humanizeActionType(actionType string) string {
	switch strings.TrimSpace(actionType) {
	case "warehouse_hold":
		return "Warehouse hold"
	case "delivery_review":
		return "Delivery review"
	case "return_review":
		return "Return review"
	case "supplier_review":
		return "Supplier review"
	case "production_review":
		return "Production review"
	default:
		return strings.ReplaceAll(strings.Title(strings.ReplaceAll(actionType, "_", " ")), "_", " ")
	}
}
