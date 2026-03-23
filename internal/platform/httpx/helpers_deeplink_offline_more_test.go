package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/workflow"
)

func TestDeepLinkAccessAndStepUpHelpers(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")
	org := organization.NewService()
	ident := identity.NewService(org)
	docs := document.NewService()
	workflowSvc := workflow.NewService()
	auditSvc := audit.NewService()
	now := time.Now().UTC()

	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "Approval Target"})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	record.Header.Number = "REQ-STEPUP-1"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save document failed: %v", err)
	}
	if err := workflowSvc.ApplyMutation(workflow.Mutation{
		Approvals: []workflow.Approval{{
			ID:          "approval-step-up",
			TargetID:    record.Header.ID,
			WorkflowKey: "generic_request_flow",
			StageKey:    "review",
			Status:      "pending",
			Metadata:    map[string]any{"resolved_assignee_user_id": "user_admin"},
		}},
	}); err != nil {
		t.Fatalf("apply workflow mutation failed: %v", err)
	}

	grant, err := ident.SaveDeepLinkGrant(identity.DeepLinkGrant{
		Kind:                  "workflow_approval",
		UserID:                "user_admin",
		Status:                "pending",
		TargetType:            "workflow_approval",
		TargetID:              "approval-step-up",
		LocationID:            "loc_hq",
		AllowedPermissionKeys: []string{"document.read", "document.approve"},
		AllowedActions:        []string{"approve"},
		RequireStepUp:         true,
		StartsAt:              now.Add(-time.Minute),
		ExpiresAt:             now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("save deep-link grant failed: %v", err)
	}

	unauthReq := httptest.NewRequest(http.MethodGet, "/link/document/"+record.Header.ID, nil)
	unauthRR := httptest.NewRecorder()
	if ensureDeepLinkDocumentAccess(unauthRR, unauthReq, ident, docs, record.Header.ID, "", "document") {
		t.Fatal("expected unauthenticated deep-link access to fail")
	}
	if unauthRR.Code != http.StatusFound {
		t.Fatalf("expected redirect for unauthenticated deep-link access, got %d", unauthRR.Code)
	}

	sessionPrincipal := principal{
		kind:              userPrincipal,
		userID:            "user_admin",
		effectiveUserID:   "user_admin",
		sessionID:         ident.Sessions()[0].ID,
		currentLocationID: "loc_hq",
		authMethod:        "cookie",
	}
	mismatchReq := httptest.NewRequest(http.MethodGet, "/link/document/"+record.Header.ID, nil)
	mismatchReq = mismatchReq.WithContext(context.WithValue(mismatchReq.Context(), principalContextKey, sessionPrincipal))
	mismatchRR := httptest.NewRecorder()
	if ensureDeepLinkDocumentAccess(mismatchRR, mismatchReq, ident, docs, record.Header.ID, "user_other", "workflow task") {
		t.Fatal("expected assigned-user mismatch to fail")
	}
	if mismatchRR.Code != http.StatusForbidden || !strings.Contains(mismatchRR.Body.String(), "not assigned") {
		t.Fatalf("unexpected assigned-user mismatch response: code=%d body=%q", mismatchRR.Code, mismatchRR.Body.String())
	}

	linkPrincipal := principal{
		kind:              userPrincipal,
		userID:            "user_admin",
		effectiveUserID:   "user_admin",
		currentLocationID: "loc_hq",
		authMethod:        "link",
		deepLinkGrantID:   grant.ID,
		deepLink:          &grant,
	}
	allowedReq := httptest.NewRequest(http.MethodGet, "/link/document/"+record.Header.ID, nil)
	allowedReq = allowedReq.WithContext(context.WithValue(allowedReq.Context(), principalContextKey, linkPrincipal))
	allowedRR := httptest.NewRecorder()
	if !ensureDeepLinkDocumentAccess(allowedRR, allowedReq, ident, docs, record.Header.ID, "user_admin", "document") {
		t.Fatal("expected deep-link document access to succeed")
	}

	tokenManager := identity.NewTokenManagerFromEnv()
	stepUpReq := httptest.NewRequest(http.MethodPost, "/link/workflow/approval/approval-step-up/actions/step-up", strings.NewReader(`{"password":"admin123!"}`))
	stepUpReq = stepUpReq.WithContext(context.WithValue(stepUpReq.Context(), principalContextKey, linkPrincipal))
	stepUpRR := httptest.NewRecorder()
	handleWorkflowApprovalStepUp(stepUpRR, stepUpReq, ident, workflowSvc, auditSvc, "approval-step-up", tokenManager)
	if stepUpRR.Code != http.StatusOK {
		t.Fatalf("expected successful step-up verification, got %d body=%s", stepUpRR.Code, stepUpRR.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stepUpRR.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode step-up response failed: %v", err)
	}
	if payload["status"] != "verified" {
		t.Fatalf("unexpected step-up payload: %+v", payload)
	}
	foundCookie := false
	for _, cookie := range stepUpRR.Result().Cookies() {
		if cookie.Name == deepLinkStepUpCookieName && cookie.Value != "" {
			foundCookie = true
			break
		}
	}
	if !foundCookie {
		t.Fatal("expected deep-link step-up cookie to be issued")
	}
	if len(auditSvc.List()) == 0 {
		t.Fatal("expected deep-link step-up audit event")
	}
}

func TestRenderWorkflowTaskLandingPageAndOfflineModelSync(t *testing.T) {
	org := organization.NewService()
	ident := identity.NewService(org)
	modules := module.NewService()
	models := model.NewService()
	now := time.Now().UTC()

	for _, permissionKey := range []string{"queue.read", "queue.create", "queue.update"} {
		if err := ident.UpsertPermission(identity.Permission{Key: permissionKey, Module: "queue", Action: "manage", Resource: "queue_model"}); err != nil {
			t.Fatalf("upsert permission failed: %v", err)
		}
		if err := ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permissionKey}); err != nil {
			t.Fatalf("grant permission failed: %v", err)
		}
	}

	if err := models.Register(model.Definition{
		Key:                 "queue_model",
		DisplayName:         "Queue Model",
		Version:             "v1",
		CreatePermissionKey: "queue.create",
		ListPermissionKey:   "queue.read",
		ReadPermissionKey:   "queue.read",
		UpdatePermissionKey: "queue.update",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string", Required: true},
			{Key: "status", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}

	if err := modules.Register(module.Manifest{
		Key:     "queue",
		Name:    "Queue",
		Version: "1.0.0",
		Models: []model.Definition{{
			Key:         "queue_model",
			DisplayName: "Queue Model",
		}},
		Offline: module.OfflineDefinition{
			Models: []module.OfflineModelDefinition{{
				ModelKey:            "queue_model",
				Title:               "Queue Model",
				CreatePermissionKey: "queue.create",
				UpdatePermissionKey: "queue.update",
				RequiredPermissions: []string{"queue.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register module failed: %v", err)
	}

	p := principal{
		kind:              userPrincipal,
		userID:            "user_admin",
		effectiveUserID:   "user_admin",
		sessionID:         ident.Sessions()[0].ID,
		currentLocationID: "loc_hq",
		authMethod:        "cookie",
	}
	baseResult := offline.SyncResultItem{
		Kind:        "model",
		ProcessedAt: now,
		Status:      offline.StatusFailedTerminal,
		AttemptCount: 1,
	}

	createResult := applyOfflineModelSync(ident, p, modules, models, nil, nil, offlineSyncItem{
		Kind:      "model",
		Operation: "create",
		ModelKey:  "queue_model",
		Values:    map[string]any{"name": "Queued Record", "status": "new"},
	}, baseResult)
	if createResult.Status != offline.StatusAccepted || createResult.TargetID == "" || createResult.Version <= 0 {
		t.Fatalf("unexpected offline model create result: %+v", createResult)
	}

	updateResult := applyOfflineModelSync(ident, p, modules, models, application.NewMemoryModelActions(models, nil, audit.NewService(), eventing.NewService()), nil, offlineSyncItem{
		Kind:            "model",
		Operation:       "update",
		ModelKey:        "queue_model",
		TargetID:        createResult.TargetID,
		ExpectedVersion: createResult.Version,
		Values:          map[string]any{"name": "Queued Record Updated", "status": "active"},
	}, baseResult)
	if updateResult.Status != offline.StatusAccepted || updateResult.Version <= createResult.Version {
		t.Fatalf("unexpected offline model update result: %+v", updateResult)
	}

	conflictResult := applyOfflineModelSync(ident, p, modules, models, nil, nil, offlineSyncItem{
		Kind:            "model",
		Operation:       "update",
		ModelKey:        "queue_model",
		TargetID:        createResult.TargetID,
		ExpectedVersion: 1,
		Values:          map[string]any{"name": "Stale Update"},
	}, baseResult)
	if conflictResult.Status != offline.StatusConflict || conflictResult.ErrorCode != "version_conflict" {
		t.Fatalf("unexpected offline model conflict result: %+v", conflictResult)
	}

	unsupportedResult := applyOfflineModelSync(ident, p, modules, models, nil, nil, offlineSyncItem{
		Kind:      "model",
		Operation: "archive",
		ModelKey:  "queue_model",
	}, baseResult)
	if unsupportedResult.ErrorCode != "unsupported_operation" {
		t.Fatalf("expected unsupported operation error, got %+v", unsupportedResult)
	}

	record := document.Record{Header: document.Header{ID: "doc-task-1", Number: "REQ-TASK-1", Status: "submitted"}}
	task := workflow.Task{ID: "task-1", WorkflowKey: "generic_request_flow", AssigneeUserID: "user_admin"}
	rr := httptest.NewRecorder()
	renderWorkflowTaskLandingPage(rr, task, record)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "REQ-TASK-1") || !strings.Contains(rr.Body.String(), "work_item_id=task-1") {
		t.Fatalf("unexpected workflow task landing page response: code=%d body=%q", rr.Code, rr.Body.String())
	}

	if id, action, ok := opsOutboxActionPath("/ops/outbox/outbox-1/retry"); !ok || id != "outbox-1" || action != "retry" {
		t.Fatalf("unexpected ops outbox action path parse: id=%q action=%q ok=%v", id, action, ok)
	}
	if _, _, ok := opsOutboxActionPath("/ops/outbox/deliveries/outbox-1/retry"); ok {
		t.Fatal("expected deliveries path to be rejected by outbox action parser")
	}
	if id, action, ok := opsOutboxDeliveryActionPath("/ops/outbox/deliveries/delivery-1/retry"); !ok || id != "delivery-1" || action != "retry" {
		t.Fatalf("unexpected ops outbox delivery action path parse: id=%q action=%q ok=%v", id, action, ok)
	}
	if _, _, ok := opsOutboxDeliveryActionPath("/ops/outbox/deliveries/delivery-1"); ok {
		t.Fatal("expected malformed outbox delivery action path to fail")
	}
}
