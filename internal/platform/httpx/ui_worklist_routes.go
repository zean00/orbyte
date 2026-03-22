package httpx

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func registerUIWorklistRoutes(mux *http.ServeMux, ident *identity.Service, docs *document.Service, workflowSvc *workflow.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service) {
	mux.HandleFunc("GET /ui/data/analytics/snapshot", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("analytics read is not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, analyticsSvc.Snapshot())
	})

	mux.HandleFunc("GET /ui/data/worklist/tasks", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("worklist is not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": filterWorkflowTasksForUI(docs, workflowSvc.ListTasks(), p.userID, r)})
	})

	mux.HandleFunc("GET /ui/data/worklist/approvals", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("worklist is not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": filterWorkflowApprovalsForUI(docs, workflowSvc.ListApprovals(), p.userID, r)})
	})

	mux.HandleFunc("GET /ui/data/worklist/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("worklist is not allowed"))
			return
		}
		tasks := filterWorkflowTasksForUI(docs, workflowSvc.ListTasks(), p.userID, r)
		approvals := filterWorkflowApprovalsForUI(docs, workflowSvc.ListApprovals(), p.userID, r)
		now := time.Now().UTC()
		overdueTasks := 0
		overdueApprovals := 0
		for _, item := range tasks {
			if item.DueAt != "" && item.Status == "open" {
				if dueAt, err := time.Parse(time.RFC3339Nano, item.DueAt); err == nil && dueAt.Before(now) {
					overdueTasks++
				}
			}
		}
		for _, item := range approvals {
			if item.DueAt != "" && item.Status == "pending" {
				if dueAt, err := time.Parse(time.RFC3339Nano, item.DueAt); err == nil && dueAt.Before(now) {
					overdueApprovals++
				}
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"tasks": map[string]any{
				"total":     len(tasks),
				"open":      countWorkflowItems(tasks, "open"),
				"overdue":   overdueTasks,
				"mine":      countWorklistMine(tasks),
				"by_status": countWorkflowStatuses(tasks),
				"workflows": countWorklistTaskWorkflowKeys(tasks),
			},
			"approvals": map[string]any{
				"total":           len(approvals),
				"pending":         countWorkflowItems(approvals, "pending"),
				"overdue":         overdueApprovals,
				"requested_by_me": countApprovalsRequestedByMe(approvals, p.userID),
				"by_status":       countWorkflowStatuses(approvals),
				"workflows":       countWorklistApprovalWorkflowKeys(approvals),
			},
		})
	})

	mux.HandleFunc("GET /ui/data/worklist/context", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.read"}) {
			respondError(w, shared.Forbidden("worklist context is not allowed"))
			return
		}
		targetType := strings.TrimSpace(r.URL.Query().Get("target_type"))
		targetID := strings.TrimSpace(r.URL.Query().Get("target_id"))
		workItemKind := strings.TrimSpace(r.URL.Query().Get("work_item_kind"))
		workItemID := strings.TrimSpace(r.URL.Query().Get("work_item_id"))
		if targetType == "" || targetID == "" {
			respondError(w, shared.Validation("target_type and target_id are required"))
			return
		}
		filteredTaskReq := r.Clone(r.Context())
		filteredTaskReq.URL.RawQuery = url.Values{"target_id": []string{targetID}}.Encode()
		tasks := filterWorkflowTasksForUI(docs, workflowSvc.ListTasks(), p.userID, filteredTaskReq)
		approvals := filterWorkflowApprovalsForUI(docs, workflowSvc.ListApprovals(), p.userID, filteredTaskReq)
		history := workflowSvc.ListHistory(targetType, targetID)
		response := map[string]any{
			"tasks":     tasks,
			"approvals": approvals,
			"history":   history,
		}
		switch workItemKind {
		case "task":
			for _, item := range tasks {
				if item.ID == workItemID {
					response["current_task"] = item
					break
				}
			}
		case "approval":
			for _, item := range approvals {
				if item.ID == workItemID {
					response["current_approval"] = item
					break
				}
			}
		}
		respondJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /ui/data/monitoring/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"monitoring.read"}) {
			respondError(w, shared.Forbidden("monitoring read is not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, monitoringSvc.Summary())
	})

}
