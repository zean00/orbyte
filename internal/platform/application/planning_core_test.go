package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestReplenishmentSummaryUsesDemandFulfillmentInboundAndRequestCoverage(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPlanningTestDocumentTypes(t, docs)
	mustRegisterPlanningTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                                  "TSHIRT-BLACK-M",
		"name":                                 "T-Shirt Black M",
		"kind":                                 "item",
		"uom_code":                             "EA",
		"base_price":                           12.0,
		"inventory_enabled":                    true,
		"replenishment_enabled":                true,
		"replenishment_mode":                   "reorder_point_target",
		"reorder_point_quantity":               5.0,
		"target_stock_quantity":                10.0,
		"default_replenishment_warehouse_code": "MAIN",
	}); err != nil {
		t.Fatalf("create stocked item: %v", err)
	}
	if _, err := models.Create("vendor_profile", "user_admin", map[string]any{
		"vendor_name": "Supplier A",
		"status":      "active",
	}); err != nil {
		t.Fatalf("create vendor: %v", err)
	}
	vendorItems, _, err := models.List("vendor_profile", model.Query{Page: 1, PageSize: 10})
	if err != nil || len(vendorItems) == 0 {
		t.Fatalf("list vendors: %v", err)
	}
	if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
		"vendor_id":              vendorItems[0].ID,
		"vendor_name":            "Supplier A",
		"item_code":              "TSHIRT-BLACK-M",
		"preferred":              true,
		"priority":               1.0,
		"purchase_uom_code":      "EA",
		"lead_time_days":         7.0,
		"minimum_order_quantity": 4.0,
		"last_purchase_price":    11.5,
		"status":                 "active",
	}); err != nil {
		t.Fatalf("create vendor item profile: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "TSHIRT-BLACK-M",
		"warehouse_code":     "MAIN",
		"quantity_delta":     3.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})

	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"party_name": "Walk In Customer",
		"lines": []map[string]any{
			{"item_code": "TSHIRT-BLACK-M", "quantity": 8.0},
		},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	order.Header.Status = "confirmed"
	order.Header.Number = "SO-PLAN-001"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save sales order: %v", err)
	}

	fulfillment, err := docs.Create("sales_fulfillment", "org_default", "loc_main", "user_admin", map[string]any{
		"lines": []map[string]any{{
			"source_order_line_index": 0,
			"item_code":               "TSHIRT-BLACK-M",
			"warehouse_code":          "MAIN",
			"quantity":                3.0,
		}},
	})
	if err != nil {
		t.Fatalf("create fulfillment: %v", err)
	}
	if _, err := docs.AddLink(fulfillment.Header.ID, order.Header.ID, "fulfillment_for", map[string]any{"source_type": "sales_order"}); err != nil {
		t.Fatalf("link fulfillment to order: %v", err)
	}

	po, err := docs.Create("purchase_order", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_name": "Supplier A",
		"lines": []map[string]any{{
			"item_code":      "TSHIRT-BLACK-M",
			"warehouse_code": "MAIN",
			"ordered_qty":    6.0,
			"received_qty":   2.0,
			"quantity":       6.0,
		}},
	})
	if err != nil {
		t.Fatalf("create purchase order: %v", err)
	}
	po.Header.Status = "approved"
	po.Header.Number = "PO-PLAN-001"
	if err := docs.Save(po); err != nil {
		t.Fatalf("save purchase order: %v", err)
	}

	request, err := docs.Create("purchase_request", "org_default", "loc_main", "user_admin", map[string]any{
		"planning_generated":      true,
		"planning_source":         "replenishment",
		"planning_warehouse_code": "MAIN",
		"lines": []map[string]any{{
			"item_code":      "TSHIRT-BLACK-M",
			"warehouse_code": "MAIN",
			"quantity":       2.0,
		}},
	})
	if err != nil {
		t.Fatalf("create purchase request: %v", err)
	}
	request.Header.Status = "submitted"
	request.Header.Number = "PR-PLAN-001"
	if err := docs.Save(request); err != nil {
		t.Fatalf("save purchase request: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, config.NewService(), models, nil)
	fulfillmentSvc := NewFulfillmentCoreService(docs, nil, inventorySvc)
	procurementSvc := NewProcurementCoreService(docs, config.NewService(), models, nil)
	planningSvc := NewPlanningCoreService(docs, models, nil, inventorySvc, fulfillmentSvc, procurementSvc)

	summary := planningSvc.ReplenishmentSummaryScoped("org_default", "loc_main", "", "", "", "", false, false, false, time.Now().UTC())
	if summary.CandidateCount != 1 {
		t.Fatalf("expected 1 replenishment candidate, got %d", summary.CandidateCount)
	}
	row := summary.Items[0]
	if got := numberValue(row["sales_demand_quantity"]); got != 5.0 {
		t.Fatalf("expected net sales demand 5 after fulfillment offset, got %v", got)
	}
	if got := numberValue(row["inbound_quantity"]); got != 4.0 {
		t.Fatalf("expected inbound quantity 4 from open PO, got %v", got)
	}
	if got := numberValue(row["requested_quantity"]); got != 2.0 {
		t.Fatalf("expected requested quantity 2 from open PR, got %v", got)
	}
	if got := numberValue(row["suggested_request_quantity"]); got != 4.0 {
		t.Fatalf("expected suggested request quantity 4 after PR coverage, got %v", got)
	}
	if got := textValue(row["preferred_vendor_name"]); got != "Supplier A" {
		t.Fatalf("expected preferred vendor Supplier A, got %s", got)
	}
	if got := textValue(row["coverage_status"]); got != "partially_received" {
		t.Fatalf("expected partially_received coverage status, got %s", got)
	}
	if got := numberValue(row["received_quantity"]); got != 2.0 {
		t.Fatalf("expected received quantity 2, got %v", got)
	}
	if refs := recordList(row["purchase_request_refs"]); len(refs) != 1 {
		t.Fatalf("expected 1 purchase request ref, got %d", len(refs))
	}
}

func TestGeneratePurchaseRequestsGroupsByWarehouseAndDefaultsVendor(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPlanningTestDocumentTypes(t, docs)
	mustRegisterPlanningTestModels(t, models)

	for _, itemCode := range []string{"MILK-1L", "BREAD-LOAF"} {
		defaultWarehouse := "MAIN"
		if itemCode == "BREAD-LOAF" {
			defaultWarehouse = "SECONDARY"
		}
		if _, err := models.Create("commercial_item", "user_admin", map[string]any{
			"sku":                                  itemCode,
			"name":                                 itemCode,
			"kind":                                 "item",
			"uom_code":                             "EA",
			"base_price":                           15.0,
			"inventory_enabled":                    true,
			"replenishment_enabled":                true,
			"replenishment_mode":                   "reorder_point_target",
			"reorder_point_quantity":               8.0,
			"target_stock_quantity":                20.0,
			"default_replenishment_warehouse_code": defaultWarehouse,
		}); err != nil {
			t.Fatalf("create item %s: %v", itemCode, err)
		}
	}
	vendor, err := models.Create("vendor_profile", "user_admin", map[string]any{
		"vendor_name": "Supplier A",
		"status":      "active",
	})
	if err != nil {
		t.Fatalf("create vendor: %v", err)
	}
	for _, itemCode := range []string{"MILK-1L", "BREAD-LOAF"} {
		if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
			"vendor_id":              vendor.ID,
			"vendor_name":            "Supplier A",
			"item_code":              itemCode,
			"preferred":              true,
			"priority":               1.0,
			"minimum_order_quantity": 2.0,
			"status":                 "active",
		}); err != nil {
			t.Fatalf("create vendor item %s: %v", itemCode, err)
		}
	}

	procurementSvc := NewProcurementCoreService(docs, config.NewService(), models, nil)
	planningSvc := NewPlanningCoreService(docs, models, nil, nil, nil, procurementSvc)
	records, err := planningSvc.GeneratePurchaseRequests("org_default", "loc_main", "user_admin", []ReplenishmentSelection{
		{ItemCode: "MILK-1L", WarehouseCode: "MAIN", Quantity: 12.0},
		{ItemCode: "BREAD-LOAF", WarehouseCode: "SECONDARY", Quantity: 8.0},
	})
	if err != nil {
		t.Fatalf("generate purchase requests: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 purchase requests grouped by warehouse, got %d", len(records))
	}
	mainFound := false
	secondaryFound := false
	for _, record := range records {
		warehouseCode := textValue(record.Body.Payload["planning_warehouse_code"])
		switch warehouseCode {
		case "MAIN":
			mainFound = true
			if got := textValue(record.Body.Payload["vendor_id"]); got != vendor.ID {
				t.Fatalf("expected vendor %s on MAIN request, got %s", vendor.ID, got)
			}
			if lines := recordList(record.Body.Payload["lines"]); len(lines) != 1 {
				t.Fatalf("expected 1 line on MAIN request, got %d", len(lines))
			} else if qty := numberValue(lines[0]["planning_normalized_quantity"]); qty <= 0 {
				t.Fatalf("expected planning_normalized_quantity on generated line, got %v", qty)
			}
		case "SECONDARY":
			secondaryFound = true
			if got := textValue(record.Body.Payload["vendor_id"]); got != vendor.ID {
				t.Fatalf("expected vendor %s on SECONDARY request, got %s", vendor.ID, got)
			}
		}
	}
	if !mainFound || !secondaryFound {
		t.Fatalf("expected MAIN and SECONDARY requests, got %+v", planningSvc.GenerationResults(records))
	}
}

