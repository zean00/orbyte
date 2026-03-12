package application

import (
	"context"
	"errors"
	"testing"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/model"
	"clinic/internal/platform/workflow"
)

func TestRunKernelCommandHandlesNilRunnerAndErrors(t *testing.T) {
	cmd := sampleKernelCommand{result: kernelSampleResult{Value: "ok"}}

	out, err := RunKernelCommand(context.Background(), nil, cmd)
	if err != nil {
		t.Fatalf("expected nil runner to be tolerated, got %v", err)
	}
	if out != (kernelSampleResult{}) {
		t.Fatalf("expected zero result on nil runner, got %+v", out)
	}

	runner := NewKernelCommandRunner(errorTransactionManager{err: errors.New("boom")})
	_, err = RunKernelCommand(context.Background(), runner, cmd)
	if err == nil {
		t.Fatal("expected runner error")
	}
}

func TestDocumentPersistCommandRunSavesOutboxAndWorkflowMutation(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	txm := NewMemoryTransactionManager(docs, models, flows, auditSvc, eventingSvc)
	record, err := docs.Create("generic_request", "org_default", "loc_hq", "u1", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	record.Header.Status = "submitted"
	record.Header.Version++
	record.Header.ETag = record.Header.ID + ":2"

	err = txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		_, err := documentPersistCommand{
			previousVersion: record.Header.Version - 1,
			record:          record,
			auditEvent:      audit.Event{ID: "a1", Action: "document.submit", TargetType: "document", TargetID: record.Header.ID, ActorID: "u1"},
			domainEvent:     eventing.Event{ID: "e1", Type: "document.submitted", AggregateType: "document", AggregateID: record.Header.ID},
			outboxRecord:    eventing.OutboxRecord{ID: "o1", EventID: "e1", EventType: "document.submitted", Status: "pending"},
			workflowMutation: workflow.Mutation{
				Tasks: []workflow.Task{{ID: "t1", WorkflowKey: "generic_request_flow", TargetType: "document", TargetID: record.Header.ID, TaskType: "review", Status: "open"}},
			},
		}.Run(context.Background(), uow)
		return err
	})
	if err != nil {
		t.Fatalf("persist command failed: %v", err)
	}
	if len(auditSvc.List()) != 1 || len(eventingSvc.ListEvents()) != 1 || len(flows.ListTasks()) != 1 {
		t.Fatalf("expected runtime artifacts to persist, got audit=%d events=%d tasks=%d", len(auditSvc.List()), len(eventingSvc.ListEvents()), len(flows.ListTasks()))
	}
}

type sampleKernelCommand struct {
	result kernelSampleResult
}

type kernelSampleResult struct {
	Value string
}

func (c sampleKernelCommand) Run(_ context.Context, _ UnitOfWork) (kernelSampleResult, error) {
	return c.result, nil
}

type errorTransactionManager struct {
	err error
}

func (m errorTransactionManager) WithinTx(_ context.Context, _ func(UnitOfWork) error) error {
	return m.err
}
