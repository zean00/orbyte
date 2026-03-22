package featureflags

import (
	"testing"
	"time"
)

func TestResolveScopedFeatureFlags(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterDefinition(Definition{
		Key:           "documents.offline_sync",
		ModuleKey:     "documents",
		AllowedScopes: []string{"deployment", "organization", "location"},
		DefaultState:  false,
	}); err != nil {
		t.Fatalf("register definition: %v", err)
	}
	if err := svc.UpsertValue(Value{FlagKey: "documents.offline_sync", Scope: "deployment", Enabled: true, Status: "active"}); err != nil {
		t.Fatalf("save deployment value: %v", err)
	}
	if err := svc.UpsertValue(Value{FlagKey: "documents.offline_sync", Scope: "location", ScopeID: "loc_hq", Enabled: false, Status: "active"}); err != nil {
		t.Fatalf("save location value: %v", err)
	}

	effective, ok := svc.Resolve("documents.offline_sync", "org_default", "loc_hq", time.Now().UTC())
	if !ok {
		t.Fatal("expected effective feature flag")
	}
	if effective.Enabled {
		t.Fatalf("expected location override to disable flag, got %+v", effective)
	}
}

func TestTargetingView(t *testing.T) {
	svc := NewService()
	if err := svc.UpsertValue(Value{
		FlagKey:   "platform.admin_console",
		Scope:     "location",
		ScopeID:   "loc_hq",
		Enabled:   false,
		Status:    "active",
		UpdatedBy: "user_admin",
	}); err != nil {
		t.Fatalf("upsert value failed: %v", err)
	}
	view, ok := svc.TargetingView("platform.admin_console", "", "loc_hq", "", time.Now().UTC())
	if !ok {
		t.Fatal("expected targeting view")
	}
	if view.Effective.Enabled {
		t.Fatalf("expected location override to disable flag, got %+v", view.Effective)
	}
	if len(view.Values) == 0 {
		t.Fatalf("expected flag values in targeting view, got %+v", view)
	}
}
