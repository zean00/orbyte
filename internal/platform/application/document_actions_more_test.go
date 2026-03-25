package application

import (
	"errors"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/notification"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/workflow"
)

func TestDocumentActionNotificationHelpers(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")

	org := organization.NewService()
	ident := identity.NewService(org)
	assignee, err := ident.CreateUser("assignee@example.com", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create assignee failed: %v", err)
	}

	docs := document.NewService()
	flows := workflow.NewService()
	notifs := notification.NewService()
	actions := NewDocumentActions(docs, flows, ident, nil, NewMemorySubmitStore(docs, flows, nil, nil))
	actions.AttachNotifications(notifs)

	now := time.Now().UTC()
	grant, err := ident.SaveDeepLinkGrant(identity.DeepLinkGrant{
		Kind:       "workflow_approval",
		UserID:     assignee.ID,
		Status:     "pending",
		TargetType: "workflow_approval",
		TargetID:   "approval-1",
		StartsAt:   now.Add(-time.Minute),
		ExpiresAt:  now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("save deep link grant failed: %v", err)
	}
	if grant.ID == "" {
		t.Fatal("expected saved grant id")
	}

	record := document.Record{
		Header: document.Header{
			ID:     "doc-1",
			Type:   "generic_request",
			Number: "REQ-001",
		},
	}
	actions.issueWorkflowNotifications(record,
		[]workflow.Task{
			{ID: "task-1", WorkflowKey: "request_flow", TaskType: "review", AssigneeUserID: assignee.ID},
			{ID: "task-2", WorkflowKey: "request_flow", TaskType: "review", Metadata: map[string]any{"resolved_assignee_user_id": assignee.ID}},
		},
		[]workflow.Approval{
			{ID: "approval-1", WorkflowKey: "request_flow", StageKey: "manager", Metadata: map[string]any{"resolved_assignee_user_id": assignee.ID}},
			{ID: "approval-empty"},
		},
		now,
	)

	items := notifs.List(notification.Filter{UserID: assignee.ID})
	if len(items) != 3 {
		t.Fatalf("expected 3 notifications, got %d: %+v", len(items), items)
	}

	var approvalItem notification.Item
	approvalCount := 0
	taskCount := 0
	for _, item := range items {
		switch item.Category {
		case "workflow_approval":
			approvalCount++
			approvalItem = item
		case "workflow_task":
			taskCount++
			if !strings.HasPrefix(item.DeepLinkPath, "/link/workflow/task/") {
				t.Fatalf("unexpected task deep link: %q", item.DeepLinkPath)
			}
		}
	}
	if approvalCount != 1 || taskCount != 2 {
		t.Fatalf("unexpected notification category counts: approvals=%d tasks=%d items=%+v", approvalCount, taskCount, items)
	}
	if approvalItem.DeepLinkPath != "/link/workflow/approval/approval-1" {
		t.Fatalf("unexpected approval deep link: %q", approvalItem.DeepLinkPath)
	}
	if !strings.Contains(approvalItem.ActionLinkPath, "/link/workflow/approval/approval-1?token=") {
		t.Fatalf("expected tokenized approval action link, got %q", approvalItem.ActionLinkPath)
	}
	if approvalItem.Metadata["recipient_user_id"] != assignee.ID {
		t.Fatalf("unexpected approval metadata: %+v", approvalItem.Metadata)
	}
}

func TestDocumentActionHelperFunctions(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")

	org := organization.NewService()
	ident := identity.NewService(org)
	user, err := ident.CreateUser("helper@example.com", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	now := time.Now().UTC()
	grant, err := ident.SaveDeepLinkGrant(identity.DeepLinkGrant{
		Kind:       "workflow_approval",
		UserID:     user.ID,
		Status:     "pending",
		TargetType: "workflow_approval",
		TargetID:   "approval-42",
		StartsAt:   now.Add(-time.Minute),
		ExpiresAt:  now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("save grant failed: %v", err)
	}

	manager := identity.NewTokenManagerFromEnv()
	actionLink, deepLink := approvalNotificationPaths(ident, workflow.Approval{
		ID:       "approval-42",
		Metadata: map[string]any{"resolved_assignee_user_id": user.ID},
	}, manager, now)
	if deepLink != "/link/workflow/approval/approval-42" {
		t.Fatalf("unexpected deep link: %q", deepLink)
	}
	if !strings.Contains(actionLink, "?token=") {
		t.Fatalf("expected tokenized action link, got %q", actionLink)
	}
	if _, parseErr := manager.Parse(strings.TrimPrefix(actionLink, deepLink+"?token=")); parseErr != nil {
		t.Fatalf("expected action link token to parse: %v", parseErr)
	}

	plainAction, plainDeepLink := approvalNotificationPaths(nil, workflow.Approval{ID: "approval-plain"}, manager, now)
	if plainAction != "" || plainDeepLink != "/link/workflow/approval/approval-plain" {
		t.Fatalf("unexpected nil identity approval paths: action=%q deep=%q", plainAction, plainDeepLink)
	}
	if got := workflowApprovalDeepLinkPath(" approval-42 "); got != "/link/workflow/approval/approval-42" {
		t.Fatalf("unexpected approval path: %q", got)
	}
	if got := workflowApprovalDeepLinkPath(""); got != "/link/workflow/approval" {
		t.Fatalf("unexpected empty approval path: %q", got)
	}
	if got := workflowTaskDeepLink(" task-1 "); got != "/link/workflow/task/task-1" {
		t.Fatalf("unexpected task path: %q", got)
	}
	if got := workflowTaskDeepLink(""); got != "/link/workflow/task" {
		t.Fatalf("unexpected empty task path: %q", got)
	}

	if got := workflowApprovalActionLink(grant, nil); got != "/link/workflow/approval/approval-42" {
		t.Fatalf("expected deep link fallback without token manager, got %q", got)
	}

	body := buildWorkflowApprovalCommunicationBody(identity.DeepLinkGrant{
		Title:   "Approval needed",
		Message: "Please review this request",
	}, "https://example.test/action", "/link/workflow/approval/approval-42")
	if !strings.Contains(body, "Approval needed") || !strings.Contains(body, "Please review this request") || !strings.Contains(body, "Open approval link:") {
		t.Fatalf("unexpected communication body: %q", body)
	}

	if got := metadataString(map[string]any{"name": "  value  "}, "name"); got != "value" {
		t.Fatalf("unexpected metadataString result: %q", got)
	}
	if got := metadataInt(map[string]any{"i": int32(3), "j": float64(4)}, "i", 9); got != 3 {
		t.Fatalf("unexpected metadataInt int32 result: %d", got)
	}
	if got := metadataInt(map[string]any{"j": float64(4)}, "j", 9); got != 4 {
		t.Fatalf("unexpected metadataInt float result: %d", got)
	}
	if got := metadataInt(map[string]any{"bad": "x"}, "bad", 9); got != 9 {
		t.Fatalf("unexpected metadataInt fallback result: %d", got)
	}
	if !metadataBool(map[string]any{"flag": true}, "flag") {
		t.Fatal("expected metadataBool true")
	}
	if got := metadataStrings(map[string]any{"tags": []string{" a ", "b"}}, "tags"); len(got) != 2 || got[0] != " a " {
		t.Fatalf("unexpected metadataStrings []string result: %+v", got)
	}
	if got := metadataStrings(map[string]any{"tags": []any{" a ", "", "b", 3}}, "tags"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected metadataStrings []any result: %+v", got)
	}
	if got := interfaceSliceToStrings([]any{" a ", "", "b", 9}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected interfaceSliceToStrings result: %+v", got)
	}
}

func TestDocumentActionsWorkflowPolicyRuntimeAndAssignmentFallback(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	firstReviewer, err := ident.CreateUser("reviewer-one", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create first reviewer failed: %v", err)
	}
	secondReviewer, err := ident.CreateUser("reviewer-two", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create second reviewer failed: %v", err)
	}

	record := document.Record{
		Header: document.Header{
			ID:             "doc-policy",
			Type:           "generic_request",
			Status:         "draft",
			CreatedBy:      "requester-1",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
		},
	}

	t.Run("workflow policy runtime missing evaluator fails closed", func(t *testing.T) {
		policySvc := policy.NewService()
		for _, def := range []policy.HookDefinition{
			{Key: "documents.workflow.transition", Kind: "workflow", Target: "document_workflow_transition"},
			{Key: "documents.workflow.assignment", Kind: "workflow", Target: "document_workflow_assignment"},
			{Key: "documents.workflow.sla", Kind: "workflow", Target: "document_workflow_sla"},
		} {
			if err := policySvc.Register(def); err != nil {
				t.Fatalf("register hook %q failed: %v", def.Key, err)
			}
		}
		actions := NewDocumentActions(document.NewService(), workflow.NewService(), ident, policySvc, NewMemorySubmitStore(document.NewService(), workflow.NewService(), nil, nil))
		if err := actions.ensureWorkflowPolicyRuntime(record, "documents.workflow.transition"); err == nil {
			t.Fatal("expected missing evaluator runtime to fail")
		}
	})

	t.Run("apply workflow runtime decisions uses evaluator outputs", func(t *testing.T) {
		policySvc := policy.NewService()
		for _, def := range []policy.HookDefinition{
			{Key: "documents.workflow.transition", Kind: "workflow", Target: "document_workflow_transition"},
			{Key: "documents.workflow.assignment", Kind: "workflow", Target: "document_workflow_assignment"},
			{Key: "documents.workflow.sla", Kind: "workflow", Target: "document_workflow_sla"},
		} {
			if err := policySvc.Register(def); err != nil {
				t.Fatalf("register hook %q failed: %v", def.Key, err)
			}
		}
		if err := policySvc.SetEvaluator("documents.workflow.transition", func(req policy.Request) policy.Decision {
			return policy.Decision{Allowed: true, Code: "transition_ok"}
		}); err != nil {
			t.Fatalf("set transition evaluator failed: %v", err)
		}
		if err := policySvc.SetEvaluator("documents.workflow.assignment", func(req policy.Request) policy.Decision {
			return policy.Decision{
				Allowed: true,
				Output: map[string]any{
					"assignment_strategy": "role_fallback",
					"assignment_mode":     "user_queue",
					"assignee_role_key":   "role_admin",
					"fallback_role_key":   "role_admin",
					"candidate_role_keys": []any{" role_admin ", "", "role_ops"},
				},
			}
		}); err != nil {
			t.Fatalf("set assignment evaluator failed: %v", err)
		}
		if err := policySvc.SetEvaluator("documents.workflow.sla", func(req policy.Request) policy.Decision {
			return policy.Decision{
				Allowed: true,
				Output: map[string]any{
					"due_after_seconds":      float64(3600),
					"escalate_after_seconds": 1800,
				},
			}
		}); err != nil {
			t.Fatalf("set sla evaluator failed: %v", err)
		}

		actions := NewDocumentActions(document.NewService(), workflow.NewService(), ident, policySvc, NewMemorySubmitStore(document.NewService(), workflow.NewService(), nil, nil))
		transition := workflow.Transition{WorkflowKey: "request_flow", WorkflowVersion: 2, Action: "submit", TaskType: "review"}
		transitionDecision, assignmentDecision, slaDecision, err := actions.applyWorkflowRuntimeDecisions(record, "approver-1", "submit", &transition)
		if err != nil {
			t.Fatalf("apply workflow runtime decisions failed: %v", err)
		}
		if !transitionDecision.Allowed || !assignmentDecision.Allowed || !slaDecision.Allowed {
			t.Fatalf("expected allowed decisions, got transition=%+v assignment=%+v sla=%+v", transitionDecision, assignmentDecision, slaDecision)
		}
		if transition.AssignmentStrategy != "role_fallback" || transition.AssignmentMode != "user_queue" || transition.AssigneeRoleKey != "role_admin" || transition.FallbackRoleKey != "role_admin" {
			t.Fatalf("unexpected transition assignment output: %+v", transition)
		}
		if len(transition.CandidateRoleKeys) != 2 || transition.CandidateRoleKeys[0] != "role_admin" || transition.CandidateRoleKeys[1] != "role_ops" {
			t.Fatalf("unexpected candidate role keys: %+v", transition.CandidateRoleKeys)
		}
		if transition.DueAfterSeconds != 3600 || transition.EscalateAfterSeconds != 1800 {
			t.Fatalf("unexpected SLA output on transition: %+v", transition)
		}
	})

	t.Run("ensure transition allowed denies same requester and policy denial", func(t *testing.T) {
		docs := document.NewService()
		if err := docs.Register(document.Definition{Type: "manager_request", DisplayName: "Manager Request", SchemaVersion: "v1", WorkflowKey: "manager_flow"}); err != nil {
			t.Fatalf("register document definition failed: %v", err)
		}
		flows := workflow.NewService()
		if err := flows.Register(workflow.Definition{
			Key:    "manager_flow",
			States: []string{"draft", "submitted"},
			Actions: []workflow.ActionRule{
				{Action: "submit", FromState: "draft", ToState: "submitted", RequiresDifferentActor: true},
			},
		}); err != nil {
			t.Fatalf("register workflow definition failed: %v", err)
		}
		_ = flows.ApplyMutation(workflow.Mutation{Approvals: []workflow.Approval{{ID: "approval-1", TargetID: "doc-policy", Status: "pending", RequestedBy: "requester-1"}}})
		actions := NewDocumentActions(docs, flows, ident, nil, NewMemorySubmitStore(docs, flows, nil, nil))
		if err := actions.ensureTransitionAllowed(document.Record{Header: document.Header{
			ID:             "doc-policy",
			Type:           "manager_request",
			Status:         "draft",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
		}}, "requester-1", "submit"); err == nil {
			t.Fatal("expected same requester transition to be denied")
		}

		policySvc := policy.NewService()
		if err := policySvc.Register(policy.HookDefinition{Key: "documents.workflow.transition", Kind: "workflow", Target: "document_workflow_transition"}); err != nil {
			t.Fatalf("register transition hook failed: %v", err)
		}
		if err := policySvc.SetEvaluator("documents.workflow.transition", func(req policy.Request) policy.Decision {
			return policy.Decision{Allowed: false, Reason: "denied by policy"}
		}); err != nil {
			t.Fatalf("set transition evaluator failed: %v", err)
		}
		actions = NewDocumentActions(docs, workflow.NewService(), ident, policySvc, NewMemorySubmitStore(docs, workflow.NewService(), nil, nil))
		if err := actions.ensureTransitionAllowed(document.Record{Header: document.Header{
			ID:             "doc-policy-2",
			Type:           "manager_request",
			Status:         "draft",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
		}}, "requester-2", "submit"); err == nil || !strings.Contains(err.Error(), "denied by policy") {
			t.Fatalf("expected policy denial, got %v", err)
		}
	})

	t.Run("resolve fallback candidates yields assignee or queue", func(t *testing.T) {
		actions := NewDocumentActions(document.NewService(), workflow.NewService(), ident, nil, NewMemorySubmitStore(document.NewService(), workflow.NewService(), nil, nil))
		transition := workflow.Transition{AssignmentStrategy: "role_fallback", FallbackRoleKey: "platform_admin"}
		resolution, err := actions.resolveFallbackCandidates("requester-1", &record, &transition, time.Now().UTC())
		if err != nil {
			t.Fatalf("resolve fallback candidates failed: %v", err)
		}
		if resolution.ResolvedVia != "fallback_role" {
			t.Fatalf("expected fallback role resolution, got %+v", resolution)
		}
		if len(resolution.CandidateUserIDs) == 0 && resolution.AssigneeUserID == "" {
			t.Fatalf("expected one assignee or candidate queue, got %+v", resolution)
		}
		if len(resolution.CandidateUserIDs) > 0 {
			foundFirst := false
			foundSecond := false
			for _, id := range resolution.CandidateUserIDs {
				if id == firstReviewer.ID {
					foundFirst = true
				}
				if id == secondReviewer.ID {
					foundSecond = true
				}
			}
			if !foundFirst || !foundSecond {
				t.Fatalf("expected created reviewers in candidate queue, got %+v", resolution)
			}
		}
	})
}

type failingSubmitStore struct {
	err error
}

func (s failingSubmitStore) Submit(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error {
	return s.err
}

func (s failingSubmitStore) UpdateDraft(previousVersion int, record document.Record, auditEvent audit.Event, domainEvent eventing.Event, outboxRecord eventing.OutboxRecord, workflowMutation workflow.Mutation) error {
	return s.err
}

func TestSubmitDoesNotPublishWorkflowArtifactsWhenPersistenceFails(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")

	org := organization.NewService()
	ident := identity.NewService(org)
	requester, err := ident.CreateUser("requester-fail", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create requester failed: %v", err)
	}
	manager, err := ident.CreateUser("manager-fail@example.com", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}
	if _, err := ident.UpsertReportingLine(identity.ReportingLine{
		SubjectUserID:    requester.ID,
		ManagerUserID:    manager.ID,
		RelationshipType: "primary_manager",
		Status:           "active",
	}); err != nil {
		t.Fatalf("save reporting line failed: %v", err)
	}

	docs := document.NewService()
	if err := docs.Register(document.Definition{
		Type:          "linked_request_fail",
		DisplayName:   "Linked Request Fail",
		SchemaVersion: "v1",
		WorkflowKey:   "linked_request_fail_flow",
	}); err != nil {
		t.Fatalf("register document definition failed: %v", err)
	}
	flows := workflow.NewService()
	if err := flows.Register(workflow.Definition{
		Key:    "linked_request_fail_flow",
		States: []string{"draft", "submitted", "approved"},
		Actions: []workflow.ActionRule{
			{Action: "submit", FromState: "draft", ToState: "submitted", CreateApproval: true, TaskType: "review", AssignmentStrategy: "requester_manager", LinkMode: "tokenized", LinkTTLSeconds: 3600, LinkAllowedActions: []string{"approve"}},
			{Action: "approve", FromState: "submitted", ToState: "approved"},
		},
	}); err != nil {
		t.Fatalf("register workflow failed: %v", err)
	}

	notifs := notification.NewService()
	activitySvc := activity.NewService()
	actions := NewDocumentActions(docs, flows, ident, nil, failingSubmitStore{err: errors.New("submit failed")})
	actions.AttachNotifications(notifs)
	actions.AttachActivities(activitySvc)

	record, err := docs.Create("linked_request_fail", "org_default", "loc_hq", requester.ID, map[string]any{"title": "Should fail"})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if _, err := actions.Submit(record.Header.ID, testActing(requester.ID), record.Header.Version, record.Header.ETag); err == nil || !strings.Contains(err.Error(), "submit failed") {
		t.Fatalf("expected submit failure, got %v", err)
	}
	if len(notifs.List(notification.Filter{})) != 0 {
		t.Fatalf("expected no notifications after failed persist, got %+v", notifs.List(notification.Filter{}))
	}
	if timeline := activitySvc.Timeline("document", record.Header.ID); len(timeline) != 0 {
		t.Fatalf("expected no activity after failed persist, got %+v", timeline)
	}
	if grants := ident.DeepLinkGrants(); len(grants) != 0 {
		t.Fatalf("expected no deep link grants after failed persist, got %+v", grants)
	}
}
