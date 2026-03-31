package app

import "testing"

func TestOrganizationStructureAdminConsoleUsesRegisteredCostCenterRoute(t *testing.T) {
	manifest := organizationStructureKernelPackManifest()
	for _, section := range manifest.AdminConsole.Sections {
		for _, link := range section.Links {
			if link.Key != "cost_centers" {
				continue
			}
			if link.RoutePath != "/ui/organization/cost_centers" {
				t.Fatalf("expected cost center admin link to use registered route, got %q", link.RoutePath)
			}
			return
		}
	}
	t.Fatal("expected cost_centers admin console link")
}
