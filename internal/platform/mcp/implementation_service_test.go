package mcp

import (
	"testing"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/featureflags"
)

func TestImplementationServiceCRUDAndCloneIsolation(t *testing.T) {
	svc := NewImplementationService()

	first := svc.Create(" actor-1 ", " First Session ", ImplementationContext{OrganizationID: "org_a"})
	second := svc.Create("actor-2", "Second Session", ImplementationContext{OrganizationID: "org_b"})
	if first.ID == "" || first.Status != "open" || first.ActorID != "actor-1" || first.Name != "First Session" {
		t.Fatalf("unexpected created session: %+v", first)
	}

	first.StagedPlan.Bundle.ConfigEntries = []config.Entry{{Key: "feature.x", Category: "runtime", Scope: "deployment", Value: map[string]any{"state": "on"}}}
	first.StagedPlan.Bundle.FeatureFlags = []featureflags.Value{{FlagKey: "flag.x", Scope: "deployment", Enabled: true, Status: "active"}}
	first.StagedPlan.RoleGrants = []implementationRoleGrant{{RoleID: "admin", PermissionKey: "config.manage"}}
	first.StagedPlan.ModuleActions = []implementationModuleAction{{ModuleKey: "analytics", Enabled: true}}
	first.StagedPlan.SystemConfigUpdates = []integrationConfigUpdate{{Key: "erp", Settings: map[string]any{"base_url": "https://example.com"}}}
	first.StagedPlan.EndpointConfigUpdates = []integrationConfigUpdate{{Key: "erp.orders", Settings: map[string]any{"enabled": true}}}
	first.StagedPlan.ReferenceRecordUpserts = []implementationReferenceUpsert{{TypeKey: "country", Key: "ID"}}
	first.StagedPlan.PolicyModuleUpdates = []implementationPolicyModuleUpdate{{HookKey: "documents.fields.profile", Source: "module"}}
	first.ChangeSets = []ImplementationChangeSet{{ID: "cs1"}}
	first.Checkpoints = []ImplementationCheckpoint{{ID: "cp1"}}
	svc.Save(first)

	got, ok := svc.Get(" " + first.ID + " ")
	if !ok || got.ID != first.ID {
		t.Fatalf("expected saved session lookup, got %+v ok=%v", got, ok)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatalf("expected save to update timestamp, got %+v", got)
	}

	// Verify returned sessions are defensive copies.
	got.StagedPlan.Bundle.ConfigEntries[0].Key = "mutated"
	got.StagedPlan.Bundle.FeatureFlags[0].FlagKey = "mutated"
	got.StagedPlan.RoleGrants[0].RoleID = "mutated"
	got.StagedPlan.ModuleActions[0].ModuleKey = "mutated"
	got.StagedPlan.SystemConfigUpdates[0].Key = "mutated"
	got.StagedPlan.EndpointConfigUpdates[0].Key = "mutated"
	got.StagedPlan.ReferenceRecordUpserts[0].Key = "mutated"
	got.StagedPlan.PolicyModuleUpdates[0].HookKey = "mutated"
	got.ChangeSets[0].ID = "mutated"
	got.Checkpoints[0].ID = "mutated"

	reread, ok := svc.Get(first.ID)
	if !ok {
		t.Fatalf("expected reread session")
	}
	if reread.StagedPlan.Bundle.ConfigEntries[0].Key != "feature.x" ||
		reread.StagedPlan.Bundle.FeatureFlags[0].FlagKey != "flag.x" ||
		reread.StagedPlan.RoleGrants[0].RoleID != "admin" ||
		reread.StagedPlan.ModuleActions[0].ModuleKey != "analytics" ||
		reread.StagedPlan.SystemConfigUpdates[0].Key != "erp" ||
		reread.StagedPlan.EndpointConfigUpdates[0].Key != "erp.orders" ||
		reread.StagedPlan.ReferenceRecordUpserts[0].Key != "ID" ||
		reread.StagedPlan.PolicyModuleUpdates[0].HookKey != "documents.fields.profile" ||
		reread.ChangeSets[0].ID != "cs1" ||
		reread.Checkpoints[0].ID != "cp1" {
		t.Fatalf("expected stored session to be insulated from caller mutation, got %+v", reread)
	}

	listed := svc.List()
	if len(listed) != 2 || listed[0].ID != second.ID || listed[1].ID != first.ID {
		t.Fatalf("expected newest-first session ordering, got %+v", listed)
	}

	closed, ok := svc.Close(" " + first.ID + " ")
	if !ok || closed.Status != "closed" {
		t.Fatalf("expected close to succeed, got %+v ok=%v", closed, ok)
	}
	if _, ok := svc.Close("missing"); ok {
		t.Fatal("expected close on missing session to fail")
	}
}

func TestAnalyticsStreamNilPaths(t *testing.T) {
	var stream *AnalyticsStream
	if _, ok := stream.Latest(); ok {
		t.Fatal("expected nil stream latest to be empty")
	}
	ch, unsubscribe := stream.Subscribe()
	if _, ok := <-ch; ok {
		t.Fatal("expected nil stream subscription channel to be closed")
	}
	unsubscribe()
	stream.Publish(analyticsSnapshotForTest())
}

func analyticsSnapshotForTest() analytics.Snapshot {
	return analytics.Snapshot{ID: "snap-1"}
}
