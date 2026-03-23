package httpx

import (
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/notification"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func registerNotificationRoutes(mux *http.ServeMux, ident *identity.Service, notifications *notification.Service, workflows *workflow.Service, docs *document.Service) {
	if ident == nil || notifications == nil {
		return
	}
	mux.HandleFunc("GET /ui/data/notifications", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.read"}) {
			respondError(w, shared.Forbidden("notifications are not allowed"))
			return
		}
		items := notifications.List(notification.Filter{UserID: principalEffectiveUserID(p), Status: strings.TrimSpace(r.URL.Query().Get("status"))})
		respondJSON(w, http.StatusOK, map[string]any{"items": enrichNotifications(items, workflows, docs), "summary": notifications.Summary(principalEffectiveUserID(p))})
	})
	mux.HandleFunc("POST /ui/data/notifications/", func(w http.ResponseWriter, r *http.Request) {
		notificationID, action, ok := notificationActionPath(r.URL.Path, "/ui/data/notifications/")
		if !ok {
			respondError(w, shared.NotFound("notification action not found"))
			return
		}
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		now := time.Now().UTC()
		switch action {
		case "read":
			item, err := notifications.MarkRead(notificationID, principalEffectiveUserID(p), now)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, item)
		case "dismiss":
			item, err := notifications.Dismiss(notificationID, principalEffectiveUserID(p), now)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, item)
		default:
			respondError(w, shared.NotFound("notification action not found"))
		}
	})
	mux.HandleFunc("GET /admin/api/notifications", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		items := notifications.List(notification.Filter{
			UserID:     strings.TrimSpace(r.URL.Query().Get("user_id")),
			Status:     strings.TrimSpace(r.URL.Query().Get("status")),
			Category:   strings.TrimSpace(r.URL.Query().Get("category")),
			TargetType: strings.TrimSpace(r.URL.Query().Get("target_type")),
			TargetID:   strings.TrimSpace(r.URL.Query().Get("target_id")),
		})
		respondJSON(w, http.StatusOK, map[string]any{"items": enrichNotifications(items, workflows, docs)})
	})
}

func enrichNotifications(items []notification.Item, workflows *workflow.Service, docs *document.Service) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := map[string]any{
			"id":               item.ID,
			"user_id":          item.UserID,
			"category":         item.Category,
			"channel":          item.Channel,
			"status":           item.Status,
			"title":            item.Title,
			"body":             item.Body,
			"target_type":      item.TargetType,
			"target_id":        item.TargetID,
			"deep_link_path":   item.DeepLinkPath,
			"action_link_path": item.ActionLinkPath,
			"metadata":         item.Metadata,
			"created_at":       item.CreatedAt,
			"read_at":          item.ReadAt,
			"dismissed_at":     item.DismissedAt,
		}
		if item.TargetType == "workflow_approval" && workflows != nil {
			if approval, ok := workflows.Approval(item.TargetID); ok {
				payload["approval"] = approval
			}
		}
		if item.TargetType == "workflow_task" && workflows != nil {
			for _, task := range workflows.ListTasks() {
				if task.ID == item.TargetID {
					payload["task"] = task
					break
				}
			}
		}
		if docs != nil {
			documentID := metadataString(item.Metadata, "document_id")
			if documentID != "" {
				if record, err := docs.Get(documentID); err == nil {
					payload["document"] = map[string]any{
						"id":     record.Header.ID,
						"type":   record.Header.Type,
						"status": record.Header.Status,
						"number": record.Header.Number,
					}
				}
			}
		}
		out = append(out, payload)
	}
	return out
}

func notificationActionPath(path, prefix string) (string, string, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) != 3 || parts[1] != "actions" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]), strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[2]) != ""
}
