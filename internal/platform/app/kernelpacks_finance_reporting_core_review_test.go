package app

import "testing"

func TestFinanceReportingJournalLedgerRequiresDocumentRead(t *testing.T) {
	manifest := financeReportingCoreKernelPackManifest()

	for _, item := range manifest.Frontend.Menus {
		if item.Key == "finance.journal_ledger" {
			if !containsValue(item.RequiredPermissions, "document.read") {
				t.Fatalf("expected finance.journal_ledger menu to require document.read, got %+v", item.RequiredPermissions)
			}
		}
	}
	for _, item := range manifest.Frontend.Actions {
		if item.Key == "finance.journal_ledger" {
			if !containsValue(item.RequiredPermissions, "document.read") {
				t.Fatalf("expected finance.journal_ledger action to require document.read, got %+v", item.RequiredPermissions)
			}
		}
	}
	for _, item := range manifest.Frontend.CustomEntries {
		if item.Key == "finance.journal_ledger" {
			if !containsValue(item.RequiredPermissions, "document.read") {
				t.Fatalf("expected finance.journal_ledger custom entry to require document.read, got %+v", item.RequiredPermissions)
			}
		}
	}
}

func TestFinanceReportingAccountingPeriodFormDoesNotExposeWritableStatus(t *testing.T) {
	manifest := financeReportingCoreKernelPackManifest()

	for _, item := range manifest.Frontend.Views {
		if item.Key != "finance.periods.form" {
			continue
		}
		for _, field := range item.Fields {
			if field.Key == "status" {
				t.Fatalf("expected finance.periods.form to exclude writable status field")
			}
		}
		return
	}
	t.Fatal("expected finance.periods.form view")
}
