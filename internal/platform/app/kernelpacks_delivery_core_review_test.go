package app

import "testing"

func TestDeliveryOperatorRoleIncludesRequiredDocumentPermissions(t *testing.T) {
	manifest := deliveryCoreKernelPackManifest()
	if len(manifest.Security.RoleTemplates) == 0 {
		t.Fatal("expected delivery role templates")
	}

	var operatorPermissions []string
	for _, item := range manifest.Security.RoleTemplates {
		if item.Key == "delivery_operator" {
			operatorPermissions = item.PermissionKeys
			break
		}
	}
	if len(operatorPermissions) == 0 {
		t.Fatal("expected delivery_operator role template")
	}

	required := []string{
		"delivery.read",
		"document.list",
		"document.read",
		"document.submit",
		"document.approve",
	}
	for _, permissionKey := range required {
		if !containsValue(operatorPermissions, permissionKey) {
			t.Fatalf("expected delivery_operator to include %s, got %+v", permissionKey, operatorPermissions)
		}
	}
}
