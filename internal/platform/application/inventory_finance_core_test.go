package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestInventoryFinanceGenerateAdjustmentFromCountSession(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryFinanceTestDocumentTypes(t, docs)
	mustRegisterInventoryFinanceTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "COUNT-ITEM",
		"name":                         "Count Item",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-COUNT",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	session, err := models.Create("inventory_count_session", "user_admin", map[string]any{
		"organization_id":         "org_default",
		"location_id":             "loc_main",
		"session_code":            "COUNT-001",
		"warehouse_code":          "MAIN",
		"status":                  "open",
		"adjustment_account_code": "5800-INV-ADJ",
		"lines": []map[string]any{{
			"item_code":        "COUNT-ITEM",
			"warehouse_code":   "MAIN",
			"system_quantity":  10.0,
			"counted_quantity": 8.0,
			"unit_cost":        15.0,
		}},
	})
	if err != nil {
		t.Fatalf("create count session: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	financeSvc := NewFinanceReportingCoreService(docs, models, nil)
	service := NewInventoryFinanceCoreService(docs, models, inventorySvc, financeSvc)
	adjustment, err := service.GenerateAdjustmentFromCountSession(session.ID, "user_admin", "org_default", "loc_main")
	if err != nil {
		t.Fatalf("generate adjustment: %v", err)
	}
	if adjustment.Header.Type != "stock_adjustment" {
		t.Fatalf("expected stock_adjustment, got %s", adjustment.Header.Type)
	}
	if !boolValue(adjustment.Body.Payload["finance_review_required"]) {
		t.Fatal("expected finance review required")
	}
	if got := numberValue(adjustment.Body.Payload["estimated_value_impact"]); got != -30.0 {
		t.Fatalf("expected value impact -30, got %v", got)
	}
	lines := recordList(adjustment.Body.Payload["lines"])
	if len(lines) != 1 || numberValue(lines[0]["quantity"]) != -2.0 {
		t.Fatalf("expected one delta line of -2, got %+v", lines)
	}
}

func TestInventoryFinanceReconciliation(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryFinanceTestDocumentTypes(t, docs)
	mustRegisterInventoryFinanceTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "RECON-ITEM",
		"name":                         "Recon Item",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-RECON",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("finance_account", "user_admin", map[string]any{
		"code":           "1200-INV-RECON",
		"name":           "Inventory Asset",
		"account_type":   "asset",
		"report_group":   "inventory",
		"normal_balance": "debit",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create finance account: %v", err)
	}
	if _, err := models.Create("inventory_valuation_snapshot", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_main",
		"item_code":         "RECON-ITEM",
		"warehouse_code":    "MAIN",
		"quantity_on_hand":  5.0,
		"average_unit_cost": 20.0,
		"inventory_value":   100.0,
	}); err != nil {
		t.Fatalf("create valuation snapshot: %v", err)
	}
	if _, err := models.Create("inventory_cost_layer", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_main",
		"item_code":       "RECON-ITEM",
		"warehouse_code":  "MAIN",
		"quantity_delta":  5.0,
		"unit_cost":       20.0,
		"total_cost":      100.0,
		"effective_at":    "2099-08-31T00:00:00Z",
		"status":          "posted",
	}); err != nil {
		t.Fatalf("create cost layer: %v", err)
	}
	posting, err := docs.Create("ledger_posting", "org_default", "loc_main", "user_admin", map[string]any{
		"posting_date": "2099-08-31",
		"journal_lines": []map[string]any{
			{"account_code": "1200-INV-RECON", "debit": 100.0, "credit": 0.0},
			{"account_code": "5800-INV-ADJ", "debit": 0.0, "credit": 100.0},
		},
		"total_amount": 100.0,
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := docs.Save(posting); err != nil {
		t.Fatalf("save posting: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	financeSvc := NewFinanceReportingCoreService(docs, models, nil)
	service := NewInventoryFinanceCoreService(docs, models, inventorySvc, financeSvc)
	report := service.InventoryGLReconciliation("org_default", "loc_main", "2099-08-31", "")
	if report.Difference != 0 {
		t.Fatalf("expected zero difference, got %v", report.Difference)
	}

	posting2, err := docs.Create("ledger_posting", "org_default", "loc_main", "user_admin", map[string]any{
		"posting_date": "2099-08-31",
		"journal_lines": []map[string]any{
			{"account_code": "1200-INV-RECON", "debit": 10.0, "credit": 0.0},
			{"account_code": "2199-SUSPENSE", "debit": 0.0, "credit": 10.0},
		},
		"total_amount": 10.0,
	})
	if err != nil {
		t.Fatalf("create mismatch posting: %v", err)
	}
	posting2.Header.Status = "posted"
	if err := docs.Save(posting2); err != nil {
		t.Fatalf("save mismatch posting: %v", err)
	}
	report = service.InventoryGLReconciliation("org_default", "loc_main", "2099-08-31", "")
	if report.Difference != -10.0 {
		t.Fatalf("expected difference -10, got %v", report.Difference)
	}
	if len(report.Mismatches) == 0 {
		t.Fatal("expected reconciliation mismatches")
	}
}

func TestInventoryFinanceReconciliationIncludesGLOnlyInventoryAccount(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryFinanceTestDocumentTypes(t, docs)
	mustRegisterInventoryFinanceTestModels(t, models)

	if _, err := models.Create("finance_account", "user_admin", map[string]any{
		"code":           "1200-INV-GLONLY",
		"name":           "GL Only Inventory",
		"account_type":   "asset",
		"report_group":   "inventory",
		"normal_balance": "debit",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create finance account: %v", err)
	}
	posting, err := docs.Create("ledger_posting", "org_default", "loc_main", "user_admin", map[string]any{
		"posting_date": "2099-08-31",
		"journal_lines": []map[string]any{
			{"account_code": "1200-INV-GLONLY", "debit": 25.0, "credit": 0.0},
			{"account_code": "2199-SUSPENSE", "debit": 0.0, "credit": 25.0},
		},
		"total_amount": 25.0,
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := docs.Save(posting); err != nil {
		t.Fatalf("save posting: %v", err)
	}

	service := NewInventoryFinanceCoreService(docs, models, NewInventoryCoreService(docs, nil, models, nil), NewFinanceReportingCoreService(docs, models, nil))
	report := service.InventoryGLReconciliation("org_default", "loc_main", "2099-08-31", "")
	if report.Difference != -25.0 {
		t.Fatalf("expected difference -25, got %v", report.Difference)
	}
	if len(report.Mismatches) != 1 {
		t.Fatalf("expected 1 mismatch, got %d", len(report.Mismatches))
	}
	if report.Mismatches[0].AccountCode != "1200-INV-GLONLY" {
		t.Fatalf("expected GL-only account mismatch, got %+v", report.Mismatches[0])
	}
}

func TestInventoryValuationAsOfDoesNotFallBackToCurrentBalances(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryFinanceTestDocumentTypes(t, docs)
	mustRegisterInventoryFinanceTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "ASOF-ITEM",
		"name":                         "As Of Item",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-ASOF",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("inventory_valuation_snapshot", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_main",
		"item_code":         "ASOF-ITEM",
		"warehouse_code":    "MAIN",
		"quantity_on_hand":  9.0,
		"average_unit_cost": 10.0,
		"inventory_value":   90.0,
	}); err != nil {
		t.Fatalf("create valuation snapshot: %v", err)
	}

	service := NewInventoryFinanceCoreService(docs, models, NewInventoryCoreService(docs, nil, models, nil), NewFinanceReportingCoreService(docs, models, nil))
	report := service.InventoryValuation("org_default", "loc_main", "", "2000-01-01")
	if len(report.Rows) != 0 {
		t.Fatalf("expected no historical rows before first cost layer, got %+v", report.Rows)
	}
	if report.Totals["inventory_value"] != 0 || report.Totals["quantity_on_hand"] != 0 {
		t.Fatalf("expected zero historical totals, got %+v", report.Totals)
	}
}

func TestInventoryAdjustmentReviewTotalsIncludePendingCountSessions(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryFinanceTestDocumentTypes(t, docs)
	mustRegisterInventoryFinanceTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "COUNT-PENDING",
		"name":                         "Pending Count Item",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-PENDING",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("inventory_count_session", "user_admin", map[string]any{
		"organization_id":         "org_default",
		"location_id":             "loc_main",
		"count_date":              "2099-01-01",
		"session_code":            "COUNT-PENDING-001",
		"warehouse_code":          "MAIN",
		"status":                  "open",
		"adjustment_account_code": "5800-INV-ADJ",
		"lines": []map[string]any{{
			"item_code":        "COUNT-PENDING",
			"warehouse_code":   "MAIN",
			"system_quantity":  5.0,
			"counted_quantity": 3.0,
			"unit_cost":        12.0,
		}},
	}); err != nil {
		t.Fatalf("create count session: %v", err)
	}
	service := NewInventoryFinanceCoreService(docs, models, NewInventoryCoreService(docs, nil, models, nil), NewFinanceReportingCoreService(docs, models, nil))
	report := service.InventoryAdjustmentReview("org_default", "loc_main", "")
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 review item, got %d", len(report.Items))
	}
	if report.Totals["documents"] != 1 {
		t.Fatalf("expected totals documents=1, got %+v", report.Totals)
	}
	if report.Totals["quantity_delta_total"] != -2.0 {
		t.Fatalf("expected quantity delta -2, got %+v", report.Totals)
	}
	if report.Totals["estimated_value_impact"] != -24.0 {
		t.Fatalf("expected value impact -24, got %+v", report.Totals)
	}
}

func TestHandleApprovedStockAdjustmentRequiresDifferentApprover(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryFinanceTestDocumentTypes(t, docs)
	mustRegisterInventoryFinanceTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "ADJ-ITEM",
		"name":                         "Adjustment Item",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-ADJ",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	adjustment, err := docs.Create("stock_adjustment", "org_default", "loc_main", "user_admin", map[string]any{
		"finance_review_required": true,
		"adjustment_account_code": "5800-INV-ADJ",
		"lines": []map[string]any{{
			"item_code":                    "ADJ-ITEM",
			"warehouse_code":               "MAIN",
			"quantity":                     -1.0,
			"unit_cost":                    10.0,
			"inventory_asset_account_code": "1200-INV-ADJ",
			"adjustment_account_code":      "5800-INV-ADJ",
		}},
	})
	if err != nil {
		t.Fatalf("create adjustment: %v", err)
	}
	adjustment.Header.Status = "posted"
	if err := docs.Save(adjustment); err != nil {
		t.Fatalf("save adjustment: %v", err)
	}

	service := NewInventoryCoreService(docs, nil, models, nil)
	if err := service.HandleApprovedDocument(adjustment, "user_admin"); err == nil {
		t.Fatal("expected same-user approval to fail")
	}
}

