package application

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/workflow"
)

func TestPOSCheckoutCreatesOperationalDocuments(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "POS-TSHIRT",
		"name":                 "POS T-Shirt",
		"description":          "Cashier stocked shirt",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           25.0,
		"tax_code":             "VAT11",
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    true,
		"allow_negative_stock": false,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "CARD",
		"name":                  "Card",
		"clearing_account_code": "1010-CARD",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create card payment method: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"checkout_mode":     "invoice_first",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}
	if _, err := models.Create("pos_tender_type", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   "CASH",
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}); err != nil {
		t.Fatalf("create tender type: %v", err)
	}
	if _, err := models.Create("pos_tender_type", "user_admin", map[string]any{
		"code":                  "CARD",
		"name":                  "Card",
		"kind":                  "card",
		"payment_method_code":   "CARD",
		"clearing_account_code": "1010-CARD",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create card tender type: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "POS-TSHIRT",
		"warehouse_code":     "MAIN",
		"quantity_delta":     5.0,
		"movement_reason":    "seed",
		"movement_direction": "in",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
	})

	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	commercialSvc := NewCommercialCoreService(docs, nil, models, nil)
	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	fulfillmentSvc := NewFulfillmentCoreService(docs, nil, inventorySvc)
	returnsSvc := NewReturnsCoreService(docs, nil, inventorySvc, commercialSvc, fulfillmentSvc)
	posSvc := NewPOSCoreService(docs, models, nil, actions, commercialSvc, inventorySvc, fulfillmentSvc, returnsSvc)

	shift, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 100.0, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}
	result, err := posSvc.Checkout("org_default", "loc_main", POSCheckoutInput{
		StoreCode:    "STORE1",
		RegisterCode: "REG1",
		ShiftID:      shift.ID,
		Lines: []POSCartLineInput{{
			ItemCode: "POS-TSHIRT",
			Quantity: 2,
		}},
		Tenders: []POSTenderInput{{
			TenderTypeCode: "CASH",
			Amount:         60,
		}},
	}, "cashier_1")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if got := textValue(result.Sale.Values["status"]); got != "completed" {
		t.Fatalf("expected completed sale, got %q", got)
	}
	if result.Order == nil || result.Invoice == nil || result.Fulfillment == nil {
		t.Fatalf("expected order, invoice, and fulfillment to be created")
	}
	if len(result.Payments) != 1 {
		t.Fatalf("expected 1 payment receipt, got %d", len(result.Payments))
	}
	balances := inventorySvc.currentBalances("org_default", "loc_main")
	if got := inventorySvc.sumBalance(balances, "POS-TSHIRT", "MAIN", ""); got != 3.0 {
		t.Fatalf("expected stock 3 after checkout, got %v", got)
	}
	postings := 0
	for _, record := range docs.List() {
		if record.Header.Type == "ledger_posting" {
			postings++
		}
	}
	if postings < 2 {
		t.Fatalf("expected at least 2 ledger postings, got %d", postings)
	}
}

func TestPOSCheckoutCreatesPromotionRedemptionFromPromoCode(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "POS-PROMO",
		"name":                 "Promo Tee",
		"description":          "Promo POS item",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           100,
		"tax_code":             "VAT11",
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    true,
		"allow_negative_stock": false,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":          "REG1",
		"name":          "Register 1",
		"store_code":    "STORE1",
		"checkout_mode": "invoice_first",
		"status":        "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}
	if _, err := models.Create("pos_tender_type", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   "CASH",
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}); err != nil {
		t.Fatalf("create tender type: %v", err)
	}
	if _, err := models.Create("promotion_campaign", "user_admin", map[string]any{
		"code":           "POS10",
		"name":           "POS Promo",
		"trigger_mode":   "code",
		"sales_channels": "pos",
		"store_codes":    "STORE1",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if _, err := models.Create("promotion_code", "user_admin", map[string]any{
		"code":                    "POS10",
		"promotion_campaign_code": "POS10",
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create promotion code: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":                    "POS10-RULE",
		"name":                    "POS 10 Percent",
		"promotion_campaign_code": "POS10",
		"scope":                   "line",
		"rule_kind":               "line_percent",
		"item_codes":              "POS-PROMO",
		"discount_percent":        10.0,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create discount rule: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "POS-PROMO",
		"warehouse_code":     "MAIN",
		"quantity_delta":     3.0,
		"movement_reason":    "seed",
		"movement_direction": "in",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
	})

	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	commercialSvc := NewCommercialCoreService(docs, nil, models, nil)
	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	fulfillmentSvc := NewFulfillmentCoreService(docs, nil, inventorySvc)
	returnsSvc := NewReturnsCoreService(docs, nil, inventorySvc, commercialSvc, fulfillmentSvc)
	posSvc := NewPOSCoreService(docs, models, nil, actions, commercialSvc, inventorySvc, fulfillmentSvc, returnsSvc)

	shift, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 100, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}
	result, err := posSvc.Checkout("org_default", "loc_main", POSCheckoutInput{
		StoreCode:      "STORE1",
		RegisterCode:   "REG1",
		ShiftID:        shift.ID,
		PromotionCodes: []string{"POS10"},
		Lines: []POSCartLineInput{{
			ItemCode: "POS-PROMO",
			Quantity: 1,
		}},
		Tenders: []POSTenderInput{{
			TenderTypeCode: "CASH",
			Amount:         99.9,
		}},
	}, "cashier_1")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if got := numberValue(result.Invoice.Body.Payload["discount_amount_total"]); got != 10 {
		t.Fatalf("expected invoice discount 10, got %v", got)
	}
	items, _, err := models.List("promotion_redemption", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list redemptions: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 promotion redemption, got %d", len(items))
	}
	if got := textValue(items[0].Values["promotion_code"]); got != "POS10" {
		t.Fatalf("expected redemption code POS10, got %q", got)
	}
}

