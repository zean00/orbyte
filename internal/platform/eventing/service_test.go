package eventing

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/search"
)

type failingHandler struct{}

func (failingHandler) Handle(context.Context, Event) error { return errors.New("boom") }

type recordingPublisher struct {
	topics []string
	events []Event
}

func (p *recordingPublisher) Publish(_ context.Context, topic string, _ string, event Event) error {
	p.topics = append(p.topics, topic)
	p.events = append(p.events, event)
	return nil
}

func TestServiceRecordCreatesEventAndOutbox(t *testing.T) {
	svc := NewService()
	err := svc.Record(Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("record failed: %v", err)
	}
	if len(svc.ListEvents()) != 1 {
		t.Fatal("expected one event")
	}
	if len(svc.ListOutbox()) != 1 {
		t.Fatal("expected one outbox record")
	}
	if len(svc.ListDeliveries()) != 1 {
		t.Fatal("expected one local delivery record")
	}
}

func TestDispatchPendingMarksDispatched(t *testing.T) {
	svc := NewService()
	_ = svc.Record(Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", OccurredAt: time.Now().UTC()})
	count, err := svc.DispatchPending(10)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one dispatch, got %d", count)
	}
	if svc.ListOutbox()[0].Status != "dispatched" {
		t.Fatalf("expected dispatched status")
	}
}

func TestDispatchPendingRunsRegisteredHandler(t *testing.T) {
	docs := document.NewService()
	searchSvc := search.NewService()
	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	svc := NewService()
	svc.RegisterHandler("document.updated", NewDocumentProjectionHandler(docs, searchSvc))
	if err := svc.Record(Event{ID: "e1", Type: "document.updated", Version: 1, AggregateType: "document", AggregateID: record.Header.ID, OccurredAt: time.Now().UTC(), Payload: map[string]any{"document_id": record.Header.ID, "status": record.Header.Status}}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	count, err := svc.DispatchPending(10)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one dispatch, got %d", count)
	}
	if len(searchSvc.ListDocuments()) != 1 {
		t.Fatal("expected projection refresh from handler")
	}
}

func TestDispatchPendingRefreshesProjectionForAllDocumentLifecycleEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status string
		event  string
	}{
		{name: "submitted", status: "submitted", event: "document.submitted"},
		{name: "approved", status: "approved", event: "document.approved"},
		{name: "rejected", status: "rejected", event: "document.reject"},
		{name: "reopened", status: "draft", event: "document.reopened"},
		{name: "cancelled", status: "cancelled", event: "document.cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			docs := document.NewService()
			searchSvc := search.NewService()
			record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
			if err != nil {
				t.Fatalf("create failed: %v", err)
			}
			record.Header.Status = tc.status
			record.Header.Version = 2
			record.Header.ETag = record.Header.ID + ":2"
			record.Header.UpdatedAt = time.Now().UTC()
			if err := docs.Save(record); err != nil {
				t.Fatalf("save failed: %v", err)
			}

			svc := NewService()
			svc.RegisterHandler(tc.event, NewDocumentProjectionHandler(docs, searchSvc))
			if err := svc.Record(Event{ID: "e1", Type: tc.event, Version: 1, AggregateType: "document", AggregateID: record.Header.ID, OccurredAt: time.Now().UTC()}); err != nil {
				t.Fatalf("record failed: %v", err)
			}
			count, err := svc.DispatchPending(10)
			if err != nil {
				t.Fatalf("dispatch failed: %v", err)
			}
			if count != 1 {
				t.Fatalf("expected one dispatch, got %d", count)
			}
			items := searchSvc.ListDocuments()
			if len(items) != 1 {
				t.Fatalf("expected projection item, got %d", len(items))
			}
			if items[0].Status != tc.status {
				t.Fatalf("expected status %s, got %s", tc.status, items[0].Status)
			}
		})
	}
}

