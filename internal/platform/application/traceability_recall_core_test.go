package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestBatchTraceIncludesProducedIntoFromProductionChain(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterTraceabilityRecallTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	rawBatch, err := models.Create("inventory_batch", "user_admin", map[string]any{
		"item_code":       "BEAN-RAW",
		"warehouse_code":  "MAIN",
		"batch_code":      "RAW-001",
		"expiration_date": time.Now().UTC().AddDate(0, 0, 45).Format("2006-01-02"),
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create raw batch: %v", err)
	}

	receipt, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"receipt_date": time.Now().UTC().Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	receipt.Header.Status = "received"
	receipt.Header.Number = "GR-TRACE-001"
	if err := docs.Save(receipt); err != nil {
		t.Fatalf("save goods receipt: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "BEAN-RAW",
		"warehouse_code":     "MAIN",
		"batch_code":         "RAW-001",
		"expiration_date":    time.Now().UTC().AddDate(0, 0, 45).Format("2006-01-02"),
		"quantity_delta":     10.0,
		"movement_reason":    "goods_receipt",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
		"source_document_id": receipt.Header.ID,
	})

	order, err := docs.Create("production_order", "org_default", "loc_main", "user_admin", map[string]any{
		"finished_item_code": "BEAN-ROASTED",
		"warehouse_code":     "MAIN",
	})
	if err != nil {
		t.Fatalf("create production order: %v", err)
	}
	order.Header.Status = "approved"
	order.Header.Number = "PO-TRACE-001"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save production order: %v", err)
	}

	issue, err := docs.Create("production_issue", "org_default", "loc_main", "user_admin", map[string]any{
		"warehouse_code": "MAIN",
		"lines": []map[string]any{{
			"item_code":       "BEAN-RAW",
			"warehouse_code":  "MAIN",
			"batch_code":      "RAW-001",
			"expiration_date": time.Now().UTC().AddDate(0, 0, 45).Format("2006-01-02"),
			"quantity":        4.0,
		}},
	})
	if err != nil {
		t.Fatalf("create production issue: %v", err)
	}
	issue.Header.Status = "issued"
	issue.Header.Number = "PI-TRACE-001"
	if err := docs.Save(issue); err != nil {
		t.Fatalf("save production issue: %v", err)
	}

	output, err := docs.Create("production_output", "org_default", "loc_main", "user_admin", map[string]any{
		"finished_item_code":  "BEAN-ROASTED",
		"warehouse_code":      "MAIN",
		"output_quantity":     3.0,
		"production_lot_code": "ROAST-001",
	})
	if err != nil {
		t.Fatalf("create production output: %v", err)
	}
	output.Header.Status = "posted"
	output.Header.Number = "POUT-TRACE-001"
	if err := docs.Save(output); err != nil {
		t.Fatalf("save production output: %v", err)
	}

	mustAddDocumentLink(t, docs, issue.Header.ID, order.Header.ID, "production_for")
	mustAddDocumentLink(t, docs, order.Header.ID, issue.Header.ID, "production_for")
	mustAddDocumentLink(t, docs, output.Header.ID, order.Header.ID, "production_for")
	mustAddDocumentLink(t, docs, order.Header.ID, output.Header.ID, "production_for")

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "BEAN-RAW",
		"warehouse_code":     "MAIN",
		"batch_code":         "RAW-001",
		"expiration_date":    time.Now().UTC().AddDate(0, 0, 45).Format("2006-01-02"),
		"quantity_delta":     -4.0,
		"movement_reason":    "production_issue",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "out",
		"source_document_id": issue.Header.ID,
	})

	traceSvc := NewTraceabilityCoreService(docs, models, NewInventoryCoreService(docs, nil, models, nil))
	trace, err := traceSvc.BatchTrace(rawBatch.ID, "org_default", "loc_main", time.Now().UTC())
	if err != nil {
		t.Fatalf("batch trace: %v", err)
	}

	producedInto := recordList(trace["produced_into"])
	if len(producedInto) != 1 {
		t.Fatalf("expected 1 produced-into row, got %d", len(producedInto))
	}
	if got := textValue(producedInto[0]["batch_code"]); got != "ROAST-001" {
		t.Fatalf("expected produced batch ROAST-001, got %s", got)
	}
	if got := textValue(producedInto[0]["production_output_number"]); got != "POUT-TRACE-001" {
		t.Fatalf("expected output number POUT-TRACE-001, got %s", got)
	}
}

