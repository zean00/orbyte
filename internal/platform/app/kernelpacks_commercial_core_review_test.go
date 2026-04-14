package app

import "testing"

func TestCommercialItemModelIncludesReplenishmentFields(t *testing.T) {
	manifest := commercialCoreKernelPackManifest()
	if len(manifest.Models) == 0 {
		t.Fatal("expected commercial models")
	}

	var itemFields map[string]bool
	for _, def := range manifest.Models {
		if def.Key != "commercial_item" {
			continue
		}
		itemFields = map[string]bool{}
		for _, field := range def.Fields {
			itemFields[field.Key] = true
		}
		break
	}
	if len(itemFields) == 0 {
		t.Fatal("expected commercial_item model definition")
	}

	required := []string{
		"replenishment_enabled",
		"replenishment_mode",
		"reorder_point_quantity",
		"target_stock_quantity",
		"default_replenishment_warehouse_code",
	}
	for _, fieldKey := range required {
		if !itemFields[fieldKey] {
			t.Fatalf("expected commercial_item to include %s", fieldKey)
		}
	}
}
