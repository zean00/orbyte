package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestGenerateProductionOrderIssueAndOutput(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "BURGER",
		"name":              "Burger",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create finished item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "BUN",
		"name":              "Burger Bun",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create bun item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                     "PATTY",
		"name":                    "Patty",
		"uom_code":                "EA",
		"inventory_enabled":       true,
		"inventory_tracking_mode": "batch",
	}); err != nil {
		t.Fatalf("create patty item: %v", err)
	}

	bom, err := models.Create("production_bom", "user_admin", map[string]any{
		"code":                 "BOM-BURGER",
		"name":                 "Burger Recipe",
		"finished_item_code":   "BURGER",
		"default_version_code": "v1",
		"status":               "active",
	})
	if err != nil {
		t.Fatalf("create bom: %v", err)
	}
	if _, err := models.Create("production_bom_version", "user_admin", map[string]any{
		"bom_id":         bom.ID,
		"bom_code":       "BOM-BURGER",
		"version_code":   "v1",
		"yield_quantity": 1.0,
		"is_active":      true,
		"status":         "active",
		"lines": []map[string]any{
			{"component_item_code": "BUN", "quantity_per_unit": 1.0, "uom_code": "EA", "warehouse_code": "MAIN"},
			{"component_item_code": "PATTY", "quantity_per_unit": 1.0, "uom_code": "EA", "warehouse_code": "MAIN", "batch_code": "PAT-001"},
		},
	}); err != nil {
		t.Fatalf("create bom version: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "BUN",
		"warehouse_code":     "MAIN",
		"quantity_delta":     10.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "PATTY",
		"warehouse_code":     "MAIN",
		"batch_code":         "PAT-001",
		"quantity_delta":     10.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})

	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"party_name": "Walk In",
		"lines": []map[string]any{
			{"item_code": "BURGER", "quantity": 3.0, "description": "Burger"},
		},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	order.Header.Status = "confirmed"
	order.Header.Number = "SO-PROD-001"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save sales order: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	productionSvc := NewProductionCoreService(docs, models, nil, inventorySvc)

	orders, err := productionSvc.GenerateProductionOrdersFromSalesOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate production orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 production order, got %d", len(orders))
	}
	productionOrder, err := docs.Get(orders[0].Header.ID)
	if err != nil {
		t.Fatalf("get production order: %v", err)
	}
	if got := len(recordList(productionOrder.Body.Payload["lines"])); got != 2 {
		t.Fatalf("expected 2 component lines, got %d", got)
	}
	productionOrder.Header.Status = "approved"
	if err := docs.Save(productionOrder); err != nil {
		t.Fatalf("approve production order: %v", err)
	}

	issue, err := productionSvc.CreateProductionIssueFromOrder(productionOrder.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create production issue: %v", err)
	}
	issue.Header.Status = "submitted"
	if err := docs.Save(issue); err != nil {
		t.Fatalf("submit production issue: %v", err)
	}
	if err := productionSvc.ValidateApprove(issue); err != nil {
		t.Fatalf("validate production issue: %v", err)
	}
	issue.Header.Status = "issued"
	if err := docs.Save(issue); err != nil {
		t.Fatalf("approve production issue: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(issue, "user_admin"); err != nil {
		t.Fatalf("handle approved production issue: %v", err)
	}

	output, err := productionSvc.CreateProductionOutputFromOrder(productionOrder.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create production output: %v", err)
	}
	output.Body.Payload["production_lot_code"] = "LOT-BURGER-001"
	output.Body.Payload["output_quantity"] = 3.0
	output.Header.Status = "submitted"
	if err := docs.Save(output); err != nil {
		t.Fatalf("submit production output: %v", err)
	}
	if err := productionSvc.ValidateApprove(output); err != nil {
		t.Fatalf("validate production output: %v", err)
	}
	output.Header.Status = "posted"
	if err := docs.Save(output); err != nil {
		t.Fatalf("approve production output: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(output, "user_admin"); err != nil {
		t.Fatalf("handle approved production output: %v", err)
	}

	balances := inventorySvc.currentBalances("org_default", "loc_main")
	if got := inventorySvc.sumBalance(balances, "BURGER", "MAIN", ""); got != 3.0 {
		t.Fatalf("expected finished stock 3, got %v", got)
	}
	if got := inventorySvc.sumBalance(balances, "BUN", "MAIN", ""); got != 7.0 {
		t.Fatalf("expected bun stock 7, got %v", got)
	}
	if got := inventorySvc.sumBalance(balances, "PATTY", "MAIN", "PAT-001"); got != 7.0 {
		t.Fatalf("expected patty stock 7, got %v", got)
	}
	updatedOrder, err := docs.Get(productionOrder.Header.ID)
	if err != nil {
		t.Fatalf("reload production order: %v", err)
	}
	if updatedOrder.Header.Status != "completed" {
		t.Fatalf("expected completed production order, got %s", updatedOrder.Header.Status)
	}
}

func TestProductionIssueAndOutputRefreshLinkedOrderReservations(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                    "SOUP",
		"name":                   "Soup",
		"uom_code":               "EA",
		"inventory_enabled":      true,
		"inventory_tracking_mode": "quantity",
	}); err != nil {
		t.Fatalf("create finished item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                    "BROTH",
		"name":                   "Broth",
		"uom_code":               "EA",
		"inventory_enabled":      true,
		"inventory_tracking_mode": "quantity",
	}); err != nil {
		t.Fatalf("create broth item: %v", err)
	}
	bom, err := models.Create("production_bom", "user_admin", map[string]any{
		"code":                 "BOM-SOUP",
		"name":                 "Soup Recipe",
		"finished_item_code":   "SOUP",
		"default_version_code": "v1",
		"status":               "active",
	})
	if err != nil {
		t.Fatalf("create bom: %v", err)
	}
	if _, err := models.Create("production_bom_version", "user_admin", map[string]any{
		"bom_id":         bom.ID,
		"bom_code":       "BOM-SOUP",
		"version_code":   "v1",
		"yield_quantity": 1.0,
		"is_active":      true,
		"status":         "active",
		"lines": []map[string]any{
			{"component_item_code": "BROTH", "quantity_per_unit": 1.0, "uom_code": "EA", "warehouse_code": "MAIN"},
		},
	}); err != nil {
		t.Fatalf("create bom version: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "BROTH",
		"warehouse_code":     "MAIN",
		"quantity_delta":     5.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})

	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"party_name": "Walk In",
		"lines": []map[string]any{
			{"item_code": "SOUP", "quantity": 2.0, "description": "Soup"},
		},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	order.Header.Status = "confirmed"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save sales order: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	productionSvc := NewProductionCoreService(docs, models, nil, inventorySvc)

	productionOrders, err := productionSvc.GenerateProductionOrdersFromSalesOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate production orders: %v", err)
	}
	productionOrder := productionOrders[0]
	productionOrder.Header.Status = "approved"
	if err := docs.Save(productionOrder); err != nil {
		t.Fatalf("approve production order: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(productionOrder, "user_admin"); err != nil {
		t.Fatalf("apply production reservations: %v", err)
	}

	issue, err := productionSvc.CreateProductionIssueFromOrder(productionOrder.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create production issue: %v", err)
	}
	issue.Header.Status = "issued"
	if err := docs.Save(issue); err != nil {
		t.Fatalf("save production issue: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(issue, "user_admin"); err != nil {
		t.Fatalf("approve production issue: %v", err)
	}

	output, err := productionSvc.CreateProductionOutputFromOrder(productionOrder.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create production output: %v", err)
	}
	output.Body.Payload["output_quantity"] = 2.0
	output.Body.Payload["production_lot_code"] = "LOT-SOUP-1"
	output.Body.Payload["stages"] = []map[string]any{}
	output.Body.Payload = productionSvc.NormalizePayload("production_output", output.Body.Payload)
	output.Header.Status = "posted"
	if err := docs.Save(output); err != nil {
		t.Fatalf("save production output: %v", err)
	}
	if err := productionSvc.HandleApprovedDocument(output, "user_admin"); err != nil {
		t.Fatalf("approve production output: %v", err)
	}

	updatedOrder, err := docs.Get(productionOrder.Header.ID)
	if err != nil {
		t.Fatalf("reload production order: %v", err)
	}
	if got := updatedOrder.Header.Status; got != "completed" {
		t.Fatalf("expected completed production order, got %s", got)
	}
	lines := recordList(updatedOrder.Body.Payload["lines"])
	if got := numberValue(lines[0]["reserved_quantity"]); got != 0 {
		t.Fatalf("expected reservation released after issue/output, got %v", got)
	}
	if got := numberValue(updatedOrder.Body.Payload["reserved_quantity_total"]); got != 0 {
		t.Fatalf("expected reserved quantity total 0, got %v", got)
	}
}

func TestGenerateProductionOrderFindsTrimmedActiveBOM(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "PASTA",
		"name":              "Pasta",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create finished item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "NOODLE",
		"name":              "Noodle",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create component item: %v", err)
	}

	bom, err := models.Create("production_bom", "user_admin", map[string]any{
		"code":               "BOM-PASTA",
		"name":               "Pasta Recipe",
		"finished_item_code": "PASTA ",
		"status":             "active",
	})
	if err != nil {
		t.Fatalf("create bom: %v", err)
	}
	if _, err := models.Create("production_bom_version", "user_admin", map[string]any{
		"bom_id":         bom.ID,
		"bom_code":       "BOM-PASTA",
		"version_code":   "v1",
		"yield_quantity": 1.0,
		"is_active":      true,
		"status":         "active",
		"lines": []map[string]any{
			{"component_item_code": "NOODLE", "quantity_per_unit": 1.0, "uom_code": "EA", "warehouse_code": "MAIN"},
		},
	}); err != nil {
		t.Fatalf("create bom version: %v", err)
	}

	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"party_name": "Walk In",
		"lines": []map[string]any{
			{"item_code": "PASTA", "quantity": 2.0, "description": "Pasta"},
		},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	order.Header.Status = "confirmed"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save sales order: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	productionSvc := NewProductionCoreService(docs, models, nil, inventorySvc)
	orders, err := productionSvc.GenerateProductionOrdersFromSalesOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate production orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 generated production order, got %d", len(orders))
	}
}