func TestPOSValidatePromotionCodesReportsAppliedAndRejectsInvalid(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "POS-PROMO",
		"name":                 "Promo Tee",
		"description":          "Promo POS item",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           100,
		"tax_code":             "VAT11",
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    true,
		"allow_negative_stock": false,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":          "REG1",
		"name":          "Register 1",
		"store_code":    "STORE1",
		"checkout_mode": "invoice_first",
		"status":        "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}
	if _, err := models.Create("pos_tender_type", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   "CASH",
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}); err != nil {
		t.Fatalf("create tender type: %v", err)
	}
	if _, err := models.Create("promotion_campaign", "user_admin", map[string]any{
		"code":           "POS10",
		"name":           "POS Promo",
		"trigger_mode":   "code",
		"sales_channels": "pos",
		"store_codes":    "STORE1",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if _, err := models.Create("promotion_code", "user_admin", map[string]any{
		"code":                    "POS10",
		"promotion_campaign_code": "POS10",
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create promotion code: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":                    "POS10-RULE",
		"name":                    "POS 10 Percent",
		"promotion_campaign_code": "POS10",
		"scope":                   "line",
		"rule_kind":               "line_percent",
		"item_codes":              "POS-PROMO",
		"discount_percent":        10.0,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create discount rule: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "POS-PROMO",
		"warehouse_code":     "MAIN",
		"quantity_delta":     3.0,
		"movement_reason":    "seed",
		"movement_direction": "in",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
	})

	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	commercialSvc := NewCommercialCoreService(docs, nil, models, nil)
	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	fulfillmentSvc := NewFulfillmentCoreService(docs, nil, inventorySvc)
	returnsSvc := NewReturnsCoreService(docs, nil, inventorySvc, commercialSvc, fulfillmentSvc)
	posSvc := NewPOSCoreService(docs, models, nil, actions, commercialSvc, inventorySvc, fulfillmentSvc, returnsSvc)

	validation, err := posSvc.ValidatePromotionCodes("org_default", "loc_main", "STORE1", "", "", []string{"POS10"}, []POSCartLineInput{{
		ItemCode: "POS-PROMO",
		Quantity: 1,
	}})
	if err != nil {
		t.Fatalf("validate promotions: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid promotion validation, got %+v", validation)
	}
	if len(validation.Codes) != 1 || validation.Codes[0].Status != "applied" {
		t.Fatalf("expected applied validation code, got %+v", validation.Codes)
	}
	if validation.DiscountAmountTotal != 10 {
		t.Fatalf("expected discount preview 10, got %+v", validation)
	}

	shift, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 100, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}
	if _, err := posSvc.Checkout("org_default", "loc_main", POSCheckoutInput{
		StoreCode:      "STORE1",
		RegisterCode:   "REG1",
		ShiftID:        shift.ID,
		PromotionCodes: []string{"BADCODE"},
		Lines: []POSCartLineInput{{
			ItemCode: "POS-PROMO",
			Quantity: 1,
		}},
		Tenders: []POSTenderInput{{
			TenderTypeCode: "CASH",
			Amount:         110,
		}},
	}, "cashier_1"); err == nil || !strings.Contains(err.Error(), "promotion validation failed") {
		t.Fatalf("expected checkout promotion validation error, got %v", err)
	}
}

