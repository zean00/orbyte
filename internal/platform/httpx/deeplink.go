package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"orbyte/internal/platform/communication"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func registerDeepLinkRoutes(mux *http.ServeMux, ident *identity.Service, docs *document.Service, workflowSvc *workflow.Service, docActions *application.DocumentActions, auditSvc *audit.Service) {
	if ident == nil || docs == nil || workflowSvc == nil || docActions == nil {
		return
	}
	tokenManager := identity.NewTokenManagerFromEnv()

	mux.HandleFunc("GET /link/workflow/approval/", func(w http.ResponseWriter, r *http.Request) {
		approvalID, ok := workflowApprovalLinkPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("workflow approval link not found"))
			return
		}
		now := time.Now().UTC()
		if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
			claims, err := tokenManager.Parse(token)
			if err != nil {
				renderDeepLinkErrorPage(w, http.StatusForbidden, "Invalid or expired link", "The approval link is no longer valid. Request a new message and try again.")
				return
			}
			if claims.Kind != "link" || claims.DeepLinkGrantID == "" {
				renderDeepLinkErrorPage(w, http.StatusForbidden, "Invalid approval link", "The approval link token is not recognized.")
				return
			}
			grant, err := ident.ActivateDeepLinkGrant(claims.DeepLinkGrantID, claims.Subject, "workflow_approval", approvalID, now)
			if err != nil {
				renderDeepLinkErrorPage(w, http.StatusForbidden, "Approval link unavailable", humanizeDeepLinkError(err))
				return
			}
			cookieToken, err := tokenManager.IssueDeepLinkToken(grant)
			if err != nil {
				respondError(w, err)
				return
			}
			http.SetCookie(w, buildDeepLinkCookie(cookieToken, grant.ExpiresAt))
			http.SetCookie(w, clearedDeepLinkStepUpCookie())
			recordDeepLinkAuditEvent(auditSvc, r, grant, "link.activate", map[string]any{"approval_id": approvalID})
			http.Redirect(w, r, approvalDeepLinkPath(approvalID), http.StatusFound)
			return
		}

		approval, ok := workflowSvc.Approval(approvalID)
		if !ok {
			renderDeepLinkErrorPage(w, http.StatusNotFound, "Approval not found", "The workflow approval referenced by this link no longer exists.")
			return
		}
		record, err := docs.Get(approval.TargetID)
		if err != nil {
			renderDeepLinkErrorPage(w, http.StatusNotFound, "Target not found", "The document referenced by this approval is no longer available.")
			return
		}
		p, authenticated := currentPrincipal(r)
		if !authenticated {
			http.Redirect(w, r, "/ui?next="+url.QueryEscape(approvalDeepLinkPath(approvalID)), http.StatusFound)
			return
		}
		if !deepLinkPrincipalMatchesApproval(p, approval) {
			renderDeepLinkErrorPage(w, http.StatusForbidden, "Approval access denied", "This approval link is not issued for the current user.")
			return
		}
		recordDeepLinkAuditEvent(auditSvc, r, principalDeepLinkGrant(p), "link.open", map[string]any{
			"approval_id": approval.ID,
			"document_id": record.Header.ID,
		})
		renderWorkflowApprovalLandingPage(w, r, ident, p, approval, record)
	})

	mux.HandleFunc("POST /link/workflow/approval/", func(w http.ResponseWriter, r *http.Request) {
		approvalID, action, ok := workflowApprovalActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("workflow approval link action not found"))
			return
		}
		if action == "step-up" {
			handleWorkflowApprovalStepUp(w, r, ident, workflowSvc, auditSvc, approvalID, tokenManager)
			return
		}
		approval, ok := workflowSvc.Approval(approvalID)
		if !ok {
			respondError(w, shared.NotFound("workflow approval not found"))
			return
		}
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !deepLinkPrincipalMatchesApproval(p, approval) {
			respondError(w, shared.Forbidden("approval link is not assigned to the current user"))
			return
		}
		if p.authMethod == "link" && p.deepLink != nil && p.deepLink.RequireStepUp && !p.stepUpVerified {
			respondError(w, shared.Forbidden("step-up verification required"))
			return
		}
		record, err := docs.Get(approval.TargetID)
		if err != nil {
			respondError(w, err)
			return
		}
		requiredPermission := "document.read"
		switch action {
		case "approve":
			requiredPermission = "document.approve"
		case "reject":
			requiredPermission = "document.reject"
		default:
			respondError(w, shared.Validation("unsupported approval action"))
			return
		}
		if !principalAllowsPermission(ident, p, requiredPermission, record.Header.LocationID) {
			respondError(w, shared.Forbidden("approval link does not allow this action"))
			return
		}
		acting := requestActingContext(r, p)
		var updated document.Record
		switch action {
		case "approve":
			updated, err = docActions.Approve(record.Header.ID, acting, record.Header.Version, record.Header.ETag)
		case "reject":
			updated, err = docActions.Reject(record.Header.ID, acting, record.Header.Version, record.Header.ETag)
		}
		if err != nil {
			respondError(w, err)
			return
		}
		if p.authMethod == "link" && p.deepLink != nil {
			if _, err := ident.ConsumeDeepLinkGrant(p.deepLink.ID, principalEffectiveUserID(p), action, time.Now().UTC()); err == nil {
				http.SetCookie(w, clearedDeepLinkCookie())
				http.SetCookie(w, clearedDeepLinkStepUpCookie())
				recordDeepLinkAuditEvent(auditSvc, r, *p.deepLink, "link.consume", map[string]any{
					"approval_id": approval.ID,
					"action":      action,
					"document_id": updated.Header.ID,
				})
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"status":          "completed",
			"action":          action,
			"approval_id":     approval.ID,
			"document_id":     updated.Header.ID,
			"document_status": updated.Header.Status,
			"open_path":       fmt.Sprintf("/ui#/documents/detail?document_id=%s", url.QueryEscape(updated.Header.ID)),
		})
	})

	mux.HandleFunc("GET /link/workflow/task/", func(w http.ResponseWriter, r *http.Request) {
		taskID, ok := workflowTaskLinkPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("workflow task link not found"))
			return
		}
		if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
			claims, err := tokenManager.Parse(token)
			if err != nil {
				renderDeepLinkErrorPage(w, http.StatusForbidden, "Invalid or expired link", "The task link is no longer valid. Request a new message and try again.")
				return
			}
			if claims.Kind != "link" || claims.DeepLinkGrantID == "" {
				renderDeepLinkErrorPage(w, http.StatusForbidden, "Invalid task link", "The task link token is not recognized.")
				return
			}
			grant, err := ident.ActivateDeepLinkGrant(claims.DeepLinkGrantID, claims.Subject, "workflow_task", taskID, time.Now().UTC())
			if err != nil {
				renderDeepLinkErrorPage(w, http.StatusForbidden, "Task link unavailable", humanizeDeepLinkError(err))
				return
			}
			cookieToken, err := tokenManager.IssueDeepLinkToken(grant)
			if err != nil {
				respondError(w, err)
				return
			}
			http.SetCookie(w, buildDeepLinkCookie(cookieToken, grant.ExpiresAt))
			http.SetCookie(w, clearedDeepLinkStepUpCookie())
			http.Redirect(w, r, workflowTaskDeepLinkPath(taskID), http.StatusFound)
			return
		}
		task, ok := workflowTaskByID(workflowSvc, taskID)
		if !ok {
			renderDeepLinkErrorPage(w, http.StatusNotFound, "Task not found", "The workflow task referenced by this link no longer exists.")
			return
		}
		if !ensureDeepLinkDocumentAccess(w, r, ident, docs, task.TargetID, task.AssigneeUserID, "workflow task") {
			return
		}
		record, err := docs.Get(task.TargetID)
		if err != nil {
			renderDeepLinkErrorPage(w, http.StatusNotFound, "Target not found", "The document referenced by this task is no longer available.")
			return
		}
		renderWorkflowTaskLandingPage(w, task, record)
	})

	mux.HandleFunc("GET /link/document/", func(w http.ResponseWriter, r *http.Request) {
		documentID, ok := documentLinkPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("document link not found"))
			return
		}
		if !ensureDeepLinkDocumentAccess(w, r, ident, docs, documentID, "", "document") {
			return
		}
		http.Redirect(w, r, "/ui#/documents/detail?document_id="+url.QueryEscape(documentID), http.StatusFound)
	})

	mux.HandleFunc("GET /ops/workflow/approvals/", func(w http.ResponseWriter, r *http.Request) {
		approvalID, action, ok := workflowOpsApprovalPath(r.URL.Path)
		if !ok || action != "communication" {
			respondError(w, shared.NotFound("workflow approval communication route not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		payload, err := workflowApprovalCommunicationPayload(r, tokenManager, ident, docs, workflowSvc, approvalID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /ops/workflow/approvals/", func(w http.ResponseWriter, r *http.Request) {
		approvalID, action, ok := workflowOpsApprovalCommunicationActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("workflow approval communication action not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		approval, ok := workflowSvc.Approval(approvalID)
		if !ok {
			respondError(w, shared.NotFound("workflow approval not found"))
			return
		}
		record, err := docs.Get(approval.TargetID)
		if err != nil {
			respondError(w, err)
			return
		}
		switch action {
		case "revoke":
			grant, ok := activeApprovalGrantForCommunication(ident, approval)
			if !ok {
				respondError(w, shared.NotFound("workflow approval grant not found"))
				return
			}
			revoked, err := ident.RevokeDeepLinkGrant(grant.ID, time.Now().UTC())
			if err != nil {
				respondError(w, err)
				return
			}
			recordDeepLinkAuditEvent(auditSvc, r, revoked, "link.revoke", map[string]any{"approval_id": approval.ID, "document_id": record.Header.ID})
			respondJSON(w, http.StatusOK, map[string]any{"status": "revoked", "grant": revoked})
		case "reissue":
			current, ok := activeApprovalGrantForCommunication(ident, approval)
			if ok && current.ID != "" {
				_, _ = ident.RevokeDeepLinkGrant(current.ID, time.Now().UTC())
			}
			grant, err := reissueWorkflowApprovalGrant(ident, approval, record, time.Now().UTC())
			if err != nil {
				respondError(w, err)
				return
			}
			token, err := tokenManager.IssueDeepLinkToken(grant)
			if err != nil {
				respondError(w, err)
				return
			}
			recordDeepLinkAuditEvent(auditSvc, r, grant, "link.reissue", map[string]any{"approval_id": approval.ID, "document_id": record.Header.ID})
			respondJSON(w, http.StatusOK, map[string]any{
				"status":          "reissued",
				"grant":           grant,
				"action_link_url": absoluteURLForPath(r, approvalDeepLinkPath(approval.ID)+"?token="+url.QueryEscape(token)),
				"deep_link_url":   absoluteURLForPath(r, approvalDeepLinkPath(approval.ID)),
				"expires_at":      grant.ExpiresAt,
			})
		case "dispatch-email":
			payload, err := workflowApprovalCommunicationPayload(r, tokenManager, ident, docs, workflowSvc, approvalID)
			if err != nil {
				respondError(w, err)
				return
			}
			recipient := strings.TrimSpace(r.URL.Query().Get("recipient"))
			if recipient == "" {
				recipientUserID, _ := payload["recipient_user_id"].(string)
				if user, ok := ident.FindUser(strings.TrimSpace(recipientUserID)); ok {
					recipient = strings.TrimSpace(user.Username)
				}
			}
			delivery, err := dispatchWorkflowApprovalEmail(payload, recipient)
			if err != nil {
				respondError(w, shared.Validation(err.Error()))
				return
			}
			recordAudit(auditSvc, audit.Event{
				ID:            fmt.Sprintf("audit:workflow-approval-email:%d", time.Now().UTC().UnixNano()),
				Action:        "workflow.approval.communication.email",
				TargetType:    "workflow_approval",
				TargetID:      approval.ID,
				ActorID:       "system",
				OccurredAt:    time.Now().UTC(),
				CorrelationID: logging.CorrelationID(r.Context()),
				Metadata:      map[string]any{"recipient": recipient, "delivery": delivery},
			})
			respondJSON(w, http.StatusOK, map[string]any{"status": "dispatched", "delivery": delivery})
		default:
			respondError(w, shared.NotFound("workflow approval communication action not found"))
		}
	})
}

func ensureDeepLinkDocumentAccess(w http.ResponseWriter, r *http.Request, ident *identity.Service, docs *document.Service, documentID, assignedUserID, title string) bool {
	record, err := docs.Get(documentID)
	if err != nil {
		renderDeepLinkErrorPage(w, http.StatusNotFound, "Target not found", "The document referenced by this "+title+" is no longer available.")
		return false
	}
	p, authenticated := currentPrincipal(r)
	if !authenticated {
		http.Redirect(w, r, "/ui?next="+url.QueryEscape(r.URL.Path), http.StatusFound)
		return false
	}
	if assignedUserID != "" && strings.TrimSpace(principalEffectiveUserID(p)) != strings.TrimSpace(assignedUserID) {
		renderDeepLinkErrorPage(w, http.StatusForbidden, "Access denied", "This workflow item is not assigned to the current user.")
		return false
	}
	if !principalAllowsPermission(ident, p, "document.read", record.Header.LocationID) {
		renderDeepLinkErrorPage(w, http.StatusForbidden, "Access denied", "The current principal does not have permission to read this document.")
		return false
	}
	return true
}

func workflowApprovalLinkPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "link" || parts[1] != "workflow" || parts[2] != "approval" {
		return "", false
	}
	approvalID := strings.TrimSpace(parts[3])
	return approvalID, approvalID != ""
}

func workflowApprovalActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "link" || parts[1] != "workflow" || parts[2] != "approval" || parts[4] != "actions" {
		return "", "", false
	}
	approvalID := strings.TrimSpace(parts[3])
	action := strings.TrimSpace(parts[5])
	return approvalID, action, approvalID != "" && action != ""
}

func workflowTaskLinkPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "link" || parts[1] != "workflow" || parts[2] != "task" {
		return "", false
	}
	taskID := strings.TrimSpace(parts[3])
	return taskID, taskID != ""
}

func documentLinkPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "link" || parts[1] != "document" {
		return "", false
	}
	documentID := strings.TrimSpace(parts[2])
	return documentID, documentID != ""
}

func workflowOpsApprovalPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "ops" || parts[1] != "workflow" || parts[2] != "approvals" {
		return "", "", false
	}
	return strings.TrimSpace(parts[3]), strings.TrimSpace(parts[4]), strings.TrimSpace(parts[3]) != "" && strings.TrimSpace(parts[4]) != ""
}

func workflowOpsApprovalCommunicationActionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 7 || parts[0] != "ops" || parts[1] != "workflow" || parts[2] != "approvals" || parts[4] != "communication" || parts[5] != "actions" {
		return "", "", false
	}
	return strings.TrimSpace(parts[3]), strings.TrimSpace(parts[6]), strings.TrimSpace(parts[3]) != "" && strings.TrimSpace(parts[6]) != ""
}

func approvalDeepLinkPath(approvalID string) string {
	return "/link/workflow/approval/" + url.PathEscape(strings.TrimSpace(approvalID))
}

func workflowTaskDeepLinkPath(taskID string) string {
	return "/link/workflow/task/" + url.PathEscape(strings.TrimSpace(taskID))
}

func deepLinkPrincipalMatchesApproval(p principal, approval workflow.Approval) bool {
	assigneeUserID := strings.TrimSpace(metadataString(approval.Metadata, "resolved_assignee_user_id"))
	if assigneeUserID == "" {
		return true
	}
	return assigneeUserID == principalEffectiveUserID(p)
}

