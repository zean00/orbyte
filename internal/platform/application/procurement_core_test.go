package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/config"
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

func TestCreateVendorBillFromReceiptPreservesLandedCostTotals(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "LANDED-BILL",
		"name":              "Landed Bill Item",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create inventory item: %v", err)
	}

	service := NewProcurementCoreService(docs, config.NewService(), models, nil)
	receiptPayload := service.NormalizePayload("goods_receipt", map[string]any{
		"vendor_id":                    "vendor-1",
		"vendor_name":                  "Supplier A",
		"currency_code":                "IDR",
		"source_purchase_order_id":     "po-1",
		"source_purchase_order_number": "PO-1",
		"lines": []map[string]any{{
			"item_code":      "LANDED-BILL",
			"description":    "Imported Item",
			"warehouse_code": "MAIN",
			"uom_code":       "EA",
			"receipt_qty":    10.0,
			"unit_price":     100.0,
		}},
		"landed_cost_lines": []map[string]any{{
			"cost_type":        "freight",
			"description":      "Freight",
			"amount":           200.0,
			"allocation_basis": "line_value",
		}},
	})
	receipt, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", receiptPayload)
	if err != nil {
		t.Fatalf("create goods receipt: %v", err)
	}
	receipt.Header.Status = "received"
	if err := docs.Save(receipt); err != nil {
		t.Fatalf("save goods receipt: %v", err)
	}

	bill, err := service.CreateVendorBillFromReceipt(receipt.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create vendor bill: %v", err)
	}
	if got := numberValue(bill.Body.Payload["landed_cost_amount"]); got != 200.0 {
		t.Fatalf("expected landed cost amount 200, got %v", got)
	}
	if got := numberValue(bill.Body.Payload["subtotal_amount"]); got != 1200.0 {
		t.Fatalf("expected subtotal 1200, got %v", got)
	}
	if got := numberValue(bill.Body.Payload["total_amount"]); got != 1200.0 {
		t.Fatalf("expected total 1200, got %v", got)
	}
	if got := numberValue(bill.Body.Payload["balance_due_amount"]); got != 1200.0 {
		t.Fatalf("expected balance due 1200, got %v", got)
	}
}

func TestVendorBillPostingLinesCapitalizesInventoryItems(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "COFFEE-BEAN",
		"name":                         "Coffee Bean",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-COFFEE",
	}); err != nil {
		t.Fatalf("create inventory item: %v", err)
	}

	service := NewProcurementCoreService(docs, nil, models, nil)
	lines := service.vendorBillPostingLines(map[string]any{
		"source_goods_receipt_id": "doc_gr_1",
		"subtotal_amount":         1500.0,
		"tax_amount":              165.0,
		"total_amount":            1665.0,
		"lines": []map[string]any{{
			"item_code":                    "COFFEE-BEAN",
			"line_subtotal":                1500.0,
			"inventory_asset_account_code": "1200-INV-COFFEE",
		}},
	})

	var inventoryDebit, expenseDebit, taxDebit, payableCredit float64
	for _, line := range lines {
		switch textValue(line["account_code"]) {
		case "1200-INV-COFFEE":
			inventoryDebit = numberValue(line["debit"])
		case "5000-EXP":
			expenseDebit = numberValue(line["debit"])
		case "2100-TAX":
			taxDebit = numberValue(line["debit"])
		case "2000-AP":
			payableCredit = numberValue(line["credit"])
		}
	}
	if inventoryDebit != 1500.0 {
		t.Fatalf("expected inventory debit 1500, got %v", inventoryDebit)
	}
	if expenseDebit != 0 {
		t.Fatalf("expected no expense debit for inventory purchase, got %v", expenseDebit)
	}
	if taxDebit != 165.0 {
		t.Fatalf("expected tax debit 165, got %v", taxDebit)
	}
	if payableCredit != 1665.0 {
		t.Fatalf("expected payable credit 1665, got %v", payableCredit)
	}
}

func TestAllocatePaymentOutDoesNotMutatePaymentWhenTargetValidationFails(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)

	service := NewProcurementCoreService(docs, nil, models, nil)
	payment, err := docs.Create("payment_out", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_id":        "vendor-1",
		"vendor_name":      "Supplier A",
		"payment_date":     "2026-03-27",
		"amount_paid":      220.0,
		"refunded_amount":  0.0,
		"unapplied_amount": 220.0,
		"allocations":      []map[string]any{},
	})
	if err != nil {
		t.Fatalf("create payment out: %v", err)
	}
	payment.Header.Status = "paid"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment out: %v", err)
	}

	err = service.AllocatePaymentOut(payment.Header.ID, "doc_missing_bill", 100.0, "user_admin")
	if err == nil {
		t.Fatal("expected allocation validation error")
	}

	reloadedPayment, err := docs.Get(payment.Header.ID)
	if err != nil {
		t.Fatalf("reload payment out: %v", err)
	}
	if got := numberValue(reloadedPayment.Body.Payload["unapplied_amount"]); got != 220.0 {
		t.Fatalf("expected unapplied amount 220, got %v", got)
	}
	if got := len(recordList(reloadedPayment.Body.Payload["allocations"])); got != 0 {
		t.Fatalf("expected no persisted allocations, got %d", got)
	}
}