func TestReplenishmentPrefersLowestPriorityVendorAndNormalizesQuantity(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPlanningTestDocumentTypes(t, docs)
	mustRegisterPlanningTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                                  "JUICE-ORANGE",
		"name":                                 "Orange Juice",
		"kind":                                 "item",
		"uom_code":                             "EA",
		"base_price":                           10.0,
		"inventory_enabled":                    true,
		"replenishment_enabled":                true,
		"replenishment_mode":                   "reorder_point_target",
		"reorder_point_quantity":               5.0,
		"target_stock_quantity":                12.0,
		"default_replenishment_warehouse_code": "MAIN",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	vendorA, err := models.Create("vendor_profile", "user_admin", map[string]any{"vendor_name": "Supplier A", "status": "active"})
	if err != nil {
		t.Fatalf("create vendor A: %v", err)
	}
	vendorB, err := models.Create("vendor_profile", "user_admin", map[string]any{"vendor_name": "Supplier B", "status": "active"})
	if err != nil {
		t.Fatalf("create vendor B: %v", err)
	}
	if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
		"vendor_id":              vendorA.ID,
		"vendor_name":            "Supplier A",
		"item_code":              "JUICE-ORANGE",
		"priority":               5.0,
		"minimum_order_quantity": 10.0,
		"pack_size":              6.0,
		"status":                 "active",
	}); err != nil {
		t.Fatalf("create vendor item A: %v", err)
	}
	if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
		"vendor_id":              vendorB.ID,
		"vendor_name":            "Supplier B",
		"item_code":              "JUICE-ORANGE",
		"priority":               1.0,
		"minimum_order_quantity": 10.0,
		"pack_size":              6.0,
		"status":                 "active",
	}); err != nil {
		t.Fatalf("create vendor item B: %v", err)
	}

	procurementSvc := NewProcurementCoreService(docs, config.NewService(), models, nil)
	planningSvc := NewPlanningCoreService(docs, models, nil, nil, nil, procurementSvc)
	records, err := planningSvc.GeneratePurchaseRequests("org_default", "loc_main", "user_admin", []ReplenishmentSelection{{
		ItemCode:      "JUICE-ORANGE",
		WarehouseCode: "MAIN",
		Quantity:      7.0,
	}})
	if err != nil {
		t.Fatalf("generate purchase request: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 purchase request, got %d", len(records))
	}
	if got := textValue(records[0].Body.Payload["vendor_id"]); got != vendorB.ID {
		t.Fatalf("expected lowest-priority vendor %s, got %s", vendorB.ID, got)
	}
	lines := recordList(records[0].Body.Payload["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 generated line, got %d", len(lines))
	}
	if got := numberValue(lines[0]["quantity"]); got != 12.0 {
		t.Fatalf("expected normalized quantity 12 after MOQ/pack rounding, got %v", got)
	}
	if got := textValue(lines[0]["planning_quantity_rule"]); got != "moq+pack" {
		t.Fatalf("expected planning quantity rule moq+pack, got %s", got)
	}
}