func TestGenerateProductionOrderFromApprovedSalesOrder(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "BURGER-APPROVED",
		"name":              "Burger Approved",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create finished item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "BUN-APPROVED",
		"name":              "Bun Approved",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create component item: %v", err)
	}

	bom, err := models.Create("production_bom", "user_admin", map[string]any{
		"code":                 "BOM-BURGER-APPROVED",
		"name":                 "Burger Approved Recipe",
		"finished_item_code":   "BURGER-APPROVED",
		"default_version_code": "v1",
		"status":               "active",
	})
	if err != nil {
		t.Fatalf("create bom: %v", err)
	}
	if _, err := models.Create("production_bom_version", "user_admin", map[string]any{
		"bom_id":         bom.ID,
		"bom_code":       "BOM-BURGER-APPROVED",
		"version_code":   "v1",
		"yield_quantity": 1.0,
		"is_active":      true,
		"status":         "active",
		"lines": []map[string]any{
			{"component_item_code": "BUN-APPROVED", "quantity_per_unit": 1.0, "uom_code": "EA", "warehouse_code": "MAIN"},
		},
	}); err != nil {
		t.Fatalf("create bom version: %v", err)
	}

	order, err := docs.Create("sales_order", "org_default", "loc_main", "user_admin", map[string]any{
		"party_name": "Walk In",
		"lines": []map[string]any{
			{"item_code": "BURGER-APPROVED", "quantity": 2.0, "description": "Burger Approved"},
		},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	order.Header.Status = "approved"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save sales order: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	productionSvc := NewProductionCoreService(docs, models, nil, inventorySvc)
	orders, err := productionSvc.GenerateProductionOrdersFromSalesOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate production orders from approved sales order: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 generated production order, got %d", len(orders))
	}
}

