package application

import (
	"testing"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/workflow"
)

func testActing(userID string) ActingContext {
	return ActingContext{ActorID: userID, EffectiveUserID: userID}
}

func TestSubmitRecordsAuditAndOutbox(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	record, err = actions.Submit(record.Header.ID, testActing("user_admin"), 1, record.Header.ETag)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if record.Header.Status != "submitted" {
		t.Fatalf("expected submitted status, got %s", record.Header.Status)
	}
	if record.Header.Number == "" {
		t.Fatal("expected numbering assignment on submit")
	}
	if len(auditSvc.List()) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(auditSvc.List()))
	}
	if len(eventingSvc.ListOutbox()) != 1 {
		t.Fatalf("expected 1 outbox record, got %d", len(eventingSvc.ListOutbox()))
	}
}

func TestSubmitRejectsVersionMismatch(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = actions.Submit(record.Header.ID, testActing("user_admin"), 99, record.Header.ETag)
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
}

func TestUpdateDraftRecordsAuditAndOutbox(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	record, err = actions.UpdateDraft(record.Header.ID, testActing("user_admin"), map[string]any{"title": "y"}, 1, record.Header.ETag)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if record.Header.Version != 2 {
		t.Fatalf("expected version 2, got %d", record.Header.Version)
	}
	if record.Body.Payload["title"] != "y" {
		t.Fatalf("expected updated payload")
	}
	if len(auditSvc.List()) != 1 || len(eventingSvc.ListOutbox()) != 1 {
		t.Fatal("expected audit and outbox entries")
	}
}

func TestUpdateDraftRejectsNonDraft(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	_, _ = actions.Submit(record.Header.ID, testActing("user_admin"), 1, record.Header.ETag)
	latest, _ := docs.Get(record.Header.ID)
	_, err := actions.UpdateDraft(record.Header.ID, testActing("user_admin"), map[string]any{"title": "z"}, latest.Header.Version, latest.Header.ETag)
	if err == nil {
		t.Fatal("expected non-draft update conflict")
	}
}

func TestApproveResolvesWorkflowArtifacts(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = actions.Submit(record.Header.ID, testActing("user_admin"), 1, record.Header.ETag)
	record, err := actions.Approve(record.Header.ID, testActing("user_admin"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if record.Header.Status != "approved" {
		t.Fatalf("expected approved status, got %s", record.Header.Status)
	}
	if len(flows.ListTasks()) != 1 || flows.ListTasks()[0].Status != "completed" {
		t.Fatal("expected completed task")
	}
	if len(flows.ListApprovals()) != 1 || flows.ListApprovals()[0].Status != "approved" {
		t.Fatal("expected approved approval")
	}
}

func TestRejectResolvesWorkflowArtifacts(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = actions.Submit(record.Header.ID, testActing("user_admin"), 1, record.Header.ETag)
	record, err := actions.Reject(record.Header.ID, testActing("user_admin"), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}
	if record.Header.Status != "rejected" {
		t.Fatalf("expected rejected status, got %s", record.Header.Status)
	}
	if flows.ListTasks()[0].Status != "cancelled" || flows.ListApprovals()[0].Status != "rejected" {
		t.Fatal("expected cancelled task and rejected approval")
	}
}

func TestReopenAndCancel(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = actions.Cancel(record.Header.ID, testActing("user_admin"), 1, record.Header.ETag)
	if record.Header.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", record.Header.Status)
	}

	record2, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "y"})
	record2, _ = actions.Submit(record2.Header.ID, testActing("user_admin"), 1, record2.Header.ETag)
	record2, _ = actions.Reject(record2.Header.ID, testActing("user_admin"), record2.Header.Version, record2.Header.ETag)
	record2, err := actions.Reopen(record2.Header.ID, testActing("user_admin"), record2.Header.Version, record2.Header.ETag)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if record2.Header.Status != "draft" {
		t.Fatalf("expected draft status after reopen, got %s", record2.Header.Status)
	}
}

