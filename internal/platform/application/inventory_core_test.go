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

func TestValidateStockAdjustmentRejectsUnknownWarehouse(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)
	if err := models.Register(model.Definition{
		Key:         "warehouse",
		DisplayName: "Warehouse",
		DefaultSort: "code",
		Fields: []model.FieldDefinition{
			{Key: "code", Type: "string", Required: true},
			{Key: "name", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register warehouse model: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                     "ADJ-ITEM",
		"name":                    "Adjustment Item",
		"inventory_enabled":       true,
		"inventory_tracking_mode": "quantity",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	record := document.Record{
		Header: document.Header{Type: "stock_adjustment"},
		Body: document.Body{Payload: map[string]any{
			"lines": []map[string]any{{
				"item_code":      "ADJ-ITEM",
				"warehouse_code": "UNKNOWN",
				"quantity":       2.0,
			}},
		}},
	}

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	err := service.ValidateApprove(record)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "warehouse not found") {
		t.Fatalf("expected warehouse not found validation, got %v", err)
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

func TestStockReceiptsUpdateWeightedAverageValuation(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "COFFEE",
		"name":                         "Coffee Beans",
		"inventory_enabled":            true,
		"inventory_tracking_mode":      "quantity",
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-COFFEE",
		"cogs_account_code":            "5000-COGS-COFFEE",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	for _, receipt := range []map[string]any{
		{"quantity": 10.0, "unit_cost": 100.0},
		{"quantity": 10.0, "unit_cost": 120.0},
	} {
		record, err := docs.Create("stock_receipt", "org_default", "loc_main", "user_admin", map[string]any{
			"lines": []map[string]any{{
				"item_code":      "COFFEE",
				"warehouse_code": "MAIN",
				"quantity":       receipt["quantity"],
				"unit_cost":      receipt["unit_cost"],
				"uom_code":       "EA",
			}},
		})
		if err != nil {
			t.Fatalf("create receipt: %v", err)
		}
		record.Header.Status = "received"
		if err := docs.Save(record); err != nil {
			t.Fatalf("save receipt: %v", err)
		}
		if err := service.HandleApprovedDocument(record, "user_admin"); err != nil {
			t.Fatalf("handle receipt: %v", err)
		}
	}

	items, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters:  map[string]string{"item_code": "COFFEE", "warehouse_code": "MAIN"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list valuation snapshots: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 valuation snapshot, got %d", len(items))
	}
	if got := numberValue(items[0].Values["quantity_on_hand"]); got != 20 {
		t.Fatalf("expected quantity on hand 20, got %v", got)
	}
	if got := numberValue(items[0].Values["average_unit_cost"]); got != 110 {
		t.Fatalf("expected average unit cost 110, got %v", got)
	}
	if got := numberValue(items[0].Values["inventory_value"]); got != 2200 {
		t.Fatalf("expected inventory value 2200, got %v", got)
	}
}

func TestFulfillmentIssueCreatesCOGSPostingFromAverageCost(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "TSHIRT",
		"name":                         "T-Shirt",
		"inventory_enabled":            true,
		"inventory_tracking_mode":      "quantity",
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-APP",
		"cogs_account_code":            "5000-COGS-APP",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("inventory_valuation_snapshot", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_main",
		"item_code":         "TSHIRT",
		"warehouse_code":    "MAIN",
		"quantity_on_hand":  20.0,
		"average_unit_cost": 110.0,
		"inventory_value":   2200.0,
	}); err != nil {
		t.Fatalf("create valuation snapshot: %v", err)
	}
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "TSHIRT",
		"warehouse_code":     "MAIN",
		"quantity_delta":     20.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
		"unit_cost":          110.0,
		"total_cost":         2200.0,
	})

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	record, err := docs.Create("sales_fulfillment", "org_default", "loc_main", "user_admin", map[string]any{
		"lines": []map[string]any{{
			"item_code":      "TSHIRT",
			"warehouse_code": "MAIN",
			"quantity":       5.0,
			"uom_code":       "EA",
		}},
	})
	if err != nil {
		t.Fatalf("create fulfillment: %v", err)
	}
	record.Header.Status = "issued"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save fulfillment: %v", err)
	}
	if err := service.HandleApprovedFulfillment(record, "user_admin"); err != nil {
		t.Fatalf("handle approved fulfillment: %v", err)
	}

	updated, err := docs.Get(record.Header.ID)
	if err != nil {
		t.Fatalf("reload fulfillment: %v", err)
	}
	line := recordList(updated.Body.Payload["lines"])[0]
	if got := numberValue(line["unit_cost"]); got != 110 {
		t.Fatalf("expected line unit cost 110, got %v", got)
	}
	if got := numberValue(line["total_cost"]); got != -550 {
		t.Fatalf("expected line total cost -550, got %v", got)
	}
	if got := numberValue(updated.Body.Payload["cost_amount_total"]); got != 550 {
		t.Fatalf("expected fulfillment cost total 550, got %v", got)
	}
	items, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters:  map[string]string{"organization_id": "org_default", "location_id": "loc_main", "item_code": "TSHIRT", "warehouse_code": "MAIN"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list valuation snapshots after fulfillment: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 valuation snapshot after fulfillment, got %d", len(items))
	}
	if got := numberValue(items[0].Values["quantity_on_hand"]); got != 15 {
		t.Fatalf("expected quantity on hand 15 after fulfillment, got %v", got)
	}
	if got := numberValue(items[0].Values["average_unit_cost"]); got != 110 {
		t.Fatalf("expected average unit cost 110 after fulfillment, got %v", got)
	}
	if got := numberValue(items[0].Values["inventory_value"]); got != 1650 {
		t.Fatalf("expected inventory value 1650 after fulfillment, got %v", got)
	}
	postingCount := 0
	for _, item := range docs.List() {
		if item.Header.Type != "ledger_posting" {
			continue
		}
		postingCount++
		journalLines := recordList(item.Body.Payload["journal_lines"])
		if len(journalLines) != 2 {
			t.Fatalf("expected 2 journal lines, got %d", len(journalLines))
		}
	}
	if postingCount != 1 {
		t.Fatalf("expected 1 ledger posting, got %d", postingCount)
	}
}

