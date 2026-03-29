package application

import (
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestProposeFulfillmentLinesSkipsExpiredBatchesAndUsesFEFO(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                     "MILK-1L",
		"name":                    "Milk 1L",
		"inventory_enabled":       true,
		"inventory_tracking_mode": "batch",
		"expiry_tracking_enabled": true,
		"allow_negative_stock":    false,
		"default_issue_strategy":  "fefo",
		"uom_code":                "EA",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	today := time.Now().UTC()
	expired := today.AddDate(0, 0, -1).Format("2006-01-02")
	soon := today.AddDate(0, 0, 5).Format("2006-01-02")
	later := today.AddDate(0, 0, 20).Format("2006-01-02")

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "MILK-1L",
		"warehouse_code":     "MAIN",
		"batch_code":         "EXP-OLD",
		"expiration_date":    expired,
		"quantity_delta":     10.0,
		"movement_reason":    "seed",
		"movement_date":      today.Format("2006-01-02"),
		"movement_direction": "in",
	})
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "MILK-1L",
		"warehouse_code":     "MAIN",
		"batch_code":         "FRESH-SOON",
		"expiration_date":    soon,
		"quantity_delta":     4.0,
		"movement_reason":    "seed",
		"movement_date":      today.Format("2006-01-02"),
		"movement_direction": "in",
	})
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "MILK-1L",
		"warehouse_code":     "MAIN",
		"batch_code":         "FRESH-LATE",
		"expiration_date":    later,
		"quantity_delta":     4.0,
		"movement_reason":    "seed",
		"movement_date":      today.Format("2006-01-02"),
		"movement_direction": "in",
	})

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	lines, err := service.ProposeFulfillmentLines("org_default", "loc_main", []map[string]any{{
		"item_code": "MILK-1L",
		"quantity":  6.0,
	}}, "")
	if err != nil {
		t.Fatalf("propose fulfillment lines: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 FEFO lines, got %d", len(lines))
	}
	if got := textValue(lines[0]["batch_code"]); got != "FRESH-SOON" {
		t.Fatalf("expected first batch FRESH-SOON, got %s", got)
	}
	if got := textValue(lines[1]["batch_code"]); got != "FRESH-LATE" {
		t.Fatalf("expected second batch FRESH-LATE, got %s", got)
	}
	if got := numberValue(lines[0]["quantity"]); got != 4.0 {
		t.Fatalf("expected first allocation 4, got %v", got)
	}
	if got := numberValue(lines[1]["quantity"]); got != 2.0 {
		t.Fatalf("expected second allocation 2, got %v", got)
	}
	for _, line := range lines {
		if textValue(line["batch_code"]) == "EXP-OLD" {
			t.Fatalf("expected expired batch to be excluded from FEFO proposal")
		}
		if exp := textValue(line["expiration_date"]); exp < today.Format("2006-01-02") {
			t.Fatalf("expected proposed batch to be non-expired, got %s", exp)
		}
	}
}

func TestValidateFulfillmentIssueRejectsExpiredBatch(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                     "YOGURT-PLAIN",
		"name":                    "Yogurt Plain",
		"inventory_enabled":       true,
		"inventory_tracking_mode": "batch",
		"expiry_tracking_enabled": true,
		"allow_negative_stock":    false,
		"default_issue_strategy":  "fefo",
		"uom_code":                "EA",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	today := time.Now().UTC()
	expired := today.AddDate(0, 0, -2).Format("2006-01-02")
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "YOGURT-PLAIN",
		"warehouse_code":     "MAIN",
		"batch_code":         "OLD-BATCH",
		"expiration_date":    expired,
		"quantity_delta":     5.0,
		"movement_reason":    "seed",
		"movement_date":      today.Format("2006-01-02"),
		"movement_direction": "in",
	})

	record, err := docs.Create("sales_fulfillment", "org_default", "loc_main", "user_admin", map[string]any{
		"lines": []map[string]any{{
			"item_code":        "YOGURT-PLAIN",
			"warehouse_code":   "MAIN",
			"batch_code":       "OLD-BATCH",
			"expiration_date":  expired,
			"quantity":         1.0,
			"ordered_quantity": 1.0,
		}},
	})
	if err != nil {
		t.Fatalf("create fulfillment: %v", err)
	}

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	err = service.ValidateFulfillmentIssue(record)
	if err == nil {
		t.Fatal("expected expired batch validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "expired batch") {
		t.Fatalf("expected expired batch error, got %v", err)
	}
}

