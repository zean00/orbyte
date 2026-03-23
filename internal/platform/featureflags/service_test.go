package featureflags

import (
	"strings"
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

func TestFeatureFlagValidationAndFallbackBranches(t *testing.T) {
	svc := NewService()

	if err := svc.RegisterDefinition(Definition{}); err == nil || !strings.Contains(err.Error(), "feature flag key") {
		t.Fatalf("expected missing key validation, got %v", err)
	}

	if err := svc.RegisterDefinition(Definition{Key: "ops.feature"}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	defs := svc.Definitions()
	var found Definition
	for _, item := range defs {
		if item.Key == "ops.feature" {
			found = item
			break
		}
	}
	if found.ModuleKey != "platform.core" || len(found.AllowedScopes) != 1 || found.AllowedScopes[0] != "deployment" {
		t.Fatalf("expected default definition fields, got %+v", found)
	}

	if err := svc.UpsertValue(Value{FlagKey: "missing.flag"}); err == nil {
		t.Fatal("expected missing definition error")
	}
	if err := svc.UpsertValue(Value{FlagKey: "ops.feature", Scope: "organization"}); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected invalid scope error, got %v", err)
	}
	if err := svc.UpsertValue(Value{FlagKey: "ops.feature"}); err != nil {
		t.Fatalf("expected default deployment upsert to succeed: %v", err)
	}
	values := svc.Values()
	if len(values) == 0 || values[len(values)-1].Status != "active" || values[len(values)-1].UpdatedBy != "system" {
		t.Fatalf("expected default value normalization, got %+v", values)
	}

	if _, ok := svc.Resolve("missing.flag", "", "", time.Now().UTC()); ok {
		t.Fatal("expected missing flag resolution to fail")
	}
}

func TestFeatureFlagResolveWithOperatingUnitAndTimeWindows(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterDefinition(Definition{
		Key:           "ops.windowed",
		ModuleKey:     "platform.core",
		AllowedScopes: []string{"deployment", "organization", "location", "operating_unit"},
		DefaultState:  false,
	}); err != nil {
		t.Fatalf("register definition: %v", err)
	}
	now := time.Now().UTC()
	if err := svc.UpsertValue(Value{FlagKey: "ops.windowed", Scope: "deployment", Enabled: true, Status: "active"}); err != nil {
		t.Fatalf("upsert deployment: %v", err)
	}
	if err := svc.UpsertValue(Value{FlagKey: "ops.windowed", Scope: "organization", ScopeID: "org_1", Enabled: false, Status: "inactive"}); err != nil {
		t.Fatalf("upsert inactive organization: %v", err)
	}
	if err := svc.UpsertValue(Value{FlagKey: "ops.windowed", Scope: "operating_unit", ScopeID: "ou_1", Enabled: false, Status: "active", EffectiveFrom: now.Add(time.Hour)}); err != nil {
		t.Fatalf("upsert future ou flag: %v", err)
	}
	if err := svc.UpsertValue(Value{FlagKey: "ops.windowed", Scope: "location", ScopeID: "loc_1", Enabled: false, Status: "active", EffectiveTo: now.Add(-time.Hour)}); err != nil {
		t.Fatalf("upsert expired location flag: %v", err)
	}

	effective, ok := svc.ResolveWithOperatingUnit("ops.windowed", "org_1", "loc_1", "ou_1", now)
	if !ok {
		t.Fatal("expected effective value")
	}
	if !effective.Enabled || effective.SourceScope != "deployment" {
		t.Fatalf("expected deployment fallback due inactive/out-of-window overrides, got %+v", effective)
	}

	all := svc.ResolveAllWithOperatingUnit("org_1", "loc_1", "ou_1", now)
	if len(all) == 0 {
		t.Fatal("expected resolved values")
	}
	view, ok := svc.TargetingView("missing.flag", "", "", "", now)
	if ok || view.Definition.Key != "" {
		t.Fatalf("expected missing targeting view, got %+v", view)
	}
}
