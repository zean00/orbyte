package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestReturnsValidateApproveRejectsUnknownWarehouse(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterReturnsTestDocumentTypes(t, docs)
	mustRegisterReturnsTestModels(t, models)

	service := NewReturnsCoreService(docs, nil, NewInventoryCoreService(docs, nil, models, nil), nil, nil)

	salesReturn, err := docs.Create("sales_return", "org_default", "loc_main", "user_admin", map[string]any{
		"source_fulfillment_id": "fulfillment_1",
		"lines": []map[string]any{{
			"source_fulfillment_line_index": 0,
			"item_code":                     "ITEM-1",
			"quantity":                      1.0,
		}},
	})
	if err != nil {
		t.Fatalf("create sales return: %v", err)
	}
	if err := docs.Save(salesReturn); err != nil {
		t.Fatalf("save sales return: %v", err)
	}

	receipt, err := docs.Create("return_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"source_return_id": salesReturn.Header.ID,
		"lines": []map[string]any{{
			"source_fulfillment_line_index": 0,
			"item_code":                     "ITEM-1",
			"warehouse_code":                "MISSING",
			"quantity":                      1.0,
		}},
	})
	if err != nil {
		t.Fatalf("create return receipt: %v", err)
	}
	if err := service.ValidateApprove(receipt); err == nil {
		t.Fatal("expected unknown warehouse to be rejected")
	}
}

func mustRegisterReturnsTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_return", DisplayName: "Sales Return", SchemaVersion: "v1"},
		{Type: "return_receipt", DisplayName: "Return Receipt", SchemaVersion: "v1"},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document %s: %v", def.Type, err)
		}
	}
}

func mustRegisterReturnsTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{Key: "commercial_item", DisplayName: "Commercial Item", Fields: []model.FieldDefinition{{Key: "sku", Type: "string"}, {Key: "inventory_enabled", Type: "bool"}, {Key: "inventory_tracking_mode", Type: "string"}}},
		{Key: "warehouse", DisplayName: "Warehouse", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}}},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{"sku": "ITEM-1", "inventory_enabled": true, "inventory_tracking_mode": "quantity"}); err != nil {
		t.Fatalf("create item: %v", err)
	}
}