func TestCreatePlanningRunPersistsForecastAndProjectedDates(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPlanningTestDocumentTypes(t, docs)
	mustRegisterPlanningTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                                  "SEASONAL-JUICE",
		"name":                                 "Seasonal Juice",
		"kind":                                 "item",
		"uom_code":                             "EA",
		"base_price":                           9.0,
		"inventory_enabled":                    true,
		"replenishment_enabled":                true,
		"replenishment_mode":                   "reorder_point_target",
		"reorder_point_quantity":               5.0,
		"target_stock_quantity":                12.0,
		"default_replenishment_warehouse_code": "MAIN",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	now := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	for week := 1; week <= 2; week++ {
		deliveryDate := now.AddDate(0, 0, -(week * 7))
		delivery, err := docs.Create("delivery_order", "org_default", "loc_main", "user_admin", map[string]any{
			"delivery_date":  deliveryDate.Format("2006-01-02"),
			"delivered_date": deliveryDate.Format("2006-01-02"),
			"lines": []map[string]any{{
				"item_code":         "SEASONAL-JUICE",
				"warehouse_code":    "MAIN",
				"delivered_quantity": 4.0,
			}},
		})
		if err != nil {
			t.Fatalf("create delivery: %v", err)
		}
		delivery.Header.Status = "delivered"
		if err := docs.Save(delivery); err != nil {
			t.Fatalf("save delivery: %v", err)
		}
	}
	po, err := docs.Create("purchase_order", "org_default", "loc_main", "user_admin", map[string]any{
		"expected_receipt_date": now.AddDate(0, 0, 3).Format("2006-01-02"),
		"lines": []map[string]any{{
			"item_code":              "SEASONAL-JUICE",
			"warehouse_code":         "MAIN",
			"ordered_qty":            10.0,
			"received_qty":           0.0,
			"quantity":               10.0,
			"expected_receipt_date":  now.AddDate(0, 0, 3).Format("2006-01-02"),
		}},
	})
	if err != nil {
		t.Fatalf("create purchase order: %v", err)
	}
	po.Header.Status = "approved"
	if err := docs.Save(po); err != nil {
		t.Fatalf("save purchase order: %v", err)
	}

	planningSvc := NewPlanningCoreService(docs, models, nil, nil, nil, nil)
	run, err := planningSvc.CreatePlanningRun("org_default", "loc_main", "user_admin", "MAIN", "", "", "", false, false, false, now)
	if err != nil {
		t.Fatalf("create planning run: %v", err)
	}
	if got := numberValue(run.Values["proposal_count"]); got != 1.0 {
		t.Fatalf("expected 1 proposal, got %v", got)
	}
	summary, err := planningSvc.PlanningRunProposalsScoped(run.ID, "org_default", "loc_main")
	if err != nil {
		t.Fatalf("load planning proposals: %v", err)
	}
	if summary.ProposalCount != 1 {
		t.Fatalf("expected 1 saved proposal, got %d", summary.ProposalCount)
	}
	row := summary.Items[0]
	if got := numberValue(row["forecast_demand_quantity"]); got <= 0 {
		t.Fatalf("expected forecast demand quantity > 0, got %v", got)
	}
	if got := textValue(row["next_inbound_date"]); got != now.AddDate(0, 0, 3).Format("2006-01-02") {
		t.Fatalf("expected next inbound date in 3 days, got %s", got)
	}
	if got := textValue(row["projected_shortage_date"]); got == "" {
		t.Fatalf("expected projected shortage date to be populated")
	}
}

