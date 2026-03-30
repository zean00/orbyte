package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func buildACPBootstrap(service *acp.Service) map[string]any {
	if service == nil {
		return map[string]any{"enabled": false, "providers": []any{}, "contract": map[string]any{}}
	}
	return map[string]any{
		"enabled":   service.Enabled(),
		"providers": service.Providers(),
		"contract":  service.ContractMetadata(),
	}
}

func registerACPRoutes(mux *http.ServeMux, ident *identity.Service, auditSvc *audit.Service, service *acp.Service) {
	mux.HandleFunc("GET /agent/api/providers", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "agent.workspace.use", "", "agent.workspace.use"); !ok {
			return
		}
		if service == nil {
			respondJSON(w, http.StatusOK, map[string]any{"enabled": false, "items": []any{}})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"enabled": service.Enabled(), "items": service.Providers()})
	})

	mux.HandleFunc("GET /agent/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "agent.workspace.use", "", "agent.workspace.use")
		if !ok {
			return
		}
		if service == nil {
			respondJSON(w, http.StatusOK, map[string]any{"items": []any{}})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": service.ListSessions(p.userID)})
	})

	mux.HandleFunc("POST /agent/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "agent.workspace.use", "", "agent.workspace.use")
		if !ok {
			return
		}
		if service == nil {
			respondError(w, shared.NotFound("acp service is not configured"))
			return
		}
		var req acp.StartSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		req.UserID = p.userID
		item, err := service.StartSession(req)
		if err != nil {
			respondError(w, err)
			return
		}
		recordAudit(auditSvc, audit.Event{
			ID:         fmt.Sprintf("audit:acp:session:start:%s", item.ID),
			Action:     "acp.session.start",
			TargetType: "acp_session",
			TargetID:   item.ID,
			ActorID:    p.userID,
			OccurredAt: time.Now().UTC(),
			Metadata: map[string]any{
				"provider_key":     item.ProviderKey,
				"shell":            item.Shell,
				"route_path":       item.RoutePath,
				"contract_version": "2026-03-23",
			},
		})
		respondJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET /agent/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "agent.workspace.use", "", "agent.workspace.use")
		if !ok {
			return
		}
		sessionID, tail := agentSessionPath(r.URL.Path)
		if sessionID == "" {
			respondError(w, shared.NotFound("acp session not found"))
			return
		}
		if tail == "/events" {
			registerACPStream(w, r, p.userID, service, sessionID)
			return
		}
		if tail == "" {
			item, ok := service.GetSession(sessionID)
			if !ok || item.UserID != p.userID {
				respondError(w, shared.NotFound("acp session not found"))
				return
			}
			respondJSON(w, http.StatusOK, item)
			return
		}
		respondError(w, shared.NotFound("acp session route not found"))
	})

	mux.HandleFunc("POST /agent/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "agent.workspace.use", "", "agent.workspace.use")
		if !ok {
			return
		}
		sessionID, tail := agentSessionPath(r.URL.Path)
		if sessionID == "" {
			respondError(w, shared.NotFound("acp session not found"))
			return
		}
		item, found := service.GetSession(sessionID)
		if !found || item.UserID != p.userID {
			respondError(w, shared.NotFound("acp session not found"))
			return
		}
		switch {
		case tail == "/prompt":
			var req acp.PromptRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid request body"))
				return
			}
			updated, err := service.SendPrompt(sessionID, req)
			if err != nil {
				respondError(w, err)
				return
			}
			recordAudit(auditSvc, audit.Event{
				ID:         fmt.Sprintf("audit:acp:prompt:%s:%d", sessionID, time.Now().UTC().UnixNano()),
				Action:     "acp.session.prompt",
				TargetType: "acp_session",
				TargetID:   sessionID,
				ActorID:    p.userID,
				OccurredAt: time.Now().UTC(),
				Metadata:   map[string]any{"shell": item.Shell, "route_path": item.RoutePath, "provider_key": item.ProviderKey, "contract_version": "2026-03-23"},
			})
			respondJSON(w, http.StatusOK, updated)
		case strings.HasSuffix(tail, "/approve"):
			approvalID := strings.TrimSuffix(strings.TrimPrefix(tail, "/approvals/"), "/approve")
			approval, err := service.Approve(sessionID, approvalID)
			if err != nil {
				respondError(w, err)
				return
			}
			recordAudit(auditSvc, audit.Event{
				ID:         fmt.Sprintf("audit:acp:approval:approve:%s:%s:%d", sessionID, approvalID, time.Now().UTC().UnixNano()),
				Action:     "acp.session.approval.approve",
				TargetType: "acp_approval",
				TargetID:   approvalID,
				ActorID:    p.userID,
				OccurredAt: time.Now().UTC(),
				Metadata:   map[string]any{"session_id": sessionID, "provider_key": item.ProviderKey, "contract_version": "2026-03-23"},
			})
			respondJSON(w, http.StatusOK, approval)
		case strings.HasSuffix(tail, "/reject"):
			approvalID := strings.TrimSuffix(strings.TrimPrefix(tail, "/approvals/"), "/reject")
			approval, err := service.Reject(sessionID, approvalID)
			if err != nil {
				respondError(w, err)
				return
			}
			recordAudit(auditSvc, audit.Event{
				ID:         fmt.Sprintf("audit:acp:approval:reject:%s:%s:%d", sessionID, approvalID, time.Now().UTC().UnixNano()),
				Action:     "acp.session.approval.reject",
				TargetType: "acp_approval",
				TargetID:   approvalID,
				ActorID:    p.userID,
				OccurredAt: time.Now().UTC(),
				Metadata:   map[string]any{"session_id": sessionID, "provider_key": item.ProviderKey, "contract_version": "2026-03-23"},
			})
			respondJSON(w, http.StatusOK, approval)
		default:
			respondError(w, shared.NotFound("acp session action not found"))
		}
	})
}

func registerACPStream(w http.ResponseWriter, r *http.Request, userID string, service *acp.Service, sessionID string) {
	if service == nil {
		respondError(w, shared.NotFound("acp service is not configured"))
		return
	}
	item, ok := service.GetSession(sessionID)
	if !ok || item.UserID != userID {
		respondError(w, shared.NotFound("acp session not found"))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, shared.Conflict("streaming is not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	events, unsubscribe := service.Subscribe(sessionID)
	defer unsubscribe()
	for _, event := range item.Trace {
		if err := writeACPEvent(w, event); err != nil {
			return
		}
		flusher.Flush()
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := writeACPEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		case tick := <-heartbeat.C:
			if _, err := fmt.Fprintf(w, "event: heartbeat\ndata: {\"time\":%q}\n\n", tick.UTC().Format(time.RFC3339)); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeACPEvent(w http.ResponseWriter, event acp.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Kind, string(body))
	return err
}

func agentSessionPath(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/agent/api/sessions/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", ""
	}
	sessionID := parts[0]
	if len(parts) == 1 {
		return sessionID, ""
	}
	return sessionID, "/" + strings.Join(parts[1:], "/")
}
