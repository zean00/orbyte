package acp

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/shared"
)

type Service struct {
	config     *config.Service
	mu         sync.RWMutex
	sessions   map[string]*Session
	runtimes   map[string]*sessionRuntime
	streams    map[string]map[chan Event]struct{}
	eventCount int64
}

type sessionRuntime struct {
	client *acpClient
	cancel context.CancelFunc
}

func NewService(cfg *config.Service) *Service {
	return &Service{
		config:   cfg,
		sessions: map[string]*Session{},
		runtimes: map[string]*sessionRuntime{},
		streams:  map[string]map[chan Event]struct{}{},
	}
}

func (s *Service) Providers() []ProviderInfo {
	providers, err := s.providerConfigs()
	if err != nil {
		return []ProviderInfo{{
			Key:             "invalid",
			Name:            "Invalid ACP Configuration",
			Available:       false,
			ContractVersion: "2026-03-23",
			Stability:       "experimental",
			SessionLifecycle: []string{
				"starting",
				"ready",
				"running",
				"error",
			},
			Error: err.Error(),
		}}
	}
	items := make([]ProviderInfo, 0, len(providers))
	for _, provider := range providers {
		items = append(items, ProviderInfo{
			Key:               provider.Key,
			Name:              provider.Name,
			Description:       provider.Description,
			Available:         strings.TrimSpace(provider.Command) != "",
			ContractVersion:   "2026-03-23",
			Stability:         "experimental",
			SupportsApprovals: true,
			SupportsStreaming: true,
			SessionLifecycle: []string{
				"starting",
				"ready",
				"running",
				"error",
			},
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) ContractMetadata() map[string]any {
	return map[string]any{
		"contract_version": "2026-03-23",
		"stability":        "experimental",
		"session_lifecycle": []string{
			"starting",
			"ready",
			"running",
			"error",
		},
		"approval_lifecycle": []string{
			"pending",
			"approved",
			"rejected",
		},
		"supports_streaming": true,
		"supports_approvals": true,
	}
}

func (s *Service) Enabled() bool {
	if s == nil || s.config == nil {
		return false
	}
	value, ok := s.config.Resolve("platform.acp", "", "")
	if !ok {
		return false
	}
	enabled, _ := value.Value["enabled"].(bool)
	return enabled
}

func (s *Service) StartSession(req StartSessionRequest) (Session, error) {
	if !s.Enabled() {
		return Session{}, shared.Conflict("acp is not enabled")
	}
	providers, err := s.providerConfigs()
	if err != nil {
		return Session{}, err
	}
	var provider Provider
	found := false
	for _, item := range providers {
		if item.Key == req.ProviderKey {
			provider = item
			found = true
			break
		}
	}
	if !found {
		return Session{}, shared.NotFound("acp provider not found")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return Session{}, shared.Validation("user_id is required")
	}
	if strings.TrimSpace(req.Shell) == "" {
		req.Shell = "workspace"
	}
	if strings.TrimSpace(req.WorkingDir) == "" {
		if wd, err := os.Getwd(); err == nil {
			req.WorkingDir = wd
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	sessionID := shared.NewID("acp-session")
	session := &Session{
		ID:            sessionID,
		ProviderKey:   provider.Key,
		ProviderName:  provider.Name,
		UserID:        req.UserID,
		Shell:         req.Shell,
		RoutePath:     strings.TrimSpace(req.RoutePath),
		Title:         strings.TrimSpace(req.Title),
		Status:        "starting",
		WorkingDir:    req.WorkingDir,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		ContextBlocks: append([]ContextBlock(nil), req.ContextBlocks...),
	}
	client, err := startACPClient(ctx, provider, func(method string, params json.RawMessage) {
		s.handleNotification(sessionID, method, params)
	}, func(id int64, method string, params json.RawMessage) {
		s.handleRequest(sessionID, id, method, params)
	})
	if err != nil {
		cancel()
		return Session{}, err
	}
	initResp, err := client.initialize()
	if err != nil {
		cancel()
		_ = client.close()
		return Session{}, err
	}
	remoteSessionID, err := client.newSession(req.WorkingDir)
	if err != nil {
		cancel()
		_ = client.close()
		return Session{}, err
	}
	session.Status = "ready"
	session.RemoteSession = remoteSessionID
	session.ProviderInfo = initResp
	session.UpdatedAt = time.Now().UTC()

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.runtimes[sessionID] = &sessionRuntime{client: client, cancel: cancel}
	s.mu.Unlock()
	s.publish(sessionID, "session_started", map[string]any{"provider_key": provider.Key, "remote_session_id": remoteSessionID})
	return *cloneSession(session), nil
}

func (s *Service) ListSessions(userID string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Session, 0, len(s.sessions))
	for _, item := range s.sessions {
		if userID != "" && item.UserID != userID {
			continue
		}
		items = append(items, *cloneSession(item))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items
}

func (s *Service) GetSession(id string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.sessions[id]
	if !ok {
		return Session{}, false
	}
	return *cloneSession(item), true
}

func (s *Service) SendPrompt(sessionID string, req PromptRequest) (Session, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return Session{}, shared.Validation("content is required")
	}
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	runtime := s.runtimes[sessionID]
	if ok {
		session.TurnInProgress = true
		session.Status = "running"
		session.UpdatedAt = time.Now().UTC()
		req.ContextBlocks = mergeContextBlocks(session.ContextBlocks, req.ContextBlocks)
		msg := Message{ID: shared.NewID("msg"), Role: "user", Format: "markdown", Content: content, CreatedAt: time.Now().UTC()}
		session.Messages = append(session.Messages, msg)
	}
	s.mu.Unlock()
	if !ok || runtime == nil || runtime.client == nil {
		return Session{}, shared.NotFound("acp session not found")
	}
	s.publish(sessionID, "user_message", map[string]any{"content": content})
	if err := runtime.client.prompt(session.RemoteSession, promptBlocks(content, req.ContextBlocks)); err != nil {
		s.mu.Lock()
		if session := s.sessions[sessionID]; session != nil {
			session.TurnInProgress = false
			session.Status = "error"
			session.LastError = err.Error()
			session.UpdatedAt = time.Now().UTC()
		}
		s.mu.Unlock()
		s.publish(sessionID, "turn_failed", map[string]any{"error": err.Error()})
		return Session{}, err
	}
	s.mu.Lock()
	if session := s.sessions[sessionID]; session != nil {
		session.TurnInProgress = false
		session.Status = "ready"
		session.UpdatedAt = time.Now().UTC()
	}
	s.mu.Unlock()
	s.publish(sessionID, "turn_completed", nil)
	updated, _ := s.GetSession(sessionID)
	return updated, nil
}

func (s *Service) Approve(sessionID, approvalID string) (Approval, error) {
	return s.resolveApproval(sessionID, approvalID, "approved")
}

func (s *Service) Reject(sessionID, approvalID string) (Approval, error) {
	return s.resolveApproval(sessionID, approvalID, "rejected")
}

func (s *Service) resolveApproval(sessionID, approvalID, status string) (Approval, error) {
	s.mu.Lock()
	session := s.sessions[sessionID]
	runtime := s.runtimes[sessionID]
	if session == nil {
		s.mu.Unlock()
		return Approval{}, shared.NotFound("acp session not found")
	}
	for idx := range session.Approvals {
		if session.Approvals[idx].ID == approvalID {
			session.Approvals[idx].Status = status
			session.Approvals[idx].ResolvedAt = time.Now().UTC()
			item := session.Approvals[idx]
			s.mu.Unlock()
			var requestID int64
			switch value := item.Payload["request_id"].(type) {
			case int64:
				requestID = value
			case int:
				requestID = int64(value)
			case float64:
				requestID = int64(value)
			}
			if runtime != nil && runtime.client != nil && requestID > 0 {
				if status == "approved" {
					if err := runtime.client.respond(requestID, map[string]any{"approved": true}, nil); err != nil {
						return Approval{}, err
					}
				} else {
					if err := runtime.client.respond(requestID, nil, &rpcResponseError{Code: -32001, Message: "request rejected by user"}); err != nil {
						return Approval{}, err
					}
				}
			}
			s.publish(sessionID, "approval_"+status, map[string]any{"approval_id": approvalID})
			return item, nil
		}
	}
	s.mu.Unlock()
	return Approval{}, shared.NotFound("approval not found")
}

func (s *Service) Subscribe(sessionID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.mu.Lock()
	if s.streams[sessionID] == nil {
		s.streams[sessionID] = map[chan Event]struct{}{}
	}
	s.streams[sessionID][ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subs := s.streams[sessionID]; subs != nil {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(s.streams, sessionID)
			}
		}
	}
}

func (s *Service) Close() error {
	s.mu.Lock()
	runtimes := make([]*sessionRuntime, 0, len(s.runtimes))
	for _, item := range s.runtimes {
		runtimes = append(runtimes, item)
	}
	s.runtimes = map[string]*sessionRuntime{}
	s.mu.Unlock()
	for _, runtime := range runtimes {
		if runtime == nil {
			continue
		}
		if runtime.cancel != nil {
			runtime.cancel()
		}
		if runtime.client != nil {
			_ = runtime.client.close()
		}
	}
	return nil
}

func (s *Service) handleNotification(sessionID, method string, params json.RawMessage) {
	switch method {
	case "session/update":
		var payload struct {
			SessionID string `json:"sessionId"`
			Update    struct {
				SessionUpdate string         `json:"sessionUpdate"`
				Content       map[string]any `json:"content"`
			} `json:"update"`
		}
		if err := json.Unmarshal(params, &payload); err != nil {
			return
		}
		s.handleSessionUpdate(sessionID, payload.Update.SessionUpdate, payload.Update.Content)
	default:
		s.publish(sessionID, "notification", map[string]any{"method": method})
	}
}

func (s *Service) handleRequest(sessionID string, id int64, method string, params json.RawMessage) {
	s.mu.Lock()
	session := s.sessions[sessionID]
	var approval Approval
	if session != nil {
		payload := map[string]any{"request_id": id}
		if len(params) > 0 {
			var decoded map[string]any
			if err := json.Unmarshal(params, &decoded); err == nil && len(decoded) > 0 {
				payload["params"] = decoded
			}
		}
		approval = Approval{
			ID:          shared.NewID("approval"),
			Status:      "pending",
			Title:       "ACP Request Approval",
			Description: "Agent requested client-side action.",
			Method:      method,
			CreatedAt:   time.Now().UTC(),
			Payload:     payload,
		}
		session.Approvals = append(session.Approvals, approval)
		session.UpdatedAt = time.Now().UTC()
	}
	s.mu.Unlock()
	s.publish(sessionID, "approval_requested", map[string]any{"approval_id": approval.ID, "method": method})
}

func (s *Service) handleSessionUpdate(sessionID, updateKind string, content map[string]any) {
	s.mu.Lock()
	session := s.sessions[sessionID]
	if session == nil {
		s.mu.Unlock()
		return
	}
	session.UpdatedAt = time.Now().UTC()
	text := strings.TrimSpace(stringValue(content["text"]))
	switch updateKind {
	case "agent_message_chunk":
		appendChunkMessage(session, "assistant", text, content)
	case "user_message_chunk":
		appendChunkMessage(session, "user", text, content)
	case "plan":
		session.CurrentPlan = append(session.CurrentPlan, PlanEntry{Content: text})
	default:
		if text != "" {
			appendChunkMessage(session, "system", text, content)
		}
	}
	s.mu.Unlock()
	s.publish(sessionID, "session_update", map[string]any{"update_kind": updateKind, "content": content})
}

func appendChunkMessage(session *Session, role, text string, meta map[string]any) {
	if session == nil || text == "" {
		return
	}
	if len(session.Messages) > 0 {
		last := &session.Messages[len(session.Messages)-1]
		if last.Role == role && time.Since(last.CreatedAt) < time.Minute {
			last.Content += text
			return
		}
	}
	session.Messages = append(session.Messages, Message{
		ID:        shared.NewID("msg"),
		Role:      role,
		Format:    "markdown",
		Content:   text,
		CreatedAt: time.Now().UTC(),
		Meta:      cloneMap(meta),
	})
}

func (s *Service) publish(sessionID, kind string, payload map[string]any) {
	event := Event{
		ID:        shared.NewID("acp-event"),
		Kind:      kind,
		SessionID: sessionID,
		CreatedAt: time.Now().UTC(),
		Payload:   cloneMap(payload),
	}
	s.mu.Lock()
	if session := s.sessions[sessionID]; session != nil {
		session.Trace = append(session.Trace, event)
		session.UpdatedAt = time.Now().UTC()
	}
	subs := make([]chan Event, 0, len(s.streams[sessionID]))
	for ch := range s.streams[sessionID] {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Service) providerConfigs() ([]Provider, error) {
	if s == nil || s.config == nil {
		return nil, nil
	}
	value, ok := s.config.Resolve("platform.acp", "", "")
	if !ok {
		return nil, nil
	}
	raw := strings.TrimSpace(stringValue(value.Value["providers_json"]))
	if raw == "" {
		raw = "[]"
	}
	var items []Provider
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, shared.Validation("platform.acp providers_json is invalid")
	}
	for idx := range items {
		items[idx].Key = strings.TrimSpace(items[idx].Key)
		items[idx].Name = strings.TrimSpace(items[idx].Name)
		items[idx].Command = strings.TrimSpace(items[idx].Command)
		if items[idx].Name == "" {
			items[idx].Name = items[idx].Key
		}
	}
	return items, nil
}

func promptBlocks(content string, contextBlocks []ContextBlock) []map[string]any {
	items := []map[string]any{{"type": "text", "text": content}}
	if summary := renderContextSummary(contextBlocks); summary != "" {
		items = append(items, map[string]any{"type": "text", "text": summary})
	}
	return items
}

func renderContextSummary(blocks []ContextBlock) string {
	selected := make([]ContextBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Selected {
			selected = append(selected, block)
		}
	}
	if len(selected) == 0 {
		return ""
	}
	payload, _ := json.MarshalIndent(selected, "", "  ")
	return "Current platform context:\n```json\n" + string(payload) + "\n```"
}

func mergeContextBlocks(existing, incoming []ContextBlock) []ContextBlock {
	if len(incoming) == 0 {
		return append([]ContextBlock(nil), existing...)
	}
	return append([]ContextBlock(nil), incoming...)
}

func cloneSession(in *Session) *Session {
	if in == nil {
		return nil
	}
	out := *in
	out.Messages = append([]Message(nil), in.Messages...)
	out.ContextBlocks = append([]ContextBlock(nil), in.ContextBlocks...)
	out.Approvals = append([]Approval(nil), in.Approvals...)
	out.Artifacts = append([]Artifact(nil), in.Artifacts...)
	out.Trace = append([]Event(nil), in.Trace...)
	out.CurrentPlan = append([]PlanEntry(nil), in.CurrentPlan...)
	out.ProviderInfo = cloneMap(in.ProviderInfo)
	return &out
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func stringValue(raw any) string {
	value, _ := raw.(string)
	return strings.TrimSpace(value)
}
