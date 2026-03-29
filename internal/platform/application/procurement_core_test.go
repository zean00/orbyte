package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestCreateGoodsReceiptFromOrderCopiesWarehouseCode(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)

	order, err := docs.Create("purchase_order", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_id":     "vendor-1",
		"vendor_name":   "Supplier A",
		"currency_code": "IDR",
		"lines": []map[string]any{{
			"item_code":            "CUFF-STD",
			"description":          "Blood Pressure Cuff",
			"warehouse_code":       "CLINIC",
			"uom_code":             "EA",
			"ordered_qty":          15.0,
			"quantity":             15.0,
			"unit_price":           100.0,
			"tax_code":             "VAT11",
			"tax_rate":             11.0,
			"tax_mode":             "exclusive",
			"tax_account_code":     "1510-VAT-IN",
			"expense_account_code": "5100-COGS",
		}},
	})
	if err != nil {
		t.Fatalf("create purchase order: %v", err)
	}
	order.Header.Status = "approved"
	if err := docs.Save(order); err != nil {
		t.Fatalf("save purchase order: %v", err)
	}

	service := NewProcurementCoreService(docs, nil, models, nil)
	receipt, err := service.CreateGoodsReceiptFromOrder(order.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	lines := recordList(receipt.Body.Payload["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 receipt line, got %d", len(lines))
	}
	if got := textValue(lines[0]["warehouse_code"]); got != "CLINIC" {
		t.Fatalf("expected generated receipt warehouse_code CLINIC, got %s", got)
	}
}

func TestCreateVendorBillFromReceiptUsesReceiptQuantityAndTotals(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)

	receipt, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_id":                    "vendor-1",
		"vendor_name":                  "Supplier A",
		"currency_code":                "IDR",
		"source_purchase_order_id":     "po-1",
		"source_purchase_order_number": "PO-1",
		"lines": []map[string]any{{
			"item_code":            "CUFF-STD",
			"description":          "Blood Pressure Cuff",
			"warehouse_code":       "MAIN",
			"uom_code":             "EA",
			"receipt_qty":          15.0,
			"unit_price":           100.0,
			"tax_code":             "VAT11",
			"tax_rate":             11.0,
			"tax_mode":             "exclusive",
			"tax_account_code":     "1510-VAT-IN",
			"expense_account_code": "5100-COGS",
		}},
	})
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	receipt.Header.Status = "received"
	if err := docs.Save(receipt); err != nil {
		t.Fatalf("save goods receipt: %v", err)
	}

	service := NewProcurementCoreService(docs, nil, models, nil)
	bill, err := service.CreateVendorBillFromReceipt(receipt.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create vendor bill: %v", err)
	}
	lines := recordList(bill.Body.Payload["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 bill line, got %d", len(lines))
	}
	if got := numberValue(lines[0]["quantity"]); got != 15.0 {
		t.Fatalf("expected billed quantity 15, got %v", got)
	}
	if got := numberValue(bill.Body.Payload["subtotal_amount"]); got != 1500.0 {
		t.Fatalf("expected subtotal 1500, got %v", got)
	}
	if got := numberValue(bill.Body.Payload["tax_amount"]); got != 165.0 {
		t.Fatalf("expected tax 165, got %v", got)
	}
	if got := numberValue(bill.Body.Payload["total_amount"]); got != 1665.0 {
		t.Fatalf("expected total 1665, got %v", got)
	}
}

func mustRegisterProcurementGenerationDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "purchase_order", DisplayName: "Purchase Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"receipt_for", "bill_for", "posting_for"}},
		{Type: "goods_receipt", DisplayName: "Goods Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"receipt_for", "bill_for", "movement_for"}},
		{Type: "vendor_bill", DisplayName: "Vendor Bill", SchemaVersion: "v1", AllowedLinkTypes: []string{"bill_for", "posting_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterProcurementGenerationModels(t *testing.T, models *model.Service) {
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
				{Key: "base_price", Type: "number"},
				{Key: "tax_code", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}