func principalDeepLinkGrant(p principal) identity.DeepLinkGrant {
	if p.deepLink == nil {
		return identity.DeepLinkGrant{}
	}
	return *p.deepLink
}

func renderDeepLinkErrorPage(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` + html.EscapeString(title) + `</title><style>` + workflowApprovalPageCSS + `</style></head><body><main class="approval-page"><section class="approval-card"><span class="approval-badge error">Link unavailable</span><h1>` + html.EscapeString(title) + `</h1><p class="approval-copy">` + html.EscapeString(message) + `</p></section></main></body></html>`))
}

func renderWorkflowApprovalLandingPage(w http.ResponseWriter, r *http.Request, ident *identity.Service, p principal, approval workflow.Approval, record document.Record) {
	title := firstNonEmptyString(strings.TrimSpace(record.Header.Number), record.Header.ID)
	stage := firstNonEmptyString(strings.TrimSpace(approval.StageKey), strings.TrimSpace(approval.WorkflowKey), "review")
	copyText := "Review the request details before choosing an action."
	grant := principalDeepLinkGrant(p)
	allowedApprove := approval.Status == "pending" && principalAllowsPermission(ident, p, "document.approve", record.Header.LocationID) && deepLinkAllowsAction(grant, metadataStrings(approval.Metadata, "link_allowed_actions"), "approve")
	allowedReject := approval.Status == "pending" && principalAllowsPermission(ident, p, "document.reject", record.Header.LocationID) && deepLinkAllowsAction(grant, metadataStrings(approval.Metadata, "link_allowed_actions"), "reject")
	payload := map[string]any{
		"approval_id":         approval.ID,
		"document_id":         record.Header.ID,
		"document_number":     record.Header.Number,
		"document_type":       record.Header.Type,
		"document_status":     record.Header.Status,
		"workflow_key":        approval.WorkflowKey,
		"workflow_stage":      approval.StageKey,
		"requested_by":        approval.RequestedBy,
		"requested_at":        approval.RequestedAt,
		"review_only":         grant.ReviewOnly,
		"require_step_up":     grant.RequireStepUp,
		"step_up_verified":    !grant.RequireStepUp || p.stepUpVerified,
		"allow_approve":       allowedApprove,
		"allow_reject":        allowedReject,
		"open_workspace_path": fmt.Sprintf("/ui#/documents/detail?document_id=%s", url.QueryEscape(record.Header.ID)),
	}
	payloadJSON, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Approval ` + html.EscapeString(title) + `</title><style>` + workflowApprovalPageCSS + `</style></head><body><main class="approval-page"><section class="approval-card"><span class="approval-badge">Workflow approval</span><h1>` + html.EscapeString(title) + `</h1><p class="approval-copy">` + html.EscapeString(copyText) + `</p><dl class="approval-meta"><div><dt>Approval</dt><dd>` + html.EscapeString(approval.ID) + `</dd></div><div><dt>Stage</dt><dd>` + html.EscapeString(stage) + `</dd></div><div><dt>Status</dt><dd>` + html.EscapeString(record.Header.Status) + `</dd></div><div><dt>Requested by</dt><dd>` + html.EscapeString(firstNonEmptyString(approval.RequestedBy, "unknown")) + `</dd></div></dl><div id="approval-status" class="approval-status"></div><div id="approval-step-up" class="approval-step-up"><label>Confirm your password to continue<input id="approval-step-up-password" type="password" name="step_up_password" autocomplete="current-password"></label><button type="button" id="approval-step-up-submit" class="secondary">Verify</button></div><div class="approval-actions">` + renderApprovalActionButton("approve", "Approve", allowedApprove) + renderApprovalActionButton("reject", "Reject", allowedReject) + `<a class="approval-link" href="/ui#/documents/detail?document_id=` + url.QueryEscape(record.Header.ID) + `">Open full context</a></div></section></main><script>window.__ORBYTE_APPROVAL__=` + string(payloadJSON) + `;` + workflowApprovalPageJS + `</script></body></html>`))
}