func TestQuantityTrackedProductionOutputUsesNonBatchBalance(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "SOUP",
		"name":              "Soup",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create finished item: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	service := NewProductionCoreService(docs, models, nil, inventorySvc)
	record, err := docs.Create("production_output", "org_default", "loc_main", "user_admin", map[string]any{
		"finished_item_code":  "SOUP",
		"finished_item_name":  "Soup",
		"warehouse_code":      "MAIN",
		"production_lot_code": "SOUP-LOT-001",
		"output_quantity":     2.0,
		"uom_code":            "EA",
	})
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	record.Header.Status = "posted"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save output: %v", err)
	}
	if err := service.HandleApprovedDocument(record, "user_admin"); err != nil {
		t.Fatalf("handle approved output: %v", err)
	}

	updatedBalances := inventorySvc.currentBalances("org_default", "loc_main")
	if got := inventorySvc.sumBalance(updatedBalances, "SOUP", "MAIN", ""); got != 2.0 {
		t.Fatalf("expected quantity-tracked stock in non-batch balance, got %v", got)
	}
	if got := inventorySvc.sumBalance(updatedBalances, "SOUP", "MAIN", "SOUP-LOT-001"); got != 0.0 {
		t.Fatalf("expected no batch-specific balance for quantity-tracked item, got %v", got)
	}
}

