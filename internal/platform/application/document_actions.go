package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

type SubmitStore interface {
	Submit(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error
	UpdateDraft(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error
}

type DocumentActions struct {
	documents *document.Service
	workflows *workflow.Service
	policy    *policy.Service
	store     SubmitStore
	runner    *KernelCommandRunner
}

func NewDocumentActions(documents *document.Service, workflows *workflow.Service, policySvc *policy.Service, store SubmitStore) *DocumentActions {
	actions := &DocumentActions{documents: documents, workflows: workflows, policy: policySvc, store: store}
	if provider, ok := store.(interface{ TransactionManager() TransactionManager }); ok {
		actions.runner = NewKernelCommandRunner(provider.TransactionManager())
	}
	return actions
}

func (a *DocumentActions) Submit(documentID, actorID string, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := a.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	def, err := a.documents.Definition(record.Header.Type)
	if err != nil {
		return document.Record{}, err
	}
	if err := a.ensureTransitionAllowed(record, actorID, "submit"); err != nil {
		return document.Record{}, err
	}
	if def.WorkflowKey == "" {
		return document.Record{}, shared.Conflict("document type has no workflow binding")
	}
	previousVersion := record.Header.Version

	transition, err := a.workflows.Execute(def.WorkflowKey, record.Header.Status, "submit")
	if err != nil {
		return document.Record{}, err
	}

	now := time.Now().UTC()
	record.Header.Status = transition.ToState
	a.assignNumberIfNeeded(&record, def, actorID, "submit", now)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = now
	record.Header.SubmittedBy = actorID
	record.Header.SubmittedAt = now

	correlationID := fmt.Sprintf("doc-submit:%s:%d", record.Header.ID, record.Header.Version)
	auditEvent := audit.Event{
		ID:            fmt.Sprintf("audit:%s", correlationID),
		Action:        "document.submit",
		TargetType:    "document",
		TargetID:      record.Header.ID,
		ActorID:       actorID,
		FromState:     transition.FromState,
		ToState:       transition.ToState,
		OccurredAt:    now,
		CorrelationID: correlationID,
		Metadata: map[string]any{
			"document_type": record.Header.Type,
			"version":       record.Header.Version,
		},
	}
	domainEvent := eventing.Event{
		ID:            fmt.Sprintf("event:%s", correlationID),
		Type:          "document.submitted",
		Version:       1,
		AggregateType: "document",
		AggregateID:   record.Header.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		Payload: map[string]any{
			"document_type": record.Header.Type,
			"from_state":    transition.FromState,
			"to_state":      transition.ToState,
			"version":       record.Header.Version,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	workflowMutation := a.workflows.PlanCreateSideEffects(transition, "document", record.Header.ID, actorID, now)
	if err := a.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation, false); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (a *DocumentActions) UpdateDraft(documentID, actorID string, payload map[string]any, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := a.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	if record.Header.Status != "draft" {
		return document.Record{}, shared.Conflict("only draft documents may be updated")
	}
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}

	previousVersion := record.Header.Version
	now := time.Now().UTC()
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = now
	record.Body.Payload = mergeBasePayload(record.Body.Payload, payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)

	correlationID := fmt.Sprintf("doc-update:%s:%d", record.Header.ID, record.Header.Version)
	auditEvent := audit.Event{
		ID:            fmt.Sprintf("audit:%s", correlationID),
		Action:        "document.update",
		TargetType:    "document",
		TargetID:      record.Header.ID,
		ActorID:       actorID,
		FromState:     "draft",
		ToState:       "draft",
		OccurredAt:    now,
		CorrelationID: correlationID,
		Metadata: map[string]any{
			"document_type": record.Header.Type,
			"version":       record.Header.Version,
		},
	}
	domainEvent := eventing.Event{
		ID:            fmt.Sprintf("event:%s", correlationID),
		Type:          "document.updated",
		Version:       1,
		AggregateType: "document",
		AggregateID:   record.Header.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		Payload: map[string]any{
			"document_type": record.Header.Type,
			"status":        record.Header.Status,
			"version":       record.Header.Version,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	if err := a.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflow.Mutation{}, true); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (a *DocumentActions) UpdateExtension(documentID, moduleKey, actorID string, extensionPayload map[string]any, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := a.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	if record.Header.Status != "draft" {
		return document.Record{}, shared.Conflict("only draft documents may be extended")
	}
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	if !hasDocumentExtension(a.documents.ExtensionDefinitions(record.Header.Type), moduleKey) {
		return document.Record{}, shared.Validation("document extension is not registered")
	}

	previousVersion := record.Header.Version
	now := time.Now().UTC()
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = now
	record.Body.Payload = document.SetExtensionPayload(record.Body.Payload, moduleKey, extensionPayload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)

	correlationID := fmt.Sprintf("doc-extension-update:%s:%s:%d", moduleKey, record.Header.ID, record.Header.Version)
	auditEvent := audit.Event{
		ID:            fmt.Sprintf("audit:%s", correlationID),
		Action:        "document.extension.update",
		TargetType:    "document",
		TargetID:      record.Header.ID,
		ActorID:       actorID,
		FromState:     record.Header.Status,
		ToState:       record.Header.Status,
		OccurredAt:    now,
		CorrelationID: correlationID,
		Metadata: map[string]any{
			"document_type": record.Header.Type,
			"module_key":    moduleKey,
			"version":       record.Header.Version,
		},
	}
	domainEvent := eventing.Event{
		ID:            fmt.Sprintf("event:%s", correlationID),
		Type:          "document.extension.updated",
		Version:       1,
		AggregateType: "document",
		AggregateID:   record.Header.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		Payload: map[string]any{
			"document_type": record.Header.Type,
			"module_key":    moduleKey,
			"status":        record.Header.Status,
			"version":       record.Header.Version,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	if err := a.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflow.Mutation{}, true); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (a *DocumentActions) Approve(documentID, actorID string, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := a.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	def, err := a.documents.Definition(record.Header.Type)
	if err != nil {
		return document.Record{}, err
	}
	transition, err := a.workflows.Execute(def.WorkflowKey, record.Header.Status, "approve")
	if err != nil {
		return document.Record{}, err
	}
	if err := a.ensureTransitionAllowed(record, actorID, "approve"); err != nil {
		return document.Record{}, err
	}
	previousVersion := record.Header.Version
	now := time.Now().UTC()
	record.Header.Status = transition.ToState
	a.assignNumberIfNeeded(&record, def, actorID, "approve", now)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = now

	correlationID := fmt.Sprintf("doc-approve:%s:%d", record.Header.ID, record.Header.Version)
	auditEvent := audit.Event{
		ID:            fmt.Sprintf("audit:%s", correlationID),
		Action:        "document.approve",
		TargetType:    "document",
		TargetID:      record.Header.ID,
		ActorID:       actorID,
		FromState:     transition.FromState,
		ToState:       transition.ToState,
		OccurredAt:    now,
		CorrelationID: correlationID,
		Metadata: map[string]any{
			"document_type": record.Header.Type,
			"version":       record.Header.Version,
		},
	}
	domainEvent := eventing.Event{
		ID:            fmt.Sprintf("event:%s", correlationID),
		Type:          "document.approved",
		Version:       1,
		AggregateType: "document",
		AggregateID:   record.Header.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		Payload: map[string]any{
			"document_type": record.Header.Type,
			"from_state":    transition.FromState,
			"to_state":      transition.ToState,
			"version":       record.Header.Version,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	workflowMutation := a.workflows.PlanResolveArtifacts(record.Header.ID, "approved", "completed", actorID, now)
	if err := a.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation, false); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (a *DocumentActions) persistDocument(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation, draft bool) error {
	if a.runner != nil {
		_, err := RunKernelCommand(context.Background(), a.runner, documentPersistCommand{
			previousVersion:  previousVersion,
			record:           record,
			auditEvent:       auditEvent,
			domainEvent:      domainEvent,
			outboxRecord:     outboxRecord,
			workflowMutation: workflowMutation,
		})
		return err
	}
	if draft {
		return a.store.UpdateDraft(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation)
	}
	return a.store.Submit(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation)
}

func (a *DocumentActions) Reject(documentID, actorID string, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitionDocument(documentID, actorID, expectedVersion, expectedETag, "reject", "document.reject")
}

func (a *DocumentActions) Reopen(documentID, actorID string, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitionDocument(documentID, actorID, expectedVersion, expectedETag, "reopen", "document.reopened")
}

func (a *DocumentActions) Cancel(documentID, actorID string, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitionDocument(documentID, actorID, expectedVersion, expectedETag, "cancel", "document.cancelled")
}

func (a *DocumentActions) transitionDocument(documentID, actorID string, expectedVersion int, expectedETag, action, eventType string) (document.Record, error) {
	record, err := a.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	def, err := a.documents.Definition(record.Header.Type)
	if err != nil {
		return document.Record{}, err
	}
	transition, err := a.workflows.Execute(def.WorkflowKey, record.Header.Status, action)
	if err != nil {
		return document.Record{}, err
	}
	if err := a.ensureTransitionAllowed(record, actorID, action); err != nil {
		return document.Record{}, err
	}
	previousVersion := record.Header.Version
	now := time.Now().UTC()
	workflowMutation := workflow.Mutation{}
	if action == "reject" || action == "cancel" {
		workflowMutation = a.workflows.PlanResolveArtifacts(documentID, transition.ToState, "cancelled", actorID, now)
	}
	record.Header.Status = transition.ToState
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = actorID
	record.Header.UpdatedAt = now

	correlationID := fmt.Sprintf("doc-%s:%s:%d", action, record.Header.ID, record.Header.Version)
	auditEvent := audit.Event{
		ID:            fmt.Sprintf("audit:%s", correlationID),
		Action:        "document." + action,
		TargetType:    "document",
		TargetID:      record.Header.ID,
		ActorID:       actorID,
		FromState:     transition.FromState,
		ToState:       transition.ToState,
		OccurredAt:    now,
		CorrelationID: correlationID,
		Metadata:      map[string]any{"document_type": record.Header.Type, "version": record.Header.Version},
	}
	domainEvent := eventing.Event{
		ID:            fmt.Sprintf("event:%s", correlationID),
		Type:          eventType,
		Version:       1,
		AggregateType: "document",
		AggregateID:   record.Header.ID,
		ActorID:       actorID,
		OccurredAt:    now,
		Payload:       map[string]any{"document_type": record.Header.Type, "from_state": transition.FromState, "to_state": transition.ToState, "version": record.Header.Version},
	}
	outboxRecord := eventing.OutboxRecord{ID: domainEvent.ID + ":outbox", EventID: domainEvent.ID, EventType: domainEvent.Type, Status: "pending", CreatedAt: now}
	if err := a.store.Submit(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (a *DocumentActions) ensureTransitionAllowed(record document.Record, actorID, action string) error {
	if a.workflows != nil {
		def, err := a.documents.Definition(record.Header.Type)
		if err == nil && def.WorkflowKey != "" {
			if transition, err := a.workflows.Execute(def.WorkflowKey, record.Header.Status, action); err == nil && transition.RequiresDifferentActor {
				for _, approval := range a.workflows.ListApprovals() {
					if approval.TargetID == record.Header.ID && approval.Status == "pending" && approval.RequestedBy != "" && approval.RequestedBy == actorID {
						return shared.Forbidden("workflow transition requires a different actor than the requester")
					}
				}
			}
		}
	}
	if a.policy == nil {
		return nil
	}
	decision := a.policy.Evaluate(policy.Request{
		HookKey:        "documents.workflow.transition",
		ActorID:        actorID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		ScopeID:        record.Header.LocationID,
		Inputs: map[string]any{
			"document_id":   record.Header.ID,
			"document_type": record.Header.Type,
			"current_state": record.Header.Status,
			"status":        record.Header.Status,
			"action":        action,
		},
	})
	if !decision.Allowed {
		return shared.Forbidden(firstNonEmpty(decision.Reason, "workflow transition denied by policy"))
	}
	return nil
}

func (a *DocumentActions) assignNumberIfNeeded(record *document.Record, def document.Definition, actorID, action string, now time.Time) {
	if record == nil || def.NumberingKey == "" || record.Header.Number != "" {
		return
	}
	if a.policy != nil {
		decision := a.policy.Evaluate(policy.Request{
			HookKey:        "documents.numbering.assign",
			ActorID:        actorID,
			OrganizationID: record.Header.OrganizationID,
			LocationID:     record.Header.LocationID,
			ScopeID:        record.Header.LocationID,
			Inputs: map[string]any{
				"document_id":     record.Header.ID,
				"document_type":   record.Header.Type,
				"numbering_key":   def.NumberingKey,
				"action":          action,
				"organization_id": record.Header.OrganizationID,
				"location_id":     record.Header.LocationID,
			},
		})
		if value, ok := decision.Output["number"].(string); ok && value != "" {
			record.Header.Number = value
			return
		}
	}
	record.Header.Number = defaultDocumentNumber(def.NumberingKey, now)
}

func defaultDocumentNumber(numberingKey string, now time.Time) string {
	return fmt.Sprintf("%s-%s", numberingKey, now.UTC().Format("20060102150405"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type MemorySubmitStore struct {
	txm TransactionManager
}

func NewMemorySubmitStore(documents *document.Service, workflows *workflow.Service, auditSvc *audit.Service, eventingSvc *eventing.Service) *MemorySubmitStore {
	return &MemorySubmitStore{txm: NewMemoryTransactionManager(documents, model.NewService(), workflows, auditSvc, eventingSvc)}
}

func (s *MemorySubmitStore) TransactionManager() TransactionManager {
	return s.txm
}

func (s *MemorySubmitStore) Submit(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, _ eventing.OutboxRecord, workflowMutation workflow.Mutation) error {
	return s.persist(previousVersion, record, auditEvent, domainEvent, workflowMutation)
}

func (s *MemorySubmitStore) UpdateDraft(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, _ eventing.OutboxRecord, workflowMutation workflow.Mutation) error {
	return s.persist(previousVersion, record, auditEvent, domainEvent, workflowMutation)
}

func (s *MemorySubmitStore) persist(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, workflowMutation workflow.Mutation) error {
	return s.txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		if err := uow.UpdateDocument(previousVersion, record); err != nil {
			return err
		}
		if err := uow.SaveAudit(auditEvent); err != nil {
			return err
		}
		if err := uow.SaveDomainEvent(domainEvent); err != nil {
			return err
		}
		return uow.ApplyWorkflowMutation(workflowMutation)
	})
}

type PostgresSubmitStore struct {
	txm TransactionManager
}

func NewPostgresSubmitStore(db *sql.DB) *PostgresSubmitStore {
	return &PostgresSubmitStore{txm: NewPostgresTransactionManager(db)}
}

func (s *PostgresSubmitStore) TransactionManager() TransactionManager {
	return s.txm
}

func (s *PostgresSubmitStore) Submit(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error {
	return s.persist(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation)
}

func (s *PostgresSubmitStore) UpdateDraft(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error {
	return s.persist(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation)
}

func (s *PostgresSubmitStore) persist(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error {
	return s.txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		if err := uow.UpdateDocument(previousVersion, record); err != nil {
			return err
		}
		if err := uow.SaveAudit(auditEvent); err != nil {
			return err
		}
		if err := uow.SaveDomainEvent(domainEvent); err != nil {
			return err
		}
		if err := uow.SaveOutbox(outboxRecord); err != nil {
			return err
		}
		return uow.ApplyWorkflowMutation(workflowMutation)
	})
}

func saveAuditEventTx(tx *sql.Tx, event audit.Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO audit_events (
			audit_event_id, action, target_type, target_id, actor_id,
			from_state, to_state, occurred_at, metadata_json, correlation_id
		) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,NULLIF($10,''))`
	_, err = tx.Exec(query,
		event.ID, event.Action, event.TargetType, event.TargetID, event.ActorID,
		event.FromState, event.ToState, event.OccurredAt, metadata, event.CorrelationID,
	)
	return err
}

func saveDomainEventTx(tx *sql.Tx, event eventing.Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO domain_events (
			event_id, event_type, event_version, schema_version, aggregate_type, aggregate_id, actor_id, correlation_id, organization_id, location_id, module_key, occurred_at, payload_json
		) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),$12,$13)`
	_, err = tx.Exec(query,
		event.ID, event.Type, event.Version, event.SchemaVersion, event.AggregateType, event.AggregateID, event.ActorID, event.CorrelationID, event.OrganizationID, event.LocationID, event.ModuleKey, event.OccurredAt, payload,
	)
	return err
}

func saveOutboxRecordTx(tx *sql.Tx, record eventing.OutboxRecord) error {
	const query = `
		INSERT INTO outbox_records (
			outbox_id, event_id, event_type, status, created_at, dispatched_at
		) VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := tx.Exec(query,
		record.ID, record.EventID, record.EventType, record.Status, record.CreatedAt, nullableTime(record.DispatchedAt),
	)
	return err
}

func applyWorkflowMutationTx(tx *sql.Tx, mutation workflow.Mutation) error {
	for _, task := range mutation.Tasks {
		const query = `
			INSERT INTO workflow_tasks (
				task_id, workflow_key, target_type, target_id, task_type, status, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if _, err := tx.ExecContext(context.Background(), query, task.ID, task.WorkflowKey, task.TargetType, task.TargetID, task.TaskType, task.Status, task.CreatedAt); err != nil {
			return err
		}
	}
	for _, approval := range mutation.Approvals {
		const query = `
			INSERT INTO workflow_approvals (
				approval_id, workflow_key, target_type, target_id, status, requested_at
			) VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := tx.ExecContext(context.Background(), query, approval.ID, approval.WorkflowKey, approval.TargetType, approval.TargetID, approval.Status, approval.RequestedAt); err != nil {
			return err
		}
	}
	for _, update := range mutation.ApprovalUpdates {
		if _, err := tx.ExecContext(context.Background(), `UPDATE workflow_approvals SET status = $1 WHERE approval_id = $2`, update.Status, update.ID); err != nil {
			return err
		}
	}
	for _, update := range mutation.TaskUpdates {
		if _, err := tx.ExecContext(context.Background(), `UPDATE workflow_tasks SET status = $1 WHERE task_id = $2`, update.Status, update.ID); err != nil {
			return err
		}
	}
	return nil
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func mergeBasePayload(existing, updated map[string]any) map[string]any {
	merged := document.NormalizePayload(updated)
	current := document.NormalizePayload(existing)
	if currentExtensions, ok := current["extensions"].(map[string]any); ok {
		merged["extensions"] = currentExtensions
	}
	return merged
}

type documentPersistCommand struct {
	previousVersion  int
	record           document.Record
	auditEvent       audit.Event
	domainEvent      eventing.Event
	outboxRecord     eventing.OutboxRecord
	workflowMutation workflow.Mutation
}

func (c documentPersistCommand) Run(_ context.Context, uow UnitOfWork) (document.Record, error) {
	if err := uow.UpdateDocument(c.previousVersion, c.record); err != nil {
		return document.Record{}, err
	}
	if err := uow.SaveAudit(c.auditEvent); err != nil {
		return document.Record{}, err
	}
	if err := uow.SaveDomainEvent(c.domainEvent); err != nil {
		return document.Record{}, err
	}
	if c.outboxRecord.ID != "" {
		if err := uow.SaveOutbox(c.outboxRecord); err != nil {
			return document.Record{}, err
		}
	}
	if err := uow.ApplyWorkflowMutation(c.workflowMutation); err != nil {
		return document.Record{}, err
	}
	return c.record, nil
}

func hasDocumentExtension(defs []document.ExtensionDefinition, moduleKey string) bool {
	for _, def := range defs {
		if def.ModuleKey == moduleKey {
			return true
		}
	}
	return false
}
