package workflow

import (
	"testing"
	"time"
)

func TestExecuteSubmitTransition(t *testing.T) {
	svc := NewService()
	transition, err := svc.Execute("generic_request_flow", "draft", "submit")
	if err != nil {
		t.Fatalf("expected workflow transition, got error: %v", err)
	}
	if transition.ToState != "submitted" {
		t.Fatalf("expected submitted state, got %s", transition.ToState)
	}
}

func TestListKeysAndGet(t *testing.T) {
	svc := NewService()
	if len(svc.ListKeys()) == 0 {
		t.Fatal("expected workflow keys")
	}
	def, err := svc.Get("generic_request_flow")
	if err != nil {
		t.Fatalf("expected workflow get to succeed: %v", err)
	}
	if def.Key != "generic_request_flow" {
		t.Fatalf("unexpected workflow key: %s", def.Key)
	}
}

func TestCreateSideEffects(t *testing.T) {
	svc := NewService()
	transition, err := svc.Execute("generic_request_flow", "draft", "submit")
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if err := svc.CreateSideEffects(transition, "document", "doc1", time.Now().UTC()); err != nil {
		t.Fatalf("side effects failed: %v", err)
	}
	if len(svc.ListTasks()) != 1 {
		t.Fatal("expected workflow task")
	}
	if len(svc.ListApprovals()) != 1 {
		t.Fatal("expected workflow approval")
	}
	if svc.ListTasks()[0].AssigneeRoleKey != "approver" {
		t.Fatalf("expected default assignee role, got %+v", svc.ListTasks()[0])
	}
	if svc.ListApprovals()[0].StageKey != "review" {
		t.Fatalf("expected approval stage metadata, got %+v", svc.ListApprovals()[0])
	}
	if err := svc.ResolveApproval("doc1"); err != nil {
		t.Fatalf("resolve approval failed: %v", err)
	}
	if svc.ListTasks()[0].Status != "completed" {
		t.Fatalf("expected completed task, got %s", svc.ListTasks()[0].Status)
	}
	if svc.ListApprovals()[0].Status != "approved" {
		t.Fatalf("expected approved approval, got %s", svc.ListApprovals()[0].Status)
	}
}

func TestExecuteAdditionalTransitions(t *testing.T) {
	svc := NewService()
	for _, tc := range []struct{ state, action, next string }{
		{"submitted", "approve", "approved"},
		{"submitted", "reject", "rejected"},
		{"approved", "reopen", "draft"},
		{"draft", "cancel", "cancelled"},
	} {
		tr, err := svc.Execute("generic_request_flow", tc.state, tc.action)
		if err != nil {
			t.Fatalf("execute %s from %s failed: %v", tc.action, tc.state, err)
		}
		if tr.ToState != tc.next {
			t.Fatalf("expected %s -> %s, got %s", tc.state, tc.next, tr.ToState)
		}
	}
}
