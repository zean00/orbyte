package featureflags

import (
	"testing"
	"time"
)

func TestResolveAllAndTargetingViewOrdering(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(repo)
	now := time.Now().UTC()

	if err := svc.RegisterDefinition(Definition{
		Key:           "zeta.flag",
		ModuleKey:     "platform.core",
		AllowedScopes: []string{"deployment", "location"},
		DefaultState:  false,
	}); err != nil {
		t.Fatalf("register zeta flag failed: %v", err)
	}
	if err := svc.RegisterDefinition(Definition{
		Key:           "alpha.flag",
		ModuleKey:     "platform.core",
		AllowedScopes: []string{"deployment", "organization", "location"},
		DefaultState:  true,
	}); err != nil {
		t.Fatalf("register alpha flag failed: %v", err)
	}

	if err := repo.SaveValue(Value{FlagKey: "alpha.flag", Scope: "organization", ScopeID: "org_a", Enabled: true, Status: "active", UpdatedAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("save organization value failed: %v", err)
	}
	if err := repo.SaveValue(Value{FlagKey: "alpha.flag", Scope: "location", ScopeID: "loc_a", Enabled: false, Status: "active", UpdatedAt: now}); err != nil {
		t.Fatalf("save location value failed: %v", err)
	}
	if err := repo.SaveValue(Value{FlagKey: "zeta.flag", Scope: "deployment", Enabled: true, Status: "active", UpdatedAt: now}); err != nil {
		t.Fatalf("save deployment value failed: %v", err)
	}

	all := svc.ResolveAll("org_a", "loc_a", now)
	if len(all) < 2 {
		t.Fatalf("expected resolved values, got %+v", all)
	}
	if all[0].Key != "alpha.flag" || all[1].Key != "platform.admin_console" && all[1].Key != "platform.mcp_templates" && all[1].Key != "zeta.flag" {
		t.Fatalf("expected results sorted by key, got %+v", all)
	}

	targeting, ok := svc.TargetingView("alpha.flag", "org_a", "loc_a", "", now)
	if !ok {
		t.Fatal("expected targeting view")
	}
	if targeting.Effective.SourceScope != "location" || targeting.Effective.Enabled {
		t.Fatalf("expected location override to win, got %+v", targeting.Effective)
	}
	if len(targeting.Values) != 2 {
		t.Fatalf("expected two scoped values, got %+v", targeting.Values)
	}
	if targeting.Values[0].Scope != "location" || targeting.Values[1].Scope != "organization" {
		t.Fatalf("expected values sorted by scope then recency, got %+v", targeting.Values)
	}
}
