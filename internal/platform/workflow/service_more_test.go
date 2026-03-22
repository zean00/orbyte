package workflow

import (
	"strings"
	"testing"
	"time"
)

func TestDraftLifecycleVersionOpsAndPublish(t *testing.T) {
	svc := NewService()

	draft, err := svc.CreateDraft("generic_request_flow", "tester")
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}
	if draft.Status != "draft" || draft.Version != 2 || draft.UpdatedBy != "tester" {
		t.Fatalf("unexpected draft: %+v", draft)
	}

	versions := svc.ListVersions("generic_request_flow")
	if len(versions) != 2 {
		t.Fatalf("expected published version plus draft, got %+v", versions)
	}

	draft.Actions = append(draft.Actions, ActionRule{
		Action:        "archive",
		FromState:     "approved",
		ToState:       "cancelled",
		PermissionKey: "document.archive",
	})
	saved, err := svc.SaveDraft(draft, "editor")
	if err != nil {
		t.Fatalf("save draft failed: %v", err)
	}
	if saved.Status != "draft" || saved.UpdatedBy != "editor" {
		t.Fatalf("unexpected saved draft: %+v", saved)
	}

	fetched, err := svc.GetVersion("generic_request_flow", 2)
	if err != nil {
		t.Fatalf("get draft version failed: %v", err)
	}
	if len(fetched.Actions) != len(saved.Actions) {
		t.Fatalf("expected saved draft actions to persist, got %+v", fetched)
	}

	published, err := svc.Publish("generic_request_flow", 2, "publisher")
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if published.Status != "published" || published.PublishedBy != "publisher" {
		t.Fatalf("unexpected published draft: %+v", published)
	}

	if _, err := svc.ExecuteVersion("generic_request_flow", 2, "approved", "archive"); err != nil {
		t.Fatalf("expected published v2 action to execute: %v", err)
	}
}

func TestWorkflowValidationSimulationAndFallbackBranches(t *testing.T) {
	svc := NewService()

	invalid := Definition{
		Key:     "broken",
		States:  []string{"draft"},
		Actions: []ActionRule{{Action: "submit", FromState: "draft", ToState: "missing"}},
	}
	validation := svc.Validate(invalid)
	if validation.Valid || len(validation.Issues) == 0 {
		t.Fatalf("expected invalid definition issues, got %+v", validation)
	}
	if err := svc.Register(invalid); err == nil {
		t.Fatal("expected invalid definition registration to fail")
	}

	if _, err := svc.SaveDraft(Definition{Key: "generic_request_flow", Version: 1, Status: "published", States: []string{"draft"}, Actions: []ActionRule{{Action: "submit", FromState: "draft", ToState: "draft"}}}, "tester"); err == nil {
		t.Fatal("expected non-draft save to fail")
	}

	simulated := svc.Simulate(invalid, SimulationInput{CurrentState: "draft", Action: "submit"})
	if simulated.Valid || len(simulated.Issues) == 0 {
		t.Fatalf("expected invalid simulation result, got %+v", simulated)
	}

	transition, err := svc.ExecuteVersion("generic_request_flow", 0, "draft", "submit")
	if err != nil || transition.ToState != "submitted" {
		t.Fatalf("expected version fallback to Execute, got transition=%+v err=%v", transition, err)
	}
	if _, err := svc.ExecuteVersion("generic_request_flow", 99, "draft", "submit"); err == nil {
		t.Fatal("expected missing version to fail")
	}
}

func TestWorkflowMutationsResolveArtifactsAndHistory(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()

	transition, err := svc.Execute("generic_request_flow", "draft", "submit")
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	mutation := svc.PlanCreateSideEffects(transition, "document", "doc-history", "actor-1", now)
	if len(mutation.Tasks) != 1 || len(mutation.Approvals) != 1 {
		t.Fatalf("expected side effects to be planned, got %+v", mutation)
	}
	if mutation.Tasks[0].DueAt.IsZero() || mutation.Tasks[0].EscalateAt.IsZero() {
		t.Fatalf("expected due/escalate times to be populated, got %+v", mutation.Tasks[0])
	}
	mutation.History = append(mutation.History, HistoryEvent{
		ID:          "hist-1",
		WorkflowKey: transition.WorkflowKey,
		TargetType:  "document",
		TargetID:    "doc-history",
		Action:      transition.Action,
		OccurredAt:  now,
	})
	if err := svc.ApplyMutation(mutation); err != nil {
		t.Fatalf("apply mutation failed: %v", err)
	}
	if len(svc.ListHistory("document", "doc-history")) != 1 {
		t.Fatalf("expected saved workflow history, got %+v", svc.ListHistory("document", "doc-history"))
	}

	if err := svc.ResolveArtifacts("doc-history", "rejected", "cancelled"); err != nil {
		t.Fatalf("resolve artifacts failed: %v", err)
	}
	if svc.ListTasks()[0].Status != "cancelled" {
		t.Fatalf("expected custom task resolution, got %+v", svc.ListTasks()[0])
	}
	if svc.ListApprovals()[0].Status != "rejected" {
		t.Fatalf("expected custom approval resolution, got %+v", svc.ListApprovals()[0])
	}

	if _, err := svc.Get("missing-workflow"); err == nil {
		t.Fatal("expected missing workflow get to fail")
	}
	if _, err := svc.GetVersion("generic_request_flow", 999); err == nil {
		t.Fatal("expected missing workflow version to fail")
	}
	if _, err := svc.Execute("generic_request_flow", "draft", "missing-action"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected invalid transition to fail, got %v", err)
	}
}

func TestWorkflowListDefinitionsAndHelper(t *testing.T) {
	svc := NewService()
	defs := svc.ListDefinitions()
	if len(defs) == 0 || defs[0].Key == "" {
		t.Fatalf("expected registered workflow definitions, got %+v", defs)
	}
	if got := firstNonEmpty("", "doc-1", "doc-2"); got != "doc-1" {
		t.Fatalf("expected firstNonEmpty to return first populated value, got %q", got)
	}
}
