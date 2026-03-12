package application

import (
	"context"
	"regexp"
	"testing"
	"time"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/shared"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresTransactionManagerWithinTxCoordinatesMultipleDocumentWrites(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	txm := NewPostgresTransactionManager(db)
	now := time.Now().UTC()
	order := document.Record{
		Header: document.Header{ID: "order-1", Type: "generic_request", Status: "submitted", Version: 1, ETag: "order-1:1", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}},
		Body:   document.Body{SchemaVersion: "v1", Payload: map[string]any{"kind": "order"}},
	}
	inventory := document.Record{
		Header: document.Header{ID: "inventory-1", Type: "generic_request", Status: "reserved", Version: 1, ETag: "inventory-1:1", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}},
		Body:   document.Body{SchemaVersion: "v1", Payload: map[string]any{"kind": "inventory"}},
	}
	auditEvent := audit.Event{ID: "a1", Action: "order.place", TargetType: "document", TargetID: order.Header.ID, ActorID: "u1", OccurredAt: now}
	domainEvent := eventing.Event{ID: "e1", Type: "order.placed", Version: 1, AggregateType: "document", AggregateID: order.Header.ID, ActorID: "u1", OccurredAt: now, Payload: map[string]any{"order_id": order.Header.ID}}
	outbox := eventing.OutboxRecord{ID: "o1", EventID: domainEvent.ID, EventType: domainEvent.Type, Status: "pending", CreatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		if err := uow.CreateDocument(order); err != nil {
			return err
		}
		if err := uow.CreateDocument(inventory); err != nil {
			return err
		}
		if err := uow.SaveAudit(auditEvent); err != nil {
			return err
		}
		if err := uow.SaveDomainEvent(domainEvent); err != nil {
			return err
		}
		return uow.SaveOutbox(outbox)
	})
	if err != nil {
		t.Fatalf("within tx failed: %v", err)
	}
}

func TestPostgresTransactionManagerWithinTxRollsBackAllWritesOnSecondDocumentFailure(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	txm := NewPostgresTransactionManager(db)
	now := time.Now().UTC()
	order := document.Record{
		Header: document.Header{ID: "order-1", Type: "generic_request", Status: "submitted", Version: 1, ETag: "order-1:1", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}},
		Body:   document.Body{SchemaVersion: "v1", Payload: map[string]any{"kind": "order"}},
	}
	inventory := document.Record{
		Header: document.Header{ID: "inventory-1", Type: "generic_request", Status: "reserved", Version: 1, ETag: "inventory-1:1", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}},
		Body:   document.Body{SchemaVersion: "v1", Payload: map[string]any{"kind": "inventory"}},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_records (")).WillReturnError(assertAnError{})
	mock.ExpectRollback()

	err := txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		if err := uow.CreateDocument(order); err != nil {
			return err
		}
		return uow.CreateDocument(inventory)
	})
	if err == nil {
		t.Fatal("expected transaction failure")
	}
}