func TestProductionOutputRequiresLotCode(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "SOUP",
		"name":              "Soup",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create finished item: %v", err)
	}

	service := NewProductionCoreService(docs, models, nil, NewInventoryCoreService(docs, nil, models, nil))
	record, err := docs.Create("production_output", "org_default", "loc_main", "user_admin", map[string]any{
		"finished_item_code": "SOUP",
		"warehouse_code":     "MAIN",
		"output_quantity":    2.0,
	})
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	record.Header.Status = "submitted"
	if err := docs.Save(record); err != nil {
		t.Fatalf("submit output: %v", err)
	}
	if err := service.ValidateApprove(record); err == nil {
		t.Fatalf("expected production output validation error for missing lot code")
	}
}

func TestProductionOrderApprovalReservesComponentsAndStages(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "SOUP",
		"name":              "Soup",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create finished item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "WATER",
		"name":              "Water",
		"uom_code":          "L",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create water item: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "WATER",
		"warehouse_code":     "MAIN",
		"quantity_delta":     3.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})

	service := NewProductionCoreService(docs, models, nil, NewInventoryCoreService(docs, nil, models, nil))
	order, err := docs.Create("production_order", "org_default", "loc_main", "user_admin", map[string]any{
		"production_pattern":       "make_to_stock",
		"finished_item_code":       "SOUP",
		"warehouse_code":           "MAIN",
		"planned_quantity":         5.0,
		"expected_output_quantity": 5.0,
		"lines": []map[string]any{{
			"component_item_code": "WATER",
			"actual_item_code":    "WATER",
			"warehouse_code":      "MAIN",
			"uom_code":            "L",
			"quantity_per_unit":   1.0,
			"quantity":            5.0,
		}},
		"stages": []map[string]any{
			{"stage_code": "prep", "stage_name": "Prep", "sequence": 1, "required": true},
			{"stage_code": "pack", "stage_name": "Pack", "sequence": 2, "required": true},
		},
	})
	if err != nil {
		t.Fatalf("create production order: %v", err)
	}
	order.Body.Payload = service.NormalizePayload("production_order", order.Body.Payload)
	order.Header.Status = "approved"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save approved production order: %v", err)
	}
	if err := service.HandleApprovedDocument(order, "user_admin"); err != nil {
		t.Fatalf("handle approved production order: %v", err)
	}

	updated, err := docs.Get(order.Header.ID)
	if err != nil {
		t.Fatalf("reload production order: %v", err)
	}
	lines := recordList(updated.Body.Payload["lines"])
	if got := numberValue(lines[0]["reserved_quantity"]); got != 3.0 {
		t.Fatalf("expected reserved quantity 3, got %v", got)
	}
	if got := numberValue(lines[0]["shortage_quantity"]); got != 2.0 {
		t.Fatalf("expected shortage quantity 2, got %v", got)
	}
	if got := textValue(lines[0]["reservation_status"]); got != "partial" {
		t.Fatalf("expected partial reservation status, got %s", got)
	}
	stages := recordList(updated.Body.Payload["stages"])
	if got := textValue(stages[0]["status"]); got != "ready" {
		t.Fatalf("expected first stage ready, got %s", got)
	}
}

