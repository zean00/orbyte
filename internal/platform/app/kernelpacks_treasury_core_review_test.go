package app

import "testing"

func TestTreasuryBankReconciliationNavigationUsesRealRouteAndPermissions(t *testing.T) {
	manifest := treasuryCoreKernelPackManifest()

	for _, link := range manifest.AdminConsole.Sections[0].Links {
		if link.Key == "bank_reconciliations" && link.RoutePath != "/ui/finance/bank-reconciliation" {
			t.Fatalf("expected bank reconciliation admin link to use singular route, got %q", link.RoutePath)
		}
	}
	for _, item := range manifest.Frontend.Menus {
		if item.Key == "finance.bank_reconciliation" && !containsValue(item.RequiredPermissions, "bank_statement.read") {
			t.Fatalf("expected finance.bank_reconciliation menu to require bank_statement.read, got %+v", item.RequiredPermissions)
		}
	}
	for _, item := range manifest.Frontend.Actions {
		if item.Key == "finance.bank_reconciliation" && !containsValue(item.RequiredPermissions, "bank_statement.read") {
			t.Fatalf("expected finance.bank_reconciliation action to require bank_statement.read, got %+v", item.RequiredPermissions)
		}
	}
	for _, item := range manifest.Frontend.CustomEntries {
		if item.Key == "finance.bank_reconciliation" && !containsValue(item.RequiredPermissions, "bank_statement.read") {
			t.Fatalf("expected finance.bank_reconciliation custom entry to require bank_statement.read, got %+v", item.RequiredPermissions)
		}
	}
}
