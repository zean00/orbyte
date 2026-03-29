package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
)

func TestProcurementCostingPostgresLandedCostAndVariance(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed procurement costing test")
	}
	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	graph := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, nil, "bootstrap-123!"); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	orgID := "org_default"
	locID := "loc_hq"
	actorID := "user_admin"
	itemCode := "LC-ITEM-" + suffix
	if _, err := graph.models.Create("commercial_item", actorID, map[string]any{
		"sku":                          itemCode,
		"name":                         "Landed Cost Item " + suffix,
		"inventory_enabled":            true,
		"inventory_tracking_mode":      "quantity",
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-LC-" + suffix,
		"cogs_account_code":            "5000-COGS-LC-" + suffix,
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	receiptPayload := graph.procurementCore.NormalizePayload("goods_receipt", map[string]any{
		"currency_code": "IDR",
		"lines": []map[string]any{{
			"item_code":      itemCode,
			"warehouse_code": "MAIN",
			"receipt_qty":    10.0,
			"unit_price":     100.0,
			"uom_code":       "EA",
		}},
		"landed_cost_lines": []map[string]any{{
			"cost_type":        "freight",
			"description":      "Freight",
			"amount":           200.0,
			"allocation_basis": "line_value",
		}},
	})
	receipt, err := graph.documents.Create("goods_receipt", orgID, locID, actorID, receiptPayload)
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	receipt.Header.Status = "received"
	if err := graph.documents.Save(receipt); err != nil {
		t.Fatalf("save goods receipt: %v", err)
	}
	if err := graph.inventoryCore.HandleApprovedDocument(receipt, actorID); err != nil {
		t.Fatalf("approve goods receipt inventory: %v", err)
	}

	issue, err := graph.documents.Create("stock_issue", orgID, locID, actorID, map[string]any{
		"lines": []map[string]any{{
			"item_code":      itemCode,
			"warehouse_code": "MAIN",
			"quantity":       5.0,
			"uom_code":       "EA",
		}},
	})
	if err != nil {
		t.Fatalf("create stock issue: %v", err)
	}
	issue.Header.Status = "issued"
	if err := graph.documents.Save(issue); err != nil {
		t.Fatalf("save stock issue: %v", err)
	}
	if err := graph.inventoryCore.HandleApprovedDocument(issue, actorID); err != nil {
		t.Fatalf("approve stock issue: %v", err)
	}

	billPayload := graph.procurementCore.NormalizePayload("vendor_bill", map[string]any{
		"vendor_id":               "vendor-test-" + suffix,
		"vendor_name":             "Vendor " + suffix,
		"currency_code":           "IDR",
		"source_goods_receipt_id": receipt.Header.ID,
		"payable_account_code":    "2000-AP-" + suffix,
		"lines": []map[string]any{{
			"item_code":                    itemCode,
			"warehouse_code":               "MAIN",
			"quantity":                     10.0,
			"unit_price":                   130.0,
			"uom_code":                     "EA",
			"inventory_asset_account_code": "1200-INV-LC-" + suffix,
		}},
	})
	bill, err := graph.documents.Create("vendor_bill", orgID, locID, actorID, billPayload)
	if err != nil {
		t.Fatalf("create vendor bill: %v", err)
	}
	bill.Header.Status = "issued"
	if err := graph.documents.Save(bill); err != nil {
		t.Fatalf("save vendor bill: %v", err)
	}
	if err := graph.procurementCore.HandleApprovedDocument(bill, actorID); err != nil {
		t.Fatalf("approve vendor bill: %v", err)
	}

	items, _, err := graph.models.List("inventory_valuation_snapshot", model.Query{
		Filters: map[string]string{
			"organization_id": orgID,
			"location_id":     locID,
			"item_code":       itemCode,
			"warehouse_code":  "MAIN",
		},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list valuation snapshot: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 valuation snapshot, got %d", len(items))
	}
	if got := testNumberValue(items[0].Values["quantity_on_hand"]); got != 5.0 {
		t.Fatalf("expected quantity on hand 5, got %v", got)
	}
	if got := testNumberValue(items[0].Values["average_unit_cost"]); got != 130.0 {
		t.Fatalf("expected average unit cost 130, got %v", got)
	}
	if got := testNumberValue(items[0].Values["inventory_value"]); got != 650.0 {
		t.Fatalf("expected inventory value 650, got %v", got)
	}

	var inventoryDebit, varianceDebit float64
	for _, record := range graph.documents.List() {
		if record.Header.Type != "ledger_posting" {
			continue
		}
		if testTextValue(record.Body.Payload["source_document_id"]) != bill.Header.ID {
			continue
		}
		for _, line := range testRecordList(record.Body.Payload["journal_lines"]) {
			switch testTextValue(line["account_code"]) {
			case "1200-INV-LC-" + suffix:
				inventoryDebit += testNumberValue(line["debit"])
			case "5100-PPV":
				varianceDebit += testNumberValue(line["debit"])
			}
		}
	}
	if inventoryDebit != 1250.0 {
		t.Fatalf("expected inventory debit 1250, got %v", inventoryDebit)
	}
	if varianceDebit != 50.0 {
		t.Fatalf("expected ppv debit 50, got %v", varianceDebit)
	}
}

func testNumberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func testTextValue(value any) string {
	if current, ok := value.(string); ok {
		return strings.TrimSpace(current)
	}
	return ""
}

func testRecordList(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		if direct, ok := value.([]map[string]any); ok {
			return direct
		}
		return nil
	}
	rows := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if row, ok := item.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}