func TestConvertPlanningProposalsGeneratesPurchaseRequestsAndMarksConverted(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPlanningTestDocumentTypes(t, docs)
	mustRegisterPlanningTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                                  "FORECAST-MILK",
		"name":                                 "Forecast Milk",
		"kind":                                 "item",
		"uom_code":                             "EA",
		"base_price":                           10.0,
		"inventory_enabled":                    true,
		"replenishment_enabled":                true,
		"replenishment_mode":                   "reorder_point_target",
		"reorder_point_quantity":               4.0,
		"target_stock_quantity":                10.0,
		"default_replenishment_warehouse_code": "MAIN",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	vendor, err := models.Create("vendor_profile", "user_admin", map[string]any{
		"vendor_name": "Planner Vendor",
		"status":      "active",
	})
	if err != nil {
		t.Fatalf("create vendor: %v", err)
	}
	if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
		"vendor_id":              vendor.ID,
		"vendor_name":            "Planner Vendor",
		"item_code":              "FORECAST-MILK",
		"preferred":              true,
		"priority":               1.0,
		"minimum_order_quantity": 2.0,
		"status":                 "active",
	}); err != nil {
		t.Fatalf("create vendor item profile: %v", err)
	}
	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"lines": []map[string]any{{"item_code": "FORECAST-MILK", "quantity": 8.0, "warehouse_code": "MAIN"}},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	order.Header.Status = "confirmed"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save sales order: %v", err)
	}

	procurementSvc := NewProcurementCoreService(docs, config.NewService(), models, nil)
	planningSvc := NewPlanningCoreService(docs, models, nil, nil, nil, procurementSvc)
	run, err := planningSvc.CreatePlanningRun("org_default", "loc_main", "user_admin", "MAIN", "", "", "", true, false, false, time.Now().UTC())
	if err != nil {
		t.Fatalf("create planning run: %v", err)
	}
	summary, err := planningSvc.PlanningRunProposalsScoped(run.ID, "org_default", "loc_main")
	if err != nil {
		t.Fatalf("load planning proposals: %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(summary.Items))
	}
	proposalID := textValue(summary.Items[0]["id"])
	records, err := planningSvc.ConvertPlanningProposals("org_default", "loc_main", "user_admin", []string{proposalID})
	if err != nil {
		t.Fatalf("convert planning proposal: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 purchase request, got %d", len(records))
	}
	proposal, err := models.Get("planning_proposal", proposalID)
	if err != nil {
		t.Fatalf("get updated proposal: %v", err)
	}
	if got := textValue(proposal.Values["conversion_status"]); got != "converted" {
		t.Fatalf("expected conversion_status converted, got %s", got)
	}
	runRecord, err := models.Get("planning_run", run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if got := textValue(runRecord.Values["status"]); got != "converted" {
		t.Fatalf("expected run status converted, got %s", got)
	}
}

func mustRegisterPlanningTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_order", DisplayName: "Sales Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"fulfillment_for"}},
		{Type: "sales_fulfillment", DisplayName: "Sales Fulfillment", SchemaVersion: "v1", AllowedLinkTypes: []string{"fulfillment_for"}},
		{Type: "delivery_order", DisplayName: "Delivery Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"delivery_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "purchase_order", DisplayName: "Purchase Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_request", "purchase_order_for", "receipt_for", "bill_for", "payment_for", "credit_for", "posting_for"}},
		{Type: "purchase_request", DisplayName: "Purchase Request", SchemaVersion: "v1", AllowedLinkTypes: []string{"source_request", "purchase_order_for", "receipt_for", "bill_for", "payment_for", "credit_for", "posting_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterPlanningTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{
			Key:         "commercial_item",
			DisplayName: "Commercial Item",
			DefaultSort: "sku",
			Fields: []model.FieldDefinition{
				{Key: "sku", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "kind", Type: "string", Required: true},
				{Key: "uom_code", Type: "string"},
				{Key: "base_price", Type: "number"},
				{Key: "inventory_enabled", Type: "bool"},
				{Key: "replenishment_enabled", Type: "bool"},
				{Key: "replenishment_mode", Type: "string"},
				{Key: "reorder_point_quantity", Type: "number"},
				{Key: "target_stock_quantity", Type: "number"},
				{Key: "default_replenishment_warehouse_code", Type: "string"},
				{Key: "category_code", Type: "string"},
			},
		},
		{
			Key:         "vendor_profile",
			DisplayName: "Vendor Profile",
			DefaultSort: "vendor_name",
			Fields: []model.FieldDefinition{
				{Key: "vendor_name", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "party_id", Type: "string"},
			},
		},
		{
			Key:         "vendor_item_profile",
			DisplayName: "Vendor Item Profile",
			DefaultSort: "item_code",
			Fields: []model.FieldDefinition{
				{Key: "vendor_id", Type: "string", Required: true},
				{Key: "vendor_name", Type: "string"},
				{Key: "item_code", Type: "string", Required: true},
				{Key: "vendor_item_code", Type: "string"},
				{Key: "preferred", Type: "bool"},
				{Key: "purchase_uom_code", Type: "string"},
				{Key: "priority", Type: "number"},
				{Key: "lead_time_days", Type: "number"},
				{Key: "minimum_order_quantity", Type: "number"},
				{Key: "pack_size", Type: "number"},
				{Key: "last_purchase_price", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "planning_run",
			DisplayName: "Planning Run",
			DefaultSort: "run_date",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "run_date", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "category_code", Type: "string"},
				{Key: "coverage_status", Type: "string"},
				{Key: "forecast_method", Type: "string"},
				{Key: "forecast_window_days", Type: "number"},
				{Key: "seasonal_history_weeks", Type: "number"},
				{Key: "proposal_count", Type: "number"},
				{Key: "projected_shortage_item_count", Type: "number"},
				{Key: "total_shortage_quantity", Type: "number"},
				{Key: "total_forecast_demand_quantity", Type: "number"},
				{Key: "total_normalized_request_quantity", Type: "number"},
				{Key: "due_soon_count", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "planning_proposal",
			DisplayName: "Planning Proposal",
			DefaultSort: "projected_shortage_date",
			Fields: []model.FieldDefinition{
				{Key: "planning_run_id", Type: "string", Required: true},
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "preferred_vendor_name", Type: "string"},
				{Key: "forecast_demand_quantity", Type: "number"},
				{Key: "projected_shortage_date", Type: "string"},
				{Key: "recommended_order_by_date", Type: "string"},
				{Key: "normalized_request_quantity", Type: "number"},
				{Key: "conversion_status", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}