func TestPOSBuildOrderPayloadUsesBusinessTimezoneTimestamp(t *testing.T) {
	models := model.NewService()
	docs := document.NewService()
	cfg := config.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "POS-TIME",
		"name":                 "POS Time Item",
		"description":          "Timezone test item",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           10.0,
		"tax_code":             "VAT11",
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    false,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	commercialSvc := NewCommercialCoreService(docs, cfg, models, nil)
	posSvc := NewPOSCoreService(docs, models, nil, nil, commercialSvc, nil, nil, nil)

	store := model.Record{Values: map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
	}}
	payload, _, err := posSvc.buildOrderPayload(store, "", "", "", nil, []POSCartLineInput{{
		ItemCode: "POS-TIME",
		Quantity: 1,
	}})
	if err != nil {
		t.Fatalf("build order payload: %v", err)
	}
	orderTime, err := time.Parse(time.RFC3339, textValue(payload["order_datetime"]))
	if err != nil {
		t.Fatalf("parse order datetime: %v", err)
	}
	if got := orderTime.Format("-07:00"); got != "+07:00" {
		t.Fatalf("expected POS order datetime to use business timezone +07:00, got %s (%s)", got, orderTime.Format(time.RFC3339))
	}
	if got := textValue(payload["order_date"]); got != orderTime.Format("2006-01-02") {
		t.Fatalf("expected order date %s to match order datetime day, got %s", orderTime.Format("2006-01-02"), got)
	}
}

func TestPOSBuildOrderPayloadRejectsUnknownCustomer(t *testing.T) {
	models := model.NewService()
	mustRegisterPOSTestModels(t, models)
	if err := models.Register(model.Definition{
		Key:         "party",
		DisplayName: "Party",
		DefaultSort: "name",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register party model: %v", err)
	}

	store, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "POS-KOPI",
		"name":              "POS Kopi",
		"kind":              "product",
		"uom_code":          "EA",
		"inventory_enabled": true,
		"status":            "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	posSvc := &POSCoreService{models: models}
	_, _, err = posSvc.buildOrderPayload(store, "party-missing", "Ghost Customer", "", nil, []POSCartLineInput{{
		ItemCode: "POS-KOPI",
		Quantity: 1,
	}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "party not found") {
		t.Fatalf("expected party not found validation, got %v", err)
	}
}

func TestPOSBuildOrderPayloadAllowsGenericPartyID(t *testing.T) {
	models := model.NewService()
	mustRegisterPOSTestModels(t, models)
	if err := models.Register(model.Definition{
		Key:         "party",
		DisplayName: "Party",
		DefaultSort: "name",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register party model: %v", err)
	}

	store, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "POS-KOPI",
		"name":              "POS Kopi",
		"kind":              "product",
		"uom_code":          "EA",
		"inventory_enabled": true,
		"status":            "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	party, err := models.Create("party", "user_admin", map[string]any{"name": "Walk-in Customer"})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}

	posSvc := &POSCoreService{models: models, commercial: NewCommercialCoreService(document.NewService(), nil, models, nil)}
	payload, orderLines, err := posSvc.buildOrderPayload(store, party.ID, "Walk-in Customer", "", nil, []POSCartLineInput{{
		ItemCode: "POS-KOPI",
		Quantity: 1,
	}})
	if err != nil {
		t.Fatalf("expected generic party to be accepted, got %v", err)
	}
	if got := textValue(payload["party_id"]); got != party.ID {
		t.Fatalf("expected party id %s, got %s", party.ID, got)
	}
	if len(orderLines) != 1 {
		t.Fatalf("expected 1 order line, got %d", len(orderLines))
	}
}

func TestPOSRefundSaleCreatesReturnFlow(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "POS-DRUG",
		"name":                 "POS Drug",
		"description":          "Counter medicine",
		"kind":                 "product",
		"uom_code":             "BOX",
		"unit_price":           20.0,
		"tax_code":             "VAT11",
		"revenue_account_code": "4010-REV-DRUG",
		"is_sellable":          true,
		"inventory_enabled":    true,
		"allow_negative_stock": false,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "CARD",
		"name":                  "Card",
		"clearing_account_code": "1010-CARD",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create card payment method: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"checkout_mode":     "invoice_first",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}
	if _, err := models.Create("pos_tender_type", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   "CASH",
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}); err != nil {
		t.Fatalf("create tender type: %v", err)
	}
	if _, err := models.Create("pos_tender_type", "user_admin", map[string]any{
		"code":                  "CARD",
		"name":                  "Card",
		"kind":                  "card",
		"payment_method_code":   "CARD",
		"clearing_account_code": "1010-CARD",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create card tender type: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "POS-DRUG",
		"warehouse_code":     "MAIN",
		"quantity_delta":     4.0,
		"movement_reason":    "seed",
		"movement_direction": "in",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
	})

	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	commercialSvc := NewCommercialCoreService(docs, nil, models, nil)
	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	fulfillmentSvc := NewFulfillmentCoreService(docs, nil, inventorySvc)
	returnsSvc := NewReturnsCoreService(docs, nil, inventorySvc, commercialSvc, fulfillmentSvc)
	posSvc := NewPOSCoreService(docs, models, nil, actions, commercialSvc, inventorySvc, fulfillmentSvc, returnsSvc)

	shift, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 0, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}
	result, err := posSvc.Checkout("org_default", "loc_main", POSCheckoutInput{
		StoreCode:    "STORE1",
		RegisterCode: "REG1",
		ShiftID:      shift.ID,
		Lines: []POSCartLineInput{{
			ItemCode: "POS-DRUG",
			Quantity: 1,
		}},
		Tenders: []POSTenderInput{{
			TenderTypeCode: "CARD",
			Amount:         10,
		}, {
			TenderTypeCode: "CASH",
			Amount:         15,
		}},
	}, "cashier_1")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	payload, err := posSvc.RefundSale(result.Sale.ID, "cashier_1")
	if err != nil {
		t.Fatalf("refund sale: %v", err)
	}
	if payload["sales_return"] == nil || payload["return_receipt"] == nil || payload["credit_note"] == nil || payload["payment_refund"] == nil {
		t.Fatalf("expected refund payload to include return, receipt, credit note, and refund")
	}
	refund, ok := payload["payment_refund"].(document.Record)
	if !ok {
		t.Fatalf("expected payment_refund document record")
	}
	balances := inventorySvc.currentBalances("org_default", "loc_main")
	if got := inventorySvc.sumBalance(balances, "POS-DRUG", "MAIN", ""); got != 4.0 {
		t.Fatalf("expected stock to return to 4, got %v", got)
	}
	var posting document.Record
	foundPosting := false
	for _, link := range refund.Links {
		if link.LinkType != "posting_for" {
			continue
		}
		record, err := docs.Get(link.LinkedDocumentID)
		if err != nil || record.Header.Type != "ledger_posting" {
			continue
		}
		posting = record
		foundPosting = true
		break
	}
	if !foundPosting {
		t.Fatalf("expected refund posting to be created")
	}
	credits := map[string]float64{}
	for _, line := range recordList(posting.Body.Payload["journal_lines"]) {
		credits[textValue(line["account_code"])] = roundMoney(credits[textValue(line["account_code"])] + numberValue(line["credit"]))
	}
	if got := credits["1010-CARD"]; got != 10.0 {
		t.Fatalf("expected card clearing credit 10, got %v", got)
	}
	if got := credits["1000-CASH"]; got != 10.0 {
		t.Fatalf("expected cash clearing credit 10, got %v", got)
	}
}

