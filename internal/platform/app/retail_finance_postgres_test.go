package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
)

func TestRetailFinancePostgresShiftReconciliationAndGiftCard(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed retail finance test")
	}
	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	graph := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	ensureWarehouseRecord(t, graph.models, "user_admin", "MAIN", "org_default", "loc_hq")
	ensurePOSTenderTypeRecord(t, graph.models, "user_admin", "CASH", "cash")
	ensurePOSTenderTypeRecord(t, graph.models, "user_admin", "CARD", "card")
	if _, err := graph.models.Create("pos_store", "user_admin", map[string]any{
		"code":           "STORE-" + suffix,
		"name":           "Retail Store " + suffix,
		"warehouse_code": "MAIN",
		"currency_code":  "IDR",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := graph.models.Create("pos_register", "user_admin", map[string]any{
		"code":              "REG-" + suffix,
		"name":              "Register " + suffix,
		"store_code":        "STORE-" + suffix,
		"cash_account_code": "1000-CASH",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create register: %v", err)
	}
	shift, err := graph.models.Create("pos_shift", "user_admin", map[string]any{
		"organization_id":     "org_default",
		"location_id":         "loc_hq",
		"shift_number":        "SHIFT-" + suffix,
		"store_code":          "STORE-" + suffix,
		"register_code":       "REG-" + suffix,
		"cashier_user_id":     "user_admin",
		"opening_cash_amount": 50.0,
		"actual_cash_amount":  105.0,
		"status":              "closed",
	})
	if err != nil {
		t.Fatalf("create shift: %v", err)
	}
	if _, err := graph.models.Create("pos_sale", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"sale_number":     "SALE-" + suffix,
		"store_code":      "STORE-" + suffix,
		"register_code":   "REG-" + suffix,
		"shift_id":        shift.ID,
		"cashier_user_id": "user_admin",
		"status":          "completed",
		"tenders_json":    `[{"tender_type_code":"CASH","kind":"cash","is_cash_like":true,"amount":60,"clearing_account_code":"1000-CASH"},{"tender_type_code":"CARD","kind":"card","amount":40,"clearing_account_code":"1010-CARD"}]`,
	}); err != nil {
		t.Fatalf("create pos sale: %v", err)
	}

	reconciliation, err := graph.retailFinance.SyncShiftReconciliation("org_default", "loc_hq", shift.ID, "user_admin")
	if err != nil {
		t.Fatalf("sync reconciliation: %v", err)
	}
	if got := reconciliation.Values["expected_cash_amount"]; got != 110.0 {
		t.Fatalf("expected cash 110, got %#v", got)
	}
	approved, err := graph.retailFinance.ApproveShiftReconciliation(reconciliation.ID, "user_admin")
	if err != nil {
		t.Fatalf("approve reconciliation: %v", err)
	}
	if got := approved.Values["posting_id"]; got == "" {
		t.Fatal("expected over/short posting id after approval")
	}
	settlements, _, err := graph.models.List("pos_tender_settlement", model.Query{
		Page:     1,
		PageSize: 20,
		Filters:  map[string]string{"reconciliation_id": reconciliation.ID},
	})
	if err != nil {
		t.Fatalf("list settlements: %v", err)
	}
	if len(settlements) != 1 {
		t.Fatalf("expected 1 open settlement, got %d", len(settlements))
	}
	if _, err := graph.retailFinance.SettleTenderSettlement(settlements[0].ID, "user_admin", 40.0, "2099-10-31", "CARD-BATCH-"+suffix, ""); err != nil {
		t.Fatalf("settle tender: %v", err)
	}

	giftCardPayload := map[string]any{
		"code":                 "GC-" + suffix,
		"store_code":           "STORE-" + suffix,
		"original_amount":      75.0,
		"amount":               75.0,
		"payment_account_code": "1000-CASH",
	}
	issued, err := graph.retailFinance.IssueGiftCard("org_default", "loc_hq", "user_admin", giftCardPayload)
	if err != nil {
		t.Fatalf("issue gift card: %v", err)
	}
	giftCard, ok := issued["gift_card"]
	if !ok || giftCard == nil {
		t.Fatalf("expected gift card in issue response")
	}
}
