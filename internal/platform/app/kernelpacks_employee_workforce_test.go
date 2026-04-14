package app

import (
	"testing"

	"orbyte/internal/platform/module"
)

func TestEmployeeWorkforceAdminConsoleUsesRegisteredRoutes(t *testing.T) {
	manifest := employeeWorkforceKernelPackManifest()
	expected := map[string]string{
		"employees":    "/ui/workforce/employees",
		"assignments":  "/ui/workforce/assignments",
		"eligibility":  "/ui/workforce/eligibility",
		"compensation": "/ui/workforce/compensation",
	}
	for _, section := range manifest.AdminConsole.Sections {
		for _, link := range section.Links {
			want, ok := expected[link.Key]
			if !ok {
				continue
			}
			if link.RoutePath != want {
				t.Fatalf("expected %s route %q, got %q", link.Key, want, link.RoutePath)
			}
			delete(expected, link.Key)
		}
	}
	if len(expected) != 0 {
		t.Fatalf("missing admin console links: %v", expected)
	}
}

func TestEmployeeWorkforceAndOperationalModelsExposeEmployeeReferences(t *testing.T) {
	workforce := employeeWorkforceKernelPackManifest()
	if !modelHasField(workforce, "employee_profile", "party_id") {
		t.Fatal("expected employee_profile to include party_id")
	}
	if !modelHasField(workforce, "employee_assignment", "effective_from") {
		t.Fatal("expected employee_assignment to include effective_from")
	}
	if !modelHasField(workforce, "employee_role_eligibility", "eligibility_type") {
		t.Fatal("expected employee_role_eligibility to include eligibility_type")
	}
	if !modelHasField(workforce, "employee_compensation_profile", "standard_hourly_rate") {
		t.Fatal("expected employee_compensation_profile to include standard_hourly_rate")
	}

	pos := posCoreKernelPackManifest()
	if !modelHasField(pos, "pos_shift", "cashier_employee_id") {
		t.Fatal("expected pos_shift to include cashier_employee_id")
	}

	productionCosting := productionCostingCoreKernelPackManifest()
	if !modelHasField(productionCosting, "production_cost_capture", "employee_id") {
		t.Fatal("expected production_cost_capture to include employee_id")
	}
}

func modelHasField(manifest module.Manifest, modelKey, fieldKey string) bool {
	for _, def := range manifest.Models {
		if def.Key != modelKey {
			continue
		}
		for _, field := range def.Fields {
			if field.Key == fieldKey {
				return true
			}
		}
	}
	return false
}
