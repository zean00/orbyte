package shared

import (
	"testing"
	"time"
)

func TestIdentifierValidate(t *testing.T) {
	if err := (Identifier{Type: "mrn", Value: "123"}).Validate(); err != nil {
		t.Fatalf("expected valid identifier, got error: %v", err)
	}
}

func TestTimeRangeValidate(t *testing.T) {
	rng := TimeRange{Start: time.Now(), End: time.Now().Add(-time.Minute)}
	if err := rng.Validate(); err == nil {
		t.Fatal("expected invalid time range")
	}
}
