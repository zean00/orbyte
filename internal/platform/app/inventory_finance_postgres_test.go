package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/store"
)

func TestInventoryFinancePostgresCountAdjustmentAndReconciliation(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed inventory finance test")
	}
	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	graph := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	orgID := "org_default"
	locID := "loc_hq"
	actorID := "user_admin"
	ensureCommercialUOMRecord(t, graph.models, actorID, "EA")
	approver, err := graph.identity.CreateUser("inventory-finance-"+suffix, "Complex12!", locID, "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create approver: %v", err)
	}

	if _, err := graph.models.Create("commercial_item", actorID, map[string]any{
		"sku":                          "INVFIN-" + suffix,
		"name":                         "Inventory Finance Item " + suffix,
		"kind":                         "stocked",
		"inventory_enabled":            true,
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-" + suffix,
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := graph.models.Create("finance_account", actorID, map[string]any{
		"code":           "1200-INV-" + suffix,
		"name":           "Inventory Asset " + suffix,
		"account_type":   "asset",
		"report_group":   "inventory",
		"normal_balance": "debit",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create finance account: %v", err)
	}

	openingAdjustmentPayload := graph.inventoryCore.NormalizePayload("stock_adjustment", map[string]any{
		"warehouse_code":          "MAIN",
		"adjustment_account_code": "3100-OPENING-" + suffix,
		"adjustment_reason":       "opening_balance",
		"lines": []map[string]any{{
			"item_code":                    "INVFIN-" + suffix,
			"warehouse_code":               "MAIN",
			"quantity":                     10.0,
			"unit_cost":                    20.0,
			"inventory_asset_account_code": "1200-INV-" + suffix,
			"adjustment_account_code":      "3100-OPENING-" + suffix,
		}},
	})
	openingAdjustment, err := graph.documents.Create("stock_adjustment", orgID, locID, actorID, openingAdjustmentPayload)
	if err != nil {
		t.Fatalf("create opening adjustment: %v", err)
	}
	openingAdjustment.Header.Status = "posted"
	openingAdjustment.Header.Number = "OPEN-" + suffix
	if err := graph.documents.Save(openingAdjustment); err != nil {
		t.Fatalf("save opening adjustment: %v", err)
	}
	if err := graph.inventoryCore.HandleApprovedDocument(openingAdjustment, actorID); err != nil {
		t.Fatalf("approve opening adjustment: %v", err)
	}

	session, err := graph.models.Create("inventory_count_session", actorID, map[string]any{
		"organization_id":         orgID,
		"location_id":             locID,
		"session_code":            "COUNT-" + suffix,
		"warehouse_code":          "MAIN",
		"status":                  "open",
		"adjustment_account_code": "5800-INV-ADJ",
		"lines": []map[string]any{{
			"item_code":        "INVFIN-" + suffix,
			"warehouse_code":   "MAIN",
			"system_quantity":  10.0,
			"counted_quantity": 8.0,
			"unit_cost":        20.0,
		}},
	})
	if err != nil {
		t.Fatalf("create count session: %v", err)
	}

	adjustment, err := graph.inventoryFinance.GenerateAdjustmentFromCountSession(session.ID, actorID, orgID, locID)
	if err != nil {
		t.Fatalf("generate adjustment: %v", err)
	}
	adjustment.Header.Status = "posted"
	adjustment.Header.Number = "ADJ-" + suffix
	if err := graph.documents.Save(adjustment); err != nil {
		t.Fatalf("save adjustment: %v", err)
	}
	if err := graph.inventoryCore.HandleApprovedDocument(adjustment, approver.ID); err != nil {
		t.Fatalf("approve adjustment: %v", err)
	}

	recon := graph.inventoryFinance.InventoryGLReconciliation(orgID, locID, "2099-12-31", "1200-INV-"+suffix)
	if recon.Difference != 0 {
		t.Fatalf("expected inventory reconciliation difference 0, got %v", recon.Difference)
	}

	posting, err := graph.documents.Create("ledger_posting", orgID, locID, actorID, map[string]any{
		"posting_date": "2099-12-31",
		"journal_lines": []map[string]any{
			{"account_code": "1200-INV-" + suffix, "debit": 5.0, "credit": 0.0},
			{"account_code": "2199-SUSPENSE", "debit": 0.0, "credit": 5.0},
		},
		"total_amount": 5.0,
	})
	if err != nil {
		t.Fatalf("create mismatch posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := graph.documents.Save(posting); err != nil {
		t.Fatalf("save mismatch posting: %v", err)
	}
	recon = graph.inventoryFinance.InventoryGLReconciliation(orgID, locID, "2099-12-31", "1200-INV-"+suffix)
	if recon.Difference != -5.0 {
		t.Fatalf("expected reconciliation difference -5, got %v", recon.Difference)
	}
	caseRecord, err := graph.inventoryFinance.OpenReconciliationCase(orgID, locID, "2099-12-31", "1200-INV-"+suffix, "inventory and GL balances differ", recon.InventoryTotal, recon.GLTotal, actorID)
	if err != nil {
		t.Fatalf("open reconciliation case: %v", err)
	}
	if caseRecord.ModelKey != "inventory_reconciliation_case" {
		t.Fatalf("expected inventory_reconciliation_case, got %s", caseRecord.ModelKey)
	}
}