func TestPOSCheckoutAppliesBuyXGetYDiscount(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                     "POS-SODA",
		"name":                    "POS Soda",
		"description":             "Promo soda",
		"kind":                    "product",
		"uom_code":                "EA",
		"unit_price":              12.0,
		"tax_code":                "VAT11",
		"revenue_account_code":    "4020-REV-FOOD",
		"is_sellable":             true,
		"inventory_enabled":       true,
		"inventory_tracking_mode": "quantity",
		"allow_negative_stock":    false,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("payment_method", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	}); err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":          "REG1",
		"name":          "Register 1",
		"store_code":    "STORE1",
		"checkout_mode": "invoice_first",
		"status":        "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}
	if _, err := models.Create("pos_tender_type", "user_admin", map[string]any{
		"code":                  "CASH",
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   "CASH",
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}); err != nil {
		t.Fatalf("create tender type: %v", err)
	}
	if _, err := models.Create("discount_rule", "user_admin", map[string]any{
		"code":            "B2G1",
		"name":            "Buy 2 Get 1",
		"scope":           "line",
		"rule_kind":       "bxgy",
		"item_codes":      "POS-SODA",
		"buy_quantity":    2,
		"reward_quantity": 1,
		"reward_percent":  100,
		"status":          "active",
	}); err != nil {
		t.Fatalf("create discount rule: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "POS-SODA",
		"warehouse_code":     "MAIN",
		"quantity_delta":     5.0,
		"movement_reason":    "seed",
		"movement_direction": "in",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
	})

	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))
	commercialSvc := NewCommercialCoreService(docs, nil, models, nil)
	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	fulfillmentSvc := NewFulfillmentCoreService(docs, nil, inventorySvc)
	returnsSvc := NewReturnsCoreService(docs, nil, inventorySvc, commercialSvc, fulfillmentSvc)
	posSvc := NewPOSCoreService(docs, models, nil, actions, commercialSvc, inventorySvc, fulfillmentSvc, returnsSvc)

	shift, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 100.0, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}
	result, err := posSvc.Checkout("org_default", "loc_main", POSCheckoutInput{
		StoreCode:    "STORE1",
		RegisterCode: "REG1",
		ShiftID:      shift.ID,
		Lines: []POSCartLineInput{{
			ItemCode: "POS-SODA",
			Quantity: 3,
		}},
		Tenders: []POSTenderInput{{
			TenderTypeCode: "CASH",
			Amount:         26.64,
		}},
	}, "cashier_1")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if got := numberValue(result.Invoice.Body.Payload["total_amount"]); got != 24 {
		t.Fatalf("expected discounted invoice total 24, got %v", got)
	}
	if got := numberValue(result.Invoice.Body.Payload["discount_amount_total"]); got != 12 {
		t.Fatalf("expected bxgy discount total 12, got %v", got)
	}
}

