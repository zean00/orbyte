package app

import "testing"

func TestProcurementGoodsReceiptAllowsInventoryMovementLinks(t *testing.T) {
	manifest := procurementCoreKernelPackManifest()
	found := false
	for _, def := range manifest.Documents {
		if def.Type != "goods_receipt" {
			continue
		}
		found = true
		if !containsValue(def.AllowedLinkTypes, "movement_for") {
			t.Fatalf("expected goods_receipt to allow movement_for links, got %+v", def.AllowedLinkTypes)
		}
	}
	if !found {
		t.Fatal("expected goods_receipt document definition")
	}
}