func TestHandleApprovedStockAdjustmentCreatesAndReversesPosting(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterInventoryFinanceTestDocumentTypes(t, docs)
	mustRegisterInventoryFinanceTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "ADJ-POST",
		"name":                         "Adjustment Posting",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-POST",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	service := NewInventoryCoreService(docs, nil, models, nil)
	financeSvc := NewFinanceReportingCoreService(docs, models, nil)
	service.SetFinanceReporting(financeSvc)

	adjustment, err := docs.Create("stock_adjustment", "org_default", "loc_main", "creator_user", map[string]any{
		"finance_review_required": true,
		"adjustment_account_code": "5800-INV-ADJ",
		"lines": []map[string]any{{
			"item_code":                    "ADJ-POST",
			"warehouse_code":               "MAIN",
			"quantity":                     -2.0,
			"unit_cost":                    10.0,
			"inventory_asset_account_code": "1200-INV-POST",
			"adjustment_account_code":      "5800-INV-ADJ",
		}},
	})
	if err != nil {
		t.Fatalf("create adjustment: %v", err)
	}
	adjustment.Header.Status = "posted"
	adjustment.Header.Number = "ADJ-001"
	if err := docs.Save(adjustment); err != nil {
		t.Fatalf("save adjustment: %v", err)
	}

	if err := service.HandleApprovedDocument(adjustment, "approver_user"); err != nil {
		t.Fatalf("approve adjustment: %v", err)
	}
	postingCount := 0
	for _, record := range docs.List() {
		if record.Header.Type == "ledger_posting" && record.Header.Status == "posted" {
			postingCount++
		}
	}
	if postingCount != 1 {
		t.Fatalf("expected 1 posting, got %d", postingCount)
	}

	reloaded, err := docs.Get(adjustment.Header.ID)
	if err != nil {
		t.Fatalf("reload adjustment: %v", err)
	}
	reloaded.Header.Status = "cancelled"
	if err := docs.Save(reloaded); err != nil {
		t.Fatalf("save cancelled adjustment: %v", err)
	}
	if err := service.HandleCanceledDocument(reloaded, "approver_user"); err != nil {
		t.Fatalf("cancel adjustment: %v", err)
	}
	postingCount = 0
	for _, record := range docs.List() {
		if record.Header.Type == "ledger_posting" && record.Header.Status == "posted" {
			postingCount++
		}
	}
	if postingCount != 2 {
		t.Fatalf("expected reversal posting, got %d postings", postingCount)
	}
}

func mustRegisterInventoryFinanceTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "stock_adjustment", DisplayName: "Stock Adjustment", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for", "posting_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterInventoryFinanceTestModels(t *testing.T, models *model.Service) {
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
				{Key: "inventory_asset_account_code", Type: "string"},
				{Key: "cogs_account_code", Type: "string"},
				{Key: "wip_account_code", Type: "string"},
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
				{Key: "status", Type: "string"},
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
				{Key: "last_calculated_at", Type: "string"},
			},
		},
		{
			Key:         "finance_account",
			DisplayName: "Finance Account",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string"},
				{Key: "account_type", Type: "string"},
				{Key: "report_group", Type: "string"},
				{Key: "normal_balance", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "inventory_count_session",
			DisplayName: "Inventory Count Session",
			DefaultSort: "count_date",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "count_date", Type: "string"},
				{Key: "session_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "adjustment_account_code", Type: "string"},
				{Key: "lines", Type: "object"},
				{Key: "generated_adjustment_id", Type: "string"},
				{Key: "generated_adjustment_number", Type: "string"},
			},
		},
		{
			Key:         "inventory_reconciliation_case",
			DisplayName: "Inventory Reconciliation Case",
			DefaultSort: "as_of_date",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "as_of_date", Type: "string"},
				{Key: "account_code", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "inventory_value", Type: "number"},
				{Key: "gl_value", Type: "number"},
				{Key: "difference", Type: "number"},
				{Key: "note", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func init() {
	time.Local = time.UTC
}
