package app

import "testing"

func TestStockAdjustmentAllowsPostingLinks(t *testing.T) {
	manifest := inventoryCoreKernelPackManifest()
	for _, def := range manifest.Documents {
		if def.Type != "stock_adjustment" {
			continue
		}
		if !containsValue(def.AllowedLinkTypes, "posting_for") {
			t.Fatalf("expected stock_adjustment to allow posting_for links, got %+v", def.AllowedLinkTypes)
		}
		return
	}
	t.Fatal("stock_adjustment document definition not found")
}
