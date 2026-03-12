package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/model"
	"clinic/internal/platform/store"
)

type KernelActions struct {
	documents *document.Service
	models    *model.Service
	audit     *audit.Service
	eventing  *eventing.Service
	txm       store.TransactionManager
}

func NewMemoryKernelActions(documents *document.Service, models *model.Service, auditSvc *audit.Service, eventingSvc *eventing.Service) *KernelActions {
	return &KernelActions{documents: documents, models: models, audit: auditSvc, eventing: eventingSvc}
}

func NewPostgresKernelActions(db *sql.DB, documents *document.Service, models *model.Service, auditSvc *audit.Service, eventingSvc *eventing.Service) *KernelActions {
	return &KernelActions{documents: documents, models: models, audit: auditSvc, eventing: eventingSvc, txm: store.NewPostgresTransactionManager(db)}
}

func (a *KernelActions) CreateDocumentAndModelBundle(record document.Record, modelKey, actorID string, mutation model.CompositeMutation) (document.Record, model.Record, map[string][]model.Record, error) {
	if a.txm == nil {
		if err := a.documents.Save(record); err != nil {
			return document.Record{}, model.Record{}, nil, err
		}
		modelRecord, related, err := a.models.CreateComposite(modelKey, actorID, mutation)
		if err != nil {
			return document.Record{}, model.Record{}, nil, err
		}
		event := buildKernelBundleEvent(record, modelRecord, actorID)
		if err := a.audit.Record(buildKernelBundleAudit(record, modelRecord, actorID)); err != nil {
			return document.Record{}, model.Record{}, nil, err
		}
		if err := a.eventing.Record(event); err != nil {
			return document.Record{}, model.Record{}, nil, err
		}
		return record, modelRecord, related, nil
	}

	var (
		modelRecord model.Record
		related     map[string][]model.Record
	)
	err := a.txm.WithinTx(context.Background(), func(tx *sql.Tx) error {
		if err := saveDocumentRecordUpsertTx(tx, record); err != nil {
			return err
		}
		svc := a.models.WithRepository(model.NewTxRepository(tx))
		var err error
		modelRecord, related, err = svc.CreateComposite(modelKey, actorID, mutation)
		if err != nil {
			return err
		}
		auditEvent := buildKernelBundleAudit(record, modelRecord, actorID)
		if err := saveAuditEventTx(tx, auditEvent); err != nil {
			return err
		}
		event := buildKernelBundleEvent(record, modelRecord, actorID)
		if err := saveDomainEventTx(tx, event); err != nil {
			return err
		}
		return saveOutboxRecordTx(tx, eventing.OutboxRecord{
			ID:        event.ID + ":outbox",
			EventID:   event.ID,
			EventType: event.Type,
			Status:    "pending",
			CreatedAt: event.OccurredAt,
		})
	})
	if err != nil {
		return document.Record{}, model.Record{}, nil, err
	}
	return record, modelRecord, related, nil
}

func buildKernelBundleAudit(record document.Record, modelRecord model.Record, actorID string) audit.Event {
	now := time.Now().UTC()
	return audit.Event{
		ID:            fmt.Sprintf("audit:kernel-bundle:%s:%s", record.Header.ID, modelRecord.ID),
		Action:        "kernel.bundle.create",
		TargetType:    "document",
		TargetID:      record.Header.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		CorrelationID: fmt.Sprintf("kernel-bundle:%s:%s", record.Header.ID, modelRecord.ID),
		Metadata: map[string]any{
			"document_type": record.Header.Type,
			"model_key":     modelRecord.ModelKey,
			"model_id":      modelRecord.ID,
		},
	}
}

func buildKernelBundleEvent(record document.Record, modelRecord model.Record, actorID string) eventing.Event {
	now := time.Now().UTC()
	return eventing.Event{
		ID:            fmt.Sprintf("event:kernel-bundle:%s:%s", record.Header.ID, modelRecord.ID),
		Type:          "kernel.bundle.created",
		Version:       1,
		AggregateType: "kernel_bundle",
		AggregateID:   record.Header.ID + ":" + modelRecord.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		Payload: map[string]any{
			"document_id":   record.Header.ID,
			"document_type": record.Header.Type,
			"model_key":     modelRecord.ModelKey,
			"model_id":      modelRecord.ID,
		},
	}
}