func TestUpdateDraftPreservesExtensions(t *testing.T) {
	docs := document.NewService()
	_ = docs.RegisterExtension(document.ExtensionDefinition{DocumentType: "generic_request", ModuleKey: "analytics", DisplayName: "Analytics", SchemaVersion: "v1"})
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = docs.ReplaceExtension(record.Header.ID, "analytics", map[string]any{"score": 9})
	record, err := actions.UpdateDraft(record.Header.ID, testActing("user_admin"), map[string]any{"title": "y"}, record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if record.Body.Payload["title"] != "y" {
		t.Fatalf("expected updated title, got %+v", record.Body.Payload)
	}
	if got := document.ExtensionPayload(record.Body.Payload, "analytics"); got["score"] != 9 {
		t.Fatalf("expected preserved extension payload, got %+v", got)
	}
}

func TestUpdateExtensionRecordsAuditAndOutbox(t *testing.T) {
	docs := document.NewService()
	_ = docs.RegisterExtension(document.ExtensionDefinition{DocumentType: "generic_request", ModuleKey: "analytics", DisplayName: "Analytics", SchemaVersion: "v1"})
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, err := actions.UpdateExtension(record.Header.ID, "analytics", testActing("user_admin"), map[string]any{"score": 7}, record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("update extension failed: %v", err)
	}
	if got := document.ExtensionPayload(record.Body.Payload, "analytics"); got["score"] != 7 {
		t.Fatalf("expected extension payload, got %+v", got)
	}
	if len(auditSvc.List()) != 1 || len(eventingSvc.ListOutbox()) != 1 {
		t.Fatal("expected audit and outbox entries")
	}
}

func TestApproveCreatesManagerChainApprovalSnapshot(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	requester, err := ident.CreateUser("requester", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create requester failed: %v", err)
	}
	supervisor, err := ident.CreateUser("supervisor", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create supervisor failed: %v", err)
	}
	manager, err := ident.CreateUser("manager", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}
	if _, err := ident.UpsertReportingLine(identity.ReportingLine{SubjectUserID: requester.ID, ManagerUserID: supervisor.ID, RelationshipType: "primary_manager", Status: "active"}); err != nil {
		t.Fatalf("save requester reporting line failed: %v", err)
	}
	if _, err := ident.UpsertReportingLine(identity.ReportingLine{SubjectUserID: supervisor.ID, ManagerUserID: manager.ID, RelationshipType: "primary_manager", Status: "active"}); err != nil {
		t.Fatalf("save supervisor reporting line failed: %v", err)
	}

	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "manager_request", DisplayName: "Manager Request", SchemaVersion: "v1", WorkflowKey: "manager_chain_flow", NumberingKey: "manager_request_number"}); err != nil {
		t.Fatalf("register document definition failed: %v", err)
	}
	flows := workflow.NewService()
	_ = flows.Register(workflow.Definition{
		Key:    "manager_chain_flow",
		States: []string{"draft", "pending_supervisor", "pending_manager", "approved", "rejected", "cancelled"},
		Actions: []workflow.ActionRule{
			{Action: "submit", FromState: "draft", ToState: "pending_supervisor", CreateApproval: true, TaskType: "review", AssignmentStrategy: "requester_manager", FallbackRoleKey: "platform_admin", ApprovalStageKey: "supervisor"},
			{Action: "approve", FromState: "pending_supervisor", ToState: "pending_manager", CreateApproval: true, TaskType: "review", AssignmentStrategy: "previous_approver_manager", FallbackRoleKey: "platform_admin", ApprovalStageKey: "manager"},
			{Action: "approve", FromState: "pending_manager", ToState: "approved"},
			{Action: "reject", FromState: "pending_supervisor", ToState: "rejected"},
			{Action: "reject", FromState: "pending_manager", ToState: "rejected"},
			{Action: "cancel", FromState: "draft", ToState: "cancelled"},
		},
	})
	transition, err := flows.Execute("manager_chain_flow", "draft", "submit")
	if err != nil {
		t.Fatalf("execute transition failed: %v", err)
	}
	if transition.AssignmentStrategy != "requester_manager" {
		t.Fatalf("expected requester_manager strategy, got %+v", transition)
	}
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, ident, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, err := docs.Create("manager_request", "org_default", "loc_hq", requester.ID, map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	record, err = actions.Submit(record.Header.ID, testActing(requester.ID), 1, record.Header.ETag)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	approvals := flows.ListApprovals()
	if got := approvals[0].Metadata["resolved_assignee_user_id"]; got != supervisor.ID {
		t.Fatalf("expected supervisor assignee, got %+v metadata=%+v", got, approvals[0].Metadata)
	}

	record, err = actions.Approve(record.Header.ID, testActing(supervisor.ID), record.Header.Version, record.Header.ETag)
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	approvals = flows.ListApprovals()
	if len(approvals) != 2 {
		t.Fatalf("expected 2 approvals, got %d", len(approvals))
	}
	if approvals[0].Status != "approved" {
		t.Fatalf("expected first approval approved, got %s", approvals[0].Status)
	}
	if got := approvals[1].Metadata["resolved_assignee_user_id"]; got != manager.ID {
		t.Fatalf("expected manager assignee, got %+v metadata=%+v", got, approvals[1].Metadata)
	}
	if record.Header.Status != "pending_manager" {
		t.Fatalf("expected pending_manager status, got %s", record.Header.Status)
	}
}
