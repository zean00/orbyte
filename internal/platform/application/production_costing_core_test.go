package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestProductionCostingSyncAndMultiOutputAllocation(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)
	mustRegisterProductionCostingTestModels(t, models)

	for _, item := range []map[string]any{
		{"sku": "FG-MAIN", "name": "Main Output", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1200-FG", "wip_account_code": "1300-WIP"},
		{"sku": "FG-BY", "name": "Byproduct", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1201-BY", "wip_account_code": "1300-WIP"},
		{"sku": "RM-A", "name": "Raw A", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1200-RM", "wip_account_code": "1300-WIP"},
		{"sku": "RM-B", "name": "Raw B", "uom_code": "EA", "inventory_enabled": true, "inventory_asset_account_code": "1200-RM", "wip_account_code": "1300-WIP"},
	} {
		if _, err := models.Create("commercial_item", "user_admin", item); err != nil {
			t.Fatalf("create item %s: %v", item["sku"], err)
		}
	}
	now := time.Now().UTC().Format("2006-01-02")
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{"item_code": "RM-A", "warehouse_code": "MAIN", "quantity_delta": 10.0, "movement_reason": "seed", "movement_date": now, "movement_direction": "in", "unit_cost": 5.0, "total_cost": 50.0})
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{"item_code": "RM-B", "warehouse_code": "MAIN", "quantity_delta": 10.0, "movement_reason": "seed", "movement_date": now, "movement_direction": "in", "unit_cost": 15.0, "total_cost": 150.0})
	for _, snapshot := range []map[string]any{
		{"organization_id": "org_default", "location_id": "loc_main", "item_code": "RM-A", "warehouse_code": "MAIN", "quantity_on_hand": 10.0, "average_unit_cost": 5.0, "inventory_value": 50.0},
		{"organization_id": "org_default", "location_id": "loc_main", "item_code": "RM-B", "warehouse_code": "MAIN", "quantity_on_hand": 10.0, "average_unit_cost": 15.0, "inventory_value": 150.0},
	} {
		if _, err := models.Create("inventory_valuation_snapshot", "user_admin", snapshot); err != nil {
			t.Fatalf("create valuation snapshot: %v", err)
		}
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	financeSvc := NewFinanceReportingCoreService(docs, models, nil)
	productionSvc := NewProductionCoreService(docs, models, nil, inventorySvc)
	costingSvc := NewProductionCostingCoreService(docs, models, inventorySvc, financeSvc)
	productionSvc.SetFinanceReporting(financeSvc)
	productionSvc.SetCosting(costingSvc)

	routing, err := models.Create("production_routing", "user_admin", map[string]any{
		"organization_id":    "org_default",
		"location_id":        "loc_main",
		"code":               "ROUTE-1",
		"name":               "Main Route",
		"produced_item_code": "FG-MAIN",
		"status":             "active",
	})
	if err != nil {
		t.Fatalf("create routing: %v", err)
	}
	for _, step := range []map[string]any{
		{"organization_id": "org_default", "location_id": "loc_main", "routing_id": routing.ID, "sequence": 1, "work_center_code": "CUT", "cost_driver": "labor", "standard_quantity": 1.0, "standard_rate": 10.0, "status": "active"},
		{"organization_id": "org_default", "location_id": "loc_main", "routing_id": routing.ID, "sequence": 2, "work_center_code": "COOK", "cost_driver": "overhead", "standard_quantity": 1.0, "standard_rate": 5.0, "status": "active"},
	} {
		if _, err := models.Create("production_routing_step", "user_admin", step); err != nil {
			t.Fatalf("create routing step: %v", err)
		}
	}

	order, err := docs.Create("production_order", "org_default", "loc_main", "user_admin", map[string]any{
		"finished_item_code":       "FG-MAIN",
		"finished_item_name":       "Main Output",
		"warehouse_code":           "MAIN",
		"planned_quantity":         2.0,
		"expected_output_quantity": 2.0,
		"lines": []map[string]any{
			{"component_item_code": "RM-A", "actual_item_code": "RM-A", "warehouse_code": "MAIN", "uom_code": "EA", "quantity_per_unit": 1.0, "quantity": 2.0},
			{"component_item_code": "RM-B", "actual_item_code": "RM-B", "warehouse_code": "MAIN", "uom_code": "EA", "quantity_per_unit": 1.0, "quantity": 2.0},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	order.Body.Payload = productionSvc.NormalizePayload("production_order", order.Body.Payload)
	order.Header.Status = "approved"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(order, "user_admin"); err != nil {
		t.Fatalf("approve order: %v", err)
	}

	approvedOrder, _ := docs.Get(order.Header.ID)
	if got := numberValue(approvedOrder.Body.Payload["standard_total_cost"]); got != 70.0 {
		t.Fatalf("expected standard total cost 70, got %v", got)
	}

	for _, capture := range []map[string]any{
		{"organization_id": "org_default", "location_id": "loc_main", "production_order_id": order.Header.ID, "capture_type": "labor", "capture_date": "2099-10-31", "quantity": 2.0, "actual_rate": 12.5, "status": "approved"},
		{"organization_id": "org_default", "location_id": "loc_main", "production_order_id": order.Header.ID, "capture_type": "overhead", "capture_date": "2099-10-31", "quantity": 2.0, "actual_rate": 5.0, "status": "approved"},
	} {
		if _, err := models.Create("production_cost_capture", "user_admin", capture); err != nil {
			t.Fatalf("create capture: %v", err)
		}
	}

	issue, err := productionSvc.CreateProductionIssueFromOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	issue.Header.Status = "issued"
	if err := docs.Save(issue); err != nil {
		t.Fatalf("save issue: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(issue, "user_admin"); err != nil {
		t.Fatalf("approve issue: %v", err)
	}
	issuedOrder, err := docs.Get(order.Header.ID)
	if err != nil {
		t.Fatalf("reload order after issue: %v", err)
	}
	if got := numberValue(issuedOrder.Body.Payload["actual_material_cost_total"]); got != 40.0 {
		t.Fatalf("expected actual material cost 40 after issue, got %v", got)
	}
	if got := numberValue(issuedOrder.Body.Payload["actual_total_cost"]); got != 75.0 {
		t.Fatalf("expected actual total cost 75 after issue, got %v", got)
	}

	output, err := productionSvc.CreateProductionOutputFromOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	output.Body.Payload["production_lot_code"] = "LOT-ALLOC-1"
	output.Body.Payload["output_quantity"] = 1.5
	output.Body.Payload["output_allocations"] = []map[string]any{
		{"output_item_code": "FG-BY", "output_item_name": "Byproduct", "warehouse_code": "MAIN", "output_quantity": 0.5, "allocation_share_percent": 20.0},
		{"output_item_code": "FG-MAIN", "output_item_name": "Main Output", "warehouse_code": "MAIN", "output_quantity": 1.0, "allocation_share_percent": 80.0},
	}
	output.Body.Payload["stages"] = []map[string]any{}
	output.Body.Payload = productionSvc.NormalizePayload("production_output", output.Body.Payload)
	output.Header.Status = "posted"
	if err := docs.Save(output); err != nil {
		t.Fatalf("save output: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(output, "user_admin"); err != nil {
		t.Fatalf("approve output: %v", err)
	}

	postedOutput, _ := docs.Get(output.Header.ID)
	if got := numberValue(postedOutput.Body.Payload["total_production_cost"]); got != 75.0 {
		t.Fatalf("expected total production cost 75, got %v", got)
	}
	if got := textValue(postedOutput.Body.Payload["finished_item_code"]); got != "FG-MAIN" {
		t.Fatalf("expected output finished item FG-MAIN, got %s", got)
	}
	if got := numberValue(postedOutput.Body.Payload["output_unit_cost"]); got != 60.0 {
		t.Fatalf("expected main output unit cost 60, got %v", got)
	}
	allocations := recordList(postedOutput.Body.Payload["output_allocations"])
	if len(allocations) != 2 {
		t.Fatalf("expected 2 output allocations, got %d", len(allocations))
	}
	if got := numberValue(allocations[0]["allocated_total_cost"]); got != 15.0 {
		t.Fatalf("expected byproduct allocation 15, got %v", got)
	}
	if got := numberValue(allocations[1]["allocated_total_cost"]); got != 60.0 {
		t.Fatalf("expected main allocation 60, got %v", got)
	}

	allocationRows, _, err := models.List("production_output_allocation", model.Query{
		Filters: map[string]string{"source_production_output_id": output.Header.ID},
		Page:    1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list output allocations: %v", err)
	}
	if len(allocationRows) != 2 {
		t.Fatalf("expected 2 allocation rows, got %d", len(allocationRows))
	}

	mainSnapshot := mustSingleSnapshot(t, models, "FG-MAIN")
	if got := numberValue(mainSnapshot.Values["inventory_value"]); got != 60.0 {
		t.Fatalf("expected FG-MAIN inventory value 60, got %v", got)
	}
	bySnapshot := mustSingleSnapshot(t, models, "FG-BY")
	if got := numberValue(bySnapshot.Values["inventory_value"]); got != 15.0 {
		t.Fatalf("expected FG-BY inventory value 15, got %v", got)
	}
	completedOrder, err := docs.Get(order.Header.ID)
	if err != nil {
		t.Fatalf("reload order after output: %v", err)
	}
	if got := numberValue(completedOrder.Body.Payload["actual_output_quantity"]); got != 1.5 {
		t.Fatalf("expected order actual output quantity 1.5, got %v", got)
	}
	if got := numberValue(completedOrder.Body.Payload["unit_actual_cost"]); got != 50.0 {
		t.Fatalf("expected order unit actual cost 50, got %v", got)
	}
	if got := numberValue(completedOrder.Body.Payload["yield_variance_amount"]); got != 22.5 {
		t.Fatalf("expected order yield variance 22.5, got %v", got)
	}

	report := costingSvc.ProductionVarianceReport("org_default", "loc_main")
	if len(report.Rows) != 1 {
		t.Fatalf("expected 1 variance row, got %d", len(report.Rows))
	}
	if got := report.Rows[0].ActualOutputQuantity; got != 1.5 {
		t.Fatalf("expected actual output quantity 1.5, got %v", got)
	}
	if got := report.Rows[0].UnitActualCost; got != 50.0 {
		t.Fatalf("expected unit actual cost 50, got %v", got)
	}
	if got := report.Rows[0].TotalVarianceAmount; got != 5.0 {
		t.Fatalf("expected total variance 5, got %v", got)
	}
	if got := report.Rows[0].LaborVarianceAmount; got != 5.0 {
		t.Fatalf("expected labor variance 5, got %v", got)
	}
	if got := report.Rows[0].YieldVarianceAmount; got != 22.5 {
		t.Fatalf("expected yield variance 22.5, got %v", got)
	}
	varianceCases, _, err := models.List("production_variance_case", model.Query{
		Filters: map[string]string{"production_order_id": order.Header.ID},
		Page:    1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list variance cases: %v", err)
	}
	if len(varianceCases) != 1 {
		t.Fatalf("expected 1 variance case, got %d", len(varianceCases))
	}
}

func mustRegisterProductionCostingTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{Key: "production_routing", DisplayName: "Production Routing", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "produced_item_code", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "production_routing_step", DisplayName: "Production Routing Step", DefaultSort: "sequence", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "routing_id", Type: "string"}, {Key: "sequence", Type: "int"}, {Key: "work_center_code", Type: "string"}, {Key: "cost_driver", Type: "string"}, {Key: "standard_quantity", Type: "number"}, {Key: "standard_rate", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "production_cost_rate", DisplayName: "Production Cost Rate", DefaultSort: "work_center_code", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "work_center_code", Type: "string"}, {Key: "rate_type", Type: "string"}, {Key: "standard_rate", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "production_cost_capture", DisplayName: "Production Cost Capture", DefaultSort: "capture_date", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "production_order_id", Type: "string"}, {Key: "capture_type", Type: "string"}, {Key: "capture_date", Type: "string"}, {Key: "quantity", Type: "number"}, {Key: "actual_rate", Type: "number"}, {Key: "actual_cost", Type: "number"}, {Key: "credit_account_code", Type: "string"}, {Key: "posting_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "production_variance_case", DisplayName: "Production Variance Case", DefaultSort: "production_order_id", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "production_order_id", Type: "string"}, {Key: "order_number", Type: "string"}, {Key: "finished_item_code", Type: "string"}, {Key: "variance_type", Type: "string"}, {Key: "amount", Type: "number"}, {Key: "status", Type: "string"}, {Key: "notes", Type: "string"}}},
		{Key: "production_output_allocation", DisplayName: "Production Output Allocation", DefaultSort: "output_item_code", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "source_production_output_id", Type: "string"}, {Key: "production_order_id", Type: "string"}, {Key: "output_item_code", Type: "string"}, {Key: "output_item_name", Type: "string"}, {Key: "warehouse_code", Type: "string"}, {Key: "output_quantity", Type: "number"}, {Key: "allocation_basis", Type: "string"}, {Key: "allocation_share_percent", Type: "number"}, {Key: "allocated_total_cost", Type: "number"}, {Key: "allocated_unit_cost", Type: "number"}, {Key: "output_date", Type: "string"}, {Key: "status", Type: "string"}}},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func mustSingleSnapshot(t *testing.T, models *model.Service, itemCode string) model.Record {
	t.Helper()
	items, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters: map[string]string{
			"organization_id": "org_default",
			"location_id":     "loc_main",
			"item_code":       itemCode,
			"warehouse_code":  "MAIN",
		},
		Page: 1, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list snapshot for %s: %v", itemCode, err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one snapshot for %s, got %d", itemCode, len(items))
	}
	return items[0]
}