func renderApprovalActionButton(action, label string, enabled bool) string {
	disabled := ""
	if !enabled {
		disabled = ` disabled aria-disabled="true"`
	}
	return `<button type="button" class="approval-button approval-button-` + html.EscapeString(action) + `" data-action="` + html.EscapeString(action) + `"` + disabled + `>` + html.EscapeString(label) + `</button>`
}

func deepLinkAllowsAction(grant identity.DeepLinkGrant, fallback []string, action string) bool {
	action = strings.TrimSpace(action)
	if action == "" {
		return false
	}
	if grant.ReviewOnly {
		return false
	}
	allowedActions := append([]string(nil), grant.AllowedActions...)
	if len(allowedActions) == 0 {
		allowedActions = append([]string(nil), fallback...)
	}
	if len(allowedActions) == 0 {
		return grant.ID == ""
	}
	for _, item := range allowedActions {
		if strings.TrimSpace(item) == action {
			return true
		}
	}
	return false
}

func activeApprovalGrantForCommunication(ident *identity.Service, approval workflow.Approval) (identity.DeepLinkGrant, bool) {
	if ident == nil {
		return identity.DeepLinkGrant{}, false
	}
	now := time.Now().UTC()
	userID := strings.TrimSpace(metadataString(approval.Metadata, "resolved_assignee_user_id"))
	if userID != "" {
		if grant, ok := ident.FindActiveDeepLinkGrant("workflow_approval", approval.ID, userID, now); ok {
			return grant, true
		}
	}
	for _, item := range ident.DeepLinkGrants() {
		if item.TargetType != "workflow_approval" || item.TargetID != approval.ID {
			continue
		}
		if item.Status != "pending" && item.Status != "active" {
			continue
		}
		if !item.ExpiresAt.IsZero() && !now.Before(item.ExpiresAt) {
			continue
		}
		return item, true
	}
	return identity.DeepLinkGrant{}, false
}

