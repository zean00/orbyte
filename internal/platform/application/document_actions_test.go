package application

import (
	"testing"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/workflow"
)

func TestSubmitRecordsAuditAndOutbox(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	record, err = actions.Submit(record.Header.ID, "user_admin", 1, record.Header.ETag)
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
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = actions.Submit(record.Header.ID, "user_admin", 99, record.Header.ETag)
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
}

func TestUpdateDraftRecordsAuditAndOutbox(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	record, err = actions.UpdateDraft(record.Header.ID, "user_admin", map[string]any{"title": "y"}, 1, record.Header.ETag)
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
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	_, _ = actions.Submit(record.Header.ID, "user_admin", 1, record.Header.ETag)
	latest, _ := docs.Get(record.Header.ID)
	_, err := actions.UpdateDraft(record.Header.ID, "user_admin", map[string]any{"title": "z"}, latest.Header.Version, latest.Header.ETag)
	if err == nil {
		t.Fatal("expected non-draft update conflict")
	}
}

func TestApproveResolvesWorkflowArtifacts(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = actions.Submit(record.Header.ID, "user_admin", 1, record.Header.ETag)
	record, err := actions.Approve(record.Header.ID, "user_admin", record.Header.Version, record.Header.ETag)
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
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = actions.Submit(record.Header.ID, "user_admin", 1, record.Header.ETag)
	record, err := actions.Reject(record.Header.ID, "user_admin", record.Header.Version, record.Header.ETag)
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
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = actions.Cancel(record.Header.ID, "user_admin", 1, record.Header.ETag)
	if record.Header.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", record.Header.Status)
	}

	record2, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "y"})
	record2, _ = actions.Submit(record2.Header.ID, "user_admin", 1, record2.Header.ETag)
	record2, _ = actions.Reject(record2.Header.ID, "user_admin", record2.Header.Version, record2.Header.ETag)
	record2, err := actions.Reopen(record2.Header.ID, "user_admin", record2.Header.Version, record2.Header.ETag)
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
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, _ = docs.ReplaceExtension(record.Header.ID, "analytics", map[string]any{"score": 9})
	record, err := actions.UpdateDraft(record.Header.ID, "user_admin", map[string]any{"title": "y"}, record.Header.Version, record.Header.ETag)
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
	actions := NewDocumentActions(docs, flows, nil, NewMemorySubmitStore(docs, flows, auditSvc, eventingSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	record, err := actions.UpdateExtension(record.Header.ID, "analytics", "user_admin", map[string]any{"score": 7}, record.Header.Version, record.Header.ETag)
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