func TestDispatcherStartStop(t *testing.T) {
	svc := NewService()
	_ = svc.Record(Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", OccurredAt: time.Now().UTC()})
	d := NewDispatcher(svc, 10*time.Millisecond, 10)
	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	d.Stop()
	if svc.ListOutbox()[0].Status != "dispatched" {
		t.Fatalf("expected dispatcher to dispatch outbox")
	}
}

func TestDispatchPendingRetriesThenDeadLetters(t *testing.T) {
	svc := NewService()
	svc.RegisterHandler("document.updated", failingHandler{})
	if err := svc.Record(Event{ID: "e1", Type: "document.updated", Version: 1, AggregateType: "document", AggregateID: "d1", OccurredAt: time.Now().UTC(), Payload: map[string]any{"document_id": "d1", "status": "draft"}}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		_, err := svc.DispatchPending(10)
		if err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}
	}
	if len(svc.ListDeadLetters()) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(svc.ListDeadLetters()))
	}
	outbox := svc.ListOutbox()
	if len(outbox) != 1 || outbox[0].Status != "dead_letter" {
		t.Fatalf("expected dead_letter outbox status")
	}
	if items := svc.ListDeadLetters(); len(items) == 0 || items[0].SinkName != "local" {
		t.Fatalf("expected local sink dead letter, got %+v", items)
	}
}

func TestDispatcherReportsFailureWhenDeliveriesFail(t *testing.T) {
	svc := NewService()
	svc.RegisterHandler("document.updated", failingHandler{})
	if err := svc.Record(Event{ID: "e1", Type: "document.updated", Version: 1, AggregateType: "document", AggregateID: "d1", OccurredAt: time.Now().UTC(), Payload: map[string]any{"document_id": "d1", "status": "draft"}}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	dispatcher := NewDispatcher(svc, 10*time.Millisecond, 10)
	var successes atomic.Int32
	var failures atomic.Int32
	dispatcher.SetHealthHooks(func() {
		successes.Add(1)
	}, func(error) {
		failures.Add(1)
	})
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()
	dispatcher.Stop()

	if failures.Load() == 0 {
		t.Fatal("expected dispatcher failure hook to run")
	}
	if successes.Load() != 0 {
		t.Fatalf("expected no success health hook, got %d", successes.Load())
	}
}

func TestDispatchPendingPublishesToBrokerSink(t *testing.T) {
	svc := NewService()
	pub := &recordingPublisher{}
	svc.RegisterBrokerSink("broker", pub, map[string]string{"document.submitted": "documents.lifecycle"})
	if err := svc.Record(Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", OccurredAt: time.Now().UTC(), CorrelationID: "c1"}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	if len(svc.ListDeliveries()) != 2 {
		t.Fatalf("expected local and broker deliveries, got %d", len(svc.ListDeliveries()))
	}
	count, err := svc.DispatchPending(10)
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected two sink dispatches, got %d", count)
	}
	if len(pub.events) != 1 || len(pub.topics) != 1 || pub.topics[0] != "documents.lifecycle" {
		t.Fatalf("expected broker publication, got topics=%v events=%d", pub.topics, len(pub.events))
	}
	if pub.events[0].CorrelationID != "c1" {
		t.Fatalf("expected stable envelope field to be preserved, got %+v", pub.events[0])
	}
}

func TestNewNATSPublisherRejectsInvalidURL(t *testing.T) {
	if _, err := NewNATSPublisher("://bad-url", time.Second); err == nil {
		t.Fatal("expected invalid nats url to fail")
	}
}

func TestRecordRejectsKnownEventSchemaViolations(t *testing.T) {
	svc := NewService()
	err := svc.Record(Event{
		ID:            "e1",
		Type:          "document.updated",
		Version:       1,
		AggregateType: "document",
		AggregateID:   "d1",
		OccurredAt:    time.Now().UTC(),
		Payload: map[string]any{
			"document_id": "d1",
		},
	})
	if err == nil {
		t.Fatal("expected event schema validation to fail")
	}
}