func TestVendorBillPostingLinesFromPurchaseOrderDoNotCapitalizeInventoryBeforeReceipt(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "COFFEE-BEAN-PO",
		"name":                         "Coffee Bean PO",
		"inventory_enabled":            true,
		"inventory_asset_account_code": "1200-INV-COFFEE-PO",
	}); err != nil {
		t.Fatalf("create inventory item: %v", err)
	}

	service := NewProcurementCoreService(docs, nil, models, nil)
	lines := service.vendorBillPostingLines(map[string]any{
		"source_purchase_order_id": "doc_po_1",
		"subtotal_amount":          1500.0,
		"tax_amount":               165.0,
		"total_amount":             1665.0,
		"lines": []map[string]any{{
			"item_code":                    "COFFEE-BEAN-PO",
			"line_subtotal":                1500.0,
			"inventory_asset_account_code": "1200-INV-COFFEE-PO",
		}},
	})

	var inventoryDebit, expenseDebit float64
	for _, line := range lines {
		switch textValue(line["account_code"]) {
		case "1200-INV-COFFEE-PO":
			inventoryDebit = numberValue(line["debit"])
		case "5000-EXP":
			expenseDebit = numberValue(line["debit"])
		}
	}
	if inventoryDebit != 0 {
		t.Fatalf("expected no inventory capitalization before receipt, got %v", inventoryDebit)
	}
	if expenseDebit != 1500.0 {
		t.Fatalf("expected expense debit 1500 before receipt, got %v", expenseDebit)
	}
}

func TestNormalizeReceiptLinesAllocatesLandedCostIntoEffectiveUnitCost(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "LANDED-ITEM",
		"name":              "Landed Cost Item",
		"inventory_enabled": true,
	}); err != nil {
		t.Fatalf("create inventory item: %v", err)
	}

	service := NewProcurementCoreService(docs, nil, models, nil)
	normalized := service.NormalizePayload("goods_receipt", map[string]any{
		"currency_code": "IDR",
		"lines": []map[string]any{{
			"item_code":      "LANDED-ITEM",
			"warehouse_code": "MAIN",
			"receipt_qty":    10.0,
			"unit_price":     100.0,
			"tax_rate":       0.0,
			"tax_mode":       "exclusive",
		}},
		"landed_cost_lines": []map[string]any{{
			"cost_type":        "freight",
			"description":      "Freight",
			"amount":           200.0,
			"allocation_basis": "line_value",
		}},
	})
	line := recordList(normalized["lines"])[0]
	if got := numberValue(line["allocated_landed_cost"]); got != 200.0 {
		t.Fatalf("expected allocated landed cost 200, got %v", got)
	}
	if got := numberValue(line["effective_unit_cost"]); got != 120.0 {
		t.Fatalf("expected effective unit cost 120, got %v", got)
	}
	if got := numberValue(normalized["total_amount"]); got != 1200.0 {
		t.Fatalf("expected total amount 1200, got %v", got)
	}
}

