package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestRetailFinanceShiftReconciliationAndSettlement(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	cfg := config.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)
	mustRegisterRetailFinanceTestModels(t, models)

	if _, err := models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG1",
		"name":              "Register 1",
		"store_code":        "STORE1",
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}
	shift, err := models.Create("pos_shift", "user_admin", map[string]any{
		"organization_id":     "org_default",
		"location_id":         "loc_main",
		"shift_number":        "SHIFT-001",
		"store_code":          "STORE1",
		"register_code":       "REG1",
		"cashier_user_id":     "cashier_1",
		"opening_cash_amount": 50.0,
		"actual_cash_amount":  105.0,
		"status":              "closed",
	})
	if err != nil {
		t.Fatalf("create shift: %v", err)
	}
	if _, err := models.Create("pos_sale", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_main",
		"sale_number":     "SALE-001",
		"store_code":      "STORE1",
		"register_code":   "REG1",
		"shift_id":        shift.ID,
		"cashier_user_id": "cashier_1",
		"status":          "completed",
		"tenders_json":    `[{"tender_type_code":"CASH","kind":"cash","is_cash_like":true,"amount":60,"clearing_account_code":"1000-CASH"},{"tender_type_code":"CARD","kind":"card","amount":40,"clearing_account_code":"1010-CARD"}]`,
	}); err != nil {
		t.Fatalf("create pos sale: %v", err)
	}

	retailSvc := NewRetailFinanceCoreService(docs, models, cfg, NewFinanceReportingCoreService(docs, models, cfg))
	reconciliation, err := retailSvc.SyncShiftReconciliation("org_default", "loc_main", shift.ID, "user_admin")
	if err != nil {
		t.Fatalf("sync reconciliation: %v", err)
	}
	if got := numberValue(reconciliation.Values["expected_cash_amount"]); got != 110 {
		t.Fatalf("expected cash 110, got %v", got)
	}
	if got := numberValue(reconciliation.Values["over_short_amount"]); got != -5 {
		t.Fatalf("expected over short -5, got %v", got)
	}
	settlements, _, err := models.List("pos_tender_settlement", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list settlements: %v", err)
	}
	if len(settlements) != 1 {
		t.Fatalf("expected 1 tender settlement, got %d", len(settlements))
	}
	if got := textValue(settlements[0].Values["tender_type_code"]); got != "CARD" {
		t.Fatalf("expected CARD settlement, got %q", got)
	}

	approved, err := retailSvc.ApproveShiftReconciliation(reconciliation.ID, "manager_1")
	if err != nil {
		t.Fatalf("approve reconciliation: %v", err)
	}
	if got := textValue(approved.Values["status"]); got != "posted" {
		t.Fatalf("expected posted reconciliation, got %q", got)
	}
	if got := textValue(approved.Values["posting_id"]); got == "" {
		t.Fatalf("expected over/short posting id")
	}
	posting, err := docs.Get(textValue(approved.Values["posting_id"]))
	if err != nil {
		t.Fatalf("get over/short posting: %v", err)
	}
	if posting.Header.Status != "posted" {
		t.Fatalf("expected posted over/short posting, got %s", posting.Header.Status)
	}

	settled, err := retailSvc.SettleTenderSettlement(settlements[0].ID, "manager_1", 40, "2026-03-30", "BATCH-1", "")
	if err != nil {
		t.Fatalf("settle tender: %v", err)
	}
	if got := textValue(settled.Values["status"]); got != "settled" {
		t.Fatalf("expected settled status, got %q", got)
	}
}