func TestGoodsReceiptUsesReceiptUnitPriceForValuation(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "CUFF-GR",
		"name":                         "Cuff Goods Receipt",
		"inventory_enabled":            true,
		"inventory_tracking_mode":      "quantity",
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-CUFF",
		"cogs_account_code":            "5000-COGS-CUFF",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	record, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"currency_code": "IDR",
		"lines": []map[string]any{{
			"item_code":        "CUFF-GR",
			"warehouse_code":   "MAIN",
			"receipt_qty":      15.0,
			"unit_price":       250000.0,
			"uom_code":         "EA",
			"description":      "Cuff receipt",
			"tax_rate":         11.0,
			"tax_mode":         "exclusive",
			"tax_account_code": "1300-VATIN-CUFF",
		}},
	})
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	record.Header.Status = "received"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save goods receipt: %v", err)
	}
	if err := service.HandleApprovedDocument(record, "user_admin"); err != nil {
		t.Fatalf("handle approved goods receipt: %v", err)
	}

	items, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters:  map[string]string{"item_code": "CUFF-GR", "warehouse_code": "MAIN"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list valuation snapshots: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 valuation snapshot, got %d", len(items))
	}
	if got := numberValue(items[0].Values["quantity_on_hand"]); got != 15 {
		t.Fatalf("expected quantity on hand 15, got %v", got)
	}
	if got := numberValue(items[0].Values["average_unit_cost"]); got != 250000 {
		t.Fatalf("expected average unit cost 250000, got %v", got)
	}
	if got := numberValue(items[0].Values["inventory_value"]); got != 3750000 {
		t.Fatalf("expected inventory value 3750000, got %v", got)
	}
}

func TestValuationSnapshotsAreScopedByOrganizationAndLocation(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryTestDocumentTypes(t, docs)
	mustRegisterInventoryTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "SCOPED-ITEM",
		"name":                         "Scoped Item",
		"inventory_enabled":            true,
		"inventory_tracking_mode":      "quantity",
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-SCOPED",
		"cogs_account_code":            "5000-COGS-SCOPED",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	service := NewInventoryCoreService(docs, config.NewService(), models, nil)
	for _, scope := range []struct {
		org       string
		loc       string
		warehouse string
		quantity  float64
		unitCost  float64
	}{
		{org: "org_default", loc: "loc_main", warehouse: "MAIN", quantity: 10, unitCost: 100},
		{org: "org_other", loc: "loc_branch", warehouse: "MAIN", quantity: 10, unitCost: 200},
	} {
		record, err := docs.Create("stock_receipt", scope.org, scope.loc, "user_admin", map[string]any{
			"lines": []map[string]any{{
				"item_code":      "SCOPED-ITEM",
				"warehouse_code": scope.warehouse,
				"quantity":       scope.quantity,
				"unit_cost":      scope.unitCost,
				"uom_code":       "EA",
			}},
		})
		if err != nil {
			t.Fatalf("create receipt: %v", err)
		}
		record.Header.Status = "received"
		if err := docs.Save(record); err != nil {
			t.Fatalf("save receipt: %v", err)
		}
		if err := service.HandleApprovedDocument(record, "user_admin"); err != nil {
			t.Fatalf("handle receipt: %v", err)
		}
	}

	first, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters:  map[string]string{"organization_id": "org_default", "location_id": "loc_main", "item_code": "SCOPED-ITEM", "warehouse_code": "MAIN"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list first scoped snapshot: %v", err)
	}
	second, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters:  map[string]string{"organization_id": "org_other", "location_id": "loc_branch", "item_code": "SCOPED-ITEM", "warehouse_code": "MAIN"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list second scoped snapshot: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected 1 snapshot per scope, got %d and %d", len(first), len(second))
	}
	if got := numberValue(first[0].Values["average_unit_cost"]); got != 100 {
		t.Fatalf("expected first scope avg cost 100, got %v", got)
	}
	if got := numberValue(second[0].Values["average_unit_cost"]); got != 200 {
		t.Fatalf("expected second scope avg cost 200, got %v", got)
	}
}

func mustRegisterInventoryTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_fulfillment", DisplayName: "Sales Fulfillment", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for", "posting_for"}},
		{Type: "goods_receipt", DisplayName: "Goods Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "stock_receipt", DisplayName: "Stock Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}},
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
				{Key: "inventory_asset_account_code", Type: "string"},
				{Key: "cogs_account_code", Type: "string"},
				{Key: "wip_account_code", Type: "string"},
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
		{
			Key:         "inventory_cost_layer",
			DisplayName: "Inventory Cost Layer",
			DefaultSort: "effective_at",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "quantity_delta", Type: "number"},
				{Key: "unit_cost", Type: "number"},
				{Key: "total_cost", Type: "number"},
				{Key: "effective_at", Type: "string"},
			},
		},
		{
			Key:         "inventory_valuation_snapshot",
			DisplayName: "Inventory Valuation Snapshot",
			DefaultSort: "item_code",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "quantity_on_hand", Type: "number"},
				{Key: "average_unit_cost", Type: "number"},
				{Key: "inventory_value", Type: "number"},
				{Key: "valuation_method", Type: "string"},
				{Key: "last_calculated_at", Type: "string"},
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