func TestHandleIssuedVendorBillAdjustsOnHandInventoryAndVariance(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)
	for _, def := range []document.Definition{
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
	for _, def := range []model.Definition{
		{
			Key:         "inventory_cost_layer",
			DisplayName: "Inventory Cost Layer",
			DefaultSort: "effective_at",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "quantity_delta", Type: "number"},
				{Key: "unit_cost", Type: "number"},
				{Key: "total_cost", Type: "number"},
				{Key: "effective_at", Type: "string"},
			},
		},
		{
			Key:         "inventory_valuation_snapshot",
			DisplayName: "Inventory Valuation Snapshot",
			DefaultSort: "item_code",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "quantity_on_hand", Type: "number"},
				{Key: "average_unit_cost", Type: "number"},
				{Key: "inventory_value", Type: "number"},
				{Key: "valuation_method", Type: "string"},
				{Key: "last_calculated_at", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "PPV-ITEM",
		"name":                         "PPV Item",
		"inventory_enabled":            true,
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-PPV",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("inventory_valuation_snapshot", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_main",
		"item_code":         "PPV-ITEM",
		"warehouse_code":    "MAIN",
		"quantity_on_hand":  5.0,
		"average_unit_cost": 100.0,
		"inventory_value":   500.0,
	}); err != nil {
		t.Fatalf("create valuation snapshot: %v", err)
	}
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "PPV-ITEM",
		"warehouse_code":     "MAIN",
		"quantity_delta":     5.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
		"unit_cost":          100.0,
		"total_cost":         500.0,
	})

	receipt, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"currency_code": "IDR",
		"lines": []map[string]any{{
			"item_code":           "PPV-ITEM",
			"warehouse_code":      "MAIN",
			"receipt_qty":         10.0,
			"unit_price":          100.0,
			"effective_unit_cost": 100.0,
		}},
	})
	if err != nil {
		t.Fatalf("create receipt: %v", err)
	}
	receipt.Header.Status = "received"
	if err := docs.Save(receipt); err != nil {
		t.Fatalf("save receipt: %v", err)
	}

	billPayload := map[string]any{
		"vendor_id":               "vendor-1",
		"vendor_name":             "Supplier A",
		"currency_code":           "IDR",
		"source_goods_receipt_id": receipt.Header.ID,
		"payable_account_code":    "2000-AP",
		"subtotal_amount":         1100.0,
		"tax_amount":              0.0,
		"total_amount":            1100.0,
		"lines": []map[string]any{{
			"item_code":                    "PPV-ITEM",
			"warehouse_code":               "MAIN",
			"quantity":                     10.0,
			"line_subtotal":                1100.0,
			"inventory_asset_account_code": "1200-INV-PPV",
			"receipt_unit_cost":            100.0,
		}},
	}
	bill, err := docs.Create("vendor_bill", "org_default", "loc_main", "user_admin", billPayload)
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}
	bill.Header.Status = "issued"
	if err := docs.Save(bill); err != nil {
		t.Fatalf("save bill: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, config.NewService(), models, nil)
	service := NewProcurementCoreService(docs, config.NewService(), models, nil)
	service.SetInventoryCore(inventorySvc)
	if err := service.HandleApprovedDocument(bill, "user_admin"); err != nil {
		t.Fatalf("handle issued bill: %v", err)
	}

	items, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters:  map[string]string{"organization_id": "org_default", "location_id": "loc_main", "item_code": "PPV-ITEM", "warehouse_code": "MAIN"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list valuation snapshot: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 valuation snapshot, got %d", len(items))
	}
	if got := numberValue(items[0].Values["inventory_value"]); got != 550.0 {
		t.Fatalf("expected inventory value 550 after adjustment, got %v", got)
	}
	if got := numberValue(items[0].Values["average_unit_cost"]); got != 110.0 {
		t.Fatalf("expected average unit cost 110 after adjustment, got %v", got)
	}
	postings := 0
	var inventoryDebit, varianceDebit float64
	for _, record := range docs.List() {
		if record.Header.Type != "ledger_posting" {
			continue
		}
		postings++
		for _, line := range recordList(record.Body.Payload["journal_lines"]) {
			switch textValue(line["account_code"]) {
			case "1200-INV-PPV":
				inventoryDebit += numberValue(line["debit"])
			case "5100-PPV":
				varianceDebit += numberValue(line["debit"])
			}
		}
	}
	if postings != 1 {
		t.Fatalf("expected 1 ledger posting, got %d", postings)
	}
	if inventoryDebit != 1050.0 {
		t.Fatalf("expected inventory debit 1050, got %v", inventoryDebit)
	}
	if varianceDebit != 50.0 {
		t.Fatalf("expected purchase variance debit 50, got %v", varianceDebit)
	}
}

func TestHandleIssuedVendorBillAppliesInventoryVarianceOnlyOnceOnRetry(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterProcurementGenerationDocumentTypes(t, docs)
	mustRegisterProcurementGenerationModels(t, models)
	for _, def := range []document.Definition{
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
	for _, def := range []model.Definition{
		{
			Key:         "inventory_cost_layer",
			DisplayName: "Inventory Cost Layer",
			DefaultSort: "effective_at",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "source_document_type", Type: "string"},
				{Key: "source_document_id", Type: "string"},
				{Key: "event_type", Type: "string"},
				{Key: "quantity_delta", Type: "number"},
				{Key: "unit_cost", Type: "number"},
				{Key: "total_cost", Type: "number"},
				{Key: "effective_at", Type: "string"},
			},
		},
		{
			Key:         "inventory_valuation_snapshot",
			DisplayName: "Inventory Valuation Snapshot",
			DefaultSort: "item_code",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "item_code", Type: "string"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "quantity_on_hand", Type: "number"},
				{Key: "average_unit_cost", Type: "number"},
				{Key: "inventory_value", Type: "number"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                          "BLOCKED-PPV",
		"name":                         "Blocked PPV Item",
		"inventory_enabled":            true,
		"uom_code":                     "EA",
		"inventory_asset_account_code": "1200-INV-BLOCKED",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("inventory_valuation_snapshot", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_main",
		"item_code":         "BLOCKED-PPV",
		"warehouse_code":    "MAIN",
		"quantity_on_hand":  5.0,
		"average_unit_cost": 100.0,
		"inventory_value":   500.0,
	}); err != nil {
		t.Fatalf("create valuation snapshot: %v", err)
	}
	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "BLOCKED-PPV",
		"warehouse_code":     "MAIN",
		"quantity_delta":     5.0,
		"movement_reason":    "seed",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
		"movement_direction": "in",
		"unit_cost":          100.0,
		"total_cost":         500.0,
	})
	receipt, err := docs.Create("goods_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"currency_code": "IDR",
		"lines": []map[string]any{{
			"item_code":           "BLOCKED-PPV",
			"warehouse_code":      "MAIN",
			"receipt_qty":         10.0,
			"unit_price":          100.0,
			"effective_unit_cost": 100.0,
		}},
	})
	if err != nil {
		t.Fatalf("create receipt: %v", err)
	}
	receipt.Header.Status = "received"
	if err := docs.Save(receipt); err != nil {
		t.Fatalf("save receipt: %v", err)
	}

	bill, err := docs.Create("vendor_bill", "org_default", "loc_main", "user_admin", map[string]any{
		"vendor_id":               "vendor-1",
		"vendor_name":             "Supplier A",
		"currency_code":           "IDR",
		"source_goods_receipt_id": receipt.Header.ID,
		"total_amount":            1100.0,
		"lines": []map[string]any{{
			"item_code":                    "BLOCKED-PPV",
			"warehouse_code":               "MAIN",
			"quantity":                     10.0,
			"line_subtotal":                1100.0,
			"inventory_asset_account_code": "1200-INV-BLOCKED",
			"receipt_unit_cost":            100.0,
		}},
	})
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}
	bill.Header.Status = "issued"
	if err := docs.Save(bill); err != nil {
		t.Fatalf("save bill: %v", err)
	}

	inventorySvc := NewInventoryCoreService(docs, config.NewService(), models, nil)
	service := NewProcurementCoreService(docs, config.NewService(), models, nil)
	service.SetInventoryCore(inventorySvc)
	if err := service.HandleApprovedDocument(bill, "user_admin"); err != nil {
		t.Fatalf("first approval failed: %v", err)
	}
	if err := service.HandleApprovedDocument(bill, "user_admin"); err != nil {
		t.Fatalf("second approval failed: %v", err)
	}

	items, _, err := models.List("inventory_valuation_snapshot", model.Query{
		Filters:  map[string]string{"organization_id": "org_default", "location_id": "loc_main", "item_code": "BLOCKED-PPV", "warehouse_code": "MAIN"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list valuation snapshot: %v", err)
	}
	if got := numberValue(items[0].Values["inventory_value"]); got != 550.0 {
		t.Fatalf("expected inventory value adjusted once to 550, got %v", got)
	}
	costLayers, _, err := models.List("inventory_cost_layer", model.Query{
		Filters: map[string]string{
			"source_document_type": "vendor_bill",
			"source_document_id":   bill.Header.ID,
			"event_type":           "vendor_bill_variance",
		},
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list variance cost layers: %v", err)
	}
	if len(costLayers) != 1 {
		t.Fatalf("expected one variance cost layer after retry, got %d", len(costLayers))
	}
}

func TestLookupReceiptUnitCostFallsBackToMatchingLineWhenIndexMissing(t *testing.T) {
	service := NewProcurementCoreService(document.NewService(), nil, model.NewService(), nil)
	receiptPayload := map[string]any{
		"lines": []map[string]any{
			{"item_code": "ITEM-A", "warehouse_code": "MAIN", "effective_unit_cost": 100.0},
			{"item_code": "ITEM-B", "warehouse_code": "SECOND", "effective_unit_cost": 200.0},
		},
	}
	got := service.lookupReceiptUnitCost(map[string]any{
		"item_code":      "ITEM-B",
		"warehouse_code": "SECOND",
	}, receiptPayload)
	if got != 200.0 {
		t.Fatalf("expected matching receipt unit cost 200, got %v", got)
	}
}

func mustRegisterProcurementGenerationDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "purchase_order", DisplayName: "Purchase Order", SchemaVersion: "v1", AllowedLinkTypes: []string{"receipt_for", "bill_for", "posting_for"}},
		{Type: "goods_receipt", DisplayName: "Goods Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"receipt_for", "bill_for", "movement_for"}},
		{Type: "vendor_bill", DisplayName: "Vendor Bill", SchemaVersion: "v1", AllowedLinkTypes: []string{"bill_for", "posting_for"}},
		{Type: "payment_out", DisplayName: "Payment Out", SchemaVersion: "v1", AllowedLinkTypes: []string{"payment_for", "posting_for"}},
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
				{Key: "inventory_enabled", Type: "bool"},
				{Key: "inventory_asset_account_code", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}