func TestRetailFinanceStoredValueRedemptionsAdjustBalances(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	cfg := config.NewService()
	mustRegisterPOSTestDocumentTypes(t, docs)
	mustRegisterPOSTestModels(t, models)
	mustRegisterRetailFinanceTestModels(t, models)

	giftCard, err := models.Create("gift_card", "user_admin", map[string]any{
		"organization_id":        "org_default",
		"location_id":            "loc_main",
		"code":                   "GC-001",
		"issued_amount":          100.0,
		"remaining_balance":      100.0,
		"liability_account_code": "2250-GIFT-CARD",
		"status":                 "active",
	})
	if err != nil {
		t.Fatalf("create gift card: %v", err)
	}
	storeCredit, err := models.Create("store_credit_account", "user_admin", map[string]any{
		"organization_id":        "org_default",
		"location_id":            "loc_main",
		"party_id":               "party_1",
		"party_name":             "Alice",
		"balance_amount":         80.0,
		"liability_account_code": "2260-STORE-CREDIT",
		"status":                 "active",
	})
	if err != nil {
		t.Fatalf("create store credit account: %v", err)
	}
	sale, err := models.Create("pos_sale", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_main",
		"sale_number":     "SALE-002",
		"store_code":      "STORE1",
		"register_code":   "REG1",
		"shift_id":        "SHIFT-2",
		"cashier_user_id": "cashier_1",
		"party_id":        "party_1",
		"status":          "completed",
	})
	if err != nil {
		t.Fatalf("create sale: %v", err)
	}
	retailSvc := NewRetailFinanceCoreService(docs, models, cfg, NewFinanceReportingCoreService(docs, models, cfg))

	tenders, err := retailSvc.ResolveStoredValueTenders("org_default", "loc_main", "party_1", []normalizedTender{
		{Kind: "gift_card", Amount: 25, Reference: "GC-001"},
		{Kind: "store_credit", Amount: 30},
	})
	if err != nil {
		t.Fatalf("resolve stored value tenders: %v", err)
	}
	if len(tenders) != 2 {
		t.Fatalf("expected 2 resolved tenders, got %d", len(tenders))
	}
	if got := tenders[0].ClearingAccountCode; got != "2250-GIFT-CARD" {
		t.Fatalf("expected gift card liability account, got %q", got)
	}
	if got := tenders[1].ClearingAccountCode; got != "2260-STORE-CREDIT" {
		t.Fatalf("expected store credit liability account, got %q", got)
	}

	if err := retailSvc.RecordStoredValueRedemptions("org_default", "loc_main", sale, nil, tenders, "user_admin", "party_1"); err != nil {
		t.Fatalf("record redemptions: %v", err)
	}
	updatedGiftCard, err := models.Get("gift_card", giftCard.ID)
	if err != nil {
		t.Fatalf("get updated gift card: %v", err)
	}
	if got := numberValue(updatedGiftCard.Values["remaining_balance"]); got != 75 {
		t.Fatalf("expected gift card balance 75, got %v", got)
	}
	updatedStoreCredit, err := models.Get("store_credit_account", storeCredit.ID)
	if err != nil {
		t.Fatalf("get updated store credit account: %v", err)
	}
	if got := numberValue(updatedStoreCredit.Values["balance_amount"]); got != 50 {
		t.Fatalf("expected store credit balance 50, got %v", got)
	}
	giftTxns, _, err := models.List("gift_card_transaction", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list gift card transactions: %v", err)
	}
	if len(giftTxns) != 1 || textValue(giftTxns[0].Values["transaction_type"]) != "redeem" {
		t.Fatalf("expected redeem gift card transaction, got %+v", giftTxns)
	}
	storeCreditTxns, _, err := models.List("store_credit_transaction", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list store credit transactions: %v", err)
	}
	if len(storeCreditTxns) != 1 || textValue(storeCreditTxns[0].Values["transaction_type"]) != "redeem" {
		t.Fatalf("expected redeem store credit transaction, got %+v", storeCreditTxns)
	}
}

