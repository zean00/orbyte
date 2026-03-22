package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
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

func (a *ModelActions) CreateComposite(modelKey string, acting ActingContext, mutation model.CompositeMutation) (model.Record, map[string][]model.Record, error) {
	if a.txm == nil {
		record, related, err := a.models.CreateComposite(modelKey, acting.EffectiveActorID(), mutation)
		if err != nil {
			return model.Record{}, nil, err
		}
		return a.afterWrite("create", record, related, acting)
	}
	result, err := RunKernelCommand(context.Background(), a.runner, createModelCommand{models: a.models, modelKey: modelKey, acting: acting, mutation: mutation})
	if err != nil {
		return model.Record{}, nil, err
	}
	return a.afterActivities("create", result.Record, result.Related, acting), result.Related, nil
}

func (a *ModelActions) UpdateComposite(modelKey, id string, acting ActingContext, mutation model.CompositeMutation) (model.Record, map[string][]model.Record, error) {
	if a.txm == nil {
		record, related, err := a.models.UpdateComposite(modelKey, id, acting.EffectiveActorID(), mutation)
		if err != nil {
			return model.Record{}, nil, err
		}
		return a.afterWrite("update", record, related, acting)
	}
	result, err := RunKernelCommand(context.Background(), a.runner, updateModelCommand{models: a.models, modelKey: modelKey, recordID: id, acting: acting, mutation: mutation})
	if err != nil {
		return model.Record{}, nil, err
	}
	return a.afterActivities("update", result.Record, result.Related, acting), result.Related, nil
}

func (a *ModelActions) PatchRelation(modelKey, id, relationKey string, acting ActingContext, mutations []model.ChildMutation) (model.Record, map[string][]model.Record, error) {
	return a.UpdateComposite(modelKey, id, acting, model.CompositeMutation{
		Relations: map[string][]model.ChildMutation{relationKey: mutations},
	})
}

func (a *ModelActions) afterWrite(action string, record model.Record, related map[string][]model.Record, acting ActingContext) (model.Record, map[string][]model.Record, error) {
	if err := a.audit.Record(buildModelAuditEvent(action, record, related, acting)); err != nil {
		return model.Record{}, nil, err
	}
	if err := a.eventing.Record(buildModelDomainEvent(action, record, related, acting)); err != nil {
		return model.Record{}, nil, err
	}
	record = a.afterActivities(action, record, related, acting)
	return record, related, nil
}

func (a *ModelActions) afterActivities(action string, record model.Record, related map[string][]model.Record, acting ActingContext) model.Record {
	if a.activities != nil {
		_, _ = a.activities.AddMessage("model:"+record.ModelKey, record.ID, acting.ActorID, "Record "+action+"d", map[string]any{"model_key": record.ModelKey, "relation_keys": relationKeys(related), "effective_user_id": acting.EffectiveActorID(), "on_behalf_of_user_id": acting.OnBehalfOfUserID})
	}
	return record
}

func saveModelRuntimeArtifactsTx(tx *sql.Tx, action string, record model.Record, related map[string][]model.Record, acting ActingContext) error {
	if err := saveAuditEventTx(tx, buildModelAuditEvent(action, record, related, acting)); err != nil {
		return err
	}
	domainEvent := buildModelDomainEvent(action, record, related, acting)
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

func buildModelAuditEvent(action string, record model.Record, related map[string][]model.Record, acting ActingContext) audit.Event {
	now := time.Now().UTC()
	return audit.Event{
		ID:                fmt.Sprintf("audit:model:%s:%s:%d", action, record.ID, record.Version),
		Action:            "model." + action,
		TargetType:        "model:" + record.ModelKey,
		TargetID:          record.ID,
		ActorID:           acting.ActorID,
		ActorKind:         "user",
		OnBehalfOfUserID:  acting.OnBehalfOfUserID,
		DelegationGrantID: acting.DelegationGrantID,
		OccurredAt:        now,
		CorrelationID:     fmt.Sprintf("model-%s:%s:%d", action, record.ID, record.Version),
		Metadata: map[string]any{
			"model_key":     record.ModelKey,
			"version":       record.Version,
			"relation_keys": relationKeys(related),
		},
	}
}

func buildModelDomainEvent(action string, record model.Record, related map[string][]model.Record, acting ActingContext) eventing.Event {
	now := time.Now().UTC()
	return eventing.Event{
		ID:            fmt.Sprintf("event:model:%s:%s:%d", action, record.ID, record.Version),
		Type:          "model.record." + action + "d",
		Version:       1,
		AggregateType: "model:" + record.ModelKey,
		AggregateID:   record.ID,
		ActorID:       acting.ActorID,
		CorrelationID: acting.CorrelationID,
		OccurredAt:    now,
		Payload: map[string]any{
			"model_key":            record.ModelKey,
			"version":              record.Version,
			"relation_keys":        relationKeys(related),
			"effective_user_id":    acting.EffectiveActorID(),
			"on_behalf_of_user_id": acting.OnBehalfOfUserID,
			"delegation_grant_id":  acting.DelegationGrantID,
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
	acting   ActingContext
	mutation model.CompositeMutation
}

func (c createModelCommand) Run(_ context.Context, uow UnitOfWork) (modelCommandResult, error) {
	svc := c.models.WithRepository(newModelUnitOfWorkRepository(uow, c.models.Repository()))
	record, related, err := svc.CreateComposite(c.modelKey, c.acting.EffectiveActorID(), c.mutation)
	if err != nil {
		return modelCommandResult{}, err
	}
	if err := uow.SaveAudit(buildModelAuditEvent("create", record, related, c.acting)); err != nil {
		return modelCommandResult{}, err
	}
	event := buildModelDomainEvent("create", record, related, c.acting)
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
	acting   ActingContext
	mutation model.CompositeMutation
}

func (c updateModelCommand) Run(_ context.Context, uow UnitOfWork) (modelCommandResult, error) {
	svc := c.models.WithRepository(newModelUnitOfWorkRepository(uow, c.models.Repository()))
	record, related, err := svc.UpdateComposite(c.modelKey, c.recordID, c.acting.EffectiveActorID(), c.mutation)
	if err != nil {
		return modelCommandResult{}, err
	}
	if err := uow.SaveAudit(buildModelAuditEvent("update", record, related, c.acting)); err != nil {
		return modelCommandResult{}, err
	}
	event := buildModelDomainEvent("update", record, related, c.acting)
	if err := uow.SaveDomainEvent(event); err != nil {
		return modelCommandResult{}, err
	}
	if err := uow.SaveOutbox(eventing.OutboxRecord{ID: event.ID + ":outbox", EventID: event.ID, EventType: event.Type, Status: "pending", CreatedAt: event.OccurredAt}); err != nil {
		return modelCommandResult{}, err
	}
	return modelCommandResult{Record: record, Related: related}, nil
}
