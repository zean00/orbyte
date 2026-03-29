package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/communication"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/notification"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/runtimeconfig"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/store"
	"orbyte/internal/platform/workflow"
)

type SubmitStore interface {
	Submit(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error
	UpdateDraft(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error
}

type DocumentActions struct {
	documents     *document.Service
	workflows     *workflow.Service
	identity      *identity.Service
	policy        *policy.Service
	activity      *activity.Service
	notifications *notification.Service
	store         SubmitStore
	runner        *KernelCommandRunner
	now           func() time.Time
	autoDispatch  func() bool
	persistence   *documentPersistenceService
	artifacts     *workflowArtifactService
	transitions   *documentTransitionService
}

func NewDocumentActions(documents *document.Service, workflows *workflow.Service, ident *identity.Service, policySvc *policy.Service, store SubmitStore) *DocumentActions {
	actions := &DocumentActions{
		documents:    documents,
		workflows:    workflows,
		identity:     ident,
		policy:       policySvc,
		store:        store,
		now:          func() time.Time { return time.Now().UTC() },
		autoDispatch: func() bool { return runtimeconfig.Current().WorkflowEmailAutoDispatch() },
	}
	if provider, ok := store.(interface{ TransactionManager() TransactionManager }); ok {
		actions.runner = NewKernelCommandRunner(provider.TransactionManager())
	}
	actions.persistence = newDocumentPersistenceService(store, actions.runner)
	actions.artifacts = newWorkflowArtifactService(actions)
	actions.transitions = newDocumentTransitionService(actions, actions.persistence, actions.artifacts)
	return actions
}

func (a *DocumentActions) AttachActivities(activities *activity.Service) {
	if a == nil {
		return
	}
	a.activity = activities
}

func (a *DocumentActions) AttachNotifications(notifications *notification.Service) {
	if a == nil {
		return
	}
	a.notifications = notifications
}

func (a *DocumentActions) Submit(documentID string, acting ActingContext, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitions.Submit(documentID, acting, expectedVersion, expectedETag)
}

func (a *DocumentActions) UpdateDraft(documentID string, acting ActingContext, payload map[string]any, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitions.UpdateDraft(documentID, acting, payload, expectedVersion, expectedETag)
}

func (a *DocumentActions) UpdateExtension(documentID, moduleKey string, acting ActingContext, extensionPayload map[string]any, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitions.UpdateExtension(documentID, moduleKey, acting, extensionPayload, expectedVersion, expectedETag)
}

func (a *DocumentActions) Approve(documentID string, acting ActingContext, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitions.Approve(documentID, acting, expectedVersion, expectedETag)
}

func (a *DocumentActions) persistDocument(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation, draft bool) error {
	result, err := a.persistence.Persist(previousVersion, record, auditEvent, domainEvent, outboxRecord, workflowMutation, draft)
	if err != nil {
		return err
	}
	a.artifacts.Publish(result)
	return nil
}

func (a *DocumentActions) Reject(documentID string, acting ActingContext, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitions.Transition(documentID, acting, expectedVersion, expectedETag, "reject", "document.reject")
}

func (a *DocumentActions) Reopen(documentID string, acting ActingContext, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitions.Transition(documentID, acting, expectedVersion, expectedETag, "reopen", "document.reopened")
}

func (a *DocumentActions) Cancel(documentID string, acting ActingContext, expectedVersion int, expectedETag string) (document.Record, error) {
	return a.transitions.Transition(documentID, acting, expectedVersion, expectedETag, "cancel", "document.cancelled")
}

func (a *DocumentActions) Transition(documentID string, acting ActingContext, expectedVersion int, expectedETag, action, eventType string) (document.Record, error) {
	return a.transitions.Transition(documentID, acting, expectedVersion, expectedETag, action, eventType)
}

func (a *DocumentActions) transitionDocument(documentID string, acting ActingContext, expectedVersion int, expectedETag, action, eventType string) (document.Record, error) {
	return a.transitions.Transition(documentID, acting, expectedVersion, expectedETag, action, eventType)
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
	if err := a.ensureWorkflowPolicyRuntime(record, "documents.workflow.transition"); err != nil {
		return err
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

func (a *DocumentActions) ensureWorkflowPolicyRuntime(record document.Record, hookKeys ...string) error {
	if a.policy == nil {
		return nil
	}
	for _, hookKey := range hookKeys {
		runtime, ok := a.policy.Runtime(hookKey, record.Header.OrganizationID, record.Header.LocationID)
		if !ok {
			return shared.Forbidden("workflow policy runtime is not configured for " + hookKey)
		}
		if runtime.Engine == policy.EngineRego {
			if !runtime.CompileValid {
				return shared.Forbidden("workflow policy runtime invalid for " + hookKey + ": " + firstNonEmpty(runtime.CompileError, "rego source is not configured"))
			}
			if !runtime.EvalValid {
				return shared.Forbidden("workflow policy runtime invalid for " + hookKey + ": " + firstNonEmpty(runtime.EvalError, "rego policy must return a decision object"))
			}
			continue
		}
		if !runtime.EvalValid {
			return shared.Forbidden("workflow policy runtime invalid for " + hookKey + ": " + firstNonEmpty(runtime.EvalError, "policy evaluator is not configured"))
		}
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

func (a *DocumentActions) buildWorkflowMutation(record document.Record, transition workflow.Transition, actorID string, now time.Time, resolveExisting bool, approvalStatus, taskStatus string) (workflow.Mutation, error) {
	mutation := workflow.Mutation{}
	if resolveExisting {
		mutation = a.workflows.PlanResolveArtifacts(record.Header.ID, approvalStatus, taskStatus, actorID, now)
	}
	created := a.workflows.PlanCreateSideEffects(transition, "document", record.Header.ID, actorID, now)
	if err := a.applyAssignmentSnapshots(&record, &transition, &created, actorID, now); err != nil {
		return workflow.Mutation{}, err
	}
	mergeWorkflowMutation(&mutation, created)
	return mutation, nil
}

func mergeWorkflowMutation(dst *workflow.Mutation, src workflow.Mutation) {
	if dst == nil {
		return
	}
	dst.Tasks = append(dst.Tasks, src.Tasks...)
	dst.Approvals = append(dst.Approvals, src.Approvals...)
	dst.TaskUpdates = append(dst.TaskUpdates, src.TaskUpdates...)
	dst.ApprovalUpdates = append(dst.ApprovalUpdates, src.ApprovalUpdates...)
	dst.History = append(dst.History, src.History...)
}

func (a *DocumentActions) applyAssignmentSnapshots(record *document.Record, transition *workflow.Transition, mutation *workflow.Mutation, actorID string, now time.Time) error {
	if record == nil || transition == nil || mutation == nil {
		return nil
	}
	resolution, err := a.resolveAssignment(record, transition, actorID, now)
	if err != nil {
		return err
	}
	if len(mutation.Tasks) == 0 && len(mutation.Approvals) == 0 {
		return nil
	}
	for i := range mutation.Tasks {
		applyTaskAssignmentSnapshot(&mutation.Tasks[i], resolution, now)
	}
	for i := range mutation.Approvals {
		applyApprovalAssignmentSnapshot(&mutation.Approvals[i], resolution, now)
	}
	applyTransitionResolution(transition, resolution)
	return nil
}

type assignmentResolution struct {
	Strategy         string
	SourceUserID     string
	ResolvedVia      string
	AssigneeUserID   string
	CandidateUserIDs []string
	Trace            []string
}

func (a *DocumentActions) resolveAssignment(record *document.Record, transition *workflow.Transition, actorID string, now time.Time) (assignmentResolution, error) {
	resolution := assignmentResolution{
		Strategy: strings.TrimSpace(transition.AssignmentStrategy),
		Trace:    []string{},
	}
	switch resolution.Strategy {
	case "", "static_user", "static_role":
		return resolution, nil
	case "requester_manager":
		sourceUserID := strings.TrimSpace(record.Header.CreatedBy)
		if sourceUserID == "" {
			return resolution, shared.Validation("requester_manager assignment requires document creator")
		}
		return a.resolveManagerAssignment(sourceUserID, record, transition, now)
	case "previous_approver_manager":
		sourceUserID := strings.TrimSpace(actorID)
		if sourceUserID == "" {
			return resolution, shared.Validation("previous_approver_manager assignment requires current approver")
		}
		return a.resolveManagerAssignment(sourceUserID, record, transition, now)
	case "role_fallback":
		return a.resolveFallbackCandidates("", record, transition, now)
	default:
		return resolution, shared.Validation("unsupported workflow assignment strategy")
	}
}

func (a *DocumentActions) resolveManagerAssignment(sourceUserID string, record *document.Record, transition *workflow.Transition, now time.Time) (assignmentResolution, error) {
	resolution := assignmentResolution{Strategy: transition.AssignmentStrategy, SourceUserID: sourceUserID}
	if a.identity == nil {
		return resolution, shared.Validation("identity service is required for manager assignment")
	}
	manager, ok := a.identity.ResolveManager(sourceUserID, record.Header.OrganizationID, record.Header.LocationID, "", now)
	if ok {
		resolution.AssigneeUserID = manager.Manager.ID
		resolution.ResolvedVia = manager.Via
		resolution.Trace = append(resolution.Trace, fmt.Sprintf("%s:%s", manager.Via, manager.Manager.ID))
		return resolution, nil
	}
	resolution.Trace = append(resolution.Trace, "manager:not_found")
	return a.resolveFallbackCandidates(sourceUserID, record, transition, now)
}

func (a *DocumentActions) resolveFallbackCandidates(sourceUserID string, record *document.Record, transition *workflow.Transition, now time.Time) (assignmentResolution, error) {
	resolution := assignmentResolution{Strategy: transition.AssignmentStrategy, SourceUserID: sourceUserID}
	fallbackRoleKey := strings.TrimSpace(transition.FallbackRoleKey)
	if fallbackRoleKey == "" {
		fallbackRoleKey = strings.TrimSpace(transition.AssigneeRoleKey)
	}
	if fallbackRoleKey == "" {
		return resolution, shared.Validation("workflow assignment could not be resolved")
	}
	if a.identity == nil {
		return resolution, shared.Validation("identity service is required for role fallback assignment")
	}
	candidates := a.identity.ResolveRoleCandidates(fallbackRoleKey, record.Header.OrganizationID, record.Header.LocationID, "", now)
	if len(candidates) == 0 {
		return resolution, shared.Validation("workflow assignment could not be resolved")
	}
	resolution.ResolvedVia = "fallback_role"
	resolution.Trace = append(resolution.Trace, "fallback_role:"+fallbackRoleKey)
	if len(candidates) == 1 {
		resolution.AssigneeUserID = candidates[0].ID
		resolution.Trace = append(resolution.Trace, "assignee:"+candidates[0].ID)
		return resolution, nil
	}
	resolution.CandidateUserIDs = make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		resolution.CandidateUserIDs = append(resolution.CandidateUserIDs, candidate.ID)
	}
	return resolution, nil
}

func applyTransitionResolution(transition *workflow.Transition, resolution assignmentResolution) {
	if transition == nil {
		return
	}
	switch {
	case resolution.AssigneeUserID != "":
		transition.AssignmentMode = "user"
	case len(resolution.CandidateUserIDs) > 0:
		transition.AssignmentMode = "user_queue"
	}
}

func applyTaskAssignmentSnapshot(task *workflow.Task, resolution assignmentResolution, now time.Time) {
	if task == nil {
		return
	}
	if task.Metadata == nil {
		task.Metadata = map[string]any{}
	}
	task.Metadata["assignment_strategy"] = resolution.Strategy
	task.Metadata["assignment_source_user_id"] = resolution.SourceUserID
	task.Metadata["resolved_via"] = resolution.ResolvedVia
	task.Metadata["resolution_time"] = now
	if len(resolution.Trace) > 0 {
		task.Metadata["resolution_trace"] = append([]string(nil), resolution.Trace...)
	}
	if resolution.AssigneeUserID != "" {
		task.AssigneeUserID = resolution.AssigneeUserID
		task.Metadata["resolved_assignee_user_id"] = resolution.AssigneeUserID
	}
	if len(resolution.CandidateUserIDs) > 0 {
		task.Metadata["resolved_candidate_user_ids"] = append([]string(nil), resolution.CandidateUserIDs...)
		task.AssignmentMode = "user_queue"
	}
}

func applyApprovalAssignmentSnapshot(approval *workflow.Approval, resolution assignmentResolution, now time.Time) {
	if approval == nil {
		return
	}
	if approval.Metadata == nil {
		approval.Metadata = map[string]any{}
	}
	approval.Metadata["assignment_strategy"] = resolution.Strategy
	approval.Metadata["assignment_source_user_id"] = resolution.SourceUserID
	approval.Metadata["resolved_via"] = resolution.ResolvedVia
	approval.Metadata["resolution_time"] = now
	if len(resolution.Trace) > 0 {
		approval.Metadata["resolution_trace"] = append([]string(nil), resolution.Trace...)
	}
	if resolution.AssigneeUserID != "" {
		approval.Metadata["resolved_assignee_user_id"] = resolution.AssigneeUserID
	}
	if len(resolution.CandidateUserIDs) > 0 {
		approval.Metadata["resolved_candidate_user_ids"] = append([]string(nil), resolution.CandidateUserIDs...)
	}
}

func (a *DocumentActions) issueWorkflowDeepLinks(record document.Record, approvals []workflow.Approval, now time.Time) {
	if a == nil || a.identity == nil || len(approvals) == 0 {
		return
	}
	tokenManager := identity.NewTokenManagerFromEnv()
	for _, approval := range approvals {
		mode := strings.TrimSpace(strings.ToLower(metadataString(approval.Metadata, "link_mode")))
		if mode != "tokenized" {
			continue
		}
		userID := strings.TrimSpace(metadataString(approval.Metadata, "resolved_assignee_user_id"))
		if userID == "" {
			continue
		}
		if _, ok := a.identity.FindActiveDeepLinkGrant("workflow_approval", approval.ID, userID, now); ok {
			continue
		}
		ttlSeconds := metadataInt(approval.Metadata, "link_ttl_seconds", 15*60)
		if ttlSeconds <= 0 {
			ttlSeconds = 15 * 60
		}
		reviewOnly := metadataBool(approval.Metadata, "link_review_only")
		requireStepUp := metadataBool(approval.Metadata, "link_require_step_up")
		allowedActions := metadataStrings(approval.Metadata, "link_allowed_actions")
		allowedPermissions := []string{"document.read"}
		if !reviewOnly {
			for _, action := range allowedActions {
				switch strings.TrimSpace(action) {
				case "approve":
					allowedPermissions = append(allowedPermissions, "document.approve")
				case "reject":
					allowedPermissions = append(allowedPermissions, "document.reject")
				case "reopen":
					allowedPermissions = append(allowedPermissions, "document.reopen")
				case "cancel":
					allowedPermissions = append(allowedPermissions, "document.cancel")
				}
			}
		}
		title := firstNonEmpty(strings.TrimSpace(record.Header.Number), strings.TrimSpace(record.Header.Type), record.Header.ID)
		message := firstNonEmpty(strings.TrimSpace(approval.StageKey), strings.TrimSpace(approval.WorkflowKey), "workflow approval")
		grant, _ := a.identity.SaveDeepLinkGrant(identity.DeepLinkGrant{
			Kind:                  "workflow_approval",
			UserID:                userID,
			Status:                "pending",
			TargetType:            "workflow_approval",
			TargetID:              approval.ID,
			LocationID:            record.Header.LocationID,
			AllowedPermissionKeys: uniqueStrings(allowedPermissions),
			AllowedActions:        uniqueStrings(allowedActions),
			ReviewOnly:            reviewOnly,
			RequireStepUp:         requireStepUp,
			OneTime:               true,
			Title:                 title,
			Message:               message,
			StartsAt:              now,
			ExpiresAt:             now.Add(time.Duration(ttlSeconds) * time.Second),
			Metadata: map[string]any{
				"document_id":    record.Header.ID,
				"document_type":  record.Header.Type,
				"workflow_key":   approval.WorkflowKey,
				"workflow_stage": approval.StageKey,
				"approval_id":    approval.ID,
			},
		})
		if a.activity != nil {
			_, _ = a.activity.AddMessage("document", record.Header.ID, "system", "Workflow approval link issued", map[string]any{
				"kind":              "workflow_communication",
				"approval_id":       approval.ID,
				"workflow_key":      approval.WorkflowKey,
				"workflow_stage":    approval.StageKey,
				"recipient_user_id": userID,
				"grant_id":          grant.ID,
				"grant_kind":        grant.Kind,
				"grant_target_type": grant.TargetType,
				"grant_target_id":   grant.TargetID,
				"deep_link_path":    fmt.Sprintf("/link/workflow/approval/%s", approval.ID),
				"review_only":       reviewOnly,
				"require_step_up":   requireStepUp,
				"expires_at":        grant.ExpiresAt,
			})
		}
		a.autoDispatchWorkflowApprovalEmail(record, approval, grant, tokenManager)
	}
}

func (a *DocumentActions) issueWorkflowNotifications(record document.Record, tasks []workflow.Task, approvals []workflow.Approval, now time.Time) {
	if a == nil || a.notifications == nil {
		return
	}
	tokenManager := identity.NewTokenManagerFromEnv()
	for _, approval := range approvals {
		userID := strings.TrimSpace(metadataString(approval.Metadata, "resolved_assignee_user_id"))
		if userID == "" {
			continue
		}
		recipient := ""
		if user, ok := a.identity.FindUser(userID); ok {
			recipient = strings.TrimSpace(user.Username)
		}
		actionLink, deepLink := approvalNotificationPaths(a.identity, approval, tokenManager, now)
		_, _ = a.notifications.Save(notification.Item{
			UserID:         userID,
			Category:       "workflow_approval",
			Channel:        "in_app",
			Status:         "unread",
			Title:          firstNonEmpty(strings.TrimSpace(record.Header.Number), strings.TrimSpace(record.Header.Type), record.Header.ID),
			Body:           firstNonEmpty(strings.TrimSpace(approval.StageKey), "Workflow approval pending"),
			TargetType:     "workflow_approval",
			TargetID:       approval.ID,
			DeepLinkPath:   deepLink,
			ActionLinkPath: actionLink,
			CreatedAt:      now,
			Metadata: map[string]any{
				"document_id":       record.Header.ID,
				"document_type":     record.Header.Type,
				"workflow_key":      approval.WorkflowKey,
				"workflow_stage":    approval.StageKey,
				"recipient_user_id": userID,
				"recipient":         recipient,
				"ops_path":          fmt.Sprintf("/ops/workflow/approvals/%s/communication", approval.ID),
			},
		})
	}
	for _, task := range tasks {
		userID := strings.TrimSpace(task.AssigneeUserID)
		if userID == "" {
			userID = strings.TrimSpace(metadataString(task.Metadata, "resolved_assignee_user_id"))
		}
		if userID == "" {
			continue
		}
		recipient := ""
		if user, ok := a.identity.FindUser(userID); ok {
			recipient = strings.TrimSpace(user.Username)
		}
		_, _ = a.notifications.Save(notification.Item{
			UserID:       userID,
			Category:     "workflow_task",
			Channel:      "in_app",
			Status:       "unread",
			Title:        firstNonEmpty(strings.TrimSpace(record.Header.Number), strings.TrimSpace(record.Header.Type), record.Header.ID),
			Body:         firstNonEmpty(strings.TrimSpace(task.TaskType), "Workflow task assigned"),
			TargetType:   "workflow_task",
			TargetID:     task.ID,
			DeepLinkPath: workflowTaskDeepLink(task.ID),
			CreatedAt:    now,
			Metadata: map[string]any{
				"document_id":       record.Header.ID,
				"document_type":     record.Header.Type,
				"workflow_key":      task.WorkflowKey,
				"task_type":         task.TaskType,
				"recipient_user_id": userID,
				"recipient":         recipient,
			},
		})
	}
}

func approvalNotificationPaths(ident *identity.Service, approval workflow.Approval, tokenManager *identity.TokenManager, now time.Time) (string, string) {
	deepLink := workflowApprovalDeepLinkPath(approval.ID)
	if ident == nil {
		return "", deepLink
	}
	userID := strings.TrimSpace(metadataString(approval.Metadata, "resolved_assignee_user_id"))
	if userID == "" {
		return "", deepLink
	}
	grant, ok := ident.FindActiveDeepLinkGrant("workflow_approval", approval.ID, userID, now)
	if !ok || tokenManager == nil {
		return "", deepLink
	}
	token, err := tokenManager.IssueDeepLinkToken(grant)
	if err != nil {
		return "", deepLink
	}
	return deepLink + "?token=" + token, deepLink
}

func workflowApprovalDeepLinkPath(approvalID string) string {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return "/link/workflow/approval"
	}
	return "/link/workflow/approval/" + approvalID
}

func workflowTaskDeepLink(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "/link/workflow/task"
	}
	return "/link/workflow/task/" + taskID
}

func (a *DocumentActions) autoDispatchWorkflowApprovalEmail(record document.Record, approval workflow.Approval, grant identity.DeepLinkGrant, tokenManager *identity.TokenManager) {
	if a == nil || !a.workflowEmailAutoDispatchEnabled() {
		return
	}
	user, ok := a.identity.FindUser(strings.TrimSpace(grant.UserID))
	if !ok {
		a.recordWorkflowCommunicationMessage(record.Header.ID, "Workflow approval email skipped", map[string]any{
			"kind":              "workflow_communication",
			"approval_id":       approval.ID,
			"recipient_user_id": grant.UserID,
			"reason":            "recipient_user_not_found",
		})
		return
	}
	recipient := strings.TrimSpace(user.Username)
	if recipient == "" {
		a.recordWorkflowCommunicationMessage(record.Header.ID, "Workflow approval email skipped", map[string]any{
			"kind":              "workflow_communication",
			"approval_id":       approval.ID,
			"recipient_user_id": grant.UserID,
			"reason":            "recipient_email_missing",
		})
		return
	}
	actionLink := workflowApprovalActionLink(grant, tokenManager)
	deepLink := workflowApprovalDeepLink(grant.TargetID)
	subject := "Approval needed: " + firstNonEmpty(strings.TrimSpace(grant.Title), strings.TrimSpace(record.Header.Number), record.Header.ID)
	body := buildWorkflowApprovalCommunicationBody(grant, actionLink, deepLink)
	delivery, err := communication.SendPlainTextEmail(subject, body, recipient)
	if err != nil {
		a.recordWorkflowCommunicationMessage(record.Header.ID, "Workflow approval email skipped", map[string]any{
			"kind":              "workflow_communication",
			"approval_id":       approval.ID,
			"recipient_user_id": grant.UserID,
			"recipient":         recipient,
			"reason":            err.Error(),
		})
		return
	}
	a.recordWorkflowCommunicationMessage(record.Header.ID, "Workflow approval email dispatched", map[string]any{
		"kind":              "workflow_communication",
		"approval_id":       approval.ID,
		"recipient_user_id": grant.UserID,
		"recipient":         recipient,
		"delivery": map[string]any{
			"channel": delivery.Channel,
			"mode":    delivery.Mode,
			"path":    delivery.Path,
			"address": delivery.Address,
		},
		"deep_link_path": deepLink,
		"expires_at":     grant.ExpiresAt,
	})
}

func (a *DocumentActions) recordWorkflowCommunicationMessage(documentID, body string, metadata map[string]any) {
	if a == nil || a.activity == nil {
		return
	}
	_, _ = a.activity.AddMessage("document", documentID, "system", body, metadata)
}

func (a *DocumentActions) workflowEmailAutoDispatchEnabled() bool {
	if a == nil || a.autoDispatch == nil {
		return false
	}
	return a.autoDispatch()
}

func (a *DocumentActions) currentTime() time.Time {
	if a == nil || a.now == nil {
		return time.Now().UTC()
	}
	return a.now()
}

func workflowApprovalActionLink(grant identity.DeepLinkGrant, tokenManager *identity.TokenManager) string {
	if tokenManager == nil || grant.ID == "" {
		return workflowApprovalDeepLink(grant.TargetID)
	}
	token, err := tokenManager.IssueDeepLinkToken(grant)
	if err != nil {
		return workflowApprovalDeepLink(grant.TargetID)
	}
	return workflowApprovalDeepLink(grant.TargetID) + "?token=" + token
}

func workflowApprovalDeepLink(approvalID string) string {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return "/link/workflow/approval"
	}
	return "/link/workflow/approval/" + approvalID
}

func buildWorkflowApprovalCommunicationBody(grant identity.DeepLinkGrant, actionLink, deepLink string) string {
	var body strings.Builder
	if title := strings.TrimSpace(grant.Title); title != "" {
		body.WriteString(title)
		body.WriteString("\n\n")
	}
	if message := strings.TrimSpace(grant.Message); message != "" {
		body.WriteString(message)
		body.WriteString("\n\n")
	}
	if link := strings.TrimSpace(actionLink); link != "" {
		body.WriteString("Open approval link:\n")
		body.WriteString(link)
		body.WriteString("\n\n")
	}
	if link := strings.TrimSpace(deepLink); link != "" {
		body.WriteString("Open in app:\n")
		body.WriteString(link)
		body.WriteString("\n")
	}
	return body.String()
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataInt(metadata map[string]any, key string, fallback int) int {
	if metadata == nil {
		return fallback
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	value, _ := metadata[key].(bool)
	return value
}

func metadataStrings(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	switch value := metadata[key].(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		items := make([]string, 0, len(value))
		for _, item := range value {
			text, _ := item.(string)
			text = strings.TrimSpace(text)
			if text != "" {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func (a *DocumentActions) applyWorkflowRuntimeDecisions(record document.Record, actorID, action string, transition *workflow.Transition) (policy.Decision, policy.Decision, policy.Decision, error) {
	if transition == nil {
		return policy.Decision{}, policy.Decision{}, policy.Decision{}, shared.Validation("workflow transition is required")
	}
	if transition.RequiresDifferentActor && a.workflows != nil {
		for _, approval := range a.workflows.ListApprovals() {
			if approval.TargetID == record.Header.ID && approval.Status == "pending" && approval.RequestedBy != "" && approval.RequestedBy == actorID {
				return policy.Decision{}, policy.Decision{}, policy.Decision{}, shared.Forbidden("workflow transition requires a different actor than the requester")
			}
		}
	}
	transitionDecision := policy.Decision{Allowed: true}
	assignmentDecision := policy.Decision{Allowed: true}
	slaDecision := policy.Decision{Allowed: true}
	if a.policy == nil {
		return transitionDecision, assignmentDecision, slaDecision, nil
	}
	if err := a.ensureWorkflowPolicyRuntime(record, "documents.workflow.transition", "documents.workflow.assignment", "documents.workflow.sla"); err != nil {
		return transitionDecision, assignmentDecision, slaDecision, err
	}
	transitionDecision = a.policy.Evaluate(policy.Request{
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
	if !transitionDecision.Allowed {
		return transitionDecision, assignmentDecision, slaDecision, shared.Forbidden(firstNonEmpty(transitionDecision.Reason, "workflow transition denied by policy"))
	}
	assignmentDecision = a.policy.Evaluate(policy.Request{
		HookKey:        "documents.workflow.assignment",
		ActorID:        actorID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		ScopeID:        record.Header.LocationID,
		Inputs: map[string]any{
			"document_id":      record.Header.ID,
			"document_type":    record.Header.Type,
			"workflow_key":     transition.WorkflowKey,
			"workflow_version": transition.WorkflowVersion,
			"action":           action,
			"task_type":        transition.TaskType,
			"current_state":    record.Header.Status,
		},
	})
	if output := assignmentDecision.Output; output != nil {
		if value, ok := output["assignment_strategy"].(string); ok && strings.TrimSpace(value) != "" {
			transition.AssignmentStrategy = value
		}
		if value, ok := output["assignment_mode"].(string); ok && strings.TrimSpace(value) != "" {
			transition.AssignmentMode = value
		}
		if value, ok := output["assignee_role_key"].(string); ok && strings.TrimSpace(value) != "" {
			transition.AssigneeRoleKey = value
		}
		if value, ok := output["fallback_role_key"].(string); ok && strings.TrimSpace(value) != "" {
			transition.FallbackRoleKey = value
		}
		if value, ok := output["candidate_role_keys"].([]any); ok {
			transition.CandidateRoleKeys = interfaceSliceToStrings(value)
		}
		if value, ok := output["candidate_role_keys"].([]string); ok && len(value) > 0 {
			transition.CandidateRoleKeys = append([]string(nil), value...)
		}
	}
	slaDecision = a.policy.Evaluate(policy.Request{
		HookKey:        "documents.workflow.sla",
		ActorID:        actorID,
		OrganizationID: record.Header.OrganizationID,
		LocationID:     record.Header.LocationID,
		ScopeID:        record.Header.LocationID,
		Inputs: map[string]any{
			"document_id":      record.Header.ID,
			"document_type":    record.Header.Type,
			"workflow_key":     transition.WorkflowKey,
			"workflow_version": transition.WorkflowVersion,
			"action":           action,
			"current_state":    record.Header.Status,
		},
	})
	if output := slaDecision.Output; output != nil {
		if value, ok := output["due_after_seconds"].(float64); ok && int(value) > 0 {
			transition.DueAfterSeconds = int(value)
		}
		if value, ok := output["due_after_seconds"].(int); ok && value > 0 {
			transition.DueAfterSeconds = value
		}
		if value, ok := output["escalate_after_seconds"].(float64); ok && int(value) > 0 {
			transition.EscalateAfterSeconds = int(value)
		}
		if value, ok := output["escalate_after_seconds"].(int); ok && value > 0 {
			transition.EscalateAfterSeconds = value
		}
	}
	return transitionDecision, assignmentDecision, slaDecision, nil
}

func boundWorkflowVersion(record document.Record) int {
	if record.Header.Metadata == nil {
		return 0
	}
	switch value := record.Header.Metadata["workflow_version"].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}

func ensureWorkflowBinding(record *document.Record, workflowKey string, workflowVersion int) {
	if record == nil {
		return
	}
	if record.Header.Metadata == nil {
		record.Header.Metadata = map[string]any{}
	}
	record.Header.Metadata["workflow_key"] = workflowKey
	record.Header.Metadata["workflow_version"] = workflowVersion
}

func decisionSummary(decision policy.Decision) map[string]any {
	summary := map[string]any{
		"allowed": decision.Allowed,
		"code":    decision.Code,
		"reason":  decision.Reason,
	}
	if len(decision.Output) > 0 {
		summary["output"] = decision.Output
	}
	if len(decision.Trace) > 0 {
		summary["trace"] = decision.Trace
	}
	return summary
}

func appendWorkflowHistory(mutation *workflow.Mutation, event workflow.HistoryEvent) {
	if mutation == nil {
		return
	}
	mutation.History = append(mutation.History, event)
}

func buildWorkflowHistoryEvent(record document.Record, transition workflow.Transition, actorID string, correlationID string, transitionDecision, assignmentDecision policy.Decision, now time.Time) workflow.HistoryEvent {
	return workflow.HistoryEvent{
		ID:              fmt.Sprintf("workflow-history:%s:%s:%d", record.Header.ID, transition.Action, record.Header.Version),
		WorkflowKey:     transition.WorkflowKey,
		WorkflowVersion: transition.WorkflowVersion,
		TargetType:      "document",
		TargetID:        record.Header.ID,
		Action:          transition.Action,
		FromState:       transition.FromState,
		ToState:         transition.ToState,
		ActorID:         actorID,
		OccurredAt:      now,
		DecisionCode:    transitionDecision.Code,
		DecisionReason:  transitionDecision.Reason,
		AssignmentSummary: map[string]any{
			"assignment_strategy": transition.AssignmentStrategy,
			"assignment_mode":     transition.AssignmentMode,
			"assignee_role_key":   transition.AssigneeRoleKey,
			"fallback_role_key":   transition.FallbackRoleKey,
			"candidate_role_keys": append([]string(nil), transition.CandidateRoleKeys...),
			"assignment_policy":   decisionSummary(assignmentDecision),
		},
		Metadata: map[string]any{
			"document_type":  record.Header.Type,
			"status":         record.Header.Status,
			"correlation_id": correlationID,
		},
	}
}

func interfaceSliceToStrings(values []any) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			items = append(items, strings.TrimSpace(text))
		}
	}
	return items
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
	return NewPostgresSubmitStoreWithDB(store.UninstrumentedDB(db))
}

func NewPostgresSubmitStoreWithDB(db store.DB) *PostgresSubmitStore {
	return &PostgresSubmitStore{txm: NewPostgresTransactionManagerWithDB(db)}
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

func saveAuditEventTx(tx store.Tx, event audit.Event) error {
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

func saveDomainEventTx(tx store.Tx, event eventing.Event) error {
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

func saveOutboxRecordTx(tx store.Tx, record eventing.OutboxRecord) error {
	const query = `
		INSERT INTO outbox_records (
			outbox_id, event_id, event_type, status, created_at, dispatched_at
		) VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := tx.Exec(query,
		record.ID, record.EventID, record.EventType, record.Status, record.CreatedAt, nullableTime(record.DispatchedAt),
	)
	return err
}

func applyWorkflowMutationTx(tx store.Tx, mutation workflow.Mutation) error {
	for _, task := range mutation.Tasks {
		const query = `
			INSERT INTO workflow_tasks (
				task_id, workflow_key, workflow_version, target_type, target_id, task_type, status, assignment_mode,
				assignee_user_id, assignee_role_key, candidate_role_keys_json, created_by, created_at, due_at, escalate_at, metadata_json
			) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), $11, NULLIF($12,''), $13, $14, $15, $16)`
		candidateRoleKeys, _ := json.Marshal(task.CandidateRoleKeys)
		metadata, _ := json.Marshal(task.Metadata)
		if _, err := tx.ExecContext(context.Background(), query, task.ID, task.WorkflowKey, task.WorkflowVersion, task.TargetType, task.TargetID, task.TaskType, task.Status, task.AssignmentMode, task.AssigneeUserID, task.AssigneeRoleKey, candidateRoleKeys, task.CreatedBy, task.CreatedAt, nullableTime(task.DueAt), nullableTime(task.EscalateAt), metadata); err != nil {
			return err
		}
	}
	for _, approval := range mutation.Approvals {
		const query = `
			INSERT INTO workflow_approvals (
				approval_id, workflow_key, workflow_version, target_type, target_id, status, stage_key, requested_by, requested_at,
				resolved_by, resolved_at, candidate_role_keys_json, due_at, metadata_json
			) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7,''), NULLIF($8,''), $9, NULLIF($10,''), $11, $12, $13, $14)`
		candidateRoleKeys, _ := json.Marshal(approval.CandidateRoleKeys)
		metadata, _ := json.Marshal(approval.Metadata)
		if _, err := tx.ExecContext(context.Background(), query, approval.ID, approval.WorkflowKey, approval.WorkflowVersion, approval.TargetType, approval.TargetID, approval.Status, approval.StageKey, approval.RequestedBy, approval.RequestedAt, approval.ResolvedBy, nullableTime(approval.ResolvedAt), candidateRoleKeys, nullableTime(approval.DueAt), metadata); err != nil {
			return err
		}
	}
	for _, update := range mutation.ApprovalUpdates {
		if _, err := tx.ExecContext(context.Background(), `UPDATE workflow_approvals SET status = $1 WHERE approval_id = $2`, update.Status, update.ID); err != nil {
			return err
		}
	}
	for _, update := range mutation.TaskUpdates {
		if _, err := tx.ExecContext(context.Background(), `UPDATE workflow_tasks SET status = $1, metadata_json = COALESCE(metadata_json,'{}'::jsonb) || jsonb_build_object('resolved_by', NULLIF($2,''), 'resolved_at', $3) WHERE task_id = $4`, update.Status, update.ResolvedBy, nullableTime(update.ResolvedAt), update.ID); err != nil {
			return err
		}
	}
	for _, event := range mutation.History {
		const query = `
			INSERT INTO workflow_history (
				history_id, workflow_key, workflow_version, target_type, target_id, action, from_state, to_state,
				actor_id, occurred_at, decision_code, decision_reason, assignment_summary_json, metadata_json
			) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,NULLIF($11,''),NULLIF($12,''),$13,$14)`
		assignment, _ := json.Marshal(event.AssignmentSummary)
		metadata, _ := json.Marshal(event.Metadata)
		if _, err := tx.ExecContext(context.Background(), query, event.ID, event.WorkflowKey, event.WorkflowVersion, event.TargetType, event.TargetID, event.Action, event.FromState, event.ToState, event.ActorID, event.OccurredAt, event.DecisionCode, event.DecisionReason, assignment, metadata); err != nil {
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

func cloneDocumentRecord(record document.Record) document.Record {
	encoded, _ := json.Marshal(record)
	var cloned document.Record
	_ = json.Unmarshal(encoded, &cloned)
	return cloned
}

func documentSnapshotEnvelope(before, after document.Record) map[string]any {
	return map[string]any{
		"before": map[string]any{
			"header": before.Header,
			"body":   before.Body,
		},
		"after": map[string]any{
			"header": after.Header,
			"body":   after.Body,
		},
	}
}

func documentChangeSummary(before, after document.Record) map[string]any {
	fields := []string{}
	if before.Header.Status != after.Header.Status {
		fields = append(fields, "header.status")
	}
	if before.Header.Number != after.Header.Number {
		fields = append(fields, "header.number")
	}
	if before.Body.ContentHash != after.Body.ContentHash {
		fields = append(fields, "body.payload")
	}
	return map[string]any{
		"fields":         fields,
		"before_version": before.Header.Version,
		"after_version":  after.Header.Version,
		"before_status":  before.Header.Status,
		"after_status":   after.Header.Status,
	}
}
