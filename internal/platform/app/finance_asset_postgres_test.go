package app

import (
	"os"
	"testing"
	"time"

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
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, nil, "bootstrap-123!"); err != nil {
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
