package shared

import "testing"

func TestErrorHelpers(t *testing.T) {
	err := Validation("bad")
	if err.Error() == "" {
		t.Fatal("expected error string")
	}
	_ = Conflict("c")
	_ = Forbidden("f")
	_ = Unauthorized("u")
	_ = NotFound("n")
}

func TestPrimitiveValidationHelpers(t *testing.T) {
	if err := (Money{AmountMinor: 100, Currency: "IDR"}).Validate(); err != nil {
		t.Fatalf("expected money valid: %v", err)
	}
	if (Money{AmountMinor: 100, Currency: "IDR"}).String() == "" {
		t.Fatal("expected money string")
	}
	if err := (Quantity{Value: 1, Unit: "item"}).Validate(); err != nil {
		t.Fatalf("expected quantity valid: %v", err)
	}
	if err := (Address{Country: "ID"}).Validate(); err != nil {
		t.Fatalf("expected address valid: %v", err)
	}
}
