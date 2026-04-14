package application

import (
	"context"
	"fmt"
	"strings"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

type documentLifecycleArtifacts struct {
	record           document.Record
	workflowMutation workflow.Mutation
	draft            bool
}

type documentPersistenceService struct {
	store  SubmitStore
	runner *KernelCommandRunner
}

func newDocumentPersistenceService(store SubmitStore, runner *KernelCommandRunner) *documentPersistenceService {
	return &documentPersistenceService{store: store, runner: runner}
}

func (s *documentPersistenceService) Persist(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation, draft bool) (documentLifecycleArtifacts, error) {
	if s.runner != nil {
		_, err := RunKernelCommand(context.Background(), s.runner, documentPersistCommand{
			previousVersion:  previousVersion,
			record:           record,
			auditEvent:       auditEvent,
			domainEvent:      domainEvent,
			outboxRecord:     outboxRecord,
			workflowMutation: workflowMutation,
		})
		if err != nil {
			return documentLifecycleArtifacts{}, err
		}
		return documentLifecycleArtifacts{record: record, workflowMutation: workflowMutation, draft: draft}, nil
	}
	if draft {
		if err := s.store.UpdateDraft(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation); err != nil {
			return documentLifecycleArtifacts{}, err
		}
		return documentLifecycleArtifacts{record: record, workflowMutation: workflowMutation, draft: true}, nil
	}
	if err := s.store.Submit(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation); err != nil {
		return documentLifecycleArtifacts{}, err
	}
	return documentLifecycleArtifacts{record: record, workflowMutation: workflowMutation, draft: false}, nil
}

type workflowArtifactService struct {
	actions *DocumentActions
}

func newWorkflowArtifactService(actions *DocumentActions) *workflowArtifactService {
	return &workflowArtifactService{actions: actions}
}

func (s *workflowArtifactService) Publish(result documentLifecycleArtifacts) {
	if s == nil || s.actions == nil || result.draft {
		return
	}
	now := s.actions.currentTime()
	s.actions.issueWorkflowDeepLinks(result.record, result.workflowMutation.Approvals, now)
	s.actions.issueWorkflowNotifications(result.record, result.workflowMutation.Tasks, result.workflowMutation.Approvals, now)
}

type documentTransitionService struct {
	actions     *DocumentActions
	persistence *documentPersistenceService
	artifacts   *workflowArtifactService
}

func newDocumentTransitionService(actions *DocumentActions, persistence *documentPersistenceService, artifacts *workflowArtifactService) *documentTransitionService {
	return &documentTransitionService{actions: actions, persistence: persistence, artifacts: artifacts}
}

func (s *documentTransitionService) Submit(documentID string, acting ActingContext, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := s.actions.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	beforeRecord := cloneDocumentRecord(record)
	def, err := s.actions.documents.Definition(record.Header.Type)
	if err != nil {
		return document.Record{}, err
	}
	if def.WorkflowKey == "" {
		return document.Record{}, shared.Conflict("document type has no workflow binding")
	}
	previousVersion := record.Header.Version

	transition, err := s.actions.workflows.Execute(def.WorkflowKey, record.Header.Status, "submit")
	if err != nil {
		return document.Record{}, err
	}
	transitionDecision, assignmentDecision, slaDecision, err := s.actions.applyWorkflowRuntimeDecisions(record, acting.EffectiveActorID(), "submit", &transition)
	if err != nil {
		return document.Record{}, err
	}

	now := s.actions.currentTime()
	record.Header.Status = transition.ToState
	ensureWorkflowBinding(&record, transition.WorkflowKey, transition.WorkflowVersion)
	s.actions.assignNumberIfNeeded(&record, def, acting.EffectiveActorID(), "submit", now)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = acting.EffectiveActorID()
	record.Header.UpdatedAt = now
	record.Header.SubmittedBy = acting.EffectiveActorID()
	record.Header.SubmittedAt = now

	correlationID := firstNonEmpty(strings.TrimSpace(acting.CorrelationID), fmt.Sprintf("doc-submit:%s:%d", record.Header.ID, record.Header.Version))
	auditEvent := audit.Event{
		ID:                fmt.Sprintf("audit:%s", correlationID),
		Action:            "document.submit",
		TargetType:        "document",
		TargetID:          record.Header.ID,
		ActorID:           acting.ActorID,
		ActorKind:         "user",
		OnBehalfOfUserID:  acting.OnBehalfOfUserID,
		DelegationGrantID: acting.DelegationGrantID,
		FromState:         transition.FromState,
		ToState:           transition.ToState,
		OrganizationID:    record.Header.OrganizationID,
		LocationID:        record.Header.LocationID,
		OccurredAt:        now,
		CorrelationID:     correlationID,
		ChangeSummary:     documentChangeSummary(beforeRecord, record),
		Metadata: map[string]any{
			"document_type":     record.Header.Type,
			"version":           record.Header.Version,
			"workflow_key":      transition.WorkflowKey,
			"workflow_version":  transition.WorkflowVersion,
			"policy_code":       transitionDecision.Code,
			"policy_reason":     transitionDecision.Reason,
			"assignment_policy": decisionSummary(assignmentDecision),
			"sla_policy":        decisionSummary(slaDecision),
		},
	}
	domainEvent := eventing.Event{
		ID:             fmt.Sprintf("event:%s", correlationID),
		Type:           "document.submitted",
		Version:        1,
		AggregateType:  "document",
		AggregateID:    record.Header.ID,
		ActorID:        acting.ActorID,
		CorrelationID:  correlationID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		OccurredAt:     now,
		Payload: map[string]any{
			"document_id":          record.Header.ID,
			"document_type":        record.Header.Type,
			"from_state":           transition.FromState,
			"to_state":             transition.ToState,
			"version":              record.Header.Version,
			"workflow_key":         transition.WorkflowKey,
			"workflow_version":     transition.WorkflowVersion,
			"transition_policy":    decisionSummary(transitionDecision),
			"assignment_policy":    decisionSummary(assignmentDecision),
			"sla_policy":           decisionSummary(slaDecision),
			"snapshots":            documentSnapshotEnvelope(beforeRecord, record),
			"effective_user_id":    acting.EffectiveActorID(),
			"on_behalf_of_user_id": acting.OnBehalfOfUserID,
			"delegation_grant_id":  acting.DelegationGrantID,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	workflowMutation, err := s.actions.buildWorkflowMutation(record, transition, acting.EffectiveActorID(), now, false, "", "")
	if err != nil {
		return document.Record{}, err
	}
	appendWorkflowHistory(&workflowMutation, buildWorkflowHistoryEvent(record, transition, acting.EffectiveActorID(), correlationID, transitionDecision, assignmentDecision, now))
	if err := s.actions.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation, false); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (s *documentTransitionService) UpdateDraft(documentID string, acting ActingContext, payload map[string]any, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := s.actions.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	beforeRecord := cloneDocumentRecord(record)
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
	now := s.actions.currentTime()
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = acting.EffectiveActorID()
	record.Header.UpdatedAt = now
	record.Body.Payload = mergeBasePayload(record.Body.Payload, payload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)

	correlationID := firstNonEmpty(strings.TrimSpace(acting.CorrelationID), fmt.Sprintf("doc-update:%s:%d", record.Header.ID, record.Header.Version))
	auditEvent := audit.Event{
		ID:                fmt.Sprintf("audit:%s", correlationID),
		Action:            "document.update",
		TargetType:        "document",
		TargetID:          record.Header.ID,
		ActorID:           acting.ActorID,
		ActorKind:         "user",
		OnBehalfOfUserID:  acting.OnBehalfOfUserID,
		DelegationGrantID: acting.DelegationGrantID,
		FromState:         "draft",
		ToState:           "draft",
		OrganizationID:    record.Header.OrganizationID,
		LocationID:        record.Header.LocationID,
		OccurredAt:        now,
		CorrelationID:     correlationID,
		ChangeSummary:     documentChangeSummary(beforeRecord, record),
		Metadata: map[string]any{
			"document_type": record.Header.Type,
			"version":       record.Header.Version,
		},
	}
	domainEvent := eventing.Event{
		ID:             fmt.Sprintf("event:%s", correlationID),
		Type:           "document.updated",
		Version:        1,
		AggregateType:  "document",
		AggregateID:    record.Header.ID,
		ActorID:        acting.ActorID,
		CorrelationID:  correlationID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		OccurredAt:     now,
		Payload: map[string]any{
			"document_id":          record.Header.ID,
			"document_type":        record.Header.Type,
			"status":               record.Header.Status,
			"version":              record.Header.Version,
			"snapshots":            documentSnapshotEnvelope(beforeRecord, record),
			"effective_user_id":    acting.EffectiveActorID(),
			"on_behalf_of_user_id": acting.OnBehalfOfUserID,
			"delegation_grant_id":  acting.DelegationGrantID,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	if err := s.actions.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflow.Mutation{}, true); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (s *documentTransitionService) UpdateExtension(documentID, moduleKey string, acting ActingContext, extensionPayload map[string]any, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := s.actions.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	beforeRecord := cloneDocumentRecord(record)
	if record.Header.Status != "draft" {
		return document.Record{}, shared.Conflict("only draft documents may be extended")
	}
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	if !hasDocumentExtension(s.actions.documents.ExtensionDefinitions(record.Header.Type), moduleKey) {
		return document.Record{}, shared.Validation("document extension is not registered")
	}

	previousVersion := record.Header.Version
	now := s.actions.currentTime()
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = acting.EffectiveActorID()
	record.Header.UpdatedAt = now
	record.Body.Payload = document.SetExtensionPayload(record.Body.Payload, moduleKey, extensionPayload)
	record.Body.ContentHash = document.ContentHash(record.Body.Payload)

	correlationID := firstNonEmpty(strings.TrimSpace(acting.CorrelationID), fmt.Sprintf("doc-extension-update:%s:%s:%d", moduleKey, record.Header.ID, record.Header.Version))
	auditEvent := audit.Event{
		ID:                fmt.Sprintf("audit:%s", correlationID),
		Action:            "document.extension.update",
		TargetType:        "document",
		TargetID:          record.Header.ID,
		ActorID:           acting.ActorID,
		ActorKind:         "user",
		OnBehalfOfUserID:  acting.OnBehalfOfUserID,
		DelegationGrantID: acting.DelegationGrantID,
		FromState:         record.Header.Status,
		ToState:           record.Header.Status,
		OrganizationID:    record.Header.OrganizationID,
		LocationID:        record.Header.LocationID,
		OccurredAt:        now,
		CorrelationID:     correlationID,
		ChangeSummary:     documentChangeSummary(beforeRecord, record),
		Metadata: map[string]any{
			"document_type": record.Header.Type,
			"module_key":    moduleKey,
			"version":       record.Header.Version,
		},
	}
	domainEvent := eventing.Event{
		ID:             fmt.Sprintf("event:%s", correlationID),
		Type:           "document.extension.updated",
		Version:        1,
		AggregateType:  "document",
		AggregateID:    record.Header.ID,
		ActorID:        acting.ActorID,
		CorrelationID:  correlationID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		OccurredAt:     now,
		Payload: map[string]any{
			"document_id":          record.Header.ID,
			"document_type":        record.Header.Type,
			"module_key":           moduleKey,
			"status":               record.Header.Status,
			"version":              record.Header.Version,
			"snapshots":            documentSnapshotEnvelope(beforeRecord, record),
			"effective_user_id":    acting.EffectiveActorID(),
			"on_behalf_of_user_id": acting.OnBehalfOfUserID,
			"delegation_grant_id":  acting.DelegationGrantID,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	if err := s.actions.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflow.Mutation{}, true); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (s *documentTransitionService) Approve(documentID string, acting ActingContext, expectedVersion int, expectedETag string) (document.Record, error) {
	record, err := s.actions.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	beforeRecord := cloneDocumentRecord(record)
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	def, err := s.actions.documents.Definition(record.Header.Type)
	if err != nil {
		return document.Record{}, err
	}
	transition, err := s.actions.workflows.Execute(def.WorkflowKey, record.Header.Status, "approve")
	if err != nil {
		return document.Record{}, err
	}
	if boundVersion := boundWorkflowVersion(record); boundVersion > 0 {
		transition, err = s.actions.workflows.ExecuteVersion(def.WorkflowKey, boundVersion, record.Header.Status, "approve")
		if err != nil {
			return document.Record{}, err
		}
	}
	transitionDecision, assignmentDecision, slaDecision, err := s.actions.applyWorkflowRuntimeDecisions(record, acting.EffectiveActorID(), "approve", &transition)
	if err != nil {
		return document.Record{}, err
	}
	previousVersion := record.Header.Version
	now := s.actions.currentTime()
	if stagedMutation, stagedPayload, staged, err := s.actions.resolveApprovalProgression(&record, acting.EffectiveActorID(), transition, now); err != nil {
		return document.Record{}, err
	} else if staged {
		record.Header.Version++
		record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
		record.Header.UpdatedBy = acting.EffectiveActorID()
		record.Header.UpdatedAt = now

		correlationID := firstNonEmpty(strings.TrimSpace(acting.CorrelationID), fmt.Sprintf("doc-approve-stage:%s:%d", record.Header.ID, record.Header.Version))
		auditEvent := audit.Event{
			ID:                fmt.Sprintf("audit:%s", correlationID),
			Action:            "document.approval.progress",
			TargetType:        "document",
			TargetID:          record.Header.ID,
			ActorID:           acting.ActorID,
			ActorKind:         "user",
			OnBehalfOfUserID:  acting.OnBehalfOfUserID,
			DelegationGrantID: acting.DelegationGrantID,
			FromState:         transition.FromState,
			ToState:           record.Header.Status,
			OrganizationID:    record.Header.OrganizationID,
			LocationID:        record.Header.LocationID,
			OccurredAt:        now,
			CorrelationID:     correlationID,
			ChangeSummary:     documentChangeSummary(beforeRecord, record),
			Metadata: map[string]any{
				"document_type":     record.Header.Type,
				"version":           record.Header.Version,
				"workflow_key":      transition.WorkflowKey,
				"workflow_version":  transition.WorkflowVersion,
				"policy_code":       transitionDecision.Code,
				"policy_reason":     transitionDecision.Reason,
				"assignment_policy": decisionSummary(assignmentDecision),
				"sla_policy":        decisionSummary(slaDecision),
				"approval_stage":    stagedPayload,
			},
		}
		domainEvent := eventing.Event{
			ID:             fmt.Sprintf("event:%s", correlationID),
			Type:           "document.approval.progressed",
			Version:        1,
			AggregateType:  "document",
			AggregateID:    record.Header.ID,
			ActorID:        acting.ActorID,
			CorrelationID:  correlationID,
			OrganizationID: record.Header.OrganizationID,
			LocationID:     record.Header.LocationID,
			OccurredAt:     now,
			Payload: map[string]any{
				"document_id":          record.Header.ID,
				"document_type":        record.Header.Type,
				"status":               record.Header.Status,
				"version":              record.Header.Version,
				"workflow_key":         transition.WorkflowKey,
				"workflow_version":     transition.WorkflowVersion,
				"transition_policy":    decisionSummary(transitionDecision),
				"assignment_policy":    decisionSummary(assignmentDecision),
				"sla_policy":           decisionSummary(slaDecision),
				"approval_stage":       stagedPayload,
				"snapshots":            documentSnapshotEnvelope(beforeRecord, record),
				"effective_user_id":    acting.EffectiveActorID(),
				"on_behalf_of_user_id": acting.OnBehalfOfUserID,
				"delegation_grant_id":  acting.DelegationGrantID,
			},
		}
		outboxRecord := eventing.OutboxRecord{
			ID:        domainEvent.ID + ":outbox",
			EventID:   domainEvent.ID,
			EventType: domainEvent.Type,
			Status:    "pending",
			CreatedAt: now,
		}
		stageTransition := transition
		stageTransition.ToState = record.Header.Status
		appendWorkflowHistory(stagedMutation, buildWorkflowHistoryEvent(record, stageTransition, acting.EffectiveActorID(), correlationID, transitionDecision, assignmentDecision, now))
		if err := s.actions.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, *stagedMutation, false); err != nil {
			return document.Record{}, err
		}
		return record, nil
	}
	record.Header.Status = transition.ToState
	ensureWorkflowBinding(&record, transition.WorkflowKey, transition.WorkflowVersion)
	s.actions.assignNumberIfNeeded(&record, def, acting.EffectiveActorID(), "approve", now)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = acting.EffectiveActorID()
	record.Header.UpdatedAt = now

	correlationID := firstNonEmpty(strings.TrimSpace(acting.CorrelationID), fmt.Sprintf("doc-approve:%s:%d", record.Header.ID, record.Header.Version))
	auditEvent := audit.Event{
		ID:                fmt.Sprintf("audit:%s", correlationID),
		Action:            "document.approve",
		TargetType:        "document",
		TargetID:          record.Header.ID,
		ActorID:           acting.ActorID,
		ActorKind:         "user",
		OnBehalfOfUserID:  acting.OnBehalfOfUserID,
		DelegationGrantID: acting.DelegationGrantID,
		FromState:         transition.FromState,
		ToState:           transition.ToState,
		OrganizationID:    record.Header.OrganizationID,
		LocationID:        record.Header.LocationID,
		OccurredAt:        now,
		CorrelationID:     correlationID,
		ChangeSummary:     documentChangeSummary(beforeRecord, record),
		Metadata: map[string]any{
			"document_type":     record.Header.Type,
			"version":           record.Header.Version,
			"workflow_key":      transition.WorkflowKey,
			"workflow_version":  transition.WorkflowVersion,
			"policy_code":       transitionDecision.Code,
			"policy_reason":     transitionDecision.Reason,
			"assignment_policy": decisionSummary(assignmentDecision),
			"sla_policy":        decisionSummary(slaDecision),
		},
	}
	domainEvent := eventing.Event{
		ID:             fmt.Sprintf("event:%s", correlationID),
		Type:           "document.approved",
		Version:        1,
		AggregateType:  "document",
		AggregateID:    record.Header.ID,
		ActorID:        acting.ActorID,
		CorrelationID:  correlationID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		OccurredAt:     now,
		Payload: map[string]any{
			"document_id":          record.Header.ID,
			"document_type":        record.Header.Type,
			"from_state":           transition.FromState,
			"to_state":             transition.ToState,
			"version":              record.Header.Version,
			"workflow_key":         transition.WorkflowKey,
			"workflow_version":     transition.WorkflowVersion,
			"transition_policy":    decisionSummary(transitionDecision),
			"assignment_policy":    decisionSummary(assignmentDecision),
			"sla_policy":           decisionSummary(slaDecision),
			"snapshots":            documentSnapshotEnvelope(beforeRecord, record),
			"effective_user_id":    acting.EffectiveActorID(),
			"on_behalf_of_user_id": acting.OnBehalfOfUserID,
			"delegation_grant_id":  acting.DelegationGrantID,
		},
	}
	outboxRecord := eventing.OutboxRecord{
		ID:        domainEvent.ID + ":outbox",
		EventID:   domainEvent.ID,
		EventType: domainEvent.Type,
		Status:    "pending",
		CreatedAt: now,
	}
	workflowMutation, err := s.actions.buildWorkflowMutation(record, transition, acting.EffectiveActorID(), now, true, "approved", "completed")
	if err != nil {
		return document.Record{}, err
	}
	appendWorkflowHistory(&workflowMutation, buildWorkflowHistoryEvent(record, transition, acting.EffectiveActorID(), correlationID, transitionDecision, assignmentDecision, now))
	if err := s.actions.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation, false); err != nil {
		return document.Record{}, err
	}
	return record, nil
}

func (s *documentTransitionService) Transition(documentID string, acting ActingContext, expectedVersion int, expectedETag, action, eventType string) (document.Record, error) {
	record, err := s.actions.documents.Get(documentID)
	if err != nil {
		return document.Record{}, err
	}
	beforeRecord := cloneDocumentRecord(record)
	if expectedVersion > 0 && record.Header.Version != expectedVersion {
		return document.Record{}, shared.Conflict("document version mismatch")
	}
	if expectedETag != "" && record.Header.ETag != expectedETag {
		return document.Record{}, shared.Conflict("document etag mismatch")
	}
	def, err := s.actions.documents.Definition(record.Header.Type)
	if err != nil {
		return document.Record{}, err
	}
	transition, err := s.actions.workflows.Execute(def.WorkflowKey, record.Header.Status, action)
	if err != nil {
		return document.Record{}, err
	}
	if boundVersion := boundWorkflowVersion(record); boundVersion > 0 {
		transition, err = s.actions.workflows.ExecuteVersion(def.WorkflowKey, boundVersion, record.Header.Status, action)
		if err != nil {
			return document.Record{}, err
		}
	}
	transitionDecision, assignmentDecision, slaDecision, err := s.actions.applyWorkflowRuntimeDecisions(record, acting.EffectiveActorID(), action, &transition)
	if err != nil {
		return document.Record{}, err
	}
	previousVersion := record.Header.Version
	now := s.actions.currentTime()
	workflowMutation := workflow.Mutation{}
	if action == "reject" || action == "cancel" {
		workflowMutation = s.actions.workflows.PlanResolveArtifacts(documentID, transition.ToState, "cancelled", acting.EffectiveActorID(), now)
	}
	if transition.TaskType != "" || transition.CreateApproval {
		createdMutation, err := s.actions.buildWorkflowMutation(record, transition, acting.EffectiveActorID(), now, false, "", "")
		if err != nil {
			return document.Record{}, err
		}
		mergeWorkflowMutation(&workflowMutation, createdMutation)
	}
	record.Header.Status = transition.ToState
	ensureWorkflowBinding(&record, transition.WorkflowKey, transition.WorkflowVersion)
	record.Header.Version++
	record.Header.ETag = fmt.Sprintf("%s:%d", record.Header.ID, record.Header.Version)
	record.Header.UpdatedBy = acting.EffectiveActorID()
	record.Header.UpdatedAt = now

	correlationID := firstNonEmpty(strings.TrimSpace(acting.CorrelationID), fmt.Sprintf("doc-%s:%s:%d", action, record.Header.ID, record.Header.Version))
	auditEvent := audit.Event{
		ID:                fmt.Sprintf("audit:%s", correlationID),
		Action:            "document." + action,
		TargetType:        "document",
		TargetID:          record.Header.ID,
		ActorID:           acting.ActorID,
		ActorKind:         "user",
		OnBehalfOfUserID:  acting.OnBehalfOfUserID,
		DelegationGrantID: acting.DelegationGrantID,
		FromState:         transition.FromState,
		ToState:           transition.ToState,
		OrganizationID:    record.Header.OrganizationID,
		LocationID:        record.Header.LocationID,
		OccurredAt:        now,
		CorrelationID:     correlationID,
		ChangeSummary:     documentChangeSummary(beforeRecord, record),
		Metadata: map[string]any{
			"document_type":     record.Header.Type,
			"version":           record.Header.Version,
			"workflow_key":      transition.WorkflowKey,
			"workflow_version":  transition.WorkflowVersion,
			"policy_code":       transitionDecision.Code,
			"policy_reason":     transitionDecision.Reason,
			"assignment_policy": decisionSummary(assignmentDecision),
			"sla_policy":        decisionSummary(slaDecision),
		},
	}
	domainEvent := eventing.Event{
		ID:             fmt.Sprintf("event:%s", correlationID),
		Type:           eventType,
		Version:        1,
		AggregateType:  "document",
		AggregateID:    record.Header.ID,
		ActorID:        acting.ActorID,
		CorrelationID:  correlationID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		OccurredAt:     now,
		Payload: map[string]any{
			"document_id":          record.Header.ID,
			"document_type":        record.Header.Type,
			"from_state":           transition.FromState,
			"to_state":             transition.ToState,
			"version":              record.Header.Version,
			"workflow_key":         transition.WorkflowKey,
			"workflow_version":     transition.WorkflowVersion,
			"transition_policy":    decisionSummary(transitionDecision),
			"assignment_policy":    decisionSummary(assignmentDecision),
			"sla_policy":           decisionSummary(slaDecision),
			"snapshots":            documentSnapshotEnvelope(beforeRecord, record),
			"effective_user_id":    acting.EffectiveActorID(),
			"on_behalf_of_user_id": acting.OnBehalfOfUserID,
			"delegation_grant_id":  acting.DelegationGrantID,
		},
	}
	outboxRecord := eventing.OutboxRecord{ID: domainEvent.ID + ":outbox", EventID: domainEvent.ID, EventType: domainEvent.Type, Status: "pending", CreatedAt: now}
	appendWorkflowHistory(&workflowMutation, buildWorkflowHistoryEvent(record, transition, acting.EffectiveActorID(), correlationID, transitionDecision, assignmentDecision, now))
	if err := s.actions.persistDocument(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation, false); err != nil {
		return document.Record{}, err
	}
	return record, nil
}
