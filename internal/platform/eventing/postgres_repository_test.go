package eventing

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresEventingRepository(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveEvent(Event{ID: "e1", Type: "document.submitted", Version: 1, AggregateType: "document", AggregateID: "d1", ActorID: "u1", OccurredAt: now, Payload: map[string]any{"x": 1}}); err != nil {
		t.Fatalf("save event failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveOutbox(OutboxRecord{ID: "o1", EventID: "e1", EventType: "document.submitted", Status: "pending", CreatedAt: now}); err != nil {
		t.Fatalf("save outbox failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_deliveries (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveDelivery(OutboxDeliveryRecord{ID: "d1", OutboxID: "o1", EventID: "e1", EventType: "document.submitted", SinkName: "local", Status: "pending", CreatedAt: now}); err != nil {
		t.Fatalf("save delivery failed: %v", err)
	}
	getRows := sqlmock.NewRows([]string{"event_id", "event_type", "event_version", "schema_version", "aggregate_type", "aggregate_id", "actor_id", "correlation_id", "organization_id", "location_id", "module_key", "occurred_at", "payload_json"}).AddRow("e1", "document.submitted", 1, "", "document", "d1", "u1", "", "", "", "", now, []byte(`{"x":1}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id, event_type, event_version, COALESCE(schema_version,''), aggregate_type, aggregate_id, COALESCE(actor_id,''), COALESCE(correlation_id,''), COALESCE(organization_id,''), COALESCE(location_id,''), COALESCE(module_key,''), occurred_at, COALESCE(payload_json,'{}'::jsonb) FROM domain_events WHERE event_id = $1")).WithArgs("e1").WillReturnRows(getRows)
	if _, ok := repo.GetEvent("e1"); !ok {
		t.Fatal("expected to load event")
	}
	eventRows := sqlmock.NewRows([]string{"event_id", "event_type", "event_version", "schema_version", "aggregate_type", "aggregate_id", "actor_id", "correlation_id", "organization_id", "location_id", "module_key", "occurred_at", "payload_json"}).AddRow("e1", "document.submitted", 1, "", "document", "d1", "u1", "", "", "", "", now, []byte(`{"x":1}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT event_id, event_type, event_version, COALESCE(schema_version,''), aggregate_type, aggregate_id, COALESCE(actor_id,''), COALESCE(correlation_id,''), COALESCE(organization_id,''), COALESCE(location_id,''), COALESCE(module_key,''), occurred_at, COALESCE(payload_json,'{}'::jsonb) FROM domain_events")).WillReturnRows(eventRows)
	if len(repo.ListEvents()) != 1 {
		t.Fatal("expected one event")
	}
	outboxRows := sqlmock.NewRows([]string{"outbox_id", "event_id", "event_type", "status", "attempt_count", "last_error", "created_at", "dispatched_at"}).AddRow("o1", "e1", "document.submitted", "pending", 0, "", now, nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT outbox_id, event_id, event_type, status, attempt_count, COALESCE(last_error,''), created_at, dispatched_at FROM outbox_records")).WillReturnRows(outboxRows)
	if len(repo.ListOutbox()) != 1 {
		t.Fatal("expected one outbox")
	}
	deliveryRows := sqlmock.NewRows([]string{"delivery_id", "outbox_id", "event_id", "event_type", "sink_name", "status", "attempt_count", "last_error", "created_at", "dispatched_at"}).AddRow("d1", "o1", "e1", "document.submitted", "local", "pending", 0, "", now, nil)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT delivery_id, outbox_id, event_id, event_type, sink_name, status, attempt_count, COALESCE(last_error,''), created_at, dispatched_at FROM outbox_deliveries")).WillReturnRows(deliveryRows)
	if len(repo.ListDeliveries()) != 1 {
		t.Fatal("expected one delivery")
	}
	claimRows := sqlmock.NewRows([]string{"delivery_id", "outbox_id", "event_id", "event_type", "sink_name", "status", "attempt_count", "last_error", "created_at", "dispatched_at"}).AddRow("d1", "o1", "e1", "document.submitted", "local", "processing", 1, "", now, nil)
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE outbox_deliveries")).WithArgs(5).WillReturnRows(claimRows)
	if len(repo.ClaimPendingDeliveries(5)) != 1 {
		t.Fatal("expected one claimed delivery")
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE outbox_records SET status = $1, dispatched_at = $2, last_error = NULL WHERE outbox_id = $3")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.MarkDispatched("o1", OutboxRecord{Status: "dispatched", DispatchedAt: now}); err != nil {
		t.Fatalf("mark dispatched failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE outbox_deliveries SET status = $1, dispatched_at = $2, last_error = NULL WHERE delivery_id = $3")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.MarkDeliveryDispatched("d1", OutboxDeliveryRecord{Status: "dispatched", DispatchedAt: now}); err != nil {
		t.Fatalf("mark delivery dispatched failed: %v", err)
	}
	deadRows := sqlmock.NewRows([]string{"dead_letter_id", "outbox_id", "event_id", "event_type", "sink_name", "reason", "attempt_count", "created_at"}).AddRow("dl1", "o1", "e1", "document.submitted", "local", "boom", 3, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT dead_letter_id, outbox_id, event_id, event_type, COALESCE(sink_name,''), reason, attempt_count, created_at FROM dead_letter_records")).WillReturnRows(deadRows)
	if len(repo.ListDeadLetters()) != 1 {
		t.Fatal("expected one dead letter")
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE outbox_records SET status = $1, last_error = NULLIF($2,''), attempt_count = $3 WHERE outbox_id = $4")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.MarkFailed("o1", OutboxRecord{Status: "pending", LastError: "boom", AttemptCount: 1}); err != nil {
		t.Fatalf("mark failed failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE outbox_deliveries SET status = $1, last_error = NULLIF($2,''), attempt_count = $3 WHERE delivery_id = $4")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.MarkDeliveryFailed("d1", OutboxDeliveryRecord{Status: "pending", LastError: "boom", AttemptCount: 1}); err != nil {
		t.Fatalf("mark delivery failed failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO dead_letter_records (dead_letter_id, outbox_id, event_id, event_type, sink_name, reason, attempt_count, created_at) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveDeadLetter(DeadLetterRecord{ID: "dl1", OutboxID: "o1", EventID: "e1", EventType: "document.submitted", SinkName: "local", Reason: "boom", AttemptCount: 3, CreatedAt: now}); err != nil {
		t.Fatalf("save dead letter failed: %v", err)
	}
}