func TestRecallCaseApprovalRecallsBatchAndCreatesActions(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterTraceabilityRecallTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	batch, err := models.Create("inventory_batch", "user_admin", map[string]any{
		"item_code":       "MED-LOT",
		"warehouse_code":  "MAIN",
		"batch_code":      "LOT-RECALL-01",
		"expiration_date": time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02"),
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create inventory batch: %v", err)
	}

	receipt, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"receipt_date": time.Now().UTC().Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	receipt.Header.Status = "received"
	receipt.Header.Number = "GR-RECALL-001"
	if err := docs.Save(receipt); err != nil {
		t.Fatalf("save goods receipt: %v", err)
	}
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "MED-LOT",
		"warehouse_code":     "MAIN",
		"batch_code":         "LOT-RECALL-01",
		"expiration_date":    time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02"),
		"quantity_delta":     5.0,
		"movement_reason":    "goods_receipt",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
		"source_document_id": receipt.Header.ID,
	})

	fulfillment, err := docs.Create("sales_fulfillment", "org_default", "loc_main", "user_admin", map[string]any{
		"lines": []map[string]any{{
			"item_code":       "MED-LOT",
			"warehouse_code":  "MAIN",
			"batch_code":      "LOT-RECALL-01",
			"expiration_date": time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02"),
			"quantity":        2.0,
		}},
	})
	if err != nil {
		t.Fatalf("create fulfillment: %v", err)
	}
	fulfillment.Header.Status = "issued"
	fulfillment.Header.Number = "FUL-RECALL-001"
	if err := docs.Save(fulfillment); err != nil {
		t.Fatalf("save fulfillment: %v", err)
	}

	delivery, err := docs.Create("delivery_order", "org_default", "loc_main", "user_admin", map[string]any{
		"delivery_date": time.Now().UTC().Format("2006-01-02"),
		"lines": []map[string]any{{
			"item_code":       "MED-LOT",
			"warehouse_code":  "MAIN",
			"batch_code":      "LOT-RECALL-01",
			"expiration_date": time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02"),
			"quantity":        2.0,
		}},
	})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	delivery.Header.Status = "delivered"
	delivery.Header.Number = "DO-RECALL-001"
	if err := docs.Save(delivery); err != nil {
		t.Fatalf("save delivery: %v", err)
	}

	mustAddDocumentLink(t, docs, fulfillment.Header.ID, delivery.Header.ID, "delivery_for")
	mustAddDocumentLink(t, docs, delivery.Header.ID, fulfillment.Header.ID, "delivery_for")

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "MED-LOT",
		"warehouse_code":     "MAIN",
		"batch_code":         "LOT-RECALL-01",
		"expiration_date":    time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02"),
		"quantity_delta":     -2.0,
		"movement_reason":    "fulfillment_issue",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "out",
		"source_document_id": fulfillment.Header.ID,
	})

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	traceSvc := NewTraceabilityCoreService(docs, models, inventorySvc)
	recallSvc := NewRecallCoreService(docs, models, nil, inventorySvc, traceSvc)

	recallCase, err := docs.Create("recall_case", "org_default", "loc_main", "user_admin", map[string]any{
		"title":            "Recall MED-LOT",
		"reason":           "Contamination risk",
		"recall_reference": "RC-001",
		"affected_batches": []map[string]any{{"batch_id": batch.ID}},
	})
	if err != nil {
		t.Fatalf("create recall case: %v", err)
	}
	recallCase.Body.Payload = recallSvc.NormalizePayload("recall_case", recallCase.Body.Payload)
	recallCase.Header.Status = "active"
	recallCase.Header.Number = "RC-001"
	if err := docs.Save(recallCase); err != nil {
		t.Fatalf("save recall case: %v", err)
	}
	if err := recallSvc.ValidateApprove(recallCase); err != nil {
		t.Fatalf("validate recall case: %v", err)
	}
	if err := recallSvc.HandleApprovedDocument(recallCase, "user_admin"); err != nil {
		t.Fatalf("approve recall case: %v", err)
	}

	updatedBatch, err := models.Get("inventory_batch", batch.ID)
	if err != nil {
		t.Fatalf("reload batch: %v", err)
	}
	if got := textValue(updatedBatch.Values["status"]); got != "recalled" {
		t.Fatalf("expected recalled batch status, got %s", got)
	}

	updatedCase, err := docs.Get(recallCase.Header.ID)
	if err != nil {
		t.Fatalf("reload recall case: %v", err)
	}
	if got := numberValue(updatedCase.Body.Payload["generated_action_count"]); got < 2 {
		t.Fatalf("expected at least 2 generated actions, got %v", got)
	}
	impact := normalizeGenericMap(updatedCase.Body.Payload["impact_summary"])
	if got := numberValue(impact["delivered_quantity"]); got != 2.0 {
		t.Fatalf("expected delivered quantity 2, got %v", got)
	}
	if got := int(numberValue(impact["affected_delivery_count"])); got != 1 {
		t.Fatalf("expected 1 affected delivery, got %d", got)
	}

	actions := 0
	hasWarehouseHold := false
	hasDeliveryReview := false
	for _, record := range docs.List() {
		if record.Header.Type != "recall_action" {
			continue
		}
		if textValue(record.Body.Payload["source_recall_case_id"]) != recallCase.Header.ID {
			continue
		}
		actions++
		switch textValue(record.Body.Payload["action_type"]) {
		case "warehouse_hold":
			hasWarehouseHold = true
		case "delivery_review":
			hasDeliveryReview = true
		}
		if record.Header.Status != "submitted" {
			t.Fatalf("expected generated recall action to be submitted, got %s", record.Header.Status)
		}
	}
	if actions < 2 {
		t.Fatalf("expected at least 2 recall action documents, got %d", actions)
	}
	if !hasWarehouseHold || !hasDeliveryReview {
		t.Fatalf("expected warehouse_hold and delivery_review actions, got warehouse_hold=%v delivery_review=%v", hasWarehouseHold, hasDeliveryReview)
	}
}

func mustRegisterTraceabilityRecallTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "goods_receipt", DisplayName: "Goods Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "sales_fulfillment", DisplayName: "Sales Fulfillment", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for", "delivery_for"}},
		{Type: "delivery_order", DisplayName: "Delivery Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"delivery_for", "related_to"}},
		{Type: "production_order", DisplayName: "Production Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"production_for"}},
		{Type: "production_issue", DisplayName: "Production Issue", SchemaVersion: "v1", AllowedLinkTypes: []string{"production_for", "movement_for"}},
		{Type: "production_output", DisplayName: "Production Output", SchemaVersion: "v1", AllowedLinkTypes: []string{"production_for", "movement_for"}},
		{Type: "recall_case", DisplayName: "Recall Case", SchemaVersion: "v1", AllowedLinkTypes: []string{"recall_for", "related_to"}},
		{Type: "recall_action", DisplayName: "Recall Action", SchemaVersion: "v1", AllowedLinkTypes: []string{"recall_for", "related_to"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustAddDocumentLink(t *testing.T, docs *document.Service, fromID, toID, linkType string) {
	t.Helper()
	if _, err := docs.AddLink(fromID, toID, linkType, map[string]any{}); err != nil {
		t.Fatalf("add link %s -> %s (%s): %v", fromID, toID, linkType, err)
	}
}