func TestPOSSearchCatalogUsesWarehouseBatchInventory(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":                  "POS-CATALOG",
		"name":                 "POS Catalog Item",
		"description":          "Catalog visibility test",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           10.0,
		"tax_code":             "VAT11",
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    true,
		"allow_negative_stock": false,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("tax_code", "user_admin", map[string]any{
		"code":             "VAT11",
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create tax code: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"default_tax_code": "VAT11",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}

	seedPostedMovement(t, docs, "org_default", "loc_main", map[string]any{
		"item_code":          "POS-CATALOG",
		"warehouse_code":     "MAIN",
		"quantity_delta":     7.0,
		"movement_reason":    "seed",
		"movement_direction": "in",
		"movement_date":      time.Now().UTC().Format("2006-01-02"),
	})

	commercialSvc := NewCommercialCoreService(docs, nil, models, nil)
	inventorySvc := NewInventoryCoreService(docs, nil, models, nil)
	posSvc := NewPOSCoreService(docs, models, nil, nil, commercialSvc, inventorySvc, nil, nil)

	items, err := posSvc.SearchCatalog("org_default", "loc_main", "STORE1", "POS-CATALOG")
	if err != nil {
		t.Fatalf("search catalog: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 catalog item, got %d", len(items))
	}
	if got := items[0].OnHandQuantity; got != 7.0 {
		t.Fatalf("expected on-hand 7, got %v", got)
	}
	if got := items[0].AvailableQuantity; got != 7.0 {
		t.Fatalf("expected available 7, got %v", got)
	}
}

func TestPOSSearchCustomersScansBeyondFirstPage(t *testing.T) {
	models := model.NewService()
	mustRegisterPOSTestModels(t, models)

	for i := 0; i < 105; i++ {
		name := "Customer"
		partyID := "party-generic"
		if i == 104 {
			name = "Target Customer"
			partyID = "party-target"
		}
		if _, err := models.Create("customer_profile", "user_admin", map[string]any{
			"party_id":      fmt.Sprintf("%s-%03d", partyID, i),
			"customer_name": name,
			"customer_type": "member",
			"member_status": "active",
			"member_tier":   "silver",
			"status":        "active",
		}); err != nil {
			t.Fatalf("create customer profile %d: %v", i, err)
		}
	}

	posSvc := NewPOSCoreService(document.NewService(), models, nil, nil, nil, nil, nil, nil)
	results, err := posSvc.SearchCustomers("Target Customer")
	if err != nil {
		t.Fatalf("search customers: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 matching customer, got %d", len(results))
	}
	if got := results[0].CustomerName; got != "Target Customer" {
		t.Fatalf("expected target customer, got %q", got)
	}
}

func TestPOSCloseShiftRequiresOwningCashier(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":           "STORE1",
		"name":           "Store 1",
		"warehouse_code": "MAIN",
		"currency_code":  "IDR",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}

	posSvc := NewPOSCoreService(docs, models, nil, nil, nil, nil, nil, nil)
	shift, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 100, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}
	if _, err := posSvc.CloseShift("org_default", "loc_main", shift.ID, "cashier_2", 100, ""); err == nil {
		t.Fatalf("expected foreign cashier close to fail")
	}
}

func TestPOSResumeShiftReturnsOpenedShiftRecord(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":           "STORE1",
		"name":           "Store 1",
		"warehouse_code": "MAIN",
		"currency_code":  "IDR",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}

	posSvc := NewPOSCoreService(docs, models, nil, nil, nil, nil, nil, nil)
	opened, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 100, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}

	resumed, err := posSvc.ResumeShift("STORE1", "REG1", opened.ID, "cashier_1")
	if err != nil {
		t.Fatalf("resume shift: %v", err)
	}
	if resumed.ID != opened.ID {
		t.Fatalf("expected resumed shift %q, got %q", opened.ID, resumed.ID)
	}
	if got := textValue(resumed.Values["status"]); got != "opened" {
		t.Fatalf("expected opened shift status, got %q", got)
	}
}

