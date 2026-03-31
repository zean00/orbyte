package app

import "testing"

func TestEmployeeSpendManifestExposesExpectedArtifacts(t *testing.T) {
	manifest := employeeSpendCoreKernelPackManifest()

	if manifest.Key != "employee_spend_core" {
		t.Fatalf("unexpected manifest key %q", manifest.Key)
	}
	if len(manifest.OwnedDocumentTypes) != 5 {
		t.Fatalf("expected 5 owned document types, got %d", len(manifest.OwnedDocumentTypes))
	}
	if len(manifest.Workflows) != 5 {
		t.Fatalf("expected 5 workflows, got %d", len(manifest.Workflows))
	}
	if len(manifest.Models) != 5 {
		t.Fatalf("expected 5 setup models, got %d", len(manifest.Models))
	}
}

func TestEmployeeSpendRoleTemplateIncludesDocumentWorkflowPermissions(t *testing.T) {
	manifest := employeeSpendCoreKernelPackManifest()
	if len(manifest.Security.RoleTemplates) == 0 {
		t.Fatal("expected role templates")
	}
	role := manifest.Security.RoleTemplates[0]
	required := []string{"document.create", "document.submit", "document.approve", "expense_category.list", "employee_spend.read"}
	for _, permission := range required {
		found := false
		for _, item := range role.PermissionKeys {
			if item == permission {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected role template to include %s", permission)
		}
	}
}

func TestEmployeeSpendManifestIncludesRateRuleMenu(t *testing.T) {
	manifest := employeeSpendCoreKernelPackManifest()
	for _, menu := range manifest.Frontend.Menus {
		if menu.Key == "employee_spend.rate_rules" {
			if menu.ActionKey != "employee_spend.rate_rules.list" {
				t.Fatalf("expected rate rules menu to use employee_spend.rate_rules.list, got %s", menu.ActionKey)
			}
			return
		}
	}
	t.Fatal("expected rate rules menu entry")
}
