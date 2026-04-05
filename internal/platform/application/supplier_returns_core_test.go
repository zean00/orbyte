package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestGenerateSupplierReturnFromReceiptAndApprove(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterSupplierReturnTestDocumentTypes(t, docs)
	mustRegisterSupplierReturnTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "MILK-1L",
		"name":              "Milk 1L",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "MILK-1L",
		"warehouse_code":     "MAIN",
		"quantity_delta":     5.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
	})

	receipt, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_id":                    "vendor-1",
		"vendor_name":                  "Supplier A",
		"source_purchase_order_id":     "po-1",
		"source_purchase_order_number": "PO-1",
		"receipt_date":                 time.Now().UTC().Format("2006-01-02"),
		"lines": []map[string]any{{
			"item_code":      "MILK-1L",
			"description":    "Milk 1L",
			"warehouse_code": "MAIN",
			"uom_code":       "EA",
			"receipt_qty":    5.0,
		}},
	})
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	receipt.Header.Status = "received"
	receipt.Header.Number = "GR-001"
	if err := docs.Save(receipt); err != nil {
		t.Fatalf("save goods receipt: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	service := NewSupplierReturnsCoreService(docs, nil, inventorySvc, NewProcurementCoreService(docs, nil, models, nil))

	supplierReturn, err := service.GenerateSupplierReturnFromReceipt(receipt.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate supplier return: %v", err)
	}
	lines := recordList(supplierReturn.Body.Payload["lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 supplier return line, got %d", len(lines))
	}
	if got := numberValue(lines[0]["quantity"]); got != 5.0 {
		t.Fatalf("expected return quantity 5, got %v", got)
	}

	supplierReturn.Header.Status = "approved"
	if err := docs.Save(supplierReturn); err != nil {
		t.Fatalf("save supplier return: %v", err)
	}
	if err := service.ValidateApprove(supplierReturn); err != nil {
		t.Fatalf("validate approve: %v", err)
	}
	if err := service.HandleApprovedDocument(supplierReturn, "user_admin"); err != nil {
		t.Fatalf("handle approved supplier return: %v", err)
	}

	balances := inventorySvc.currentBalances("org_default", "loc_main")
	if got := inventorySvc.sumBalance(balances, "MILK-1L", "MAIN", ""); got != 0.0 {
		t.Fatalf("expected zero on-hand after return, got %v", got)
	}
}

func TestCreateVendorCreditFromSupplierReturn(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterSupplierReturnTestDocumentTypes(t, docs)
	mustRegisterSupplierReturnTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "BREAD-LOAF",
		"name":              "Bread Loaf",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	bill, err := docs.Create("vendor_bill", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_id":            "vendor-1",
		"vendor_name":          "Supplier A",
		"currency_code":        "IDR",
		"default_tax_code":     "VAT",
		"tax_profile_code":     "STANDARD",
		"payable_account_code": "2000-AP",
		"lines": []map[string]any{{
			"item_code":   "BREAD-LOAF",
			"description": "Bread Loaf",
			"uom_code":    "EA",
			"quantity":    4.0,
			"unit_price":  10.0,
			"tax_rate":    10.0,
			"tax_code":    "VAT",
			"line_total":  44.0,
		}},
	})
	if err != nil {
		t.Fatalf("create vendor bill: %v", err)
	}
	bill.Header.Status = "issued"
	bill.Header.Number = "VB-001"
	if err := docs.Save(bill); err != nil {
		t.Fatalf("save vendor bill: %v", err)
	}

	supplierReturn, err := docs.Create("supplier_return", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_id":                 "vendor-1",
		"vendor_name":               "Supplier A",
		"source_vendor_bill_id":     bill.Header.ID,
		"source_vendor_bill_number": bill.Header.Number,
		"return_date":               time.Now().UTC().Format("2006-01-02"),
		"warehouse_code":            "MAIN",
		"lines": []map[string]any{{
			"item_code":         "BREAD-LOAF",
			"description":       "Bread Loaf",
			"warehouse_code":    "MAIN",
			"uom_code":          "EA",
			"received_quantity": 4.0,
			"quantity":          2.0,
		}},
	})
	if err != nil {
		t.Fatalf("create supplier return: %v", err)
	}
	supplierReturn.Header.Status = "approved"
	supplierReturn.Header.Number = "SR-001"
	if err := docs.Save(supplierReturn); err != nil {
		t.Fatalf("save supplier return: %v", err)
	}

	service := NewSupplierReturnsCoreService(docs, nil, NewInventoryCoreService(docs, nil, models, nil), NewProcurementCoreService(docs, nil, models, nil))
	credit, err := service.CreateVendorCreditFromReturn(supplierReturn.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create vendor credit from supplier return: %v", err)
	}
	if credit.Header.Type != "vendor_credit_note" {
		t.Fatalf("expected vendor_credit_note, got %s", credit.Header.Type)
	}
	if got := numberValue(credit.Body.Payload["total_amount"]); got != 22.0 {
		t.Fatalf("expected credit total 22, got %v", got)
	}
}

func TestSupplierReturnValidateApproveRejectsUnknownWarehouse(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterSupplierReturnTestDocumentTypes(t, docs)
	mustRegisterSupplierReturnTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "MILK-1L",
		"name":              "Milk 1L",
		"uom_code":          "EA",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	service := NewSupplierReturnsCoreService(docs, nil, NewInventoryCoreService(docs, nil, models, nil), NewProcurementCoreService(docs, nil, models, nil))
	record, err := docs.Create("supplier_return", "org_default", "loc_main", "user_admin", map[string]any{
		"lines": []map[string]any{{
			"item_code":      "MILK-1L",
			"warehouse_code": "MISSING",
			"quantity":       1.0,
		}},
	})
	if err != nil {
		t.Fatalf("create supplier return: %v", err)
	}
	if err := service.ValidateApprove(record); err == nil {
		t.Fatal("expected unknown warehouse to be rejected")
	}
}

func mustRegisterSupplierReturnTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "goods_receipt", DisplayName: "Goods Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"return_for", "movement_for"}},
		{Type: "vendor_bill", DisplayName: "Vendor Bill", SchemaVersion: "v1", AllowedLinkTypes: []string{"return_for", "credit_for"}},
		{Type: "vendor_credit_note", DisplayName: "Vendor Credit", SchemaVersion: "v1", AllowedLinkTypes: []string{"credit_for", "return_for"}},
		{Type: "supplier_return", DisplayName: "Supplier Return", SchemaVersion: "v1", AllowedLinkTypes: []string{"return_for", "credit_for", "movement_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterSupplierReturnTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{
			Key:         "warehouse",
			DisplayName: "Warehouse",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
			},
		},
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
				{Key: "default_replenishment_warehouse_code", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
	if _, err := models.Create("warehouse", "user_admin", map[string]any{"code": "MAIN"}); err != nil {
		t.Fatalf("create warehouse: %v", err)
	}
}
