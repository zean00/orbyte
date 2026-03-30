package app

import (
	"slices"
	"testing"

	"orbyte/internal/platform/module"
)

type testModulePack struct {
	manifests []module.Manifest
}

func (p testModulePack) Manifests() []module.Manifest {
	return append([]module.Manifest(nil), p.manifests...)
}

func TestBuiltInModulePacksExposeExpectedKernelPacks(t *testing.T) {
	packs := builtInModulePacks()
	if len(packs) != 30 {
		t.Fatalf("expected 30 built-in module packs, got %d", len(packs))
	}

	expectedKeys := []string{
		"reference_masterdata",
		"masterdata",
		"platform.core",
		"identity",
		"documents",
		"commercial_core",
		"discount_core",
		"promotion_core",
		"finance_reporting_core",
		"finance_manual_journal_core",
		"finance_collections_core",
		"finance_asset_core",
		"inventory_finance_core",
		"retail_finance_core",
		"treasury_core",
		"procurement_core",
		"inventory_core",
		"fulfillment_core",
		"delivery_core",
		"returns_core",
		"supplier_returns_core",
		"planning_core",
		"production_core",
		"production_costing_core",
		"pos_core",
		"traceability_core",
		"recall_core",
		"analytics",
		"monitoring",
		"integration",
	}
	var gotKeys []string
	for i, pack := range packs {
		manifests := pack.Manifests()
		if len(manifests) != 1 {
			t.Fatalf("expected pack %d to expose exactly one manifest, got %d", i, len(manifests))
		}
		gotKeys = append(gotKeys, manifests[0].Key)
	}
	if !slices.Equal(gotKeys, expectedKeys) {
		t.Fatalf("unexpected built-in pack order: got %v want %v", gotKeys, expectedKeys)
	}
}

func TestBuiltInModuleManifestsPreservePackOrder(t *testing.T) {
	expectedKeys := []string{
		"reference_masterdata",
		"masterdata",
		"platform.core",
		"identity",
		"documents",
		"commercial_core",
		"discount_core",
		"promotion_core",
		"finance_reporting_core",
		"finance_manual_journal_core",
		"finance_collections_core",
		"finance_asset_core",
		"inventory_finance_core",
		"retail_finance_core",
		"treasury_core",
		"procurement_core",
		"inventory_core",
		"fulfillment_core",
		"delivery_core",
		"returns_core",
		"supplier_returns_core",
		"planning_core",
		"production_core",
		"production_costing_core",
		"pos_core",
		"traceability_core",
		"recall_core",
		"analytics",
		"monitoring",
		"integration",
	}

	manifests := builtInModuleManifests()
	if len(manifests) != len(expectedKeys) {
		t.Fatalf("unexpected built-in manifest count: got %d want %d", len(manifests), len(expectedKeys))
	}
	var gotKeys []string
	for _, manifest := range manifests {
		gotKeys = append(gotKeys, manifest.Key)
	}
	if !slices.Equal(gotKeys, expectedKeys) {
		t.Fatalf("unexpected built-in manifest order: got %v want %v", gotKeys, expectedKeys)
	}
}

func TestBuiltInModuleManifestsReturnCopy(t *testing.T) {
	first := BuiltInModuleManifests()
	if len(first) == 0 {
		t.Fatal("expected built-in manifests")
	}
	first[0].Key = "mutated"

	second := BuiltInModuleManifests()
	if len(second) == 0 {
		t.Fatal("expected built-in manifests on second read")
	}
	if second[0].Key == "mutated" {
		t.Fatal("expected BuiltInModuleManifests to return an isolated copy")
	}
}

func TestCollectModuleManifestsRejectsEmptyKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty manifest key")
		}
	}()
	collectModuleManifests([]modulePack{
		testModulePack{manifests: []module.Manifest{{Key: ""}}},
	})
}

func TestCollectModuleManifestsRejectsDuplicateKeys(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate manifest key")
		}
	}()
	collectModuleManifests([]modulePack{
		testModulePack{manifests: []module.Manifest{{Key: "platform.core"}}},
		testModulePack{manifests: []module.Manifest{{Key: "platform.core"}}},
	})
}
