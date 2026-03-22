package engagement

import (
	"context"
	"testing"
	"time"

	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/jobs"
)

func TestProgramPublishesProcessesEventsAndReplays(t *testing.T) {
	svc := NewService()
	events := eventing.NewService()
	jobSvc := jobs.NewService()
	svc.AttachRuntime(events, jobSvc)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobSvc.Start(ctx)
	defer jobSvc.Stop()

	if _, err := svc.CreateProgram("loyalty", "Customer Loyalty", "customer", "tester"); err != nil {
		t.Fatalf("create program failed: %v", err)
	}
	version, err := svc.CreateDraftVersion("loyalty", "tester", "initial")
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}
	version, err = svc.SaveVersion("loyalty", version.Version, []Rule{
		{Key: "earn_purchase", Action: "credit_points", SourceEventTypes: []string{"order.completed"}, SubjectSource: "actor_id", AccountKey: "points", FixedAmount: 10},
		{Key: "first_order", Action: "grant_achievement", SourceEventTypes: []string{"order.completed"}, SubjectSource: "actor_id", AccountKey: "points", Threshold: 10, AchievementKey: "first_order"},
		{Key: "bronze_tier", Action: "set_tier", SourceEventTypes: []string{"order.completed"}, SubjectSource: "actor_id", AccountKey: "points", Threshold: 10, TierKey: "bronze"},
	}, "tester", "initial rules")
	if err != nil {
		t.Fatalf("save draft failed: %v", err)
	}
	if _, err := svc.PublishVersion("loyalty", version.Version, "tester"); err != nil {
		t.Fatalf("publish version failed: %v", err)
	}

	event := eventing.Event{
		ID:            "evt1",
		Type:          "order.completed",
		AggregateType: "order",
		AggregateID:   "ord1",
		ActorID:       "cust1",
		OccurredAt:    time.Now().UTC(),
	}
	if err := events.Record(event); err != nil {
		t.Fatalf("record event failed: %v", err)
	}
	if _, err := events.DispatchPending(10); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	balance, ok := svc.GetBalance("loyalty", "cust1", "points")
	if !ok || balance.Balance != 10 {
		t.Fatalf("expected balance 10, got %+v", balance)
	}
	if len(svc.ListJournal("loyalty", "cust1", "points")) != 1 {
		t.Fatal("expected one journal entry")
	}
	if len(svc.ListAchievements("loyalty", "cust1")) != 1 {
		t.Fatal("expected one achievement")
	}
	qualification, ok := svc.GetQualification("loyalty", "cust1")
	if !ok || qualification.TierKey != "bronze" {
		t.Fatalf("expected bronze qualification, got %+v", qualification)
	}

	if err := svc.ProcessEvent(event); err != nil {
		t.Fatalf("idempotent reprocess failed: %v", err)
	}
	balance, _ = svc.GetBalance("loyalty", "cust1", "points")
	if balance.Balance != 10 {
		t.Fatalf("expected balance to remain 10 after duplicate processing, got %d", balance.Balance)
	}

	run, _, err := svc.StartReplay("loyalty", version.Version, "tester")
	if err != nil {
		t.Fatalf("start replay failed: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, ok := svc.GetReplayRun(run.ID)
		if ok && (current.Status == jobs.StatusSucceeded || current.Status == jobs.StatusFailed) {
			if current.Status != jobs.StatusSucceeded {
				t.Fatalf("expected succeeded replay, got %+v", current)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	balance, _ = svc.GetBalance("loyalty", "cust1", "points")
	if balance.Balance != 10 {
		t.Fatalf("expected balance 10 after replay, got %d", balance.Balance)
	}
}

func TestReplayRejectsNonPublishedVersions(t *testing.T) {
	svc := NewService()
	if _, err := svc.CreateProgram("loyalty", "Customer Loyalty", "customer", "tester"); err != nil {
		t.Fatalf("create program failed: %v", err)
	}
	version1, err := svc.CreateDraftVersion("loyalty", "tester", "v1")
	if err != nil {
		t.Fatalf("create draft v1 failed: %v", err)
	}
	version1, err = svc.SaveVersion("loyalty", version1.Version, []Rule{
		{Key: "earn_purchase", Action: "credit_points", SourceEventTypes: []string{"order.completed"}, SubjectSource: "actor_id", AccountKey: "points", FixedAmount: 10},
	}, "tester", "v1")
	if err != nil {
		t.Fatalf("save v1 failed: %v", err)
	}
	if _, err := svc.PublishVersion("loyalty", version1.Version, "tester"); err != nil {
		t.Fatalf("publish v1 failed: %v", err)
	}

	version2, err := svc.CreateDraftVersion("loyalty", "tester", "v2")
	if err != nil {
		t.Fatalf("create draft v2 failed: %v", err)
	}
	if _, err := svc.SaveVersion("loyalty", version2.Version, []Rule{
		{Key: "earn_purchase_v2", Action: "credit_points", SourceEventTypes: []string{"order.completed"}, SubjectSource: "actor_id", AccountKey: "points", FixedAmount: 20},
	}, "tester", "v2"); err != nil {
		t.Fatalf("save v2 failed: %v", err)
	}

	if _, err := svc.ReplayPlan("loyalty", version2.Version); err == nil {
		t.Fatal("expected replay plan for non-published version to fail")
	}
}

func TestInactiveProgramDoesNotProcessEvents(t *testing.T) {
	svc := NewService()
	events := eventing.NewService()
	svc.AttachRuntime(events, nil)

	if _, err := svc.CreateProgram("loyalty", "Customer Loyalty", "customer", "tester"); err != nil {
		t.Fatalf("create program failed: %v", err)
	}
	version, err := svc.CreateDraftVersion("loyalty", "tester", "initial")
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}
	version, err = svc.SaveVersion("loyalty", version.Version, []Rule{
		{Key: "earn_purchase", Action: "credit_points", SourceEventTypes: []string{"order.completed"}, SubjectSource: "actor_id", AccountKey: "points", FixedAmount: 10},
	}, "tester", "initial rules")
	if err != nil {
		t.Fatalf("save draft failed: %v", err)
	}
	if _, err := svc.PublishVersion("loyalty", version.Version, "tester"); err != nil {
		t.Fatalf("publish version failed: %v", err)
	}
	if _, err := svc.UpdateProgram("loyalty", "", "", "inactive", "tester"); err != nil {
		t.Fatalf("deactivate program failed: %v", err)
	}

	event := eventing.Event{
		ID:            "evt-inactive",
		Type:          "order.completed",
		AggregateType: "order",
		AggregateID:   "ord1",
		ActorID:       "cust1",
		OccurredAt:    time.Now().UTC(),
	}
	if err := svc.ProcessEvent(event); err != nil {
		t.Fatalf("process event failed: %v", err)
	}
	if _, ok := svc.GetBalance("loyalty", "cust1", "points"); ok {
		t.Fatal("expected inactive program to skip balance accrual")
	}
	if len(svc.ListJournal("loyalty", "cust1", "points")) != 0 {
		t.Fatal("expected inactive program to skip journal entries")
	}
}
