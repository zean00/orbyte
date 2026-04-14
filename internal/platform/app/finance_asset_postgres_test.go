package app

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
)

func TestFinanceAssetPostgresDepreciationAndAmortization(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed finance asset test")
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
	orgID := "org_default"
	locID := "loc_hq"
	period, err := graph.models.Create("accounting_period", "user_admin", map[string]any{
		"organization_id": orgID,
		"location_id":     locID,
		"period_key":      "2099-10-" + suffix,
		"start_date":      "2099-10-01",
		"end_date":        "2099-10-31",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create accounting period: %v", err)
	}
	if _, err := graph.models.Create("accounting_period", "user_admin", map[string]any{
		"organization_id": orgID,
		"location_id":     locID,
		"period_key":      "2099-11-" + suffix,
		"start_date":      "2099-11-01",
		"end_date":        "2099-11-30",
		"status":          "open",
	}); err != nil {
		t.Fatalf("create next accounting period: %v", err)
	}

	vendorBill, err := graph.documents.Create("vendor_bill", orgID, locID, "user_admin", map[string]any{
		"bill_date": "2099-10-10",
		"lines": []map[string]any{
			{"description": "Delivery Van", "line_subtotal": 1200.0, "line_total": 1320.0},
		},
	})
	if err != nil {
		t.Fatalf("create vendor bill: %v", err)
	}
	fixedAssetResult, err := graph.financeAssets.CreateFixedAssetFromVendorBill(vendorBill.Header.ID, 0, orgID, locID, "user_admin", map[string]any{
		"code":          "FA-VAN-" + suffix,
		"name":          "Delivery Van " + suffix,
		"method":        "straight_line",
		"cadence":       "monthly",
		"total_periods": 12,
	})
	if err != nil {
		t.Fatalf("create fixed asset from vendor bill: %v", err)
	}
	fixedAsset := fixedAssetResult["asset"].(model.Record)
	prepaidResult, err := graph.financeAssets.CreatePrepaidExpense(orgID, locID, "user_admin", map[string]any{
		"code":                   "PRE-INS-" + suffix,
		"name":                   "Insurance " + suffix,
		"basis_amount":           600.0,
		"recognition_start_date": "2099-10-01",
		"method":                 "declining_balance",
		"declining_rate_percent": 50.0,
		"cadence":                "monthly",
		"total_periods":          3,
	})
	if err != nil {
		t.Fatalf("create prepaid: %v", err)
	}
	prepaid := prepaidResult["asset"].(model.Record)

	pack, err := graph.financePeriodEnd.GenerateJournalRuns(period.ID, "user_admin", orgID, locID)
	if err != nil {
		t.Fatalf("generate period-end runs: %v", err)
	}
	if len(pack.JournalRuns) < 2 {
		t.Fatalf("expected at least 2 journal runs, got %d", len(pack.JournalRuns))
	}
	for _, run := range pack.JournalRuns {
		posting, getErr := graph.documents.Get(run.PostingID)
		if getErr != nil {
			t.Fatalf("get posting %s: %v", run.PostingID, getErr)
		}
		posting.Header.Status = "posted"
		if err := graph.documents.Save(posting); err != nil {
			t.Fatalf("save posting %s: %v", run.PostingID, err)
		}
		if err := graph.financePeriodEnd.HandleApprovedLedgerPosting(posting, "user_admin"); err != nil {
			t.Fatalf("period-end approve %s: %v", run.PostingID, err)
		}
		if err := graph.financeAssets.HandleApprovedLedgerPosting(posting, "user_admin"); err != nil {
			t.Fatalf("asset approve %s: %v", run.PostingID, err)
		}
	}

	fixedPreview, err := graph.financeAssets.FixedAssetPreview(fixedAsset.ID, orgID, locID)
	if err != nil {
		t.Fatalf("fixed asset preview: %v", err)
	}
	if fixedPreview.BookToDate != 100 {
		t.Fatalf("expected fixed asset booked amount 100, got %v", fixedPreview.BookToDate)
	}
	if fixedPreview.RemainingAmount != 1100 {
		t.Fatalf("expected fixed asset remaining amount 1100, got %v", fixedPreview.RemainingAmount)
	}
	prepaidPreview, err := graph.financeAssets.PrepaidPreview(prepaid.ID, orgID, locID)
	if err != nil {
		t.Fatalf("prepaid preview: %v", err)
	}
	if prepaidPreview.BookToDate != 300 {
		t.Fatalf("expected prepaid booked amount 300, got %v", prepaidPreview.BookToDate)
	}
	if prepaidPreview.RemainingAmount != 300 {
		t.Fatalf("expected prepaid remaining amount 300, got %v", prepaidPreview.RemainingAmount)
	}

	closePack, err := graph.financePeriodEnd.ClosePack(period.ID, "user_admin", orgID, locID)
	if err != nil {
		t.Fatalf("reload close pack: %v", err)
	}
	for _, task := range closePack.Tasks {
		if task.TaskType == "checklist" {
			if _, err := graph.financePeriodEnd.CompleteTask(task.ID, "user_admin", orgID, locID); err != nil {
				t.Fatalf("complete close task %s: %v", task.TaskCode, err)
			}
		}
	}
	if _, err := graph.financePeriodEnd.CloseAccountingPeriod(period.ID, "user_admin", orgID, locID); err != nil {
		t.Fatalf("close accounting period: %v", err)
	}

	ledger := graph.financeReporting.JournalLedger(orgID, locID, "2099-10-01", "2099-10-31")
	if len(ledger.Rows) < 4 {
		t.Fatalf("expected depreciation/amortization postings in journal ledger, got %d rows", len(ledger.Rows))
	}
}

func TestFinanceAssetPostgresLifecycleEvents(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed finance asset lifecycle test")
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
	orgID := "org_default"
	locID := "loc_hq"
	ensureLocationRecord(t, graph.models, "user_admin", locID)
	assetResult, err := graph.financeAssets.CreateFixedAsset(orgID, locID, "user_admin", map[string]any{
		"code":             "FA-LIFE-" + suffix,
		"name":             "Lifecycle " + suffix,
		"basis_amount":     1200.0,
		"method":           "straight_line",
		"cadence":          "monthly",
		"total_periods":    12,
		"acquisition_date": "2099-10-01",
		"cost_center_code": "OPS",
	})
	if err != nil {
		t.Fatalf("create fixed asset: %v", err)
	}
	asset := assetResult["asset"].(model.Record)
	templateID := textValue(asset.Values["linked_journal_template_id"])
	posting := mustCreatePostedJournalForPostgres(t, graph, templateID, "2099-10-31", 100)
	if err := graph.financeAssets.HandleApprovedLedgerPosting(posting, "user_admin"); err != nil {
		t.Fatalf("apply depreciation posting: %v", err)
	}

	if _, err := graph.financeAssets.TransferFixedAsset(asset.ID, orgID, locID, "user_admin", map[string]any{
		"effective_date":      "2099-11-01",
		"to_location_id":      locID,
		"to_cost_center_code": "FIN",
	}); err != nil {
		t.Fatalf("transfer asset: %v", err)
	}
	if _, err := graph.financeAssets.ImpairFixedAsset(asset.ID, orgID, locID, "user_admin", map[string]any{
		"impairment_date":   "2099-11-10",
		"impairment_amount": 150.0,
	}); err != nil {
		t.Fatalf("impair asset: %v", err)
	}
	if _, err := graph.financeAssets.RevalueFixedAsset(asset.ID, orgID, locID, "user_admin", map[string]any{
		"revaluation_date":   "2099-11-20",
		"revaluation_amount": 25.0,
	}); err != nil {
		t.Fatalf("revalue asset: %v", err)
	}
	if _, err := graph.financeAssets.DisposeFixedAsset(asset.ID, orgID, locID, "user_admin", map[string]any{
		"disposal_date":   "2099-11-25",
		"disposal_type":   "sale",
		"proceeds_amount": 980.0,
	}); err != nil {
		t.Fatalf("dispose asset: %v", err)
	}

	updated, err := graph.models.Get("fixed_asset", asset.ID)
	if err != nil {
		t.Fatalf("get updated asset: %v", err)
	}
	if got := textValue(updated.Values["current_location_id"]); got != locID {
		t.Fatalf("expected current location %s, got %q", locID, got)
	}
	if got := textValue(updated.Values["current_cost_center_code"]); got != "FIN" {
		t.Fatalf("expected current cost center FIN, got %q", got)
	}
	if got := textValue(updated.Values["status"]); got != "disposed" {
		t.Fatalf("expected disposed status, got %q", got)
	}
	if got := numberValue(updated.Values["impairment_amount_total"]); got != 150 {
		t.Fatalf("expected impairment total 150, got %v", got)
	}
	if got := numberValue(updated.Values["revaluation_amount_total"]); got != 25 {
		t.Fatalf("expected revaluation total 25, got %v", got)
	}
}

func mustCreatePostedJournalForPostgres(t *testing.T, graph *serviceGraph, templateID, postingDate string, amount float64) document.Record {
	t.Helper()
	template, err := graph.models.Get("journal_template", templateID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	period := ensureAccountingPeriodForDate(t, graph.models, "user_admin", textValue(template.Values["organization_id"]), textValue(template.Values["location_id"]), postingDate)
	posting, err := graph.documents.Create("ledger_posting", textValue(template.Values["organization_id"]), textValue(template.Values["location_id"]), "user_admin", map[string]any{
		"source_document_type": "journal_template",
		"source_document_id":   templateID,
		"posting_date":         postingDate,
		"currency_code":        "IDR",
		"posting_rule_key":     "period_end_" + textValue(template.Values["journal_kind"]),
		"journal_source_kind":  textValue(template.Values["journal_kind"]),
		"journal_template_id":  templateID,
		"total_amount":         amount,
		"journal_lines": []map[string]any{
			{"account_code": "6100-T", "description": "Debit", "debit": amount, "credit": 0.0},
			{"account_code": "1500-T", "description": "Credit", "debit": 0.0, "credit": amount},
		},
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := graph.documents.Save(posting); err != nil {
		t.Fatalf("save posting: %v", err)
	}
	if _, err := graph.models.Create("journal_run", "user_admin", map[string]any{
		"organization_id":          textValue(template.Values["organization_id"]),
		"location_id":              textValue(template.Values["location_id"]),
		"accounting_period_id":     period.ID,
		"period_key":               postingDate[:7],
		"journal_template_id":      templateID,
		"template_code":            textValue(template.Values["code"]),
		"template_name":            textValue(template.Values["name"]),
		"generated_posting_id":     posting.Header.ID,
		"status":                   "generated",
		"generated_posting_status": "posted",
		"reversal_status":          "none",
	}); err != nil {
		t.Fatalf("create journal run: %v", err)
	}
	return posting
}

func textValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}