func TestValidateFulfillmentIssueRejectsBlockedBatch(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                     "MED-001",
		"name":                    "Test Medicine",
		"inventory_enabled":       true,
		"inventory_tracking_mode": "batch",
		"expiry_tracking_enabled": true,
		"allow_negative_stock":    false,
		"default_issue_strategy":  "fefo",
		"uom_code":                "EA",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("inventory_batch", "user_admin", map[string]any{
		"item_code":        "MED-001",
		"warehouse_code":   "MAIN",
		"batch_code":       "BLOCKED-01",
		"expiration_date":  time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02"),
		"status":           "blocked",
		"hold_reason":      "qa_hold",
		"hold_notes":       "Do not issue",
		"recall_reference": "",
	}); err != nil {
		t.Fatalf("create batch: %v", err)
	}

	record, err := docs.Create("sales_fulfillment", "org_default", "loc_main", "user_admin", map[string]any{
		"lines": []map[string]any{{
			"item_code":        "MED-001",
			"warehouse_code":   "MAIN",
			"batch_code":       "BLOCKED-01",
			"expiration_date":  time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02"),
			"quantity":         1.0,
			"ordered_quantity": 1.0,
		}},
	})
	if err != nil {
		t.Fatalf("create fulfillment: %v", err)
	}

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	err = service.ValidateFulfillmentIssue(record)
	if err == nil {
		t.Fatal("expected blocked batch validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not issuable") {
		t.Fatalf("expected blocked batch error, got %v", err)
	}
}

func TestDecorateBatchRecordReflectsEffectiveStatusAndAvailability(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	record, err := models.Create("inventory_batch", "user_admin", map[string]any{
		"item_code":       "MILK-1L",
		"warehouse_code":  "MAIN",
		"batch_code":      "NEAR-01",
		"expiration_date": time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02"),
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "MILK-1L",
		"warehouse_code":     "MAIN",
		"batch_code":         "NEAR-01",
		"expiration_date":    time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02"),
		"quantity_delta":     7.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	decorated := service.DecorateBatchRecord(record, "org_default", "loc_main", time.Now().UTC())
	if got := textValue(decorated.Values["status"]); got != "near_expiry" {
		t.Fatalf("expected near_expiry, got %s", got)
	}
	if !boolValue(decorated.Values["is_issuable"]) {
		t.Fatal("expected near-expiry batch to remain issuable")
	}
	if got := numberValue(decorated.Values["on_hand_quantity"]); got != 7.0 {
		t.Fatalf("expected on hand 7, got %v", got)
	}
}

func mustRegisterInventoryTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_fulfillment", DisplayName: "Sales Fulfillment", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterInventoryTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{
			Key:         "commercial_item",
			DisplayName: "Commercial Item",
			DefaultSort: "sku",
			Fields: []model.FieldDefinition{
				{Key: "sku", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "inventory_enabled", Type: "bool"},
				{Key: "inventory_tracking_mode", Type: "string"},
				{Key: "expiry_tracking_enabled", Type: "bool"},
				{Key: "allow_negative_stock", Type: "bool"},
				{Key: "default_issue_strategy", Type: "string"},
				{Key: "uom_code", Type: "string"},
			},
		},
		{
			Key:         "inventory_batch",
			DisplayName: "Inventory Batch",
			DefaultSort: "batch_code",
			Fields: []model.FieldDefinition{
				{Key: "item_code", Type: "string", Required: true},
				{Key: "warehouse_code", Type: "string", Required: true},
				{Key: "batch_code", Type: "string", Required: true},
				{Key: "expiration_date", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "hold_reason", Type: "string"},
				{Key: "hold_notes", Type: "string"},
				{Key: "recall_reference", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func seedPostedMovement(t *testing.T, docs *document.Service, organizationID, locationID string, payload map[string]any) {
	t.Helper()
	record, err := docs.Create("stock_movement", organizationID, locationID, "system", payload)
	if err != nil {
		t.Fatalf("create stock movement: %v", err)
	}
	record.Header.Status = "posted"
	record.Header.UpdatedAt = time.Now().UTC()
	if err := docs.Save(record); err != nil {
		t.Fatalf("save stock movement: %v", err)
	}
}
