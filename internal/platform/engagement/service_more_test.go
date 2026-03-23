package engagement

import (
	"testing"
	"time"

	"orbyte/internal/platform/eventing"
)

func TestServiceReadModelsAndSimulation(t *testing.T) {
	svc := NewService()
	if _, err := svc.CreateProgram("loyalty", "Customer Loyalty", "customer", "tester"); err != nil {
		t.Fatalf("create program failed: %v", err)
	}
	version, err := svc.CreateDraftVersion("loyalty", "tester", "initial")
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}
	version, err = svc.SaveVersion("loyalty", version.Version, []Rule{
		{Key: "purchase-credit", Action: "credit_points", SourceEventTypes: []string{"order.completed"}, SubjectSource: "payload.customer.id", AccountKey: "points", AmountField: "order.total"},
		{Key: "purchase-tier", Action: "set_tier", SourceEventTypes: []string{"order.completed"}, SubjectSource: "payload.customer.id", AccountKey: "points", Threshold: 40, TierKey: "silver"},
		{Key: "purchase-achievement", Action: "grant_achievement", SourceEventTypes: []string{"order.completed"}, SubjectSource: "payload.customer.id", AccountKey: "points", Threshold: 40, AchievementKey: "spender"},
	}, "tester", "rules")
	if err != nil {
		t.Fatalf("save version failed: %v", err)
	}
	if _, err := svc.PublishVersion("loyalty", version.Version, "tester"); err != nil {
		t.Fatalf("publish version failed: %v", err)
	}

	event := eventing.Event{
		ID:         "evt-sim",
		Type:       "order.completed",
		OccurredAt: time.Now().UTC(),
		Payload: map[string]any{
			"customer": map[string]any{"id": "cust-1"},
			"order":    map[string]any{"total": "40"},
		},
	}
	outcome, err := svc.SimulationRun("loyalty", 0, event)
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}
	outcomes, ok := outcome["outcomes"].([]map[string]any)
	if !ok {
		t.Fatalf("expected outcomes slice, got %#v", outcome["outcomes"])
	}
	if len(outcomes) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(outcomes))
	}

	if err := svc.ProcessEvent(event); err != nil {
		t.Fatalf("process event failed: %v", err)
	}

	listedPrograms := svc.ListPrograms()
	if len(listedPrograms) != 1 || listedPrograms[0].Key != "loyalty" {
		t.Fatalf("unexpected programs: %+v", listedPrograms)
	}
	gotProgram, ok := svc.GetProgram(" loyalty ")
	if !ok || gotProgram.Key != "loyalty" {
		t.Fatalf("expected trimmed program lookup, got %+v", gotProgram)
	}
	gotVersion, ok := svc.GetVersion("loyalty", 0)
	if !ok || gotVersion.Version != version.Version {
		t.Fatalf("expected latest version, got %+v", gotVersion)
	}

	accounts := svc.ListAccounts("loyalty", "cust-1")
	if len(accounts) != 1 || accounts[0].Balance != 40 || accounts[0].AccountKey != "points" {
		t.Fatalf("unexpected accounts: %+v", accounts)
	}
	subject := svc.GetSubject("loyalty", "cust-1")
	if subject.Qualification == nil || subject.Qualification.TierKey != "silver" {
		t.Fatalf("unexpected subject qualification: %+v", subject.Qualification)
	}
	if len(subject.Achievements) != 1 || subject.Achievements[0].AchievementKey != "spender" {
		t.Fatalf("unexpected achievements: %+v", subject.Achievements)
	}
	if len(subject.RecentJournal) != 1 || subject.RecentJournal[0].Amount != 40 {
		t.Fatalf("unexpected journal: %+v", subject.RecentJournal)
	}

	consumers := svc.ListConsumers()
	if len(consumers) != 1 || consumers[0].Processed != 3 || consumers[0].Status != "active" {
		t.Fatalf("unexpected consumers: %+v", consumers)
	}
	consumer, ok := svc.GetConsumer(" loyalty:v1 ")
	if !ok || consumer.ID != "loyalty:v1" {
		t.Fatalf("expected trimmed consumer lookup, got %+v", consumer)
	}
}

