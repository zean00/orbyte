package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
)

func TestProductionCostingPostgresActualVarianceAndAllocation(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed production costing test")
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

	orgID := "org_default"
	locID := "loc_hq"
	suffix := time.Now().UTC().Format("20060102150405")
	actorID := "user_admin"
	ensureCommercialUOMRecord(t, graph.models, actorID, "EA")
	ensureFinanceAccountRecord(t, graph.models, actorID, "1200-FG-"+suffix, "Finished Goods "+suffix, "asset", "inventory", "debit")
	ensureFinanceAccountRecord(t, graph.models, actorID, "1201-BY-"+suffix, "Byproduct "+suffix, "asset", "inventory", "debit")
	ensureFinanceAccountRecord(t, graph.models, actorID, "1200-RM-"+suffix, "Raw Materials "+suffix, "asset", "inventory", "debit")
	ensureFinanceAccountRecord(t, graph.models, actorID, "1300-WIP-"+suffix, "Work In Progress "+suffix, "asset", "inventory", "debit")
	ensureWarehouseRecord(t, graph.models, actorID, "MAIN", orgID, locID)
	ensureProductionWorkCenterRecord(t, graph.models, actorID, "CUT")
	ensureProductionWorkCenterRecord(t, graph.models, actorID, "COOK")
	for _, item := range []map[string]any{
		{"sku": "FGP-" + suffix, "name": "Finished Good " + suffix, "kind": "stocked", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1200-FG-" + suffix, "wip_account_code": "1300-WIP-" + suffix},
		{"sku": "BYP-" + suffix, "name": "Byproduct " + suffix, "kind": "stocked", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1201-BY-" + suffix, "wip_account_code": "1300-WIP-" + suffix},
		{"sku": "RMA-" + suffix, "name": "Raw A " + suffix, "kind": "stocked", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1200-RM-" + suffix, "wip_account_code": "1300-WIP-" + suffix},
		{"sku": "RMB-" + suffix, "name": "Raw B " + suffix, "kind": "stocked", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1200-RM-" + suffix, "wip_account_code": "1300-WIP-" + suffix},
	} {
		if _, err := graph.models.Create("commercial_item", actorID, item); err != nil {
			t.Fatalf("create item %s: %v", item["sku"], err)
		}
	}
	now := "2099-10-31"
	for _, movement := range []map[string]any{
		{"item_code": "RMA-" + suffix, "warehouse_code": "MAIN", "quantity_delta": 10.0, "movement_reason": "seed", "movement_date": now, "movement_direction": "in", "unit_cost": 5.0, "total_cost": 50.0},
		{"item_code": "RMB-" + suffix, "warehouse_code": "MAIN", "quantity_delta": 10.0, "movement_reason": "seed", "movement_date": now, "movement_direction": "in", "unit_cost": 15.0, "total_cost": 150.0},
	} {
		record, err := graph.documents.Create("stock_movement", orgID, locID, actorID, movement)
		if err != nil {
			t.Fatalf("create seed movement: %v", err)
		}
		record.Header.Status = "posted"
		if err := graph.documents.Save(record); err != nil {
			t.Fatalf("save seed movement: %v", err)
		}
	}
	for _, snapshot := range []map[string]any{
		{"organization_id": orgID, "location_id": locID, "item_code": "RMA-" + suffix, "warehouse_code": "MAIN", "quantity_on_hand": 10.0, "average_unit_cost": 5.0, "inventory_value": 50.0},
		{"organization_id": orgID, "location_id": locID, "item_code": "RMB-" + suffix, "warehouse_code": "MAIN", "quantity_on_hand": 10.0, "average_unit_cost": 15.0, "inventory_value": 150.0},
	} {
		if _, err := graph.models.Create("inventory_valuation_snapshot", actorID, snapshot); err != nil {
			t.Fatalf("create snapshot: %v", err)
		}
	}
	routing, err := graph.models.Create("production_routing", actorID, map[string]any{
		"organization_id":    orgID,
		"location_id":        locID,
		"code":               "ROUTE-" + suffix,
		"name":               "Route " + suffix,
		"produced_item_code": "FGP-" + suffix,
		"status":             "active",
	})
	if err != nil {
		t.Fatalf("create routing: %v", err)
	}
	for _, step := range []map[string]any{
		{"organization_id": orgID, "location_id": locID, "routing_id": routing.ID, "sequence": 1, "work_center_code": "CUT", "cost_driver": "labor", "standard_quantity": 1.0, "standard_rate": 10.0, "status": "active"},
		{"organization_id": orgID, "location_id": locID, "routing_id": routing.ID, "sequence": 2, "work_center_code": "COOK", "cost_driver": "overhead", "standard_quantity": 1.0, "standard_rate": 5.0, "status": "active"},
	} {
		if _, err := graph.models.Create("production_routing_step", actorID, step); err != nil {
			t.Fatalf("create routing step: %v", err)
		}
	}

	order, err := graph.documents.Create("production_order", orgID, locID, actorID, map[string]any{
		"finished_item_code":       "FGP-" + suffix,
		"finished_item_name":       "Finished Good " + suffix,
		"warehouse_code":           "MAIN",
		"planned_quantity":         2.0,
		"expected_output_quantity": 2.0,
		"lines": []map[string]any{
			{"component_item_code": "RMA-" + suffix, "actual_item_code": "RMA-" + suffix, "warehouse_code": "MAIN", "uom_code": "EA", "quantity_per_unit": 1.0, "quantity": 2.0},
			{"component_item_code": "RMB-" + suffix, "actual_item_code": "RMB-" + suffix, "warehouse_code": "MAIN", "uom_code": "EA", "quantity_per_unit": 1.0, "quantity": 2.0},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	order.Body.Payload = graph.productionCore.NormalizePayload("production_order", order.Body.Payload)
	order.Header.Status = "approved"
	if err := graph.documents.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}
	if err := graph.productionCore.HandleApprovedDocument(order, actorID); err != nil {
		t.Fatalf("approve order: %v", err)
	}

	for _, capture := range []map[string]any{
		{"organization_id": orgID, "location_id": locID, "production_order_id": order.Header.ID, "capture_type": "labor", "capture_date": now, "quantity": 2.0, "actual_rate": 12.5, "status": "approved"},
		{"organization_id": orgID, "location_id": locID, "production_order_id": order.Header.ID, "capture_type": "overhead", "capture_date": now, "quantity": 2.0, "actual_rate": 5.0, "status": "approved"},
	} {
		if _, err := graph.models.Create("production_cost_capture", actorID, capture); err != nil {
			t.Fatalf("create capture: %v", err)
		}
	}

	issue, err := graph.productionCore.CreateProductionIssueFromOrder(order.Header.ID, actorID)
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	issue.Header.Status = "issued"
	if err := graph.documents.Save(issue); err != nil {
		t.Fatalf("save issue: %v", err)
	}
	if err := graph.productionCore.HandleApprovedDocument(issue, actorID); err != nil {
		t.Fatalf("approve issue: %v", err)
	}

	output, err := graph.productionCore.CreateProductionOutputFromOrder(order.Header.ID, actorID)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	output.Body.Payload["production_lot_code"] = "LOT-" + suffix
	output.Body.Payload["output_quantity"] = 2.0
	output.Body.Payload["output_allocations"] = []map[string]any{
		{"output_item_code": "FGP-" + suffix, "output_item_name": "Finished Good " + suffix, "warehouse_code": "MAIN", "output_quantity": 2.0, "allocation_share_percent": 80.0},
		{"output_item_code": "BYP-" + suffix, "output_item_name": "Byproduct " + suffix, "warehouse_code": "MAIN", "output_quantity": 1.0, "allocation_share_percent": 20.0},
	}
	output.Body.Payload["stages"] = []map[string]any{}
	output.Body.Payload = graph.productionCore.NormalizePayload("production_output", output.Body.Payload)
	output.Header.Status = "posted"
	if err := graph.documents.Save(output); err != nil {
		t.Fatalf("save output: %v", err)
	}
	if err := graph.productionCore.HandleApprovedDocument(output, actorID); err != nil {
		t.Fatalf("approve output: %v", err)
	}

	summary := graph.productionCosting.ProductionCostSummary(orgID, locID)
	var matchedRowIndex = -1
	for i := range summary.Rows {
		if summary.Rows[i].FinishedItemCode == "FGP-"+suffix {
			matchedRowIndex = i
			break
		}
	}
	if matchedRowIndex < 0 {
		t.Fatalf("expected production summary row for %s", "FGP-"+suffix)
	}
	if got := summary.Rows[matchedRowIndex].ActualTotalCost; got != 75.0 {
		t.Fatalf("expected actual total cost 75, got %v", got)
	}
	if got := summary.Rows[matchedRowIndex].TotalVarianceAmount; got != 5.0 {
		t.Fatalf("expected total variance 5, got %v", got)
	}

	allocations, _, err := graph.models.List("production_output_allocation", model.Query{
		Filters: map[string]string{"source_production_output_id": output.Header.ID},
		Page:    1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list allocations: %v", err)
	}
	if len(allocations) != 2 {
		t.Fatalf("expected 2 output allocations, got %d", len(allocations))
	}
	bySnapshots, _, err := graph.models.List("inventory_valuation_snapshot", model.Query{
		Filters: map[string]string{"organization_id": orgID, "location_id": locID, "item_code": "BYP-" + suffix, "warehouse_code": "MAIN"},
		Page:    1, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list byproduct snapshot: %v", err)
	}
	if len(bySnapshots) != 1 || testNumberValue(bySnapshots[0].Values["inventory_value"]) != 15.0 {
		t.Fatalf("expected byproduct inventory value 15, got %+v", bySnapshots)
	}
}
