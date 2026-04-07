package acp

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/shared"
)

type Service struct {
	config          *config.Service
	instrumentation *Instrumentation
	mu              sync.RWMutex
	sessions        map[string]*Session
	runtimes        map[string]*sessionRuntime
	streams         map[string]map[chan Event]struct{}
	eventCount      int64
}

type sessionRuntime struct {
	client   *acpClient
	cancel   context.CancelFunc
	provider Provider
}

var currentModelResolver = resolveCurrentModel
var providerModelCatalogResolver = resolveProviderModelCatalog
var acpClientStarter = startACPClient
var dashboardArtifactBlockPattern = regexp.MustCompile(`(?s)<orbyte-dashboard-artifact>\s*(\{.*?\})\s*</orbyte-dashboard-artifact>`)

var sessionLifecycle = []string{
	"starting",
	"ready",
	"running",
	"awaiting_input",
	"error",
}

func NewService(cfg *config.Service, instr *Instrumentation) *Service {
	return &Service{
		config:          cfg,
		instrumentation: instr,
		sessions:        map[string]*Session{},
		runtimes:        map[string]*sessionRuntime{},
		streams:         map[string]map[chan Event]struct{}{},
	}
}

func (s *Service) Providers() []ProviderInfo {
	providers, err := s.providerConfigs()
	if err != nil {
		return []ProviderInfo{{
			Key:              "invalid",
			Name:             "Invalid ACP Configuration",
			Available:        false,
			ContractVersion:  "2026-03-23",
			Stability:        "experimental",
			SessionLifecycle: append([]string(nil), sessionLifecycle...),
			Error:            err.Error(),
		}}
	}
	items := make([]ProviderInfo, 0, len(providers))
	for _, provider := range providers {
		models, _ := providerModelCatalogResolver(provider)
		items = append(items, ProviderInfo{
			Key:                    provider.Key,
			Name:                   provider.Name,
			Description:            provider.Description,
			Available:              strings.TrimSpace(provider.Command) != "",
			ContractVersion:        "2026-03-23",
			Stability:              "experimental",
			SupportsApprovals:      true,
			SupportsStreaming:      true,
			SupportsModelListing:   len(models) > 0,
			SupportsModelSelection: providerSupportsModelSelection(provider),
			SupportsPlanUpdates:    providerSupportsPlanUpdates(provider),
			DefaultModel:           strings.TrimSpace(provider.DefaultModel),
			SessionLifecycle:       append([]string(nil), sessionLifecycle...),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) ContractMetadata() map[string]any {
	return map[string]any{
		"contract_version":  "2026-03-23",
		"stability":         "experimental",
		"session_lifecycle": append([]string(nil), sessionLifecycle...),
		"approval_lifecycle": []string{
			"pending",
			"approved",
			"rejected",
		},
		"supports_streaming":    true,
		"supports_approvals":    true,
		"supports_plan_updates": true,
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
	requestedModel := strings.TrimSpace(req.Model)
	if requestedModel != "" {
		if !providerSupportsModelSelection(provider) {
			return Session{}, shared.Validation("acp provider does not support model selection")
		}
		models, err := providerModelCatalogResolver(provider)
		if err != nil {
			return Session{}, err
		}
		if !containsSelectableModel(models, requestedModel) {
			return Session{}, shared.Validation("requested model is not available for acp provider")
		}
	}
	if strings.TrimSpace(req.UserID) == "" {
		return Session{}, shared.Validation("user_id is required")
	}
	if strings.TrimSpace(req.Shell) == "" {
		req.Shell = "workspace"
	}
	sessionID := shared.NewID("acp-session")
	if workingDir, err := resolveWorkingDir(req.Shell, req.WorkingDir, provider.Cwd, sessionID); err != nil {
		return Session{}, err
	} else {
		req.WorkingDir = workingDir
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &Session{
		ID:             sessionID,
		ProviderKey:    provider.Key,
		ProviderName:   provider.Name,
		RequestedModel: requestedModel,
		UserID:         req.UserID,
		Shell:          req.Shell,
		RoutePath:      strings.TrimSpace(req.RoutePath),
		Title:          strings.TrimSpace(req.Title),
		Status:         "starting",
		WorkingDir:     req.WorkingDir,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		ContextBlocks:  defaultContextBlocks(req.Shell, req.RoutePath, req.ContextBlocks),
	}
	client, err := acpClientStarter(ctx, provider, func(method string, params json.RawMessage) {
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
	newSession, err := client.newSession(req.WorkingDir)
	if err != nil {
		cancel()
		_ = client.close()
		return Session{}, err
	}
	if requestedModel == "" {
		requestedModel = strings.TrimSpace(provider.DefaultModel)
	}
	if requestedModel != "" {
		currentModel, err := client.setSessionModel(newSession.SessionID, requestedModel)
		if err != nil {
			cancel()
			_ = client.close()
			return Session{}, err
		}
		session.RequestedModel = requestedModel
		session.CurrentModel = strings.TrimSpace(currentModel)
	} else {
		session.CurrentModel = strings.TrimSpace(newSession.Models.CurrentModelID)
	}
	session.Status = "ready"
	session.RemoteSession = newSession.SessionID
	session.ProviderInfo = initResp
	session.UpdatedAt = time.Now().UTC()

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.runtimes[sessionID] = &sessionRuntime{client: client, cancel: cancel, provider: provider}
	s.mu.Unlock()
	if s.instrumentation != nil {
		s.instrumentation.RecordSessionStarted()
	}
	s.publish(sessionID, "session_started", map[string]any{"provider_key": provider.Key, "remote_session_id": newSession.SessionID})
	return *cloneSession(session), nil
}

func (s *Service) ProviderModels(providerKey string) ([]ModelInfo, error) {
	providers, err := s.providerConfigs()
	if err != nil {
		return nil, err
	}
	for _, provider := range providers {
		if provider.Key == strings.TrimSpace(providerKey) {
			return providerModelCatalogResolver(provider)
		}
	}
	return nil, shared.NotFound("acp provider not found")
}

func (s *Service) ListSessions(userID string) []Session {
	s.mu.RLock()
	source := make([]*Session, 0, len(s.sessions))
	for _, item := range s.sessions {
		if userID != "" && item.UserID != userID {
			continue
		}
		source = append(source, item)
	}
	s.mu.RUnlock()
	items := make([]Session, 0, len(source))
	for _, item := range source {
		if item != nil && item.CurrentModel == "" && item.RemoteSession != "" {
			s.syncCurrentModel(item.ID)
		}
		if current, ok := s.GetSession(item.ID); ok {
			items = append(items, current)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items
}

func (s *Service) GetSession(id string) (Session, bool) {
	s.syncCurrentModel(id)
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
	mode := strings.TrimSpace(strings.ToLower(req.Mode))
	displayContent := strings.TrimSpace(req.DisplayContent)
	if displayContent == "" {
		displayContent = content
	}
	clientRequestID := strings.TrimSpace(req.ClientRequestID)
	turnID := shared.NewID("acp-turn")
	resolvedQuestionSetID := ""
	hadPendingClarification := false
	var previousPendingQuestions []ClarificationQuestion
	previousPendingQuestionSetID := ""
	previousAwaitingInputKind := ""
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	runtime := s.runtimes[sessionID]
	if ok {
		if duplicatePromptRequestID(session, clientRequestID) ||
			duplicatePromptReplay(session, displayContent, content) {
			updated := *cloneSession(session)
			s.mu.Unlock()
			return updated, nil
		}
		session.TurnInProgress = true
		session.CurrentTurnID = turnID
		session.Status = "running"
		if session.PendingQuestionSetID != "" || len(session.PendingQuestions) > 0 {
			hadPendingClarification = true
			resolvedQuestionSetID = session.PendingQuestionSetID
			previousPendingQuestions = append([]ClarificationQuestion(nil), session.PendingQuestions...)
			previousPendingQuestionSetID = session.PendingQuestionSetID
			previousAwaitingInputKind = session.AwaitingInputKind
		}
		session.UpdatedAt = time.Now().UTC()
		req.ContextBlocks = mergeContextBlocks(session.ContextBlocks, req.ContextBlocks)
		msg := Message{
			ID:        shared.NewID("msg"),
			Role:      "user",
			Format:    "markdown",
			Content:   displayContent,
			CreatedAt: time.Now().UTC(),
			Meta:      map[string]any{"turn_id": turnID},
		}
		session.Messages = append(session.Messages, msg)
		recordPromptRequestID(session, clientRequestID)
	}
	s.mu.Unlock()
	if !ok || runtime == nil || runtime.client == nil {
		return Session{}, shared.NotFound("acp session not found")
	}
	s.publish(sessionID, "turn_started", map[string]any{"turn_id": turnID})
	s.publish(sessionID, "user_message", map[string]any{"content": content, "turn_id": turnID})
	if mode != "" {
		if err := runtime.client.setSessionMode(session.RemoteSession, mode); err != nil {
			s.mu.Lock()
			if session := s.sessions[sessionID]; session != nil {
				session.TurnInProgress = false
				session.CurrentTurnID = ""
				if hadPendingClarification {
					session.Status = "awaiting_input"
					session.PendingQuestions = append([]ClarificationQuestion(nil), previousPendingQuestions...)
					session.PendingQuestionSetID = previousPendingQuestionSetID
					session.AwaitingInputKind = previousAwaitingInputKind
				} else {
					session.Status = "error"
				}
				session.LastError = err.Error()
				session.UpdatedAt = time.Now().UTC()
			}
			s.mu.Unlock()
			s.publish(sessionID, "turn_failed", map[string]any{"error": err.Error(), "turn_id": turnID})
			return Session{}, err
		}
	}
	if err := runtime.client.prompt(session.RemoteSession, promptBlocks(content, req.ContextBlocks)); err != nil {
		s.mu.Lock()
		if session := s.sessions[sessionID]; session != nil {
			session.TurnInProgress = false
			session.CurrentTurnID = ""
			if hadPendingClarification {
				session.Status = "awaiting_input"
				session.PendingQuestions = append([]ClarificationQuestion(nil), previousPendingQuestions...)
				session.PendingQuestionSetID = previousPendingQuestionSetID
				session.AwaitingInputKind = previousAwaitingInputKind
			} else {
				session.Status = "error"
			}
			session.LastError = err.Error()
			session.UpdatedAt = time.Now().UTC()
		}
		s.mu.Unlock()
		s.publish(sessionID, "turn_failed", map[string]any{"error": err.Error(), "turn_id": turnID})
		return Session{}, err
	}
	if resolvedQuestionSetID != "" {
		s.mu.Lock()
		if session := s.sessions[sessionID]; session != nil {
			session.PendingQuestions = nil
			session.PendingQuestionSetID = ""
			session.AwaitingInputKind = ""
			session.UpdatedAt = time.Now().UTC()
		}
		s.mu.Unlock()
		s.publish(sessionID, "clarification_resolved", map[string]any{
			"question_set_id": resolvedQuestionSetID,
			"turn_id":         turnID,
		})
	}
	s.mu.Lock()
	if session := s.sessions[sessionID]; session != nil {
		s.promoteDashboardArtifactsFromTurn(session, turnID)
		session.TurnInProgress = false
		session.CurrentTurnID = ""
		if questions, sourceMessageID := deriveClarificationQuestionsForTurn(session, turnID); len(questions) > 0 {
			session.Status = "awaiting_input"
			session.AwaitingInputKind = "clarification"
			session.PendingQuestionSetID = shared.NewID("clarification")
			session.PendingQuestions = questions
			session.UpdatedAt = time.Now().UTC()
			payload := map[string]any{
				"turn_id":             turnID,
				"question_set_id":     session.PendingQuestionSetID,
				"awaiting_input_kind": session.AwaitingInputKind,
				"source_message_id":   sourceMessageID,
				"questions":           clarificationQuestionsPayload(questions),
			}
			s.mu.Unlock()
			s.publish(sessionID, "clarification_requested", payload)
			s.publish(sessionID, "turn_completed", map[string]any{"turn_id": turnID})
			s.syncCurrentModel(sessionID)
			updated, _ := s.GetSession(sessionID)
			return updated, nil
		}
		session.Status = "ready"
		session.UpdatedAt = time.Now().UTC()
	}
	s.mu.Unlock()
	s.publish(sessionID, "turn_completed", map[string]any{"turn_id": turnID})
	s.syncCurrentModel(sessionID)
	updated, _ := s.GetSession(sessionID)
	return updated, nil
}

func (s *Service) DeleteSession(sessionID, userID string) error {
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if !ok || session == nil || session.UserID != userID {
		s.mu.Unlock()
		return shared.NotFound("acp session not found")
	}
	runtime := s.runtimes[sessionID]
	subs := make([]chan Event, 0, len(s.streams[sessionID]))
	for ch := range s.streams[sessionID] {
		subs = append(subs, ch)
	}
	delete(s.sessions, sessionID)
	delete(s.runtimes, sessionID)
	delete(s.streams, sessionID)
	s.mu.Unlock()

	if runtime != nil {
		if runtime.cancel != nil {
			runtime.cancel()
		}
		if runtime.client != nil {
			_ = runtime.client.close()
		}
	}
	for _, ch := range subs {
		close(ch)
	}
	if s.instrumentation != nil {
		s.instrumentation.RecordSessionEnded("deleted")
		s.instrumentation.RecordSessionDuration(time.Since(session.CreatedAt))
	}
	return nil
}

func duplicatePromptRequestID(session *Session, requestID string) bool {
	if session == nil || strings.TrimSpace(requestID) == "" {
		return false
	}
	expirePromptRequestIDs(session, time.Now().UTC())
	_, ok := session.recentPromptIDs[strings.TrimSpace(requestID)]
	return ok
}

func recordPromptRequestID(session *Session, requestID string) {
	if session == nil || strings.TrimSpace(requestID) == "" {
		return
	}
	now := time.Now().UTC()
	expirePromptRequestIDs(session, now)
	if session.recentPromptIDs == nil {
		session.recentPromptIDs = map[string]time.Time{}
	}
	session.recentPromptIDs[strings.TrimSpace(requestID)] = now
}

func expirePromptRequestIDs(session *Session, now time.Time) {
	if session == nil || len(session.recentPromptIDs) == 0 {
		return
	}
	for key, createdAt := range session.recentPromptIDs {
		if now.Sub(createdAt) > 10*time.Minute {
			delete(session.recentPromptIDs, key)
		}
	}
}

func duplicatePromptReplay(session *Session, displayContent, content string) bool {
	if session == nil {
		return false
	}
	displayContent = strings.TrimSpace(displayContent)
	content = strings.TrimSpace(content)
	now := time.Now().UTC()
	for index := len(session.Messages) - 1; index >= 0; index-- {
		item := session.Messages[index]
		if item.Role != "user" {
			continue
		}
		stored := strings.TrimSpace(item.Content)
		if stored != displayContent && stored != content {
			return false
		}
		if now.Sub(item.CreatedAt) > 90*time.Second {
			return false
		}
		return true
	}
	return false
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
	sessions := make([]*Session, 0, len(s.sessions))
	for _, item := range s.runtimes {
		runtimes = append(runtimes, item)
	}
	for _, item := range s.sessions {
		sessions = append(sessions, item)
	}
	s.runtimes = map[string]*sessionRuntime{}
	s.sessions = map[string]*Session{}
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
	if s.instrumentation != nil {
		for _, session := range sessions {
			s.instrumentation.RecordSessionEnded("closed")
			s.instrumentation.RecordSessionDuration(time.Since(session.CreatedAt))
		}
	}
	return nil
}

func (s *Service) handleNotification(sessionID, method string, params json.RawMessage) {
	switch method {
	case "session/update":
		var payload map[string]any
		if err := json.Unmarshal(params, &payload); err != nil {
			return
		}
		update := nestedMap(payload, "update")
		updateKind := stringValue(update["sessionUpdate"])
		if updateKind == "" {
			return
		}
		s.handleSessionUpdate(sessionID, updateKind, normalizeSessionUpdate(update))
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
	turnID := session.CurrentTurnID
	text := strings.TrimSpace(stringValue(content["text"]))
	meta := cloneMap(content)
	if turnID != "" {
		if meta == nil {
			meta = map[string]any{}
		}
		meta["turn_id"] = turnID
	}
	switch updateKind {
	case "agent_message_chunk":
		appendChunkMessage(session, "assistant", text, meta)
	case "user_message_chunk":
		// The submitted user prompt is already persisted at send time.
		// Provider-echoed user chunks are trace/status signals only.
	case "plan":
		session.CurrentPlan = append(session.CurrentPlan, PlanEntry{Content: text})
	case "artifact":
		if artifact, ok := artifactFromContent(content); ok {
			appendSessionArtifact(session, artifact)
		}
	default:
		if text != "" {
			appendChunkMessage(session, "system", text, meta)
		}
	}
	s.mu.Unlock()
	payload := map[string]any{"update_kind": updateKind, "content": content}
	if turnID != "" {
		payload["turn_id"] = turnID
	}
	s.publish(sessionID, "session_update", payload)
	if activity, ok := extractToolActivity(updateKind, content); ok {
		if turnID != "" {
			activity["turn_id"] = turnID
		}
		s.publish(sessionID, toolActivityEventKind(updateKind, activity), activity)
	}
	if model := firstNonEmptyString(content["modelID"], content["model_id"], nestedMapString(content, "model", "id")); model != "" {
		s.setCurrentModel(sessionID, model)
	}
}

func artifactFromContent(content map[string]any) (Artifact, bool) {
	kind := strings.TrimSpace(stringValue(content["kind"]))
	title := strings.TrimSpace(stringValue(content["title"]))
	metadata := nestedMap(content, "metadata")
	if kind == "" {
		metadata = cloneMap(content)
		kind = strings.TrimSpace(stringValue(metadata["kind"]))
		title = firstNonEmptyString(title, strings.TrimSpace(stringValue(metadata["title"])))
	}
	if kind == "" {
		return Artifact{}, false
	}
	id := strings.TrimSpace(stringValue(content["id"]))
	if id == "" {
		id = shared.NewID("artifact")
	}
	return Artifact{
		ID:          id,
		Kind:        kind,
		Title:       firstNonEmptyString(title, "Artifact"),
		ContentType: strings.TrimSpace(stringValue(content["content_type"])),
		Content:     strings.TrimSpace(stringValue(content["content"])),
		CreatedAt:   time.Now().UTC(),
		Metadata:    metadata,
	}, true
}

func appendSessionArtifact(session *Session, artifact Artifact) {
	if session == nil || strings.TrimSpace(artifact.Kind) == "" {
		return
	}
	for index, item := range session.Artifacts {
		if item.ID == artifact.ID {
			session.Artifacts[index] = artifact
			return
		}
	}
	session.Artifacts = append(session.Artifacts, artifact)
}

func (s *Service) promoteDashboardArtifactsFromTurn(session *Session, turnID string) {
	if session == nil || strings.TrimSpace(turnID) == "" {
		return
	}
	for _, message := range session.Messages {
		if message.Role != "assistant" {
			continue
		}
		if stringValue(message.Meta["turn_id"]) != turnID {
			continue
		}
		for _, artifact := range extractDashboardArtifactsFromMessage(message.Content) {
			appendSessionArtifact(session, artifact)
		}
	}
}

func extractDashboardArtifactsFromMessage(content string) []Artifact {
	matches := dashboardArtifactBlockPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	artifacts := make([]Artifact, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &payload); err != nil {
			continue
		}
		if artifact, ok := artifactFromContent(payload); ok {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts
}

func (s *Service) setCurrentModel(sessionID, model string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[sessionID]; session != nil {
		session.CurrentModel = model
		session.UpdatedAt = time.Now().UTC()
	}
}

func (s *Service) syncCurrentModel(sessionID string) {
	s.mu.RLock()
	session := s.sessions[sessionID]
	runtime := s.runtimes[sessionID]
	if session == nil || strings.TrimSpace(session.CurrentModel) != "" || strings.TrimSpace(session.RemoteSession) == "" || runtime == nil {
		s.mu.RUnlock()
		return
	}
	provider := runtime.provider
	remoteSession := session.RemoteSession
	s.mu.RUnlock()

	model, err := currentModelResolver(provider, remoteSession)
	if err != nil || strings.TrimSpace(model) == "" {
		return
	}
	s.setCurrentModel(sessionID, model)
}

func resolveCurrentModel(provider Provider, remoteSessionID string) (string, error) {
	if strings.TrimSpace(remoteSessionID) == "" {
		return "", nil
	}
	dbPath := opencodeDBPath(provider)
	if strings.TrimSpace(dbPath) == "" {
		return "", nil
	}
	script := `
import json, sqlite3, sys
db_path, session_id = sys.argv[1], sys.argv[2]
conn = sqlite3.connect(db_path)
cur = conn.cursor()
rows = cur.execute('select data from message where session_id=? order by time_created desc limit 8', (session_id,)).fetchall()
for (data,) in rows:
    obj = json.loads(data)
    model = obj.get("modelID") or ((obj.get("model") or {}).get("modelID"))
    if model:
        print(model)
        break
`
	cmd := exec.Command("python3", "-c", script, dbPath, remoteSessionID)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func opencodeDBPath(provider Provider) string {
	if home := firstNonEmptyString(
		strings.TrimSpace(provider.Env["HOME"]),
		strings.TrimSpace(os.Getenv("HOME")),
	); home != "" {
		return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	}
	return ""
}

func providerSupportsModelSelection(provider Provider) bool {
	command := strings.ToLower(strings.TrimSpace(filepath.Base(provider.Command)))
	return command == "opencode"
}

func providerSupportsPlanUpdates(provider Provider) bool {
	command := strings.ToLower(strings.TrimSpace(filepath.Base(provider.Command)))
	switch command {
	case "opencode":
		return false
	default:
		return false
	}
}

func resolveProviderModelCatalog(provider Provider) ([]ModelInfo, error) {
	sessionResult, err := probeProviderSessionModels(provider)
	if err == nil {
		models := make([]ModelInfo, 0, len(sessionResult.Models.AvailableModels))
		defaultModel := firstNonEmptyString(provider.DefaultModel, strings.TrimSpace(sessionResult.Models.CurrentModelID))
		for _, item := range sessionResult.Models.AvailableModels {
			modelID := strings.TrimSpace(item.ModelID)
			if modelID == "" {
				continue
			}
			if len(provider.AllowedModels) > 0 && !containsString(provider.AllowedModels, modelID) {
				continue
			}
			models = append(models, ModelInfo{
				ID:          modelID,
				Label:       firstNonEmptyString(strings.TrimSpace(item.Name), modelID),
				ProviderKey: provider.Key,
				RawModelID:  modelID,
				Selectable:  providerSupportsModelSelection(provider),
				Default:     modelID == defaultModel,
			})
		}
		if len(models) > 0 {
			sort.Slice(models, func(i, j int) bool {
				if models[i].Default != models[j].Default {
					return models[i].Default
				}
				return strings.ToLower(models[i].Label) < strings.ToLower(models[j].Label)
			})
			return models, nil
		}
	}
	if len(provider.AllowedModels) == 0 {
		return nil, err
	}
	models := make([]ModelInfo, 0, len(provider.AllowedModels))
	for _, modelID := range provider.AllowedModels {
		models = append(models, ModelInfo{
			ID:          modelID,
			Label:       modelID,
			ProviderKey: provider.Key,
			RawModelID:  modelID,
			Selectable:  providerSupportsModelSelection(provider),
			Default:     modelID == provider.DefaultModel,
		})
	}
	return models, nil
}

func probeProviderSessionModels(provider Provider) (newSessionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := startACPClient(ctx, provider, nil, nil)
	if err != nil {
		return newSessionResult{}, err
	}
	defer func() { _ = client.close() }()
	if _, err := client.initialize(); err != nil {
		return newSessionResult{}, err
	}
	cwd, err := resolveWorkingDir("workspace", "", provider.Cwd, shared.NewID("acp-probe"))
	if err != nil {
		return newSessionResult{}, err
	}
	return client.newSession(cwd)
}

func normalizeAllowedModels(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func containsSelectableModel(items []ModelInfo, model string) bool {
	model = strings.TrimSpace(model)
	for _, item := range items {
		if item.Selectable && item.ID == model {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func appendChunkMessage(session *Session, role, text string, meta map[string]any) {
	if session == nil || text == "" {
		return
	}
	turnID := stringValue(meta["turn_id"])
	if len(session.Messages) > 0 {
		last := &session.Messages[len(session.Messages)-1]
		if last.Role == role && time.Since(last.CreatedAt) < time.Minute {
			last.Content += text
			return
		}
	}
	if turnID != "" {
		for index := len(session.Messages) - 1; index >= 0; index-- {
			item := &session.Messages[index]
			if item.Role != role {
				continue
			}
			if stringValue(item.Meta["turn_id"]) != turnID {
				continue
			}
			if role == "user" {
				if strings.TrimSpace(item.Content) == strings.TrimSpace(text) {
					return
				}
				item.Content += text
				return
			}
			if role == "assistant" && time.Since(item.CreatedAt) < time.Minute {
				item.Content += text
				item.CreatedAt = time.Now().UTC()
				return
			}
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
		items[idx].DefaultModel = strings.TrimSpace(items[idx].DefaultModel)
		items[idx].AllowedModels = normalizeAllowedModels(items[idx].AllowedModels)
		if items[idx].Name == "" {
			items[idx].Name = items[idx].Key
		}
		if items[idx].DefaultModel != "" && len(items[idx].AllowedModels) > 0 && !containsString(items[idx].AllowedModels, items[idx].DefaultModel) {
			return nil, shared.Validation("platform.acp default_model must be included in allowed_models")
		}
	}
	return items, nil
}

func resolveWorkingDir(shell, requested, providerDir, sessionID string) (string, error) {
	if trimmed := strings.TrimSpace(requested); trimmed != "" {
		return trimmed, nil
	}
	if strings.TrimSpace(shell) == "agent_surface" {
		base := filepath.Join(os.TempDir(), "orbyte-agent-surface")
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", err
		}
		dir := filepath.Join(base, strings.ReplaceAll(sessionID, ":", "_"))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}
	if trimmed := strings.TrimSpace(providerDir); trimmed != "" {
		return trimmed, nil
	}
	if wd, err := os.Getwd(); err == nil {
		return wd, nil
	}
	return "", shared.Conflict("unable to determine acp working directory")
}

func defaultContextBlocks(shell, routePath string, existing []ContextBlock) []ContextBlock {
	blocks := append([]ContextBlock(nil), existing...)
	if strings.TrimSpace(shell) != "agent_surface" || hasContextBlock(blocks, "agent_workspace_guidance") {
		return blocks
	}
	blocks = append(blocks, ContextBlock{
		Key:      "agent_workspace_guidance",
		Label:    "Agent workspace guidance",
		Kind:     "instructions",
		Selected: true,
		Value: map[string]any{
			"route_path": strings.TrimSpace(routePath),
			"instructions": []string{
				"Use connected Orbyte MCP tools as the source of truth for business data.",
				"Do not use local files in the working directory as evidence for business answers.",
				"If a search does not find a matching record after a few focused retrieval attempts, stop and say what Orbyte data you checked instead of continuing indefinitely.",
				"When answering status, quantity, or amount questions, cite the relevant Orbyte record or document type briefly.",
				"If a business record or document search returns matching records for the requested employee, document, or code, stop searching broadly and answer from those retrieved records.",
				"Do not keep searching for alternate tools once you already have matching records and their statuses or amounts.",
			},
		},
	})
	return blocks
}

func hasContextBlock(blocks []ContextBlock, key string) bool {
	for _, block := range blocks {
		if strings.TrimSpace(block.Key) == key {
			return true
		}
	}
	return false
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

func normalizeSessionUpdate(update map[string]any) map[string]any {
	if len(update) == 0 {
		return nil
	}
	normalized := map[string]any{}
	for key, value := range update {
		if key == "sessionUpdate" {
			continue
		}
		if key == "content" {
			switch content := value.(type) {
			case map[string]any:
				for contentKey, contentValue := range content {
					normalized[contentKey] = contentValue
				}
				normalized["content"] = cloneMap(content)
			case string:
				normalized["text"] = content
				normalized["content"] = content
			case nil:
				normalized["content"] = nil
			default:
				normalized["content"] = value
			}
			continue
		}
		normalized[key] = value
	}
	return normalized
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
	out.PendingQuestions = append([]ClarificationQuestion(nil), in.PendingQuestions...)
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

func clarificationQuestionsPayload(items []ClarificationQuestion) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":                item.ID,
			"content":           item.Content,
			"source_message_id": item.SourceMessageID,
		})
	}
	return out
}

func deriveClarificationQuestionsForTurn(session *Session, turnID string) ([]ClarificationQuestion, string) {
	if session == nil || strings.TrimSpace(turnID) == "" {
		return nil, ""
	}
	for index := len(session.Messages) - 1; index >= 0; index-- {
		message := session.Messages[index]
		if message.Role != "assistant" {
			continue
		}
		if stringValue(message.Meta["turn_id"]) != turnID {
			continue
		}
		questions := extractClarificationQuestions(message.Content, message.ID)
		if len(questions) == 0 {
			return nil, ""
		}
		return questions, message.ID
	}
	return nil, ""
}

func extractClarificationQuestions(markdown, sourceMessageID string) []ClarificationQuestion {
	text := strings.TrimSpace(dashboardArtifactBlockPattern.ReplaceAllString(markdown, ""))
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	heading := false
	intro := false
	questions := make([]ClarificationQuestion, 0, 5)
	seen := map[string]struct{}{}
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		normalizedLine := normalizeClarificationLine(line)
		lower := strings.ToLower(normalizedLine)
		switch {
		case lower == "clarification needed",
			lower == "clarifications needed",
			lower == "need your input",
			lower == "need more input":
			heading = true
			continue
		case strings.HasPrefix(lower, "before finalizing"),
			strings.HasPrefix(lower, "before i finalize"),
			strings.HasPrefix(lower, "before we finalize"),
			strings.HasPrefix(lower, "before moving ahead"),
			strings.HasPrefix(lower, "before proceeding"),
			strings.HasPrefix(lower, "to finalize this plan"):
			intro = true
			continue
		}
		questionText := candidateClarificationQuestion(normalizedLine)
		if questionText == "" {
			continue
		}
		key := strings.ToLower(questionText)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		questions = append(questions, ClarificationQuestion{
			ID:              shared.NewID("question"),
			Content:         questionText,
			SourceMessageID: sourceMessageID,
		})
		if len(questions) == 5 {
			break
		}
	}
	if len(questions) == 0 {
		return nil
	}
	if heading || intro {
		return questions
	}
	return nil
}

func normalizeClarificationLine(line string) string {
	value := strings.TrimSpace(line)
	value = strings.TrimLeft(value, "-*• \t")
	if matches := regexp.MustCompile(`^\d+[\).\:]?\s+`).FindString(value); matches != "" {
		value = strings.TrimSpace(strings.TrimPrefix(value, matches))
	}
	return strings.TrimSpace(value)
}

func candidateClarificationQuestion(line string) string {
	value := strings.TrimSpace(line)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "?") {
		return ""
	}
	question := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(question), "clarification needed") {
		return ""
	}
	if len(question) > 280 {
		return ""
	}
	if idx := strings.Index(question, "?"); idx >= 0 {
		question = strings.TrimSpace(question[:idx+1])
	}
	return question
}

func extractToolActivity(updateKind string, content map[string]any) (map[string]any, bool) {
	if len(content) == 0 {
		return nil, false
	}
	toolName := firstNonEmptyString(
		content["tool_name"],
		content["toolName"],
		content["tool"],
		content["name"],
		content["title"],
		content["kind"],
		nestedMapString(content, "tool_call", "name"),
		nestedMapString(content, "toolCall", "name"),
	)
	if strings.EqualFold(toolName, "other") {
		if better := firstNonEmptyString(
			content["title"],
			nestedMapString(content, "tool_call", "name"),
			nestedMapString(content, "toolCall", "name"),
		); better != "" {
			toolName = better
		}
	}
	toolCallID := firstNonEmptyString(
		content["tool_call_id"],
		content["toolCallId"],
		content["toolCallID"],
		content["id"],
		nestedMapString(content, "tool_call", "id"),
		nestedMapString(content, "toolCall", "id"),
	)
	status := firstNonEmptyString(
		content["status"],
		content["phase"],
		content["state"],
		content["event"],
	)
	if strings.Contains(strings.ToLower(updateKind), "tool") && status == "" {
		status = updateKind
	}
	summary := firstNonEmptyString(
		content["summary"],
		content["message"],
		content["text"],
		content["title"],
	)
	if toolName == "" && !strings.Contains(strings.ToLower(updateKind), "tool") {
		return nil, false
	}
	if toolCallID == "" {
		toolCallID = shared.NewID("toolcall")
	}
	payload := map[string]any{
		"tool_call_id": toolCallID,
		"tool_name":    toolName,
	}
	if status != "" {
		payload["status"] = status
	}
	if summary != "" {
		payload["summary"] = summary
	}
	if arguments := nestedMap(content, "arguments"); len(arguments) > 0 {
		payload["arguments"] = arguments
	} else if arguments := nestedMap(content, "args"); len(arguments) > 0 {
		payload["arguments"] = arguments
	}
	for key, value := range extractDraftToolMetadata(toolName, summary, content) {
		payload[key] = value
	}
	return payload, true
}

var documentIDPattern = regexp.MustCompile(`\b(?:doc_[A-Za-z0-9]+|[a-z_]+:[A-Za-z0-9]+)\b`)

func extractDraftToolMetadata(toolName, summary string, content map[string]any) map[string]any {
	rawOutputText := firstNonEmptyString(
		findStringField(nestedMap(content, "rawOutput"), "output", 0),
		findStringField(nestedMap(content, "raw_output"), "output", 0),
		findStringField(nestedMap(content, "rawOutput"), "text", 0),
		findStringField(nestedMap(content, "raw_output"), "text", 0),
	)
	lowerToolName := strings.ToLower(strings.TrimSpace(toolName))
	lowerSummary := strings.ToLower(strings.TrimSpace(summary))
	lowerRawOutput := strings.ToLower(strings.TrimSpace(rawOutputText))
	if !strings.Contains(lowerToolName, "draft") &&
		!strings.Contains(lowerSummary, "draft") &&
		!strings.Contains(lowerRawOutput, "draft") {
		return nil
	}
	documentID := firstNonEmptyString(
		findStringField(content, "document_id", 0),
		findDocumentID(content, 0),
	)
	if documentID == "" {
		return nil
	}
	openPath := firstNonEmptyString(
		findStringField(content, "open_path", 0),
		findOpenPath(content, 0),
	)
	if openPath == "" {
		openPath = documentOpenPath(documentID)
	}
	result := map[string]any{
		"document_id": documentID,
		"open_path":   openPath,
	}
	if title := firstNonEmptyString(
		findStringField(nestedMap(content, "rawInput"), "title", 0),
		findStringField(content, "title", 0),
	); title != "" && !strings.Contains(strings.ToLower(title), "draft_create") {
		result["title"] = title
	}
	return result
}

func findDocumentID(value any, depth int) string {
	if depth > 6 || value == nil {
		return ""
	}
	switch current := value.(type) {
	case string:
		match := documentIDPattern.FindString(current)
		return strings.TrimSpace(match)
	case []any:
		for _, item := range current {
			if match := findDocumentID(item, depth+1); match != "" {
				return match
			}
		}
	case map[string]any:
		for _, item := range current {
			if match := findDocumentID(item, depth+1); match != "" {
				return match
			}
		}
	}
	return ""
}

func findStringField(value any, field string, depth int) string {
	if depth > 6 || value == nil {
		return ""
	}
	switch current := value.(type) {
	case map[string]any:
		if direct := stringValue(current[field]); direct != "" {
			return direct
		}
		for _, item := range current {
			if match := findStringField(item, field, depth+1); match != "" {
				return match
			}
		}
	case []any:
		for _, item := range current {
			if match := findStringField(item, field, depth+1); match != "" {
				return match
			}
		}
	}
	return ""
}

func findOpenPath(value any, depth int) string {
	if depth > 6 || value == nil {
		return ""
	}
	switch current := value.(type) {
	case string:
		for _, token := range strings.Fields(current) {
			trimmed := strings.TrimSpace(strings.Trim(token, `"'()[]<>.,`))
			if strings.HasPrefix(trimmed, "/ui/") && strings.Contains(trimmed, "?id=") {
				return trimmed
			}
			if strings.HasPrefix(trimmed, "/ui/documents/detail?id=") {
				return trimmed
			}
			if strings.HasPrefix(trimmed, "/ui#/documents/detail?document_id=") {
				documentID := strings.TrimPrefix(trimmed, "/ui#/documents/detail?document_id=")
				if documentID != "" {
					return documentOpenPath(documentID)
				}
			}
		}
	case map[string]any:
		for _, item := range current {
			if match := findOpenPath(item, depth+1); match != "" {
				return match
			}
		}
	case []any:
		for _, item := range current {
			if match := findOpenPath(item, depth+1); match != "" {
				return match
			}
		}
	}
	return ""
}

func documentOpenPath(documentID string) string {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return ""
	}
	return "/ui/documents/detail?id=" + url.QueryEscape(documentID)
}

func toolActivityEventKind(updateKind string, payload map[string]any) string {
	status := strings.ToLower(stringValue(payload["status"]))
	switch {
	case strings.Contains(strings.ToLower(updateKind), "completed"),
		strings.Contains(strings.ToLower(updateKind), "finished"),
		strings.Contains(strings.ToLower(updateKind), "result"),
		status == "completed",
		status == "finished",
		status == "success":
		return "tool_call_completed"
	case strings.Contains(strings.ToLower(updateKind), "start"),
		status == "started",
		status == "running":
		return "tool_call_started"
	default:
		return "tool_call_updated"
	}
}

func nestedMapString(content map[string]any, key, field string) string {
	return stringValue(nestedMap(content, key)[field])
}

func nestedMap(content map[string]any, key string) map[string]any {
	if content == nil {
		return nil
	}
	switch value := content[key].(type) {
	case map[string]any:
		return value
	}
	return nil
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		switch current := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(current); trimmed != "" {
				return trimmed
			}
		case json.Number:
			if trimmed := strings.TrimSpace(current.String()); trimmed != "" {
				return trimmed
			}
		case int:
			return strconv.Itoa(current)
		case int64:
			return strconv.FormatInt(current, 10)
		case float64:
			if current == float64(int64(current)) {
				return strconv.FormatInt(int64(current), 10)
			}
		}
	}
	return ""
}
