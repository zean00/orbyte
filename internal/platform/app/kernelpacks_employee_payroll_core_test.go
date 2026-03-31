package app

import "testing"

func TestEmployeePayrollManifestExposesExpectedArtifacts(t *testing.T) {
	manifest := employeePayrollCoreKernelPackManifest()

	if manifest.Key != "employee_payroll_core" {
		t.Fatalf("unexpected manifest key %q", manifest.Key)
	}
	if len(manifest.OwnedDocumentTypes) != 4 {
		t.Fatalf("expected 4 owned document types, got %d", len(manifest.OwnedDocumentTypes))
	}
	if len(manifest.Workflows) != 4 {
		t.Fatalf("expected 4 workflows, got %d", len(manifest.Workflows))
	}
	if len(manifest.Models) != 7 {
		t.Fatalf("expected 7 payroll models, got %d", len(manifest.Models))
	}
}

func TestEmployeePayrollRoleTemplateIncludesWorkflowPermissions(t *testing.T) {
	manifest := employeePayrollCoreKernelPackManifest()
	if len(manifest.Security.RoleTemplates) == 0 {
		t.Fatal("expected role templates")
	}
	role := manifest.Security.RoleTemplates[0]
	required := []string{"payroll.read", "document.create", "document.submit", "document.approve", "pay_component.list"}
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
