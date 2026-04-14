package mcp

import (
	"context"
	"encoding/json"
	"testing"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
)

func TestPlanningReplenishmentToolsSupportInsightAndDraftExecution(t *testing.T) {
	modules := module.NewService()
	if err := modules.Register(module.Manifest{
		Key:     "planning_core",
		Name:    "Planning Core",
		Version: "1.0.0",
	}, "system"); err != nil {
		t.Fatalf("register planning module: %v", err)
	}

	docs := document.NewService()
	for _, def := range []document.Definition{
		{Type: "sales_order", DisplayName: "Sales Order", SchemaVersion: "v1"},
		{Type: "purchase_request", DisplayName: "Purchase Request", SchemaVersion: "v1"},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}

	models := model.NewService()
	for _, def := range []model.Definition{
		{
			Key:                 "commercial_item",
			DisplayName:         "Commercial Item",
			CreatePermissionKey: "item.create",
			ListPermissionKey:   "item.list",
			ReadPermissionKey:   "item.read",
			UpdatePermissionKey: "item.update",
			Fields: []model.FieldDefinition{
				{Key: "sku", Type: "string"},
				{Key: "name", Type: "string"},
				{Key: "uom_code", Type: "string"},
				{Key: "base_price", Type: "number"},
				{Key: "inventory_enabled", Type: "bool"},
				{Key: "replenishment_enabled", Type: "bool"},
				{Key: "replenishment_mode", Type: "string"},
				{Key: "reorder_point_quantity", Type: "number"},
				{Key: "target_stock_quantity", Type: "number"},
				{Key: "default_replenishment_warehouse_code", Type: "string"},
			},
		},
		{
			Key:                 "vendor_profile",
			DisplayName:         "Vendor Profile",
			CreatePermissionKey: "vendor.create",
			ListPermissionKey:   "vendor.list",
			ReadPermissionKey:   "vendor.read",
			UpdatePermissionKey: "vendor.update",
			Fields: []model.FieldDefinition{
				{Key: "vendor_name", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:                 "vendor_item_profile",
			DisplayName:         "Vendor Item Profile",
			CreatePermissionKey: "vendor_item.create",
			ListPermissionKey:   "vendor_item.list",
			ReadPermissionKey:   "vendor_item.read",
			UpdatePermissionKey: "vendor_item.update",
			Fields: []model.FieldDefinition{
				{Key: "vendor_id", Type: "string"},
				{Key: "vendor_name", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "preferred", Type: "bool"},
				{Key: "priority", Type: "number"},
				{Key: "minimum_order_quantity", Type: "number"},
				{Key: "pack_size", Type: "number"},
				{Key: "lead_time_days", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model definition %s: %v", def.Key, err)
		}
	}

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                                  "BEANS-A",
		"name":                                 "Cold Brew Beans",
		"uom_code":                             "EA",
		"base_price":                           100,
		"inventory_enabled":                    true,
		"replenishment_enabled":                true,
		"replenishment_mode":                   "reorder_point_target",
		"reorder_point_quantity":               4,
		"target_stock_quantity":                10,
		"default_replenishment_warehouse_code": "MAIN",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                                  "MILK-A",
		"name":                                 "Oat Milk",
		"uom_code":                             "EA",
		"base_price":                           80,
		"inventory_enabled":                    true,
		"replenishment_enabled":                true,
		"replenishment_mode":                   "reorder_point_target",
		"reorder_point_quantity":               5,
		"target_stock_quantity":                12,
		"default_replenishment_warehouse_code": "MAIN",
	}); err != nil {
		t.Fatalf("create milk item: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                                  "MATCHA-A",
		"name":                                 "Matcha Powder",
		"uom_code":                             "EA",
		"base_price":                           120,
		"inventory_enabled":                    true,
		"replenishment_enabled":                true,
		"replenishment_mode":                   "reorder_point_target",
		"reorder_point_quantity":               0,
		"target_stock_quantity":                0,
		"default_replenishment_warehouse_code": "MAIN",
	}); err != nil {
		t.Fatalf("create covered item: %v", err)
	}
	vendor, err := models.Create("vendor_profile", "user_admin", map[string]any{
		"vendor_name": "North Roast",
		"status":      "active",
	})
	if err != nil {
		t.Fatalf("create vendor: %v", err)
	}
	if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
		"vendor_id":              vendor.ID,
		"vendor_name":            "North Roast",
		"item_code":              "BEANS-A",
		"preferred":              true,
		"priority":               1,
		"minimum_order_quantity": 5,
		"pack_size":              5,
		"lead_time_days":         4,
		"status":                 "active",
	}); err != nil {
		t.Fatalf("create vendor item profile: %v", err)
	}
	if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
		"vendor_id":              vendor.ID,
		"vendor_name":            "North Roast",
		"item_code":              "MILK-A",
		"preferred":              true,
		"priority":               1,
		"minimum_order_quantity": 4,
		"pack_size":              4,
		"lead_time_days":         3,
		"status":                 "active",
	}); err != nil {
		t.Fatalf("create milk vendor item profile: %v", err)
	}
	if _, err := models.Create("vendor_item_profile", "user_admin", map[string]any{
		"vendor_id":              vendor.ID,
		"vendor_name":            "North Roast",
		"item_code":              "MATCHA-A",
		"preferred":              true,
		"priority":               1,
		"minimum_order_quantity": 2,
		"pack_size":              2,
		"lead_time_days":         5,
		"status":                 "active",
	}); err != nil {
		t.Fatalf("create covered vendor item profile: %v", err)
	}
	order, err := docs.Create("sales_order", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_name": "Cafe Horizon",
		"lines": []map[string]any{
			{"item_code": "BEANS-A", "warehouse_code": "MAIN", "quantity": 6.0},
			{"item_code": "MILK-A", "warehouse_code": "MAIN", "quantity": 4.0},
		},
	})
	if err != nil {
		t.Fatalf("create sales order: %v", err)
	}
	order.Header.Status = "confirmed"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save sales order: %v", err)
	}

	server := NewServer(ServerDeps{
		Modules:   modules,
		Documents: docs,
		Models:    models,
		Planning:  application.NewPlanningCoreService(docs, models, nil, nil, nil, nil),
	})
	actor := ActorContext{
		ActorID:         "user_admin",
		EffectiveUserID: "user_admin",
		OrganizationID:  "org_default",
		LocationID:      "loc_hq",
		PermissionChecker: func(permissionKey string) bool {
			switch permissionKey {
			case "module.read", "document.create":
				return true
			default:
				return false
			}
		},
	}

	listResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}, actor)
	if listResp.Error != nil {
		t.Fatalf("tools/list failed: %+v", listResp.Error)
	}
	params := listResp.Result.(map[string]any)
	rawTools := params["tools"].([]ToolDescriptor)
	foundInsight := false
	foundDraft := false
	for _, item := range rawTools {
		switch item.Name {
		case "planning_core.replenishment.insight.summary":
			foundInsight = true
		case "planning_core.purchase_requests.draft.create":
			foundDraft = true
		}
	}
	if !foundInsight || !foundDraft {
		t.Fatalf("expected planning tools to be listed, got insight=%v draft=%v", foundInsight, foundDraft)
	}

	insightResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name":      "planning_core.replenishment.insight.summary",
			"arguments": map[string]any{"warehouse_code": "MAIN"},
		}),
	}, actor)
	if insightResp.Error != nil {
		t.Fatalf("insight tool failed: %+v", insightResp.Error)
	}
	insight := insightResp.Result.(map[string]any)["structuredContent"].(map[string]any)
	atRisk, _ := insight["at_risk_items"].([]replenishmentRecommendation)
	if len(atRisk) == 0 {
		t.Fatal("expected at-risk replenishment items")
	}

	draftByNameResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "planning_core.purchase_requests.draft.create",
			"arguments": map[string]any{
				"selections": []map[string]any{{
					"item_code":      "Cold Brew Beans",
					"warehouse_code": "MAIN",
					"quantity":       6.0,
				}},
			},
		}),
	}, actor)
	if draftByNameResp.Error != nil {
		t.Fatalf("draft create by item name failed: %+v", draftByNameResp.Error)
	}
	structured := draftByNameResp.Result.(map[string]any)["structuredContent"].(map[string]any)
	documents, _ := structured["documents"].([]map[string]any)
	if len(documents) != 1 {
		t.Fatalf("expected 1 generated purchase request, got %d", len(documents))
	}
	item := documents[0]
	if item["document_type"] != "purchase_request" {
		t.Fatalf("expected purchase_request document type, got %+v", item["document_type"])
	}
	if openPath, _ := item["open_path"].(string); openPath == "" {
		t.Fatalf("expected open_path on generated draft, got %+v", item)
	}

	rawPayload, err := json.Marshal(item["record"])
	if err != nil {
		t.Fatalf("marshal record payload: %v", err)
	}
	if len(rawPayload) == 0 {
		t.Fatal("expected sanitized record payload on generated draft")
	}
	record, ok := item["record"].(document.Record)
	if !ok {
		t.Fatalf("expected record payload to be a document record, got %T", item["record"])
	}
	payload := record.Body.Payload
	lines := payloadLines(t, payload["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected exactly one selected line in draft payload, got %+v", payload["lines"])
	}
	line := lines[0]
	if got := line["item_code"]; got != "BEANS-A" {
		t.Fatalf("expected selected item code only, got %+v", got)
	}
	if got := line["quantity"]; got != float64(6) {
		t.Fatalf("expected selected quantity 6, got %+v", got)
	}
	if got := textValue(line["line_id"]); got == "" {
		t.Fatal("expected generated line_id on selected draft line")
	}

	draftResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "planning_core.purchase_requests.draft.create",
			"arguments": map[string]any{
				"selections": []map[string]any{{
					"item_code":      "MILK-A",
					"warehouse_code": "MAIN",
					"quantity":       10.0,
				}},
			},
		}),
	}, actor)
	if draftResp.Error != nil {
		t.Fatalf("draft create tool failed: %+v", draftResp.Error)
	}
	exactDocs, _ := draftResp.Result.(map[string]any)["structuredContent"].(map[string]any)["documents"].([]map[string]any)
	exactRecord := exactDocs[0]["record"].(document.Record)
	exactLines := payloadLines(t, exactRecord.Body.Payload["lines"])
	if len(exactLines) != 1 {
		t.Fatalf("expected one selected line for exact quantity draft, got %+v", exactRecord.Body.Payload["lines"])
	}
	if got := exactLines[0]["item_code"]; got != "MILK-A" {
		t.Fatalf("expected only selected MILK-A line, got %+v", got)
	}
	if got := exactLines[0]["quantity"]; got != float64(10) {
		t.Fatalf("expected selected quantity 10, got %+v", got)
	}
	if got := textValue(exactLines[0]["line_id"]); got == "" {
		t.Fatal("expected generated line_id on exact quantity draft line")
	}

	coveredResp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: mustJSON(t, map[string]any{
			"name": "planning_core.purchase_requests.draft.create",
			"arguments": map[string]any{
				"selections": []map[string]any{{
					"item_code":      "MATCHA-A",
					"warehouse_code": "MAIN",
					"quantity":       2.0,
				}},
			},
		}),
	}, actor)
	if coveredResp.Error == nil {
		t.Fatal("expected covered item selection to fail")
	}
}

func payloadLines(t *testing.T, raw any) []map[string]any {
	t.Helper()
	switch lines := raw.(type) {
	case []map[string]any:
		return lines
	case []any:
		result := make([]map[string]any, 0, len(lines))
		for _, line := range lines {
			lineMap, ok := line.(map[string]any)
			if !ok {
				t.Fatalf("expected line to be a map, got %T", line)
			}
			result = append(result, lineMap)
		}
		return result
	default:
		t.Fatalf("expected payload lines to be a slice, got %T", raw)
		return nil
	}
}
