package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
	"orbyte/internal/modules"
)

type posTerminalSeedManifest struct {
	SeededAt       string `json:"seeded_at"`
	OrganizationID string `json:"organization_id"`
	LocationID     string `json:"location_id"`
	StoreCode      string `json:"store_code"`
	StoreName      string `json:"store_name"`
	RegisterCode   string `json:"register_code"`
	RegisterName   string `json:"register_name"`
	ShiftID        string `json:"shift_id"`
	ShiftNumber    string `json:"shift_number"`
	Customer struct {
		PartyID      string `json:"party_id"`
		Name         string `json:"name"`
		MemberStatus string `json:"member_status"`
		MemberTier   string `json:"member_tier"`
	} `json:"customer"`
	Promotion struct {
		CampaignCode string `json:"campaign_code"`
		PromoCode    string `json:"promo_code"`
		DiscountRule string `json:"discount_rule"`
	} `json:"promotion"`
	Items []struct {
		SKU   string  `json:"sku"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	} `json:"items"`
	StoredValue struct {
		GiftCardCode       string  `json:"gift_card_code"`
		GiftCardBalance    float64 `json:"gift_card_balance"`
		StoreCreditBalance float64 `json:"store_credit_balance"`
	} `json:"stored_value"`
	Transactions struct {
		CompletedSaleID     string `json:"completed_sale_id"`
		CompletedSaleNumber string `json:"completed_sale_number"`
		HeldSaleID          string `json:"held_sale_id"`
		HeldSaleNumber      string `json:"held_sale_number"`
	} `json:"transactions"`
}

func TestSeedPOSTerminalSyntheticScenario(t *testing.T) {
	if os.Getenv("POS_SEED") != "1" {
		t.Skip("set POS_SEED=1 to seed the postgres-backed POS scenario")
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed POS seed")
	}

	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	manifests, err := modules.ForProfile(modules.ProfileAll)
	if err != nil {
		t.Fatalf("load business manifests: %v", err)
	}
	graph := constructServiceGraph(postgres, manifests)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, manifests, "bootstrap-123!"); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}

	orgID := "org_default"
	locID := "loc_hq"
	actorID := "user_admin"
	suffix := time.Now().UTC().Format("20060102150405")

	taxCode := ensureModelByCode(t, graph.models, "commercial_tax_code", "code", "VATPOS-"+suffix, map[string]any{
		"code":             "VATPOS-" + suffix,
		"name":             "VAT POS " + suffix,
		"rate_percent":     11.0,
		"mode":             "exclusive",
		"tax_account_code": "2100-VATOUT",
		"status":           "active",
	}, actorID)

	paymentCash := ensureModelByCode(t, graph.models, "payment_method", "code", "CASHPOS-"+suffix, map[string]any{
		"code":                  "CASHPOS-" + suffix,
		"name":                  "Cash POS " + suffix,
		"clearing_account_code": "1000-CASH",
		"status":                "active",
	}, actorID)
	paymentCard := ensureModelByCode(t, graph.models, "payment_method", "code", "CARDPOS-"+suffix, map[string]any{
		"code":                  "CARDPOS-" + suffix,
		"name":                  "Card POS " + suffix,
		"clearing_account_code": "1010-CARD",
		"status":                "active",
	}, actorID)
	paymentVoucher := ensureModelByCode(t, graph.models, "payment_method", "code", "VOUCHERPOS-"+suffix, map[string]any{
		"code":                  "VOUCHERPOS-" + suffix,
		"name":                  "Voucher POS " + suffix,
		"clearing_account_code": "1020-VOUCHER",
		"status":                "active",
	}, actorID)
	paymentGiftCard := ensureModelByCode(t, graph.models, "payment_method", "code", "GIFTPOS-"+suffix, map[string]any{
		"code":                  "GIFTPOS-" + suffix,
		"name":                  "Gift Card POS " + suffix,
		"clearing_account_code": "2250-GIFT-CARD",
		"status":                "active",
	}, actorID)
	paymentStoreCredit := ensureModelByCode(t, graph.models, "payment_method", "code", "STORECREDITPOS-"+suffix, map[string]any{
		"code":                  "STORECREDITPOS-" + suffix,
		"name":                  "Store Credit POS " + suffix,
		"clearing_account_code": "2260-STORE-CREDIT",
		"status":                "active",
	}, actorID)

	storeCode := "STORE-SEED-" + suffix
	registerCode := "REG-SEED-" + suffix
	storeRecord := ensureModelByCode(t, graph.models, "pos_store", "code", storeCode, map[string]any{
		"code":             storeCode,
		"name":             "Seed Counter " + suffix,
		"warehouse_code":   "MAIN",
		"default_tax_code": textValue(taxCode.Values["code"]),
		"currency_code":    "IDR",
		"checkout_mode":    "invoice_first",
		"status":           "active",
	}, actorID)
	registerRecord := ensureModelByCode(t, graph.models, "pos_register", "code", registerCode, map[string]any{
		"code":              registerCode,
		"name":              "Front Register " + suffix,
		"store_code":        storeCode,
		"checkout_mode":     "invoice_first",
		"cash_account_code": "1000-CASH",
		"card_account_code": "1010-CARD",
		"status":            "active",
	}, actorID)

	_ = paymentCash
	_ = paymentCard
	_ = paymentVoucher
	_ = paymentGiftCard
	_ = paymentStoreCredit
	ensureModelByCode(t, graph.models, "pos_tender_type", "code", "CASH-"+suffix, map[string]any{
		"code":                  "CASH-" + suffix,
		"name":                  "Cash",
		"kind":                  "cash",
		"payment_method_code":   textValue(paymentCash.Values["code"]),
		"clearing_account_code": "1000-CASH",
		"is_cash_like":          true,
		"status":                "active",
	}, actorID)
	ensureModelByCode(t, graph.models, "pos_tender_type", "code", "CARD-"+suffix, map[string]any{
		"code":                  "CARD-" + suffix,
		"name":                  "Card",
		"kind":                  "card",
		"payment_method_code":   textValue(paymentCard.Values["code"]),
		"clearing_account_code": "1010-CARD",
		"requires_reference":    true,
		"status":                "active",
	}, actorID)
	ensureModelByCode(t, graph.models, "pos_tender_type", "code", "VOUCHER-"+suffix, map[string]any{
		"code":                  "VOUCHER-" + suffix,
		"name":                  "Voucher",
		"kind":                  "voucher",
		"payment_method_code":   textValue(paymentVoucher.Values["code"]),
		"clearing_account_code": "1020-VOUCHER",
		"requires_reference":    true,
		"status":                "active",
	}, actorID)
	ensureModelByCode(t, graph.models, "pos_tender_type", "code", "GIFT-"+suffix, map[string]any{
		"code":                  "GIFT-" + suffix,
		"name":                  "Gift Card",
		"kind":                  "gift_card",
		"payment_method_code":   textValue(paymentGiftCard.Values["code"]),
		"clearing_account_code": "2250-GIFT-CARD",
		"requires_reference":    true,
		"status":                "active",
	}, actorID)
	ensureModelByCode(t, graph.models, "pos_tender_type", "code", "STORECREDIT-"+suffix, map[string]any{
		"code":                  "STORECREDIT-" + suffix,
		"name":                  "Store Credit",
		"kind":                  "store_credit",
		"payment_method_code":   textValue(paymentStoreCredit.Values["code"]),
		"clearing_account_code": "2260-STORE-CREDIT",
		"requires_party":        true,
		"status":                "active",
	}, actorID)

	customerPartyID := "party_pos_" + suffix
	customerRecord := ensureModelByCode(t, graph.models, "customer_profile", "party_id", customerPartyID, map[string]any{
		"party_id":         customerPartyID,
		"customer_name":    "Alya Santoso " + suffix,
		"customer_type":    "member",
		"member_status":    "active",
		"member_tier":      "gold",
		"member_valid_from": "2026-01-01",
		"member_valid_to":   "2099-12-31",
	}, actorID)

	promoCampaignCode := "PROMO-" + suffix
	promoCode := "APR-" + suffix
	ensureModelByCode(t, graph.models, "promotion_campaign", "code", promoCampaignCode, map[string]any{
		"code":           promoCampaignCode,
		"name":           "POS Spring Promo " + suffix,
		"trigger_mode":   "code",
		"sales_channels": "pos",
		"store_codes":    storeCode,
		"status":         "active",
	}, actorID)
	ensureModelByCode(t, graph.models, "promotion_code", "code", promoCode, map[string]any{
		"code":                    promoCode,
		"promotion_campaign_code": promoCampaignCode,
		"status":                  "active",
	}, actorID)

	itemCoffee := ensureModelByCode(t, graph.models, "commercial_item", "sku", "POS-ESPRESSO-"+suffix, map[string]any{
		"sku":                  "POS-ESPRESSO-" + suffix,
		"name":                 "Espresso Double " + suffix,
		"description":          "Fresh espresso for POS terminal demo",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           28000.0,
		"tax_code":             textValue(taxCode.Values["code"]),
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    false,
		"allow_negative_stock": true,
		"status":               "active",
	}, actorID)
	itemCroissant := ensureModelByCode(t, graph.models, "commercial_item", "sku", "POS-CROISSANT-"+suffix, map[string]any{
		"sku":                  "POS-CROISSANT-" + suffix,
		"name":                 "Butter Croissant " + suffix,
		"description":          "Counter pastry for POS terminal demo",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           22000.0,
		"tax_code":             textValue(taxCode.Values["code"]),
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    false,
		"allow_negative_stock": true,
		"status":               "active",
	}, actorID)
	itemBeans := ensureModelByCode(t, graph.models, "commercial_item", "sku", "POS-BEANS-"+suffix, map[string]any{
		"sku":                  "POS-BEANS-" + suffix,
		"name":                 "House Beans 1kg " + suffix,
		"description":          "Packaged beans for POS terminal demo",
		"kind":                 "product",
		"uom_code":             "EA",
		"unit_price":           95000.0,
		"tax_code":             textValue(taxCode.Values["code"]),
		"revenue_account_code": "4000-REV",
		"is_sellable":          true,
		"inventory_enabled":    false,
		"allow_negative_stock": true,
		"status":               "active",
	}, actorID)

	ensureModelByCode(t, graph.models, "discount_rule", "code", "PROMO-RULE-"+suffix, map[string]any{
		"code":                    "PROMO-RULE-" + suffix,
		"name":                    "Terminal Demo Discount " + suffix,
		"promotion_campaign_code": promoCampaignCode,
		"scope":                   "line",
		"rule_kind":               "line_percent",
		"item_codes":              textValue(itemCoffee.Values["sku"]),
		"discount_percent":        10.0,
		"status":                  "active",
	}, actorID)

	giftCardCode := "GC-POS-" + suffix
	issuedGiftCard, err := graph.retailFinance.IssueGiftCard(orgID, locID, actorID, map[string]any{
		"code":                 giftCardCode,
		"store_code":           storeCode,
		"party_id":             customerPartyID,
		"party_name":           textValue(customerRecord.Values["customer_name"]),
		"original_amount":      150000.0,
		"amount":               150000.0,
		"payment_account_code": "1000-CASH",
	})
	if err != nil {
		t.Fatalf("issue gift card: %v", err)
	}
	if _, err := graph.models.Create("store_credit_account", actorID, map[string]any{
		"organization_id":        orgID,
		"location_id":            locID,
		"party_id":               customerPartyID,
		"party_name":             textValue(customerRecord.Values["customer_name"]),
		"currency_code":          "IDR",
		"balance_amount":         90000.0,
		"liability_account_code": "2260-STORE-CREDIT",
		"status":                 "active",
	}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Fatalf("create store credit account: %v", err)
	}

	shift, err := graph.posCore.OpenShift(orgID, locID, storeCode, registerCode, actorID, actorID, 300000.0, "Synthetic terminal seed")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}

	checkoutResult, err := graph.posCore.Checkout(orgID, locID, application.POSCheckoutInput{
		StoreCode:      storeCode,
		RegisterCode:   registerCode,
		ShiftID:        shift.ID,
		PartyID:        customerPartyID,
		PartyName:      textValue(customerRecord.Values["customer_name"]),
		PromotionCodes: []string{promoCode},
		Lines: []application.POSCartLineInput{
			{ItemCode: textValue(itemCoffee.Values["sku"]), Quantity: 2},
			{ItemCode: textValue(itemCroissant.Values["sku"]), Quantity: 1},
		},
		Tenders: []application.POSTenderInput{
			{TenderTypeCode: "CASH-" + suffix, Amount: 50000.0},
			{TenderTypeCode: "CARD-" + suffix, Amount: 50000.0, Reference: "APPROVAL-" + suffix},
		},
		Reference: "SEED-CHECKOUT-" + suffix,
	}, actorID)
	if err != nil {
		t.Fatalf("checkout sale: %v", err)
	}

	heldSale, err := graph.posCore.HoldSale(application.POSHoldSaleInput{
		StoreCode:      storeCode,
		RegisterCode:   registerCode,
		ShiftID:        shift.ID,
		PartyID:        customerPartyID,
		PartyName:      textValue(customerRecord.Values["customer_name"]),
		PromotionCodes: []string{promoCode},
		Lines: []application.POSCartLineInput{
			{ItemCode: textValue(itemBeans.Values["sku"]), Quantity: 1},
		},
		Tenders: []application.POSTenderInput{
			{TenderTypeCode: "VOUCHER-" + suffix, Amount: 10000.0, Reference: "VOUCHER-" + suffix},
		},
		Reference: "SEED-HOLD-" + suffix,
	}, actorID)
	if err != nil {
		t.Fatalf("hold sale: %v", err)
	}

	storeCredit, _, err := graph.models.List("store_credit_account", model.Query{
		Filters:  map[string]string{"party_id": customerPartyID},
		Page:     1,
		PageSize: 5,
	})
	if err != nil || len(storeCredit) == 0 {
		t.Fatalf("load store credit account: %v", err)
	}
	giftCardRecord, ok := issuedGiftCard["gift_card"].(model.Record)
	if !ok {
		t.Fatalf("gift card response missing record")
	}

	manifest := posTerminalSeedManifest{
		SeededAt:       time.Now().UTC().Format(time.RFC3339),
		OrganizationID: orgID,
		LocationID:     locID,
		StoreCode:      storeCode,
		StoreName:      textValue(storeRecord.Values["name"]),
		RegisterCode:   registerCode,
		RegisterName:   textValue(registerRecord.Values["name"]),
		ShiftID:        shift.ID,
		ShiftNumber:    textValue(shift.Values["shift_number"]),
	}
	manifest.Customer.PartyID = customerPartyID
	manifest.Customer.Name = textValue(customerRecord.Values["customer_name"])
	manifest.Customer.MemberStatus = textValue(customerRecord.Values["member_status"])
	manifest.Customer.MemberTier = textValue(customerRecord.Values["member_tier"])
	manifest.Promotion.CampaignCode = promoCampaignCode
	manifest.Promotion.PromoCode = promoCode
	manifest.Promotion.DiscountRule = "PROMO-RULE-" + suffix
	manifest.Items = []struct {
		SKU   string  `json:"sku"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}{
		{SKU: textValue(itemCoffee.Values["sku"]), Name: textValue(itemCoffee.Values["name"]), Price: numberValue(itemCoffee.Values["unit_price"])},
		{SKU: textValue(itemCroissant.Values["sku"]), Name: textValue(itemCroissant.Values["name"]), Price: numberValue(itemCroissant.Values["unit_price"])},
		{SKU: textValue(itemBeans.Values["sku"]), Name: textValue(itemBeans.Values["name"]), Price: numberValue(itemBeans.Values["unit_price"])},
	}
	manifest.StoredValue.GiftCardCode = giftCardCode
	manifest.StoredValue.GiftCardBalance = numberValue(giftCardRecord.Values["remaining_balance"])
	manifest.StoredValue.StoreCreditBalance = numberValue(storeCredit[0].Values["balance_amount"])
	manifest.Transactions.CompletedSaleID = checkoutResult.Sale.ID
	manifest.Transactions.CompletedSaleNumber = textValue(checkoutResult.Sale.Values["sale_number"])
	manifest.Transactions.HeldSaleID = heldSale.ID
	manifest.Transactions.HeldSaleNumber = textValue(heldSale.Values["sale_number"])

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestPath := filepath.Join(os.TempDir(), "orbyte-pos-seed.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Logf("seeded POS terminal scenario: %s", manifestPath)
	t.Logf("store=%s register=%s customer=%s gift_card=%s promo=%s completed_sale=%s held_sale=%s",
		manifest.StoreCode,
		manifest.RegisterCode,
		manifest.Customer.Name,
		manifest.StoredValue.GiftCardCode,
		manifest.Promotion.PromoCode,
		manifest.Transactions.CompletedSaleNumber,
		manifest.Transactions.HeldSaleNumber,
	)
}

func ensureModelByCode(t *testing.T, models *model.Service, modelKey, fieldKey, fieldValue string, values map[string]any, actorID string) model.Record {
	t.Helper()
	items, _, err := models.List(modelKey, model.Query{
		Filters:  map[string]string{fieldKey: fieldValue},
		Page:     1,
		PageSize: 5,
	})
	if err == nil && len(items) > 0 {
		return items[0]
	}
	record, err := models.Create(modelKey, actorID, values)
	if err != nil {
		t.Fatalf("create %s %s=%s: %v", modelKey, fieldKey, fieldValue, err)
	}
	return record
}
