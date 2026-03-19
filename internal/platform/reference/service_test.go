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

func TestTypeRecordsAndValidationHelpers(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterType(TypeDefinition{Key: "currency", DisplayName: "Currency", AllowedScopes: []string{"deployment", "organization"}}); err != nil {
		t.Fatalf("register type failed: %v", err)
	}
	if def, ok := svc.Type(" currency "); !ok || def.Key != "currency" {
		t.Fatalf("expected trimmed type lookup, got ok=%v def=%+v", ok, def)
	}
	if len(svc.Types()) != 1 || svc.Types()[0].Key != "currency" {
		t.Fatalf("unexpected types list: %+v", svc.Types())
	}
	if err := svc.UpsertRecord(Record{TypeKey: "currency", Key: "USD", DisplayName: "US Dollar", Scope: "organization", ScopeID: "org_default"}); err != nil {
		t.Fatalf("upsert record failed: %v", err)
	}
	if len(svc.Records(" currency ")) != 1 {
		t.Fatalf("unexpected records list: %+v", svc.Records("currency"))
	}
	if err := svc.UpsertRecord(Record{TypeKey: "currency", Key: "EUR", DisplayName: "Euro", Scope: "location", ScopeID: "loc_hq"}); err == nil {
		t.Fatal("expected disallowed scope to fail")
	}
	if normalizeScope("") != "deployment" || !contains([]string{"a", "b"}, "b") || firstNonEmpty("", "x") != "x" {
		t.Fatal("expected helper functions to behave deterministically")
	}
	if !isInScope(Record{Scope: "organization", ScopeID: "org_default"}, "org_default", "loc_hq") {
		t.Fatal("expected organization scope match")
	}
	if isInScope(Record{Scope: "location", ScopeID: "loc_branch"}, "org_default", "loc_hq") {
		t.Fatal("expected location scope mismatch")
	}
	if scopeRank("location") <= scopeRank("organization") {
		t.Fatal("expected location scope to outrank organization")
	}
}
