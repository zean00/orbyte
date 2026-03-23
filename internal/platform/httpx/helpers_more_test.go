package httpx

import (
	"crypto/tls"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/notification"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func TestNotificationAndDeepLinkHelpers(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")

	org := organization.NewService()
	ident := identity.NewService(org)
	user, err := ident.CreateUser("helper@example.com", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	docs := document.NewService()
	record, err := docs.Create("generic_request", "org_default", "loc_hq", user.ID, map[string]any{"title": "Helper"})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	record.Header.Number = "REQ-001"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save document failed: %v", err)
	}
	record, err = docs.Get(record.Header.ID)
	if err != nil {
		t.Fatalf("get document failed: %v", err)
	}

	flows := workflow.NewService()
	now := time.Now().UTC()
	_ = flows.ApplyMutation(workflow.Mutation{
		Approvals: []workflow.Approval{{
			ID:          "approval-1",
			TargetID:    record.Header.ID,
			WorkflowKey: "request_flow",
			StageKey:    "manager",
			Status:      "pending",
			Metadata:    map[string]any{"resolved_assignee_user_id": user.ID, "link_allowed_actions": []any{"approve", "reject"}},
		}},
		Tasks: []workflow.Task{{
			ID:             "task-1",
			TargetID:       record.Header.ID,
			WorkflowKey:    "request_flow",
			TaskType:       "review",
			AssigneeUserID: user.ID,
			Status:         "pending",
		}},
	})
	grant, err := ident.SaveDeepLinkGrant(identity.DeepLinkGrant{
		Kind:           "workflow_approval",
		UserID:         user.ID,
		Status:         "pending",
		TargetType:     "workflow_approval",
		TargetID:       "approval-1",
		AllowedActions: []string{"approve"},
		StartsAt:       now.Add(-time.Minute),
		ExpiresAt:      now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("save deep link grant failed: %v", err)
	}

	items := enrichNotifications([]notification.Item{
		{
			ID:         "notif-approval",
			UserID:     user.ID,
			Category:   "workflow_approval",
			TargetType: "workflow_approval",
			TargetID:   "approval-1",
			Metadata:   map[string]any{"document_id": record.Header.ID},
			CreatedAt:  now,
		},
		{
			ID:         "notif-task",
			UserID:     user.ID,
			Category:   "workflow_task",
			TargetType: "workflow_task",
			TargetID:   "task-1",
			Metadata:   map[string]any{"document_id": record.Header.ID},
			CreatedAt:  now,
		},
	}, flows, docs)
	if len(items) != 2 {
		t.Fatalf("expected 2 enriched notifications, got %d", len(items))
	}
	if items[0]["approval"] == nil || items[0]["document"] == nil {
		t.Fatalf("expected approval notification enrichment, got %+v", items[0])
	}
	if items[1]["task"] == nil || items[1]["document"] == nil {
		t.Fatalf("expected task notification enrichment, got %+v", items[1])
	}

	if id, action, ok := notificationActionPath("/ui/data/notifications/notif-approval/actions/read", "/ui/data/notifications/"); !ok || id != "notif-approval" || action != "read" {
		t.Fatalf("unexpected notification action path parse: id=%q action=%q ok=%v", id, action, ok)
	}
	if _, _, ok := notificationActionPath("/ui/data/notifications/notif-approval/read", "/ui/data/notifications/"); ok {
		t.Fatal("expected malformed notification action path to fail")
	}

	if id, ok := workflowTaskLinkPath("/link/workflow/task/task-1"); !ok || id != "task-1" {
		t.Fatalf("unexpected workflow task link parse: id=%q ok=%v", id, ok)
	}
	if id, ok := documentLinkPath("/link/document/" + record.Header.ID); !ok || id != record.Header.ID {
		t.Fatalf("unexpected document link parse: id=%q ok=%v", id, ok)
	}
	if got := workflowTaskDeepLinkPath("task-1"); got != "/link/workflow/task/task-1" {
		t.Fatalf("unexpected workflow task deep link path: %q", got)
	}

	if !deepLinkAllowsAction(grant, nil, "approve") {
		t.Fatal("expected explicit allowed action to pass")
	}
	if deepLinkAllowsAction(identity.DeepLinkGrant{ReviewOnly: true}, []string{"approve"}, "approve") {
		t.Fatal("expected review-only grant to deny approve")
	}
	if !deepLinkAllowsAction(identity.DeepLinkGrant{}, []string{"reject"}, "reject") {
		t.Fatal("expected fallback actions to allow reject")
	}
	if deepLinkAllowsAction(identity.DeepLinkGrant{}, nil, "") {
		t.Fatal("expected empty action to fail")
	}

	if resolvedGrant, ok := activeApprovalGrantForCommunication(ident, flows.ListApprovals()[0]); !ok || resolvedGrant.ID != grant.ID {
		t.Fatalf("expected active approval grant, got %+v ok=%v", resolvedGrant, ok)
	}
	if task, ok := workflowTaskByID(flows, "task-1"); !ok || task.ID != "task-1" {
		t.Fatalf("expected workflow task lookup, got %+v ok=%v", task, ok)
	}
	if _, ok := workflowTaskByID(nil, "task-1"); ok {
		t.Fatal("expected nil workflow service lookup to fail")
	}

	req := httptest.NewRequest("GET", "http://example.test/link/workflow/approval/approval-1", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := absoluteURLForPath(req, "/link/workflow/approval/approval-1"); got != "https://example.test/link/workflow/approval/approval-1" {
		t.Fatalf("unexpected absolute url: %q", got)
	}
	secureReq := httptest.NewRequest("GET", "http://secure.test/path", nil)
	secureReq.TLS = &tls.ConnectionState{}
	if got := absoluteURLForPath(secureReq, "/path"); got != "https://secure.test/path" {
		t.Fatalf("unexpected tls absolute url: %q", got)
	}

	rr := httptest.NewRecorder()
	renderDeepLinkErrorPage(rr, 403, "<Denied>", "Bad <reason>")
	if rr.Code != 403 || !strings.Contains(rr.Body.String(), "&lt;Denied&gt;") || !strings.Contains(rr.Body.String(), "Bad &lt;reason&gt;") {
		t.Fatalf("unexpected deep link error page response: code=%d body=%q", rr.Code, rr.Body.String())
	}
	if got := humanizeDeepLinkError(shared.Validation("custom message")); got != "custom message" {
		t.Fatalf("unexpected humanized platform error: %q", got)
	}
	if got := humanizeDeepLinkError(assertErr{}); got != "The approval link could not be activated." {
		t.Fatalf("unexpected fallback deep link error: %q", got)
	}
}

func TestAdminAndAuthPathHelpers(t *testing.T) {
	if id, action, ok := adminRoleTemplateActionPath("/admin/api/security/role-templates/template-1/location/actions/apply"); !ok || id != "template-1" || action != "location" {
		t.Fatalf("unexpected role template action path parse: id=%q action=%q ok=%v", id, action, ok)
	}
	if _, _, ok := adminRoleTemplateActionPath("/admin/api/security/role-templates/template-1/actions/apply"); ok {
		t.Fatal("expected malformed role template action path to fail")
	}
	if id, ok := userNavigationPreferencesPath("/users/user-1/preferences/navigation"); !ok || id != "user-1" {
		t.Fatalf("unexpected user navigation path parse: id=%q ok=%v", id, ok)
	}
	if id, ok := roleNavigationDefaultsPath("/roles/role-1/defaults/navigation"); !ok || id != "role-1" {
		t.Fatalf("unexpected role navigation path parse: id=%q ok=%v", id, ok)
	}
	if id, ok := roleBindingPriorityPath("/role-bindings/rb-1/priority"); !ok || id != "rb-1" {
		t.Fatalf("unexpected role binding priority path parse: id=%q ok=%v", id, ok)
	}
	if id, action, ok := delegationOutgoingActionPath("/me/delegations/outgoing/grant-1/revoke"); !ok || id != "grant-1" || action != "revoke" {
		t.Fatalf("unexpected delegation outgoing path parse: id=%q action=%q ok=%v", id, action, ok)
	}
	if _, _, ok := delegationOutgoingActionPath("/me/delegations/grant-1/revoke"); ok {
		t.Fatal("expected malformed delegation outgoing path to fail")
	}

	t.Setenv("APP_ENV", "dev")
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	cookie := buildDeepLinkStepUpCookie("token-1", expiresAt)
	if cookie.Name != deepLinkStepUpCookieName || cookie.Value != "token-1" || cookie.Secure {
		t.Fatalf("unexpected deep link step-up cookie: %+v", cookie)
	}
}

func TestHierarchyAndWorkflowAdminHelpers(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	requester, err := ident.CreateUser("requester", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create requester failed: %v", err)
	}
	manager, err := ident.CreateUser("manager", "Password123!", "loc_hq", "role_admin", "deployment", "")
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}
	if _, err := ident.UpsertReportingLine(identity.ReportingLine{
		SubjectUserID:    requester.ID,
		ManagerUserID:    manager.ID,
		RelationshipType: "primary_manager",
		Status:           "active",
		LocationID:       "loc_hq",
		EffectiveFrom:    time.Now().UTC().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("upsert reporting line failed: %v", err)
	}

	chain := hierarchyChain(ident, requester.ID, "", "loc_hq", "")
	if len(chain) != 2 || chain[0]["manager_user_id"] != manager.ID {
		t.Fatalf("unexpected hierarchy chain: %+v", chain)
	}

	if got := mapUsers([]identity.User{{ID: requester.ID, Username: requester.Username}, {ID: manager.ID, Username: manager.Username}}, func(item identity.User) string { return item.Username }); len(got) != 2 || got[0] != requester.Username || got[1] != manager.Username {
		t.Fatalf("unexpected mapped users: %+v", got)
	}
	if got := firstTemplateScope(nil); got != "deployment" {
		t.Fatalf("unexpected empty template scope default: %q", got)
	}
	if got := firstTemplateScope([]string{"location", "deployment"}); got != "location" {
		t.Fatalf("unexpected first template scope: %q", got)
	}

	def := workflow.Definition{
		Key:     "request_flow",
		Version: 2,
		Actions: []workflow.ActionRule{
			{Action: "submit", FromState: "draft", ToState: "pending_manager", AssignmentStrategy: "requester_manager", FallbackRoleKey: "platform_admin"},
		},
	}
	preview := workflowRoutingPreview(ident, def, workflow.SimulationInput{
		CurrentState:   "draft",
		Action:         "submit",
		ActorID:        requester.ID,
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
	})
	if valid, _ := preview["valid"].(bool); !valid {
		t.Fatalf("expected valid workflow routing preview, got %+v", preview)
	}
	if preview["resolved_assignee_user_id"] != manager.ID {
		t.Fatalf("expected manager assignment in preview, got %+v", preview)
	}

	if issues := workflowPolicyRuntimeIssues(nil, def, "", ""); issues != nil {
		t.Fatalf("expected nil policy issues without service, got %+v", issues)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "assert" }