func workflowTaskByID(workflowSvc *workflow.Service, taskID string) (workflow.Task, bool) {
	if workflowSvc == nil {
		return workflow.Task{}, false
	}
	for _, item := range workflowSvc.ListTasks() {
		if item.ID == strings.TrimSpace(taskID) {
			return item, true
		}
	}
	return workflow.Task{}, false
}

func handleWorkflowApprovalStepUp(w http.ResponseWriter, r *http.Request, ident *identity.Service, workflowSvc *workflow.Service, auditSvc *audit.Service, approvalID string, tokenManager *identity.TokenManager) {
	p, ok := requireInteractivePrincipal(w, r)
	if !ok {
		return
	}
	if p.authMethod != "link" || p.deepLink == nil {
		respondError(w, shared.Forbidden("step-up verification is only available for deep-link sessions"))
		return
	}
	approval, ok := workflowSvc.Approval(approvalID)
	if !ok {
		respondError(w, shared.NotFound("workflow approval not found"))
		return
	}
	if !deepLinkPrincipalMatchesApproval(p, approval) {
		respondError(w, shared.Forbidden("approval link is not assigned to the current user"))
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.Validation("invalid step-up payload"))
		return
	}
	user, ok := ident.FindUser(principalEffectiveUserID(p))
	if !ok {
		respondError(w, shared.Forbidden("current user not found"))
		return
	}
	if _, err := ident.AuthenticatePassword(user.Username, strings.TrimSpace(req.Password), p.currentLocationID, map[string]any{"source": "deep_link_step_up"}, time.Minute); err != nil {
		respondError(w, shared.Forbidden("step-up verification failed"))
		return
	}
	stepUpToken, err := tokenManager.IssueDeepLinkStepUpToken(*p.deepLink, 10*time.Minute)
	if err != nil {
		respondError(w, err)
		return
	}
	http.SetCookie(w, buildDeepLinkStepUpCookie(stepUpToken, time.Now().UTC().Add(10*time.Minute)))
	recordDeepLinkAuditEvent(auditSvc, r, *p.deepLink, "link.step_up", map[string]any{"approval_id": approvalID})
	respondJSON(w, http.StatusOK, map[string]any{"status": "verified"})
}

