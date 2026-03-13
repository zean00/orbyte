package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"clinic/internal/platform/activity"
	"clinic/internal/platform/audit"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/model"
	"clinic/internal/platform/store"
)

type ModelActions struct {
	models     *model.Service
	activities *activity.Service
	audit      *audit.Service
	eventing   *eventing.Service
	txm        store.TransactionManager
	runner     *KernelCommandRunner
}

func NewMemoryModelActions(models *model.Service, activities *activity.Service, auditSvc *audit.Service, eventingSvc *eventing.Service) *ModelActions {
	return &ModelActions{models: models, activities: activities, audit: auditSvc, eventing: eventingSvc}
}

func NewPostgresModelActions(db *sql.DB, models *model.Service, activities *activity.Service, auditSvc *audit.Service, eventingSvc *eventing.Service) *ModelActions {
	txm := store.NewPostgresTransactionManager(db)
	return &ModelActions{models: models, activities: activities, audit: auditSvc, eventing: eventingSvc, txm: txm, runner: NewKernelCommandRunner(NewPostgresTransactionManager(db))}
}

func (a *ModelActions) CreateComposite(modelKey, actorID string, mutation model.CompositeMutation) (model.Record, map[string][]model.Record, error) {
	if a.txm == nil {
		record, related, err := a.models.CreateComposite(modelKey, actorID, mutation)
		if err != nil {
			return model.Record{}, nil, err
		}
		return a.afterWrite("create", record, related, actorID)
	}
	result, err := RunKernelCommand(context.Background(), a.runner, createModelCommand{models: a.models, modelKey: modelKey, actorID: actorID, mutation: mutation})
	if err != nil {
		return model.Record{}, nil, err
	}
	return a.afterActivities("create", result.Record, result.Related, actorID), result.Related, nil
}

func (a *ModelActions) UpdateComposite(modelKey, id, actorID string, mutation model.CompositeMutation) (model.Record, map[string][]model.Record, error) {
	if a.txm == nil {
		record, related, err := a.models.UpdateComposite(modelKey, id, actorID, mutation)
		if err != nil {
			return model.Record{}, nil, err
		}
		return a.afterWrite("update", record, related, actorID)
	}
	result, err := RunKernelCommand(context.Background(), a.runner, updateModelCommand{models: a.models, modelKey: modelKey, recordID: id, actorID: actorID, mutation: mutation})
	if err != nil {
		return model.Record{}, nil, err
	}
	return a.afterActivities("update", result.Record, result.Related, actorID), result.Related, nil
}

func (a *ModelActions) PatchRelation(modelKey, id, relationKey, actorID string, mutations []model.ChildMutation) (model.Record, map[string][]model.Record, error) {
	return a.UpdateComposite(modelKey, id, actorID, model.CompositeMutation{
		Relations: map[string][]model.ChildMutation{relationKey: mutations},
	})
}

func (a *ModelActions) afterWrite(action string, record model.Record, related map[string][]model.Record, actorID string) (model.Record, map[string][]model.Record, error) {
	if err := a.audit.Record(buildModelAuditEvent(action, record, related, actorID)); err != nil {
		return model.Record{}, nil, err
	}
	if err := a.eventing.Record(buildModelDomainEvent(action, record, related, actorID)); err != nil {
		return model.Record{}, nil, err
	}
	record = a.afterActivities(action, record, related, actorID)
	return record, related, nil
}

func (a *ModelActions) afterActivities(action string, record model.Record, related map[string][]model.Record, actorID string) model.Record {
	if a.activities != nil {
		_, _ = a.activities.AddMessage("model:"+record.ModelKey, record.ID, actorID, "Record "+action+"d", map[string]any{"model_key": record.ModelKey, "relation_keys": relationKeys(related)})
	}
	return record
}

func saveModelRuntimeArtifactsTx(tx *sql.Tx, action string, record model.Record, related map[string][]model.Record, actorID string) error {
	if err := saveAuditEventTx(tx, buildModelAuditEvent(action, record, related, actorID)); err != nil {
		return err
	}
	domainEvent := buildModelDomainEvent(action, record, related, actorID)
	if err := saveDomainEventTx(tx, domainEvent); err != nil {
		return err
	}
	return saveOutboxRecordTx(tx, eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: domainEvent.OccurredAt,
	})
}