func TestPOSValidateSaleContextRejectsMismatchedStoreRegisterAndShift(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)

	if _, err := models.Create("commercial_item", "user_admin", map[string]any{
		"sku":               "POS-CONTEXT",
		"name":              "POS Context Item",
		"kind":              "product",
		"uom_code":          "EA",
		"unit_price":        10.0,
		"is_sellable":       true,
		"inventory_enabled": true,
		"status":            "active",
	}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE1",
		"name":             "Store 1",
		"warehouse_code":   "MAIN",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"default_tax_code": "",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store1: %v", err)
	}
	if _, err := models.Create("pos_store", "user_admin", map[string]any{
		"code":             "STORE2",
		"name":             "Store 2",
		"warehouse_code":   "ALT",
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"default_tax_code": "",
		"status":           "active",
	}); err != nil {
		t.Fatalf("create store2: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register1: %v", err)
	}
	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG2",
		"name":              "Register 2",
		"store_code":        "STORE2",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register2: %v", err)
	}

	posSvc := NewPOSCoreService(docs, models, nil, nil, NewCommercialCoreService(docs, nil, models, nil), NewInventoryCoreService(docs, nil, models, nil), nil, nil)
	shift, err := posSvc.OpenShift("org_default", "loc_main", "STORE1", "REG1", "cashier_1", "cashier_1", 0, "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}

	_, err = posSvc.HoldSale(POSHoldSaleInput{
		StoreCode:    "STORE2",
		RegisterCode: "REG1",
		ShiftID:      shift.ID,
		Lines:        []POSCartLineInput{{ItemCode: "POS-CONTEXT", Quantity: 1}},
	}, "cashier_1")
	if err == nil {
		t.Fatalf("expected mismatched store/register to fail")
	}

	_, err = posSvc.HoldSale(POSHoldSaleInput{
		StoreCode:    "STORE1",
		RegisterCode: "REG2",
		ShiftID:      shift.ID,
		Lines:        []POSCartLineInput{{ItemCode: "POS-CONTEXT", Quantity: 1}},
	}, "cashier_1")
	if err == nil {
		t.Fatalf("expected mismatched shift/register to fail")
	}
}

func TestPOSHeldSalesAndTransactionLookupAreCashierScoped(t *testing.T) {
	models := model.NewService()
	mustRegisterPOSTestModels(t, models)
	now := time.Now().UTC().Format(time.RFC3339)
	for _, values := range []map[string]any{
		{
			"sale_number":     "SALE-1",
			"status":          "held",
			"cashier_user_id": "cashier_1",
			"register_code":   "REG1",
			"shift_id":        "SHIFT1",
			"store_code":      "STORE1",
			"party_name":      "Alice",
		},
		{
			"sale_number":     "SALE-2",
			"status":          "held",
			"cashier_user_id": "cashier_2",
			"register_code":   "REG2",
			"shift_id":        "SHIFT2",
			"store_code":      "STORE2",
			"party_name":      "Bob",
		},
		{
			"sale_number":     "SALE-3",
			"status":          "completed",
			"cashier_user_id": "cashier_1",
			"register_code":   "REG1",
			"shift_id":        "SHIFT1",
			"store_code":      "STORE1",
			"party_name":      "Alice",
			"invoice_number":  "INV-1",
			"created_at":      now,
		},
		{
			"sale_number":     "SALE-4",
			"status":          "completed",
			"cashier_user_id": "cashier_2",
			"register_code":   "REG2",
			"shift_id":        "SHIFT2",
			"store_code":      "STORE2",
			"party_name":      "Bob",
			"invoice_number":  "INV-2",
			"created_at":      now,
		},
	} {
		if _, err := models.Create("pos_sale", "user_admin", values); err != nil {
			t.Fatalf("create pos sale: %v", err)
		}
	}

	posSvc := NewPOSCoreService(document.NewService(), models, nil, nil, nil, nil, nil, nil)
	held, err := posSvc.HeldSales("cashier_1", "", "")
	if err != nil {
		t.Fatalf("held sales: %v", err)
	}
	if len(held) != 1 || textValue(held[0].Values["cashier_user_id"]) != "cashier_1" {
		t.Fatalf("expected only cashier_1 held sales, got %+v", held)
	}
	transactions, err := posSvc.TransactionLookup("sale", "cashier_1", "", "")
	if err != nil {
		t.Fatalf("transaction lookup: %v", err)
	}
	if len(transactions) != 1 || textValue(transactions[0].Values["cashier_user_id"]) != "cashier_1" {
		t.Fatalf("expected only cashier_1 completed sales, got %+v", transactions)
	}
}

func mustRegisterPOSTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "sales_order", DisplayName: "Sales Order", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for", "return_for", "exchange_for"}},
		{Type: "invoice", DisplayName: "Invoice", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for", "return_for"}},
		{Type: "credit_note", DisplayName: "Credit Note", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for", "return_for"}},
		{Type: "payment_receipt", DisplayName: "Payment Receipt", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for"}},
		{Type: "payment_refund", DisplayName: "Payment Refund", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"source_order", "invoice_for", "payment_for", "refund_for", "posting_for", "fulfillment_for", "return_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"posting_for"}},
		{Type: "sales_fulfillment", DisplayName: "Sales Fulfillment", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"movement_for", "fulfillment_for", "return_for"}},
		{Type: "stock_movement", DisplayName: "Stock Movement", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"movement_for"}},
		{Type: "sales_return", DisplayName: "Sales Return", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"return_for", "exchange_for"}},
		{Type: "return_receipt", DisplayName: "Return Receipt", SchemaVersion: "v1", WorkflowKey: "generic_request_flow", AllowedLinkTypes: []string{"return_for", "movement_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func mustRegisterPOSTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{
			Key:         "commercial_item",
			DisplayName: "Commercial Item",
			DefaultSort: "sku",
			Fields: []model.FieldDefinition{
				{Key: "sku", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "description", Type: "string"},
				{Key: "product_code", Type: "string"},
				{Key: "is_variant", Type: "bool"},
				{Key: "variant_signature", Type: "string"},
				{Key: "item_type", Type: "string"},
				{Key: "kind", Type: "string", Required: true},
				{Key: "uom_code", Type: "string"},
				{Key: "base_price", Type: "number"},
				{Key: "unit_price", Type: "number"},
				{Key: "tax_code", Type: "string"},
				{Key: "revenue_account_code", Type: "string"},
				{Key: "expense_account_code", Type: "string"},
				{Key: "is_sellable", Type: "bool"},
				{Key: "inventory_enabled", Type: "bool"},
				{Key: "inventory_tracking_mode", Type: "string"},
				{Key: "expiry_tracking_enabled", Type: "bool"},
				{Key: "allow_negative_stock", Type: "bool"},
				{Key: "default_issue_strategy", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "inventory_batch",
			DisplayName: "Inventory Batch",
			DefaultSort: "batch_code",
			Fields: []model.FieldDefinition{
				{Key: "item_code", Type: "string", Required: true},
				{Key: "warehouse_code", Type: "string", Required: true},
				{Key: "batch_code", Type: "string", Required: true},
				{Key: "expiration_date", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "hold_reason", Type: "string"},
				{Key: "hold_notes", Type: "string"},
				{Key: "recall_reference", Type: "string"},
			},
		},
		{
			Key:         "tax_code",
			DisplayName: "Tax Code",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "rate_percent", Type: "number"},
				{Key: "mode", Type: "string"},
				{Key: "tax_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "payment_method",
			DisplayName: "Payment Method",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "clearing_account_code", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_store",
			DisplayName: "POS Store",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "warehouse_code", Type: "string", Required: true},
				{Key: "price_list_code", Type: "string"},
				{Key: "tax_profile_code", Type: "string"},
				{Key: "default_tax_code", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "checkout_mode", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_register",
			DisplayName: "POS Register",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "store_code", Type: "string", Required: true},
				{Key: "checkout_mode", Type: "string"},
				{Key: "cash_account_code", Type: "string"},
				{Key: "card_account_code", Type: "string"},
				{Key: "hardware_profile", Type: "string"},
				{Key: "receipt_template_key", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_tender_type",
			DisplayName: "POS Tender Type",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "kind", Type: "string", Required: true},
				{Key: "payment_method_code", Type: "string", Required: true},
				{Key: "clearing_account_code", Type: "string"},
				{Key: "requires_reference", Type: "bool"},
				{Key: "is_cash_like", Type: "bool"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "pos_shift",
			DisplayName: "POS Shift",
			DefaultSort: "shift_number",
			Fields: []model.FieldDefinition{
				{Key: "shift_number", Type: "string", Required: true},
				{Key: "store_code", Type: "string", Required: true},
				{Key: "register_code", Type: "string", Required: true},
				{Key: "cashier_user_id", Type: "string", Required: true},
				{Key: "opened_at", Type: "string"},
				{Key: "closed_at", Type: "string"},
				{Key: "opening_cash_amount", Type: "number"},
				{Key: "expected_cash_amount", Type: "number"},
				{Key: "actual_cash_amount", Type: "number"},
				{Key: "over_short_amount", Type: "number"},
				{Key: "status", Type: "string"},
				{Key: "notes", Type: "string"},
			},
		},
		{
			Key:         "pos_sale",
			DisplayName: "POS Sale",
			DefaultSort: "sale_number",
			Fields: []model.FieldDefinition{
				{Key: "sale_number", Type: "string", Required: true},
				{Key: "store_code", Type: "string", Required: true},
				{Key: "register_code", Type: "string", Required: true},
				{Key: "shift_id", Type: "string", Required: true},
				{Key: "cashier_user_id", Type: "string", Required: true},
				{Key: "party_id", Type: "string"},
				{Key: "party_name", Type: "string"},
				{Key: "checkout_mode", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "reference", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "subtotal_amount", Type: "number"},
				{Key: "tax_amount", Type: "number"},
				{Key: "total_amount", Type: "number"},
				{Key: "tendered_amount", Type: "number"},
				{Key: "change_due_amount", Type: "number"},
				{Key: "warehouse_code", Type: "string"},
				{Key: "price_list_code", Type: "string"},
				{Key: "tax_profile_code", Type: "string"},
				{Key: "promotion_codes_json", Type: "string"},
				{Key: "lines_json", Type: "string"},
				{Key: "tenders_json", Type: "string"},
				{Key: "source_document_type", Type: "string"},
				{Key: "source_document_id", Type: "string"},
				{Key: "order_id", Type: "string"},
				{Key: "order_number", Type: "string"},
				{Key: "invoice_id", Type: "string"},
				{Key: "invoice_number", Type: "string"},
				{Key: "fulfillment_id", Type: "string"},
				{Key: "fulfillment_number", Type: "string"},
				{Key: "payment_ids_json", Type: "string"},
				{Key: "device_id", Type: "string"},
				{Key: "offline_cached", Type: "bool"},
				{Key: "notes", Type: "string"},
			},
		},
		{
			Key:         "discount_rule",
			DisplayName: "Discount Rule",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string"},
				{Key: "promotion_campaign_code", Type: "string"},
				{Key: "scope", Type: "string"},
				{Key: "rule_kind", Type: "string"},
				{Key: "item_codes", Type: "string"},
				{Key: "member_statuses", Type: "string"},
				{Key: "buy_quantity", Type: "number"},
				{Key: "reward_quantity", Type: "number"},
				{Key: "reward_percent", Type: "number"},
				{Key: "discount_percent", Type: "number"},
				{Key: "minimum_order_total", Type: "number"},
				{Key: "minimum_line_quantity", Type: "number"},
				{Key: "fixed_price", Type: "number"},
				{Key: "priority", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "promotion_campaign",
			DisplayName: "Promotion Campaign",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "trigger_mode", Type: "string"},
				{Key: "start_at", Type: "string"},
				{Key: "end_at", Type: "string"},
				{Key: "sales_channels", Type: "string"},
				{Key: "store_codes", Type: "string"},
				{Key: "global_usage_cap", Type: "number"},
				{Key: "per_customer_usage_cap", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "promotion_code",
			DisplayName: "Promotion Code",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "promotion_campaign_code", Type: "string", Required: true},
				{Key: "start_at", Type: "string"},
				{Key: "end_at", Type: "string"},
				{Key: "party_ids", Type: "string"},
				{Key: "member_statuses", Type: "string"},
				{Key: "member_tiers", Type: "string"},
				{Key: "total_redemption_limit", Type: "number"},
				{Key: "per_customer_redemption_limit", Type: "number"},
				{Key: "total_redemptions", Type: "number"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "promotion_redemption",
			DisplayName: "Promotion Redemption",
			DefaultSort: "redeemed_at",
			Fields: []model.FieldDefinition{
				{Key: "promotion_campaign_code", Type: "string", Required: true},
				{Key: "promotion_code", Type: "string"},
				{Key: "source_document_type", Type: "string", Required: true},
				{Key: "source_document_id", Type: "string", Required: true},
				{Key: "party_id", Type: "string"},
				{Key: "sales_channel", Type: "string"},
				{Key: "store_code", Type: "string"},
				{Key: "discount_amount_total", Type: "number"},
				{Key: "redeemed_at", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "customer_profile",
			DisplayName: "Customer Profile",
			DefaultSort: "customer_name",
			Fields: []model.FieldDefinition{
				{Key: "party_id", Type: "string", Required: true},
				{Key: "party_name", Type: "string"},
				{Key: "customer_name", Type: "string"},
				{Key: "customer_type", Type: "string"},
				{Key: "member_status", Type: "string"},
				{Key: "member_tier", Type: "string"},
				{Key: "member_valid_to", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}
