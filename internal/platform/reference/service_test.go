package reference

import (
	"testing"
	"time"
)

func TestResolvePrefersMoreSpecificScope(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterType(TypeDefinition{Key: "currency", DisplayName: "Currency"}); err != nil {
		t.Fatalf("register type failed: %v", err)
	}
	now := time.Now().UTC()
	records := []Record{
		{TypeKey: "currency", Key: "IDR", DisplayName: "Rupiah", Scope: "deployment", UpdatedAt: now, Value: map[string]any{"symbol": "Rp"}},
		{TypeKey: "currency", Key: "IDR", DisplayName: "Rupiah Branch", Scope: "location", ScopeID: "loc_hq", UpdatedAt: now, Value: map[string]any{"symbol": "Rp-HQ"}},
	}
	for _, record := range records {
		if err := svc.UpsertRecord(record); err != nil {
			t.Fatalf("upsert record failed: %v", err)
		}
	}
	resolved, err := svc.Resolve("currency", "org_default", "loc_hq", now)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if len(resolved.Items) != 1 || resolved.Items[0].Value["symbol"] != "Rp-HQ" {
		t.Fatalf("expected location override, got %+v", resolved.Items)
	}
}

func TestResolveFiltersByEffectiveWindow(t *testing.T) {
	svc := NewService()
	_ = svc.RegisterType(TypeDefinition{Key: "visit_priority", DisplayName: "Visit Priority"})
	now := time.Now().UTC()
	_ = svc.UpsertRecord(Record{
		TypeKey: "visit_priority", Key: "urgent", DisplayName: "Urgent", Scope: "deployment",
		EffectiveFrom: now.Add(time.Hour), UpdatedAt: now,
	})
	resolved, err := svc.Resolve("visit_priority", "", "", now)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if len(resolved.Items) != 0 {
		t.Fatalf("expected future-dated record to be excluded, got %+v", resolved.Items)
	}
}