func buildModelAuditEvent(action string, record model.Record, related map[string][]model.Record, actorID string) audit.Event {
	now := time.Now().UTC()
	return audit.Event{
		ID:            fmt.Sprintf("audit:model:%s:%s:%d", action, record.ID, record.Version),
		Action:        "model." + action,
		TargetType:    "model:" + record.ModelKey,
		TargetID:      record.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		CorrelationID: fmt.Sprintf("model-%s:%s:%d", action, record.ID, record.Version),
		Metadata: map[string]any{
			"model_key":     record.ModelKey,
			"version":       record.Version,
			"relation_keys": relationKeys(related),
		},
	}
}

func buildModelDomainEvent(action string, record model.Record, related map[string][]model.Record, actorID string) eventing.Event {
	now := time.Now().UTC()
	return eventing.Event{
		ID:            fmt.Sprintf("event:model:%s:%s:%d", action, record.ID, record.Version),
		Type:          "model.record." + action + "d",
		Version:       1,
		AggregateType: "model:" + record.ModelKey,
		AggregateID:   record.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		Payload: map[string]any{
			"model_key":     record.ModelKey,
			"version":       record.Version,
			"relation_keys": relationKeys(related),
		},
	}
}

func relationKeys(related map[string][]model.Record) []string {
	if len(related) == 0 {
		return nil
	}
	keys := make([]string, 0, len(related))
	for key := range related {
		keys = append(keys, key)
	}
	return keys
}

type modelCommandResult struct {
	Record  model.Record
	Related map[string][]model.Record
}

type createModelCommand struct {
	models   *model.Service
	modelKey string
	actorID  string
	mutation model.CompositeMutation
}

func (c createModelCommand) Run(_ context.Context, uow UnitOfWork) (modelCommandResult, error) {
	svc := c.models.WithRepository(newModelUnitOfWorkRepository(uow, c.models.Repository()))
	record, related, err := svc.CreateComposite(c.modelKey, c.actorID, c.mutation)
	if err != nil {
		return modelCommandResult{}, err
	}
	if err := uow.SaveAudit(buildModelAuditEvent("create", record, related, c.actorID)); err != nil {
		return modelCommandResult{}, err
	}
	event := buildModelDomainEvent("create", record, related, c.actorID)
	if err := uow.SaveDomainEvent(event); err != nil {
		return modelCommandResult{}, err
	}
	if err := uow.SaveOutbox(eventing.OutboxRecord{ID: event.ID + ":outbox", EventID: event.ID, EventType: event.Type, Status: "pending", CreatedAt: event.OccurredAt}); err != nil {
		return modelCommandResult{}, err
	}
	return modelCommandResult{Record: record, Related: related}, nil
}

type updateModelCommand struct {
	models   *model.Service
	modelKey string
	recordID string
	actorID  string
	mutation model.CompositeMutation
}

func (c updateModelCommand) Run(_ context.Context, uow UnitOfWork) (modelCommandResult, error) {
	svc := c.models.WithRepository(newModelUnitOfWorkRepository(uow, c.models.Repository()))
	record, related, err := svc.UpdateComposite(c.modelKey, c.recordID, c.actorID, c.mutation)
	if err != nil {
		return modelCommandResult{}, err
	}
	if err := uow.SaveAudit(buildModelAuditEvent("update", record, related, c.actorID)); err != nil {
		return modelCommandResult{}, err
	}
	event := buildModelDomainEvent("update", record, related, c.actorID)
	if err := uow.SaveDomainEvent(event); err != nil {
		return modelCommandResult{}, err
	}
	if err := uow.SaveOutbox(eventing.OutboxRecord{ID: event.ID + ":outbox", EventID: event.ID, EventType: event.Type, Status: "pending", CreatedAt: event.OccurredAt}); err != nil {
		return modelCommandResult{}, err
	}
	return modelCommandResult{Record: record, Related: related}, nil
}
