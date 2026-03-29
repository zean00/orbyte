package app

import "testing"

func TestProductionBOMVersionDefaultSortUsesExistingField(t *testing.T) {
	manifest := productionCoreKernelPackManifest()
	for _, def := range manifest.Models {
		if def.Key != "production_bom_version" {
			continue
		}
		if def.DefaultSort != "version_code" {
			t.Fatalf("expected production_bom_version default sort version_code, got %s", def.DefaultSort)
		}
		return
	}
	t.Fatal("expected production_bom_version model definition")
}