func TestServiceReplayListingsAndHelpers(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(repo)

	if _, err := svc.CreateProgram("zeta", "Zeta", "customer", "tester"); err != nil {
		t.Fatalf("create zeta program failed: %v", err)
	}
	if _, err := svc.CreateProgram("alpha", "Alpha", "customer", "tester"); err != nil {
		t.Fatalf("create alpha program failed: %v", err)
	}
	v1, err := svc.CreateDraftVersion("zeta", "tester", "v1")
	if err != nil {
		t.Fatalf("create zeta draft failed: %v", err)
	}
	if _, err := svc.SaveVersion("zeta", v1.Version, []Rule{
		{Key: "credit", Action: "credit_points", SourceEventTypes: []string{"purchase.created"}, SubjectSource: "actor_id", FixedAmount: 5},
	}, "tester", "v1"); err != nil {
		t.Fatalf("save zeta v1 failed: %v", err)
	}
	if _, err := svc.PublishVersion("zeta", v1.Version, "tester"); err != nil {
		t.Fatalf("publish zeta v1 failed: %v", err)
	}

	if _, err := svc.UpdateProgram("zeta", "", "", "inactive", "tester"); err != nil {
		t.Fatalf("deactivate zeta failed: %v", err)
	}
	if published := svc.publishedVersions(); len(published) != 0 {
		t.Fatalf("expected inactive program to be excluded, got %+v", published)
	}
	if _, err := svc.UpdateProgram("zeta", "", "", "active", "tester"); err != nil {
		t.Fatalf("reactivate zeta failed: %v", err)
	}

	alphaV1, err := svc.CreateDraftVersion("alpha", "tester", "v1")
	if err != nil {
		t.Fatalf("create alpha draft failed: %v", err)
	}
	if _, err := svc.SaveVersion("alpha", alphaV1.Version, []Rule{
		{Key: "credit-alpha", Action: "credit_points", SourceEventTypes: []string{"purchase.created"}, SubjectSource: "actor_id", FixedAmount: 3},
	}, "tester", "v1"); err != nil {
		t.Fatalf("save alpha v1 failed: %v", err)
	}
	if _, err := svc.PublishVersion("alpha", alphaV1.Version, "tester"); err != nil {
		t.Fatalf("publish alpha v1 failed: %v", err)
	}

	published := svc.publishedVersions()
	if len(published) != 2 || published[0].ProgramKey != "alpha" || published[1].ProgramKey != "zeta" {
		t.Fatalf("expected sorted published versions, got %+v", published)
	}
	if version, ok := svc.resolveVersion("zeta", 0); !ok || version.Version != 1 {
		t.Fatalf("expected published version fallback, got %+v", version)
	}

	older := time.Now().Add(-time.Minute)
	newer := time.Now()
	if err := repo.SaveReplayRun(ReplayRun{ID: "older", StartedAt: older}); err != nil {
		t.Fatalf("save older replay failed: %v", err)
	}
	if err := repo.SaveReplayRun(ReplayRun{ID: "newer", StartedAt: newer}); err != nil {
		t.Fatalf("save newer replay failed: %v", err)
	}
	replays := svc.ListReplayRuns()
	if len(replays) != 2 || replays[0].ID != "newer" || replays[1].ID != "older" {
		t.Fatalf("expected replays sorted by newest first, got %+v", replays)
	}
}

func TestEngagementHelperFunctions(t *testing.T) {
	event := eventing.Event{
		ActorID:        "actor-1",
		AggregateID:    "order-1",
		OrganizationID: "org-1",
		LocationID:     "loc-1",
		Payload: map[string]any{
			"customer": map[string]any{
				"id": "cust-1",
				"profile": map[string]any{
					"level": "gold",
				},
			},
			"amount_int":    7,
			"amount_int64":  int64(8),
			"amount_float":  float64(9),
			"amount_string": "10",
			"amount_bad":    true,
		},
	}

	cases := map[string]string{
		"actor_id":                 "actor-1",
		"aggregate_id":             "order-1",
		"organization_id":          "org-1",
		"location_id":              "loc-1",
		"payload.customer.id":      "cust-1",
		"payload.customer.profile": "map[level:gold]",
		"unknown":                  "",
	}
	for source, want := range cases {
		if got := resolveSubjectID(source, event); got != want {
			t.Fatalf("resolveSubjectID(%q) = %q, want %q", source, got, want)
		}
	}

	if got := resolvePayloadPath(event.Payload, "customer.profile.level"); got != "gold" {
		t.Fatalf("resolvePayloadPath returned %v", got)
	}
	if got := resolvePayloadPath(event.Payload, "customer.profile.level.value"); got != nil {
		t.Fatalf("expected nil for over-deep path, got %v", got)
	}

	if got := resolveAmount(Rule{FixedAmount: 4}, event); got != 4 {
		t.Fatalf("expected fixed amount, got %d", got)
	}
	if got := resolveAmount(Rule{AmountField: "amount_int"}, event); got != 7 {
		t.Fatalf("expected int amount, got %d", got)
	}
	if got := resolveAmount(Rule{AmountField: "amount_int64"}, event); got != 8 {
		t.Fatalf("expected int64 amount, got %d", got)
	}
	if got := resolveAmount(Rule{AmountField: "amount_float"}, event); got != 9 {
		t.Fatalf("expected float amount, got %d", got)
	}
	if got := resolveAmount(Rule{AmountField: "amount_string"}, event); got != 10 {
		t.Fatalf("expected string amount, got %d", got)
	}
	if got := resolveAmount(Rule{AmountField: "amount_bad"}, event); got != 0 {
		t.Fatalf("expected unsupported amount type to resolve to 0, got %d", got)
	}

	if qualificationPtr(QualificationState{}, false) != nil {
		t.Fatal("expected nil qualification pointer when not found")
	}
	if got := qualificationPtr(QualificationState{TierKey: "gold"}, true); got == nil || got.TierKey != "gold" {
		t.Fatalf("unexpected qualification pointer: %+v", got)
	}
}
