package application

import (
	"regexp"
	"testing"
	"time"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/shared"
	"clinic/internal/platform/workflow"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresSubmitStoreSubmit(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	store := NewPostgresSubmitStore(db)
	now := time.Now().UTC()
	record := document.Record{Header: document.Header{ID: "d1", Type: "generic_request", Status: "submitted", Version: 2, ETag: "d1:2", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, SubmittedBy: "u1", SubmittedAt: now, TotalAmount: shared.Money{Currency: "IDR"}}, Body: document.Body{SchemaVersion: "v1", Payload: map[string]any{"title": "x"}}}
	auditEvent := audit.Event{ID: "a1", Action: "document.submit", TargetType: "document", TargetID: "d1", ActorID: "u1", OccurredAt: now}
	domainEvent := eventing.Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", ActorID: "u1", OccurredAt: now, Payload: map[string]any{"x": 1}}
	outbox := eventing.OutboxRecord{ID: "o1", EventID: "e1", EventType: "document.submitted", Status: "pending", CreatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE document_records")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := store.Submit(1, record, auditEvent, domainEvent, outbox, workflow.Mutation{}); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
}

func TestPostgresSubmitStoreUpdateDraftConflict(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	store := NewPostgresSubmitStore(db)
	now := time.Now().UTC()
	record := document.Record{Header: document.Header{ID: "d1", Type: "generic_request", Status: "draft", Version: 2, ETag: "d1:2", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}}, Body: document.Body{SchemaVersion: "v1", Payload: map[string]any{"title": "x"}}}
	auditEvent := audit.Event{ID: "a1", Action: "document.update", TargetType: "document", TargetID: "d1", ActorID: "u1", OccurredAt: now}
	domainEvent := eventing.Event{ID: "e1", Type: "document.updated", Version: 1, AggregateType: "document", AggregateID: "d1", ActorID: "u1", OccurredAt: now, Payload: map[string]any{"x": 1}}
	outbox := eventing.OutboxRecord{ID: "o1", EventID: "e1", EventType: "document.updated", Status: "pending", CreatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE document_records")).WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectRollback()

	if err := store.UpdateDraft(1, record, auditEvent, domainEvent, outbox, workflow.Mutation{}); err == nil {
		t.Fatal("expected version conflict")
	}
}

func TestPostgresSubmitStorePersistsWorkflowMutationInTransaction(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	store := NewPostgresSubmitStore(db)
	now := time.Now().UTC()
	record := document.Record{Header: document.Header{ID: "d1", Type: "generic_request", Status: "submitted", Version: 2, ETag: "d1:2", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, SubmittedBy: "u1", SubmittedAt: now, TotalAmount: shared.Money{Currency: "IDR"}}, Body: document.Body{SchemaVersion: "v1", Payload: map[string]any{"title": "x"}}}
	auditEvent := audit.Event{ID: "a1", Action: "document.submit", TargetType: "document", TargetID: "d1", ActorID: "u1", OccurredAt: now}
	domainEvent := eventing.Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", ActorID: "u1", OccurredAt: now, Payload: map[string]any{"x": 1}}
	outbox := eventing.OutboxRecord{ID: "o1", EventID: "e1", EventType: "document.submitted", Status: "pending", CreatedAt: now}
	mutation := workflow.Mutation{
		Tasks:     []workflow.Task{{ID: "task:1", WorkflowKey: "wf", TargetType: "document", TargetID: "d1", TaskType: "review", Status: "open", CreatedAt: now}},
		Approvals: []workflow.Approval{{ID: "approval:1", WorkflowKey: "wf", TargetType: "document", TargetID: "d1", Status: "pending", RequestedAt: now}},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE document_records")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_tasks (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_approvals (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := store.Submit(1, record, auditEvent, domainEvent, outbox, mutation); err != nil {
		t.Fatalf("submit with workflow mutation failed: %v", err)
	}
}

func TestPostgresSubmitStoreRollsBackWhenWorkflowMutationFails(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	store := NewPostgresSubmitStore(db)
	now := time.Now().UTC()
	record := document.Record{Header: document.Header{ID: "d1", Type: "generic_request", Status: "submitted", Version: 2, ETag: "d1:2", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, SubmittedBy: "u1", SubmittedAt: now, TotalAmount: shared.Money{Currency: "IDR"}}, Body: document.Body{SchemaVersion: "v1", Payload: map[string]any{"title": "x"}}}
	auditEvent := audit.Event{ID: "a1", Action: "document.submit", TargetType: "document", TargetID: "d1", ActorID: "u1", OccurredAt: now}
	domainEvent := eventing.Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", ActorID: "u1", OccurredAt: now, Payload: map[string]any{"x": 1}}
	outbox := eventing.OutboxRecord{ID: "o1", EventID: "e1", EventType: "document.submitted", Status: "pending", CreatedAt: now}
	mutation := workflow.Mutation{
		Tasks: []workflow.Task{{ID: "task:1", WorkflowKey: "wf", TargetType: "document", TargetID: "d1", TaskType: "review", Status: "open", CreatedAt: now}},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE document_records")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO workflow_tasks (")).WillReturnError(assertAnError{})
	mock.ExpectRollback()

	if err := store.Submit(1, record, auditEvent, domainEvent, outbox, mutation); err == nil {
		t.Fatal("expected workflow mutation failure")
	}
}

type assertAnError struct{}

func (assertAnError) Error() string { return "workflow mutation failed" }