func TestProductionIssueAllowsApprovedSubstitute(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProductionTestDocumentTypes(t, docs)
	mustRegisterProductionTestModels(t, models)

	for _, item := range []map[string]any{
		{"sku": "SAUCE", "name": "Sauce", "uom_code": "EA", "inventory_enabled": true},
		{"sku": "SAUCE-ALT", "name": "Alt Sauce", "uom_code": "EA", "inventory_enabled": true},
		{"sku": "MEAL", "name": "Meal", "uom_code": "EA", "inventory_enabled": true},
	} {
		if _, err := models.Create("commercial_item", "user_admin", item); err != nil {
			t.Fatalf("create item %v: %v", item["sku"], err)
		}
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "SAUCE-ALT",
		"warehouse_code":     "MAIN",
		"quantity_delta":     4.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	service := NewProductionCoreService(docs, models, nil, inventorySvc)
	order, err := docs.Create("production_order", "org_default", "loc_main", "user_admin", map[string]any{
		"finished_item_code":       "MEAL",
		"warehouse_code":           "MAIN",
		"planned_quantity":         2.0,
		"expected_output_quantity": 2.0,
		"lines": []map[string]any{{
			"component_item_code":           "SAUCE",
			"actual_item_code":              "SAUCE-ALT",
			"warehouse_code":                "MAIN",
			"uom_code":                      "EA",
			"quantity_per_unit":             1.0,
			"quantity":                      2.0,
			"allowed_substitute_item_codes": []string{"SAUCE-ALT"},
		}},
	})
	if err != nil {
		t.Fatalf("create production order: %v", err)
	}
	order.Body.Payload = service.NormalizePayload("production_order", order.Body.Payload)
	order.Header.Status = "approved"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save production order: %v", err)
	}
	if err := service.HandleApprovedDocument(order, "user_admin"); err != nil {
		t.Fatalf("approve production order: %v", err)
	}

	issue, err := service.CreateProductionIssueFromOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create production issue: %v", err)
	}
	issue.Header.Status = "submitted"
	if err := docs.Save(issue); err != nil {
		t.Fatalf("submit production issue: %v", err)
	}
	if err := service.ValidateApprove(issue); err != nil {
		t.Fatalf("validate production issue with approved substitute: %v", err)
	}
	issue.Header.Status = "issued"
	if err := docs.Save(issue); err != nil {
		t.Fatalf("issue production issue: %v", err)
	}
	if err := service.HandleApprovedDocument(issue, "user_admin"); err != nil {
		t.Fatalf("handle approved production issue: %v", err)
	}

	balances := inventorySvc.currentBalances("org_default", "loc_main")
	if got := inventorySvc.sumBalance(balances, "SAUCE-ALT", "MAIN", ""); got != 2.0 {
		t.Fatalf("expected SAUCE-ALT stock 2 after issue, got %v", got)
	}
}

func mustRegisterProductionTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_order", DisplayName: "Sales Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"production_for"}},
		{Type: "production_order", DisplayName: "Production Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"production_for", "movement_for"}},
		{Type: "production_issue", DisplayName: "Production Issue", SchemaVersion: "v1", AllowedLinkTypes: []string{"production_for", "movement_for"}},
		{Type: "production_output", DisplayName: "Production Output", SchemaVersion: "v1", AllowedLinkTypes: []string{"production_for", "movement_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterProductionTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{
			Key:         "commercial_item",
			DisplayName: "Commercial Item",
			DefaultSort: "sku",
			Fields: []model.FieldDefinition{
				{Key: "sku", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "uom_code", Type: "string"},
				{Key: "inventory_enabled", Type: "bool"},
				{Key: "inventory_tracking_mode", Type: "string"},
				{Key: "expiry_tracking_enabled", Type: "bool"},
				{Key: "allow_negative_stock", Type: "bool"},
				{Key: "default_issue_strategy", Type: "string"},
				{Key: "default_replenishment_warehouse_code", Type: "string"},
			},
		},
		{
			Key:         "production_bom",
			DisplayName: "Production BOM",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "finished_item_code", Type: "string", Required: true},
				{Key: "default_version_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "production_bom_version",
			DisplayName: "Production BOM Version",
			DefaultSort: "version_code",
			Fields: []model.FieldDefinition{
				{Key: "bom_id", Type: "string", Required: true},
				{Key: "bom_code", Type: "string"},
				{Key: "version_code", Type: "string", Required: true},
				{Key: "yield_quantity", Type: "number"},
				{Key: "is_active", Type: "bool"},
				{Key: "lines", Type: "object"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "inventory_batch",
			DisplayName: "Inventory Batch",
			DefaultSort: "batch_code",
			Fields: []model.FieldDefinition{
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "batch_code", Type: "string"},
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