func TestRetailFinanceStoredValueLookupsAreScoped(t *testing.T) {
	models := model.NewService()
	mustRegisterRetailFinanceTestModels(t, models)
	retailSvc := NewRetailFinanceCoreService(nil, models, config.NewService(), nil)

	cardA, err := models.Create("gift_card", "user_admin", map[string]any{
		"organization_id":   "org_a",
		"location_id":       "loc_a",
		"code":              "GC-SHARED",
		"remaining_balance": 25.0,
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create gift card A: %v", err)
	}
	cardB, err := models.Create("gift_card", "user_admin", map[string]any{
		"organization_id":   "org_b",
		"location_id":       "loc_b",
		"code":              "GC-SHARED",
		"remaining_balance": 80.0,
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create gift card B: %v", err)
	}
	gotCard, err := retailSvc.LookupGiftCard("org_b", "loc_b", "GC-SHARED")
	if err != nil {
		t.Fatalf("lookup scoped gift card: %v", err)
	}
	if gotCard.ID != cardB.ID || gotCard.ID == cardA.ID {
		t.Fatalf("expected org_b/loc_b gift card, got %s", gotCard.ID)
	}

	creditA, err := models.Create("store_credit_account", "user_admin", map[string]any{
		"organization_id": "org_a",
		"location_id":     "loc_a",
		"party_id":        "party_shared",
		"balance_amount":  10.0,
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create store credit A: %v", err)
	}
	creditB, err := models.Create("store_credit_account", "user_admin", map[string]any{
		"organization_id": "org_b",
		"location_id":     "loc_b",
		"party_id":        "party_shared",
		"balance_amount":  55.0,
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create store credit B: %v", err)
	}
	gotCredit, err := retailSvc.LookupStoreCredit("org_b", "loc_b", "party_shared")
	if err != nil {
		t.Fatalf("lookup scoped store credit: %v", err)
	}
	if gotCredit.ID != creditB.ID || gotCredit.ID == creditA.ID {
		t.Fatalf("expected org_b/loc_b store credit, got %s", gotCredit.ID)
	}
}

func TestRetailFinancePostingConfigUsesScopedOverrides(t *testing.T) {
	cfg := config.NewService()
	if err := cfg.Save(config.Entry{
		Key:       "retail_finance.posting",
		ModuleKey: "retail_finance_core",
		Category:  "finance",
		Scope:     "organization",
		ScopeID:   "org_default",
		Value: map[string]any{
			"cash_over_gain_account_code":         "4891-ORG-GAIN",
			"cash_over_short_loss_account_code":   "5891-ORG-LOSS",
			"gift_card_liability_account_code":    "2251-ORG-GC",
			"store_credit_liability_account_code": "2261-ORG-SC",
		},
		UpdatedAt: time.Now().UTC(),
		UpdatedBy: "user_admin",
	}); err != nil {
		t.Fatalf("save scoped retail config: %v", err)
	}
	retailSvc := NewRetailFinanceCoreService(nil, nil, cfg, nil)
	values := retailSvc.retailPostingConfig("org_default", "loc_main")
	if got := values["gift_card_liability_account_code"]; got != "2251-ORG-GC" {
		t.Fatalf("expected scoped gift card account, got %q", got)
	}
	if got := values["store_credit_liability_account_code"]; got != "2261-ORG-SC" {
		t.Fatalf("expected scoped store credit account, got %q", got)
	}
}

func TestRetailFinanceTenderSettlementReportFiltersStatus(t *testing.T) {
	models := model.NewService()
	mustRegisterRetailFinanceTestModels(t, models)
	retailSvc := NewRetailFinanceCoreService(nil, models, nil, nil)
	for _, values := range []map[string]any{
		{
			"organization_id":   "org_default",
			"location_id":       "loc_main",
			"shift_id":          "SHIFT-1",
			"shift_number":      "SHIFT-1",
			"store_code":        "STORE1",
			"register_code":     "REG1",
			"tender_type_code":  "CARD",
			"expected_amount":   40.0,
			"settled_amount":    0.0,
			"difference_amount": 40.0,
			"status":            "open",
		},
		{
			"organization_id":   "org_default",
			"location_id":       "loc_main",
			"shift_id":          "SHIFT-2",
			"shift_number":      "SHIFT-2",
			"store_code":        "STORE1",
			"register_code":     "REG1",
			"tender_type_code":  "CARD",
			"expected_amount":   30.0,
			"settled_amount":    30.0,
			"difference_amount": 0.0,
			"status":            "settled",
		},
	} {
		if _, err := models.Create("pos_tender_settlement", "user_admin", values); err != nil {
			t.Fatalf("create settlement: %v", err)
		}
	}
	report := retailSvc.TenderSettlementReport("org_default", "loc_main", "", "", "", "settled")
	if len(report.Rows) != 1 || report.Rows[0].Status != "settled" {
		t.Fatalf("expected only settled rows, got %+v", report.Rows)
	}
}

func mustRegisterRetailFinanceTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{
			Key:         "pos_tender_reconciliation",
			DisplayName: "POS Tender Reconciliation",
			DefaultSort: "shift_number",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "shift_id", Type: "string", Required: true},
				{Key: "shift_number", Type: "string"},
				{Key: "store_code", Type: "string"},
				{Key: "register_code", Type: "string"},
				{Key: "cashier_user_id", Type: "string"},
				{Key: "reconciliation_date", Type: "string"},
				{Key: "expected_cash_amount", Type: "number"},
				{Key: "actual_cash_amount", Type: "number"},
				{Key: "over_short_amount", Type: "number"},
				{Key: "expected_total_amount", Type: "number"},
				{Key: "counted_total_amount", Type: "number"},
				{Key: "tender_summary_json", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "posting_id", Type: "string"},
				{Key: "approved_by", Type: "string"},
				{Key: "approved_at", Type: "string"},
				{Key: "notes", Type: "string"},
			},
		},
		{
			Key:         "pos_tender_settlement",
			DisplayName: "POS Tender Settlement",
			DefaultSort: "shift_number",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "reconciliation_id", Type: "string"},
				{Key: "shift_id", Type: "string"},
				{Key: "shift_number", Type: "string"},
				{Key: "store_code", Type: "string"},
				{Key: "register_code", Type: "string"},
				{Key: "tender_type_code", Type: "string"},
				{Key: "tender_kind", Type: "string"},
				{Key: "clearing_account_code", Type: "string"},
				{Key: "expected_amount", Type: "number"},
				{Key: "settled_amount", Type: "number"},
				{Key: "difference_amount", Type: "number"},
				{Key: "settlement_date", Type: "string"},
				{Key: "settlement_reference", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "notes", Type: "string"},
			},
		},
		{
			Key:         "gift_card",
			DisplayName: "Gift Card",
			DefaultSort: "code",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "code", Type: "string", Required: true},
				{Key: "store_code", Type: "string"},
				{Key: "party_id", Type: "string"},
				{Key: "party_name", Type: "string"},
				{Key: "issued_amount", Type: "number"},
				{Key: "remaining_balance", Type: "number"},
				{Key: "liability_account_code", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "notes", Type: "string"},
			},
		},
		{
			Key:         "gift_card_transaction",
			DisplayName: "Gift Card Transaction",
			DefaultSort: "gift_card_code",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "gift_card_id", Type: "string"},
				{Key: "gift_card_code", Type: "string"},
				{Key: "party_id", Type: "string"},
				{Key: "transaction_type", Type: "string"},
				{Key: "amount", Type: "number"},
				{Key: "balance_after", Type: "number"},
				{Key: "reference", Type: "string"},
				{Key: "pos_sale_id", Type: "string"},
				{Key: "payment_id", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "store_credit_account",
			DisplayName: "Store Credit Account",
			DefaultSort: "party_id",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "party_id", Type: "string", Required: true},
				{Key: "party_name", Type: "string"},
				{Key: "balance_amount", Type: "number"},
				{Key: "liability_account_code", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "last_activity_at", Type: "string"},
			},
		},
		{
			Key:         "store_credit_transaction",
			DisplayName: "Store Credit Transaction",
			DefaultSort: "party_id",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "store_credit_account_id", Type: "string"},
				{Key: "party_id", Type: "string"},
				{Key: "transaction_type", Type: "string"},
				{Key: "amount", Type: "number"},
				{Key: "balance_after", Type: "number"},
				{Key: "pos_sale_id", Type: "string"},
				{Key: "payment_id", Type: "string"},
				{Key: "source_document_id", Type: "string"},
				{Key: "reference", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register retail finance model %s: %v", def.Key, err)
		}
	}
}