func renderWorkflowTaskLandingPage(w http.ResponseWriter, task workflow.Task, record document.Record) {
	title := firstNonEmptyString(strings.TrimSpace(record.Header.Number), record.Header.ID)
	copyText := "Open the linked document to continue working on this task."
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Task ` + html.EscapeString(title) + `</title><style>` + workflowApprovalPageCSS + `</style></head><body><main class="approval-page"><section class="approval-card"><span class="approval-badge warning">Workflow task</span><h1>` + html.EscapeString(title) + `</h1><p class="approval-copy">` + html.EscapeString(copyText) + `</p><dl class="approval-meta"><div><dt>Task</dt><dd>` + html.EscapeString(task.ID) + `</dd></div><div><dt>Workflow</dt><dd>` + html.EscapeString(firstNonEmptyString(task.WorkflowKey, task.TaskType, "task")) + `</dd></div><div><dt>Status</dt><dd>` + html.EscapeString(record.Header.Status) + `</dd></div><div><dt>Assigned</dt><dd>` + html.EscapeString(firstNonEmptyString(task.AssigneeUserID, task.AssigneeRoleKey, "queue")) + `</dd></div></dl><div class="approval-actions"><a class="approval-link" href="/ui#/documents/detail?document_id=` + url.QueryEscape(record.Header.ID) + `&work_item_kind=task&work_item_id=` + url.QueryEscape(task.ID) + `">Open full context</a></div></section></main></body></html>`))
}

func workflowApprovalCommunicationPayload(r *http.Request, tokenManager *identity.TokenManager, ident *identity.Service, docs *document.Service, workflowSvc *workflow.Service, approvalID string) (map[string]any, error) {
	approval, ok := workflowSvc.Approval(approvalID)
	if !ok {
		return nil, shared.NotFound("workflow approval not found")
	}
	record, err := docs.Get(approval.TargetID)
	if err != nil {
		return nil, err
	}
	grant, _ := activeApprovalGrantForCommunication(ident, approval)
	recipientUserID := strings.TrimSpace(metadataString(approval.Metadata, "resolved_assignee_user_id"))
	if recipientUserID == "" {
		recipientUserID = strings.TrimSpace(grant.UserID)
	}
	payload := map[string]any{
		"approval": approval,
		"document": map[string]any{
			"id":              record.Header.ID,
			"type":            record.Header.Type,
			"status":          record.Header.Status,
			"number":          record.Header.Number,
			"organization_id": record.Header.OrganizationID,
			"location_id":     record.Header.LocationID,
		},
		"recipient_user_id": recipientUserID,
		"title":             firstNonEmptyString(strings.TrimSpace(grant.Title), strings.TrimSpace(record.Header.Number), record.Header.ID),
		"message":           firstNonEmptyString(strings.TrimSpace(grant.Message), strings.TrimSpace(approval.StageKey), "Workflow approval pending"),
		"deep_link_url":     absoluteURLForPath(r, approvalDeepLinkPath(approval.ID)),
		"allowed_actions":   metadataStrings(approval.Metadata, "link_allowed_actions"),
	}
	if grant.ID != "" {
		token, err := tokenManager.IssueDeepLinkToken(grant)
		if err == nil {
			payload["action_link_url"] = absoluteURLForPath(r, approvalDeepLinkPath(approval.ID)+"?token="+url.QueryEscape(token))
			payload["expires_at"] = grant.ExpiresAt
			payload["grant"] = grant
		}
	}
	return payload, nil
}

func reissueWorkflowApprovalGrant(ident *identity.Service, approval workflow.Approval, record document.Record, now time.Time) (identity.DeepLinkGrant, error) {
	if ident == nil {
		return identity.DeepLinkGrant{}, shared.Validation("identity service is not configured")
	}
	userID := strings.TrimSpace(metadataString(approval.Metadata, "resolved_assignee_user_id"))
	if userID == "" {
		for _, item := range ident.DeepLinkGrants() {
			if item.TargetType == "workflow_approval" && item.TargetID == approval.ID && strings.TrimSpace(item.UserID) != "" {
				userID = strings.TrimSpace(item.UserID)
				break
			}
		}
	}
	if userID == "" {
		return identity.DeepLinkGrant{}, shared.Validation("workflow approval assignee is required for tokenized reissue")
	}
	ttlSeconds := metadataInt(approval.Metadata, "link_ttl_seconds", 15*60)
	if ttlSeconds <= 0 {
		ttlSeconds = 15 * 60
	}
	reviewOnly := metadataBool(approval.Metadata, "link_review_only")
	requireStepUp := metadataBool(approval.Metadata, "link_require_step_up")
	allowedActions := uniqueStrings(metadataStrings(approval.Metadata, "link_allowed_actions"))
	allowedPermissions := []string{"document.read"}
	if !reviewOnly {
		for _, item := range allowedActions {
			switch strings.TrimSpace(item) {
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
	return ident.SaveDeepLinkGrant(identity.DeepLinkGrant{
		Kind:                  "workflow_approval",
		UserID:                userID,
		Status:                "pending",
		TargetType:            "workflow_approval",
		TargetID:              approval.ID,
		LocationID:            record.Header.LocationID,
		AllowedPermissionKeys: uniqueStrings(allowedPermissions),
		AllowedActions:        allowedActions,
		ReviewOnly:            reviewOnly,
		RequireStepUp:         requireStepUp,
		OneTime:               true,
		Title:                 firstNonEmptyString(strings.TrimSpace(record.Header.Number), strings.TrimSpace(record.Header.Type), record.Header.ID),
		Message:               firstNonEmptyString(strings.TrimSpace(approval.StageKey), strings.TrimSpace(approval.WorkflowKey), "workflow approval"),
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
}

func absoluteURLForPath(r *http.Request, path string) string {
	if r == nil {
		return path
	}
	scheme := "http"
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") || r.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host + path
}

func humanizeDeepLinkError(err error) string {
	var platformErr shared.Error
	if errors.As(err, &platformErr) {
		return platformErr.Message
	}
	return "The approval link could not be activated."
}

func recordDeepLinkAuditEvent(auditSvc *audit.Service, r *http.Request, grant identity.DeepLinkGrant, action string, metadata map[string]any) {
	if auditSvc == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["grant_id"] = grant.ID
	metadata["target_type"] = grant.TargetType
	metadata["target_id"] = grant.TargetID
	event := audit.Event{
		ID:         fmt.Sprintf("audit:%s:%d", action, time.Now().UTC().UnixNano()),
		Action:     action,
		TargetType: "deep_link_grant",
		TargetID:   grant.ID,
		ActorID:    grant.UserID,
		ActorKind:  "user",
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	}
	if r != nil {
		event.CorrelationID = strings.TrimSpace(logging.CorrelationID(r.Context()))
	}
	_ = auditSvc.Record(event)
}

const workflowApprovalPageCSS = `
body { margin: 0; font-family: ui-sans-serif, system-ui, sans-serif; background: #f3f5f8; color: #172033; }
.approval-page { min-height: 100vh; display: grid; place-items: center; padding: 24px; }
.approval-card { width: min(720px, 100%); background: #fff; border-radius: 20px; box-shadow: 0 20px 48px rgba(17,24,39,.08); padding: 32px; }
.approval-badge { display: inline-flex; align-items: center; border-radius: 999px; background: #e9f4ff; color: #1359a7; padding: 6px 12px; font-size: 12px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; }
.approval-badge.warning { background: #fff5d9; color: #8d5d00; }
.approval-badge.error { background: #fde7e7; color: #a32626; }
h1 { margin: 16px 0 8px; font-size: 32px; line-height: 1.1; }
.approval-copy { margin: 0 0 20px; color: #526072; line-height: 1.6; }
.approval-copy.subtle { margin-top: 16px; font-size: 14px; }
.approval-meta { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px,1fr)); gap: 16px; margin: 0 0 28px; }
.approval-meta div { background: #f6f8fb; border-radius: 14px; padding: 14px 16px; }
.approval-meta dt { font-size: 12px; font-weight: 700; color: #66748a; text-transform: uppercase; letter-spacing: .03em; margin-bottom: 6px; }
.approval-meta dd { margin: 0; font-size: 15px; color: #172033; }
.approval-actions { display: flex; flex-wrap: wrap; gap: 12px; }
.approval-button, .approval-link { border: 0; border-radius: 12px; padding: 12px 18px; font-size: 15px; font-weight: 700; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; justify-content: center; }
.approval-button-approve { background: #14532d; color: #fff; }
.approval-button-reject { background: #b42318; color: #fff; }
.approval-button[disabled] { cursor: not-allowed; opacity: .45; }
.approval-link { background: #e8edf5; color: #172033; }
.approval-status { min-height: 24px; margin: 0 0 16px; font-size: 14px; color: #526072; }
.approval-status.success { color: #14532d; }
.approval-status.error { color: #b42318; }
.approval-step-up { display: grid; gap: 12px; margin: 0 0 16px; padding: 16px; border-radius: 14px; background: #f6f8fb; }
.approval-step-up.hidden { display: none; }
.approval-step-up label { display: grid; gap: 6px; font-size: 14px; color: #172033; }
.approval-step-up input { border: 1px solid #c6d0df; border-radius: 10px; padding: 12px 14px; font-size: 15px; }
`

const workflowApprovalPageJS = `
(function () {
  const state = window.__ORBYTE_APPROVAL__ || {};
  const statusEl = document.getElementById('approval-status');
  const buttons = Array.from(document.querySelectorAll('[data-action]'));
  const stepUpPanel = document.getElementById('approval-step-up');
  const stepUpInput = document.getElementById('approval-step-up-password');
  const stepUpButton = document.getElementById('approval-step-up-submit');
  function stepUpSatisfied() {
    return !state.require_step_up || !!state.step_up_verified;
  }
  function setStatus(text, kind) {
    if (!statusEl) return;
    statusEl.textContent = text || '';
    statusEl.className = 'approval-status' + (kind ? ' ' + kind : '');
  }
  function setBusy(busy) {
    buttons.forEach(function (button) { button.disabled = busy || button.getAttribute('aria-disabled') === 'true'; });
    if (stepUpButton) stepUpButton.disabled = !!busy;
  }
  function syncStepUpUI() {
    if (stepUpPanel) stepUpPanel.classList.toggle('hidden', stepUpSatisfied());
    buttons.forEach(function (button) {
      const forcedDisabled = button.getAttribute('aria-disabled') === 'true';
      button.disabled = forcedDisabled || !stepUpSatisfied();
    });
  }
  async function submitStepUp() {
    if (!stepUpInput || !stepUpInput.value) {
      setStatus('Password is required for step-up verification.', 'error');
      return;
    }
    setBusy(true);
    setStatus('Verifying password…', '');
    try {
      const response = await fetch('/link/workflow/approval/' + encodeURIComponent(state.approval_id) + '/actions/step-up', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'Accept': 'application/json'},
        credentials: 'same-origin',
        body: JSON.stringify({password: stepUpInput.value})
      });
      const payload = await response.json();
      if (!response.ok) {
        const message = payload && payload.error && payload.error.message ? payload.error.message : 'Step-up verification failed';
        throw new Error(message);
      }
      state.step_up_verified = true;
      stepUpInput.value = '';
      syncStepUpUI();
      setStatus('Step-up verified. You can continue with the approval action.', 'success');
    } catch (error) {
      setStatus(error && error.message ? error.message : 'Step-up verification failed', 'error');
    } finally {
      setBusy(false);
    }
  }
  if (stepUpButton) stepUpButton.addEventListener('click', submitStepUp);
  if (stepUpInput) stepUpInput.addEventListener('keydown', function (event) { if (event.key === 'Enter') { event.preventDefault(); submitStepUp(); } });
  syncStepUpUI();
  buttons.forEach(function (button) {
    button.addEventListener('click', async function () {
      if (!stepUpSatisfied()) {
        setStatus('Step-up verification required before this action.', 'error');
        return;
      }
      const action = button.getAttribute('data-action');
      if (!action) return;
      setBusy(true);
      setStatus('Submitting ' + action + '...', '');
      try {
        const response = await fetch('/link/workflow/approval/' + encodeURIComponent(state.approval_id) + '/actions/' + encodeURIComponent(action), {
          method: 'POST',
          headers: { 'Accept': 'application/json' },
          credentials: 'same-origin'
        });
        const payload = await response.json();
        if (!response.ok) {
          const message = payload && payload.error && payload.error.message ? payload.error.message : 'Action failed';
          throw new Error(message);
        }
        setStatus('Action completed. Opening the latest document state…', 'success');
        buttons.forEach(function (item) { item.disabled = true; });
        if (payload.open_path) {
          window.setTimeout(function () { window.location.assign(payload.open_path); }, 400);
        }
      } catch (error) {
        setStatus(error && error.message ? error.message : 'Action failed', 'error');
        setBusy(false);
      }
    });
  });
}());
`

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataStrings(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		items := make([]string, 0, len(values))
		for _, item := range values {
			item = strings.TrimSpace(item)
			if item != "" {
				items = append(items, item)
			}
		}
		return items
	case []any:
		items := make([]string, 0, len(values))
		for _, item := range values {
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dispatchWorkflowApprovalEmail(payload map[string]any, recipient string) (map[string]any, error) {
	subject := "Workflow approval pending"
	if title, _ := payload["title"].(string); strings.TrimSpace(title) != "" {
		subject = "Approval needed: " + strings.TrimSpace(title)
	}
	messageBody := buildWorkflowApprovalEmailBody(payload)
	delivery, err := communication.SendPlainTextEmail(subject, messageBody, recipient)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"channel":   delivery.Channel,
		"mode":      delivery.Mode,
		"recipient": delivery.Recipient,
		"path":      delivery.Path,
		"address":   delivery.Address,
	}, nil
}

func buildWorkflowApprovalEmailBody(payload map[string]any) string {
	title, _ := payload["title"].(string)
	message, _ := payload["message"].(string)
	actionLink, _ := payload["action_link_url"].(string)
	deepLink, _ := payload["deep_link_url"].(string)
	var body strings.Builder
	if strings.TrimSpace(title) != "" {
		body.WriteString(title)
		body.WriteString("\n\n")
	}
	if strings.TrimSpace(message) != "" {
		body.WriteString(message)
		body.WriteString("\n\n")
	}
	if strings.TrimSpace(actionLink) != "" {
		body.WriteString("Open approval link:\n")
		body.WriteString(actionLink)
		body.WriteString("\n\n")
	}
	if strings.TrimSpace(deepLink) != "" {
		body.WriteString("Open in app:\n")
		body.WriteString(deepLink)
		body.WriteString("\n")
	}
	return body.String()
}
