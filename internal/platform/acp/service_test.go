package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"orbyte/internal/platform/config"
)

func newACPService(t *testing.T, enabled bool, providersJSON string) *Service {
	t.Helper()
	cfg := config.NewServiceWithRepository(config.NewMemoryRepository([]config.Entry{{
		Key:       "platform.acp",
		ModuleKey: "platform.core",
		Category:  "platform",
		Scope:     "deployment",
		Value: map[string]any{
			"enabled":        enabled,
			"providers_json": providersJSON,
		},
	}}))
	return NewService(cfg, nil)
}

func testClientWithResponses(t *testing.T, handler func(message map[string]any, c *acpClient) error) *acpClient {
	t.Helper()
	client := &acpClient{pending: map[int64]chan rpcResponse{}}
	client.writeMessageFn = func(message any) error {
		raw, err := json.Marshal(message)
		if err != nil {
			return err
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return err
		}
		return handler(decoded, client)
	}
	return client
}

func TestProvidersAndEnabled(t *testing.T) {
	svc := newACPService(t, true, `[{"key":"opencode","name":"OpenCode ACP","description":"ACP","command":"opencode"}]`)
	original := providerModelCatalogResolver
	providerModelCatalogResolver = func(provider Provider) ([]ModelInfo, error) {
		return []ModelInfo{{ID: "opencode/default", Label: "Default", ProviderKey: provider.Key, Selectable: true, Default: true}}, nil
	}
	defer func() { providerModelCatalogResolver = original }()
	if !svc.Enabled() {
		t.Fatal("expected service enabled")
	}
	items := svc.Providers()
	if len(items) != 1 {
		t.Fatalf("expected one provider, got %d", len(items))
	}
	if items[0].Key != "opencode" || !items[0].Available {
		t.Fatalf("unexpected provider info: %#v", items[0])
	}
	if !items[0].SupportsModelListing || !items[0].SupportsModelSelection {
		t.Fatalf("unexpected model capability flags: %#v", items[0])
	}
}

func TestProvidersInvalidConfig(t *testing.T) {
	svc := newACPService(t, true, `{`)
	items := svc.Providers()
	if len(items) != 1 || items[0].Error == "" {
		t.Fatalf("expected invalid config provider result, got %#v", items)
	}
	if _, err := svc.providerConfigs(); err == nil {
		t.Fatal("expected invalid providers_json error")
	}
}

func TestProviderConfigsDefaultsAndTrims(t *testing.T) {
	svc := newACPService(t, true, `[{"key":"  codex  ","name":" ","command":" /bin/echo ","args":["ok"]}]`)
	items, err := svc.providerConfigs()
	if err != nil {
		t.Fatalf("providerConfigs failed: %v", err)
	}
	if items[0].Key != "codex" || items[0].Name != "codex" || items[0].Command != "/bin/echo" {
		t.Fatalf("unexpected normalized provider: %#v", items[0])
	}
}

func TestProviderConfigsModelPolicyValidation(t *testing.T) {
	svc := newACPService(t, true, `[{"key":"codex","name":"Codex","command":"/bin/echo","default_model":"codex/default","allowed_models":["codex/other"]}]`)
	if _, err := svc.providerConfigs(); err == nil {
		t.Fatal("expected invalid default model policy")
	}
}

func TestProviderModelsReturnsCatalog(t *testing.T) {
	svc := newACPService(t, true, `[{"key":"opencode","name":"OpenCode ACP","command":"opencode","allowed_models":["opencode/default","opencode/nano"],"default_model":"opencode/default"}]`)
	original := providerModelCatalogResolver
	providerModelCatalogResolver = func(provider Provider) ([]ModelInfo, error) {
		return []ModelInfo{
			{ID: "opencode/default", Label: "Default", ProviderKey: provider.Key, Selectable: true, Default: true},
			{ID: "opencode/nano", Label: "Nano", ProviderKey: provider.Key, Selectable: true},
		}, nil
	}
	defer func() { providerModelCatalogResolver = original }()

	items, err := svc.ProviderModels("opencode")
	if err != nil {
		t.Fatalf("ProviderModels failed: %v", err)
	}
	if len(items) != 2 || items[0].ID != "opencode/default" || !items[0].Selectable {
		t.Fatalf("unexpected provider models: %#v", items)
	}
}

func TestListSessionsFiltersAndSorts(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	now := time.Now().UTC()
	svc.sessions["a"] = &Session{ID: "a", UserID: "user-1", UpdatedAt: now.Add(-time.Minute)}
	svc.sessions["b"] = &Session{ID: "b", UserID: "user-2", UpdatedAt: now}
	svc.sessions["c"] = &Session{ID: "c", UserID: "user-1", UpdatedAt: now.Add(-time.Second)}

	items := svc.ListSessions("user-1")
	if len(items) != 2 {
		t.Fatalf("expected two sessions, got %d", len(items))
	}
	if items[0].ID != "c" || items[1].ID != "a" {
		t.Fatalf("unexpected ordering: %#v", items)
	}
}

func TestStartSessionValidationAndDisabled(t *testing.T) {
	svc := newACPService(t, false, `[]`)
	if _, err := svc.StartSession(StartSessionRequest{ProviderKey: "codex", UserID: "user-1"}); err == nil {
		t.Fatal("expected disabled error")
	}

	svc = newACPService(t, true, `[{"key":"codex","name":"Codex"}]`)
	if _, err := svc.StartSession(StartSessionRequest{ProviderKey: "missing", UserID: "user-1"}); err == nil {
		t.Fatal("expected missing provider error")
	}
	if _, err := svc.StartSession(StartSessionRequest{ProviderKey: "codex"}); err == nil {
		t.Fatal("expected missing user error")
	}
	if _, err := svc.StartSession(StartSessionRequest{ProviderKey: "codex", UserID: "user-1", Model: "codex/default"}); err == nil {
		t.Fatal("expected unsupported model selection error")
	}
}

func TestStartSessionAppliesSelectedModelForOpenCode(t *testing.T) {
	svc := newACPService(t, true, `[{"key":"opencode","name":"OpenCode ACP","command":"opencode"}]`)
	originalCatalog := providerModelCatalogResolver
	originalStarter := acpClientStarter
	providerModelCatalogResolver = func(provider Provider) ([]ModelInfo, error) {
		return []ModelInfo{{ID: "opencode/minimax-m2.5-free", Label: "MiniMax M2.5", ProviderKey: provider.Key, Selectable: true}}, nil
	}
	acpClientStarter = func(ctx context.Context, provider Provider, onNotification func(method string, params json.RawMessage), onRequest func(id int64, method string, params json.RawMessage)) (*acpClient, error) {
		client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
			id := int64(message["id"].(float64))
			method := message["method"].(string)
			c.mu.Lock()
			ch := c.pending[id]
			c.mu.Unlock()
			switch method {
			case "initialize":
				ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"protocolVersion":1}`)}
			case "session/new":
				ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"sessionId":"remote-1","models":{"currentModelId":"opencode/big-pickle"}}`)}
			case "session/set_model":
				params := message["params"].(map[string]any)
				if params["modelId"] != "opencode/minimax-m2.5-free" {
					t.Fatalf("unexpected requested model: %#v", params)
				}
				ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"_meta":{"opencode":{"modelId":"opencode/minimax-m2.5-free"}}}`)}
			default:
				t.Fatalf("unexpected method: %s", method)
			}
			close(ch)
			return nil
		})
		return client, nil
	}
	defer func() {
		providerModelCatalogResolver = originalCatalog
		acpClientStarter = originalStarter
	}()

	session, err := svc.StartSession(StartSessionRequest{ProviderKey: "opencode", UserID: "user-1", Model: "opencode/minimax-m2.5-free"})
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if session.RequestedModel != "opencode/minimax-m2.5-free" || session.CurrentModel != "opencode/minimax-m2.5-free" {
		t.Fatalf("unexpected session models: %#v", session)
	}
}

func TestGetSessionReturnsClone(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{
		ID:            "session-1",
		Messages:      []Message{{ID: "msg-1", Content: "hello"}},
		ContextBlocks: []ContextBlock{{Key: "ctx"}},
		ProviderInfo:  map[string]any{"agent": "codex"},
	}
	session, ok := svc.GetSession("session-1")
	if !ok {
		t.Fatal("expected session")
	}
	session.Messages[0].Content = "changed"
	session.ProviderInfo["agent"] = "other"
	again, _ := svc.GetSession("session-1")
	if again.Messages[0].Content != "hello" || again.ProviderInfo["agent"] != "codex" {
		t.Fatalf("expected cloned session, got %#v", again)
	}
}

func TestSendPromptValidationNotFoundAndSuccess(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	if _, err := svc.SendPrompt("missing", PromptRequest{Content: "hello"}); err == nil {
		t.Fatal("expected missing session error")
	}
	if _, err := svc.SendPrompt("missing", PromptRequest{Content: "   "}); err == nil {
		t.Fatal("expected validation error")
	}

	session := &Session{
		ID:            "session-1",
		UserID:        "user-1",
		RemoteSession: "remote-1",
		ContextBlocks: []ContextBlock{{Key: "ctx", Label: "Ctx", Selected: true, Value: map[string]any{"route": "/docs"}}},
	}
	svc.sessions["session-1"] = session
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		id := int64(message["id"].(float64))
		method := message["method"].(string)
		if method != "session/prompt" {
			return errors.New("unexpected method: " + method)
		}
		params := message["params"].(map[string]any)
		if params["sessionId"] != "remote-1" {
			return errors.New("unexpected session id")
		}
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		ch <- rpcResponse{ID: id, Result: json.RawMessage(`{}`)}
		close(ch)
		return nil
	})
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	updated, err := svc.SendPrompt("session-1", PromptRequest{Content: "hello", ContextBlocks: []ContextBlock{{Key: "custom", Selected: true}}})
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}
	if len(updated.Messages) != 1 || updated.Messages[0].Content != "hello" {
		t.Fatalf("expected user message appended, got %#v", updated.Messages)
	}
	if updated.Status != "ready" {
		t.Fatalf("expected ready status, got %q", updated.Status)
	}
}

func TestSendPromptStoresDisplayContentSeparately(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{
		ID:            "session-1",
		UserID:        "user-1",
		RemoteSession: "remote-1",
	}
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		id := int64(message["id"].(float64))
		method := message["method"].(string)
		if method != "session/prompt" {
			return errors.New("unexpected method: " + method)
		}
		params := message["params"].(map[string]any)
		parts := params["prompt"].([]any)
		if len(parts) == 0 || parts[0].(map[string]any)["text"] != "Use Orbyte MCP tools as the source of truth.\n\nhello" {
			return errors.New("unexpected prompt payload")
		}
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		ch <- rpcResponse{ID: id, Result: json.RawMessage(`{}`)}
		close(ch)
		return nil
	})
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	updated, err := svc.SendPrompt("session-1", PromptRequest{
		Content:        "Use Orbyte MCP tools as the source of truth.\n\nhello",
		DisplayContent: "hello",
	})
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}
	if len(updated.Messages) != 1 || updated.Messages[0].Content != "hello" {
		t.Fatalf("expected stored display content only, got %#v", updated.Messages)
	}
}

func TestSendPromptFailureMarksSessionError(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1", RemoteSession: "remote-1"}
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		return errors.New("write failed")
	})
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	if _, err := svc.SendPrompt("session-1", PromptRequest{Content: "hello"}); err == nil {
		t.Fatal("expected prompt error")
	}
	session, _ := svc.GetSession("session-1")
	if session.Status != "error" || session.LastError == "" {
		t.Fatalf("expected error session state, got %#v", session)
	}
}

func TestSendPromptIgnoresRapidDuplicateReplay(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	turnID := "acp-turn:existing"
	svc.sessions["session-1"] = &Session{
		ID:            "session-1",
		RemoteSession: "remote-1",
		Status:        "ready",
		Messages: []Message{{
			ID:        "msg-1",
			Role:      "user",
			Content:   "hello",
			Format:    "markdown",
			CreatedAt: time.Now().UTC(),
			Meta:      map[string]any{"turn_id": turnID},
		}},
	}
	called := false
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		called = true
		return nil
	})
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	updated, err := svc.SendPrompt("session-1", PromptRequest{Content: "hello"})
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}
	if called {
		t.Fatal("expected duplicate replay to skip remote prompt")
	}
	if len(updated.Messages) != 1 {
		t.Fatalf("expected existing messages only, got %#v", updated.Messages)
	}
}

func TestSendPromptIgnoresRapidDuplicateReplayWithDisplayContent(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	turnID := "acp-turn:existing"
	svc.sessions["session-1"] = &Session{
		ID:            "session-1",
		RemoteSession: "remote-1",
		Status:        "ready",
		Messages: []Message{{
			ID:        "msg-1",
			Role:      "user",
			Content:   "hello",
			Format:    "markdown",
			CreatedAt: time.Now().UTC(),
			Meta:      map[string]any{"turn_id": turnID},
		}},
	}
	called := false
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		called = true
		return nil
	})
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	updated, err := svc.SendPrompt("session-1", PromptRequest{
		Content:        "Use Orbyte MCP tools as the source of truth for this answer.\n\nhello",
		DisplayContent: "hello",
	})
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}
	if called {
		t.Fatal("expected duplicate replay with display content to skip remote prompt")
	}
	if len(updated.Messages) != 1 {
		t.Fatalf("expected existing messages only, got %#v", updated.Messages)
	}
}

func TestSendPromptIgnoresDuplicateClientRequestID(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{
		ID:            "session-1",
		RemoteSession: "remote-1",
		CurrentModel:  "opencode/default",
		Status:        "ready",
	}
	callCount := 0
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		callCount += 1
		id := int64(message["id"].(float64))
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		ch <- rpcResponse{ID: id, Result: json.RawMessage(`{}`)}
		close(ch)
		return nil
	})
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	req := PromptRequest{
		Content:         "Use Orbyte MCP tools as the source of truth for this answer.\n\nhello",
		DisplayContent:  "hello",
		ClientRequestID: "req-1",
	}
	if _, err := svc.SendPrompt("session-1", req); err != nil {
		t.Fatalf("first SendPrompt failed: %v", err)
	}
	if _, err := svc.SendPrompt("session-1", req); err != nil {
		t.Fatalf("second SendPrompt failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected one remote prompt call, got %d", callCount)
	}
}

func TestResolveWorkingDirUsesIsolatedAgentSurfaceDir(t *testing.T) {
	dir, err := resolveWorkingDir("agent_surface", "", "", "acp-session:test")
	if err != nil {
		t.Fatalf("resolveWorkingDir failed: %v", err)
	}
	expected := filepath.Join(os.TempDir(), "orbyte-agent-surface", "acp-session_test")
	if dir != expected {
		t.Fatalf("expected %q, got %q", expected, dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected isolated working dir to exist, err=%v", err)
	}
}

func TestOpencodeDBPathFallsBackToProcessHome(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("HOME", temp)
	got := opencodeDBPath(Provider{})
	expected := filepath.Join(temp, ".local", "share", "opencode", "opencode.db")
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestDefaultContextBlocksAddsAgentWorkspaceGuidance(t *testing.T) {
	blocks := defaultContextBlocks("agent_surface", "/agent/workspace", nil)
	if len(blocks) != 1 {
		t.Fatalf("expected one default block, got %#v", blocks)
	}
	if blocks[0].Key != "agent_workspace_guidance" || !blocks[0].Selected {
		t.Fatalf("unexpected guidance block: %#v", blocks[0])
	}
	value := blocks[0].Value
	if value["route_path"] != "/agent/workspace" {
		t.Fatalf("expected route path in guidance block, got %#v", value)
	}
	instructions, ok := value["instructions"].([]string)
	if !ok || len(instructions) < 3 {
		t.Fatalf("expected instructions slice, got %#v", value["instructions"])
	}
}

func TestSubscribeUnsubscribeDoesNotPanicDuringPublish(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1"}

	ch, unsubscribe := svc.Subscribe("session-1")
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			svc.publish("session-1", "session_update", map[string]any{"seq": i})
		}
	}()

	time.Sleep(5 * time.Millisecond)
	unsubscribe()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publish did not complete")
	}
	select {
	case <-ch:
	default:
	}
}

func TestCloseCancelsRuntimeAndClearsMap(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	cancelled := false
	svc.runtimes["session-1"] = &sessionRuntime{
		cancel: func() { cancelled = true },
	}
	_ = svc.Close()
	if !cancelled {
		t.Fatal("expected runtime cancel")
	}
	if len(svc.runtimes) != 0 {
		t.Fatalf("expected cleared runtimes, got %d", len(svc.runtimes))
	}
}

func TestDeleteSessionRemovesOwnedSessionAndClosesRuntime(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	cancelled := false
	svc.sessions["session-1"] = &Session{
		ID:        "session-1",
		UserID:    "user-1",
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	svc.runtimes["session-1"] = &sessionRuntime{
		cancel: func() { cancelled = true },
	}
	ch, _ := svc.Subscribe("session-1")

	if err := svc.DeleteSession("session-1", "user-1"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if !cancelled {
		t.Fatal("expected runtime cancel")
	}
	if _, ok := svc.GetSession("session-1"); ok {
		t.Fatal("expected deleted session to be gone")
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected subscription channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("expected subscription channel close")
	}
}

func TestDeleteSessionRejectsOtherUsers(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1", UserID: "user-1"}
	if err := svc.DeleteSession("session-1", "user-2"); err == nil {
		t.Fatal("expected ownership enforcement")
	}
}

func TestHandleNotificationAndSessionUpdate(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1", CurrentTurnID: "turn-1"}

	svc.handleNotification("session-1", "session/update", json.RawMessage(`{"sessionId":"remote","update":{"sessionUpdate":"agent_message_chunk","content":{"text":"hello"}}}`))
	svc.handleNotification("session-1", "other/event", json.RawMessage(`{}`))
	svc.handleSessionUpdate("session-1", "plan", map[string]any{"text": "step 1"})
	svc.handleSessionUpdate("session-1", "user_message_chunk", map[string]any{"text": "user"})
	svc.handleSessionUpdate("session-1", "unknown", map[string]any{"text": "system"})
	svc.handleSessionUpdate("session-1", "tool_call_started", map[string]any{"tool_name": "orbyte_module_list", "status": "running", "text": "listing modules"})

	session, _ := svc.GetSession("session-1")
	if len(session.Messages) < 2 {
		t.Fatalf("expected accumulated messages, got %#v", session.Messages)
	}
	for _, item := range session.Messages {
		if item.Role == "user" {
			t.Fatalf("expected user chunks to stay out of stored messages, got %#v", session.Messages)
		}
	}
	if len(session.CurrentPlan) != 1 || session.CurrentPlan[0].Content != "step 1" {
		t.Fatalf("unexpected plan entries: %#v", session.CurrentPlan)
	}
	var foundToolEvent bool
	for _, item := range session.Trace {
		if item.Kind == "tool_call_started" {
			foundToolEvent = true
			if got := item.Payload["tool_name"]; got != "orbyte_module_list" {
				t.Fatalf("unexpected tool payload: %#v", item.Payload)
			}
		}
	}
	if !foundToolEvent {
		t.Fatal("expected tool_call_started event in trace")
	}
}

func TestHandleSessionUpdatePersistsCurrentModel(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1", CurrentTurnID: "turn-1"}

	svc.handleSessionUpdate("session-1", "agent_message_chunk", map[string]any{
		"text":    "hello",
		"modelID": "minimax-m2.7",
	})

	session, _ := svc.GetSession("session-1")
	if session.CurrentModel != "minimax-m2.7" {
		t.Fatalf("expected current model to persist, got %q", session.CurrentModel)
	}
}

func TestGetSessionHydratesCurrentModelFromResolver(t *testing.T) {
	previousResolver := currentModelResolver
	currentModelResolver = func(provider Provider, remoteSessionID string) (string, error) {
		if provider.Key != "opencode" || remoteSessionID != "remote-1" {
			t.Fatalf("unexpected resolver inputs: %#v %q", provider, remoteSessionID)
		}
		return "minimax-m2.7", nil
	}
	defer func() { currentModelResolver = previousResolver }()

	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{
		ID:            "session-1",
		ProviderKey:   "opencode",
		RemoteSession: "remote-1",
	}
	svc.runtimes["session-1"] = &sessionRuntime{provider: Provider{Key: "opencode"}}

	session, ok := svc.GetSession("session-1")
	if !ok {
		t.Fatal("expected session")
	}
	if session.CurrentModel != "minimax-m2.7" {
		t.Fatalf("expected hydrated current model, got %q", session.CurrentModel)
	}
}

func TestExtractToolActivity(t *testing.T) {
	payload, ok := extractToolActivity("tool_call_completed", map[string]any{
		"tool_call": map[string]any{"id": "call-1", "name": "orbyte_module_list"},
		"status":    "completed",
		"text":      "finished listing modules",
	})
	if !ok {
		t.Fatal("expected tool activity extraction")
	}
	if payload["tool_call_id"] != "call-1" || payload["tool_name"] != "orbyte_module_list" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if got := toolActivityEventKind("tool_call_completed", payload); got != "tool_call_completed" {
		t.Fatalf("unexpected tool event kind: %s", got)
	}
}

func TestExtractToolActivitySynthesizesDraftOpenPath(t *testing.T) {
	payload, ok := extractToolActivity("tool_call_update", map[string]any{
		"title":      "orbyte-agentproof-promotion_core_strategy_plan_draft_create",
		"toolCallId": "call-draft-1",
		"status":     "completed",
		"rawInput": map[string]any{
			"title": "Promotion Plan 20260404-002446",
		},
		"rawOutput": map[string]any{
			"output": "Created promotion strategy draft doc_01KNAY520Q98F95WCV2YQ6CSCB as generic_request.",
		},
	})
	if !ok {
		t.Fatal("expected tool activity extraction")
	}
	if payload["document_id"] != "doc_01KNAY520Q98F95WCV2YQ6CSCB" {
		t.Fatalf("expected synthesized document_id, got %#v", payload)
	}
	if payload["title"] != "Promotion Plan 20260404-002446" {
		t.Fatalf("expected synthesized title, got %#v", payload)
	}
	if payload["open_path"] != "/ui/documents/detail?id=doc_01KNAY520Q98F95WCV2YQ6CSCB" {
		t.Fatalf("expected synthesized open_path, got %#v", payload)
	}
}

func TestExtractToolActivitySynthesizesDraftOpenPathFromRawOutput(t *testing.T) {
	payload, ok := extractToolActivity("tool_call_update", map[string]any{
		"kind":       "other",
		"toolCallId": "call-draft-2",
		"status":     "completed",
		"rawInput": map[string]any{
			"title": "Promotion Plan 20260404-002446",
		},
		"rawOutput": map[string]any{
			"output": "Created promotion strategy draft doc_01KNAZ7YCDRQV22YWXQ4KS9M9Y as generic_request. Open draft: /ui/promotion/plans/form?id=doc_01KNAZ7YCDRQV22YWXQ4KS9M9Y",
		},
	})
	if !ok {
		t.Fatal("expected tool activity extraction")
	}
	if payload["document_id"] != "doc_01KNAZ7YCDRQV22YWXQ4KS9M9Y" {
		t.Fatalf("expected raw output document_id, got %#v", payload)
	}
	if payload["open_path"] != "/ui/promotion/plans/form?id=doc_01KNAZ7YCDRQV22YWXQ4KS9M9Y" {
		t.Fatalf("expected raw output open_path, got %#v", payload)
	}
}

func TestExtractToolActivityPrefersTitleOverGenericKind(t *testing.T) {
	payload, ok := extractToolActivity("tool_call_update", map[string]any{
		"kind":       "other",
		"title":      "orbyte-agentproof-employee_spend_core_business_records_search",
		"toolCallId": "call-2",
		"status":     "failed",
	})
	if !ok {
		t.Fatal("expected tool activity extraction")
	}
	if payload["tool_name"] != "orbyte-agentproof-employee_spend_core_business_records_search" {
		t.Fatalf("unexpected tool name payload: %#v", payload)
	}
}

func TestNormalizeSessionUpdatePreservesToolMetadata(t *testing.T) {
	update := normalizeSessionUpdate(map[string]any{
		"sessionUpdate": "tool_call_update",
		"content":       nil,
		"toolCallId":    "call-42",
		"toolName":      "orbyte_document_read",
		"status":        "running",
		"title":         "Reading reimbursement payment",
		"arguments": map[string]any{
			"document_id": "doc_123",
		},
	})
	if update["toolCallId"] != "call-42" || update["toolName"] != "orbyte_document_read" {
		t.Fatalf("expected tool metadata preserved, got %#v", update)
	}
	if update["content"] != nil {
		t.Fatalf("expected explicit nil content preserved, got %#v", update["content"])
	}
}

func TestApprovalResolutionRespondsToClient(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1"}
	var mu sync.Mutex
	var responses []map[string]any
	client := &acpClient{
		writeMessageFn: func(message any) error {
			mu.Lock()
			defer mu.Unlock()
			payload, _ := json.Marshal(message)
			var decoded map[string]any
			_ = json.Unmarshal(payload, &decoded)
			responses = append(responses, decoded)
			return nil
		},
	}
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	svc.handleRequest("session-1", 42, "client/approve", json.RawMessage(`{"kind":"write"}`))
	session, ok := svc.GetSession("session-1")
	if !ok {
		t.Fatal("expected session")
	}
	if len(session.Approvals) != 1 {
		t.Fatalf("expected one approval, got %d", len(session.Approvals))
	}
	if _, err := svc.Approve("session-1", session.Approvals[0].ID); err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(responses) != 1 {
		t.Fatalf("expected one upstream response, got %d", len(responses))
	}
	if got := responses[0]["id"]; got != float64(42) {
		t.Fatalf("expected request id 42, got %#v", got)
	}
	if _, ok := responses[0]["result"]; !ok {
		t.Fatalf("expected approval result payload, got %#v", responses[0])
	}
}

func TestRejectResolutionRespondsToClientError(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1"}
	var mu sync.Mutex
	var responses []map[string]any
	client := &acpClient{
		writeMessageFn: func(message any) error {
			mu.Lock()
			defer mu.Unlock()
			payload, _ := json.Marshal(message)
			var decoded map[string]any
			_ = json.Unmarshal(payload, &decoded)
			responses = append(responses, decoded)
			return nil
		},
	}
	svc.runtimes["session-1"] = &sessionRuntime{client: client}

	svc.handleRequest("session-1", 7, "client/approve", nil)
	session, ok := svc.GetSession("session-1")
	if !ok || len(session.Approvals) != 1 {
		t.Fatalf("expected one pending approval, got %#v", session)
	}
	if _, err := svc.Reject("session-1", session.Approvals[0].ID); err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(responses) != 1 {
		t.Fatalf("expected one upstream response, got %d", len(responses))
	}
	errBody, ok := responses[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error payload, got %#v", responses[0])
	}
	if errBody["message"] != "request rejected by user" {
		t.Fatalf("unexpected rejection message: %#v", errBody)
	}
}

func TestPromptBlockHelpers(t *testing.T) {
	contextBlocks := []ContextBlock{{Key: "ctx", Label: "Ctx", Selected: true, Value: map[string]any{"route": "/docs"}}}
	blocks := promptBlocks("hello", contextBlocks)
	if len(blocks) != 2 {
		t.Fatalf("expected prompt + context summary, got %#v", blocks)
	}
	if !bytes.Contains([]byte(blocks[1]["text"].(string)), []byte("Current platform context")) {
		t.Fatalf("expected context summary text, got %#v", blocks[1])
	}
	if merge := mergeContextBlocks(contextBlocks, nil); len(merge) != 1 || merge[0].Key != "ctx" {
		t.Fatalf("unexpected merged existing context: %#v", merge)
	}
	if merge := mergeContextBlocks(contextBlocks, []ContextBlock{{Key: "incoming"}}); len(merge) != 1 || merge[0].Key != "incoming" {
		t.Fatalf("unexpected merged incoming context: %#v", merge)
	}
	if got := renderContextSummary([]ContextBlock{{Key: "off", Selected: false}}); got != "" {
		t.Fatalf("expected empty context summary, got %q", got)
	}
	if got := stringValue("  hi  "); got != "hi" {
		t.Fatalf("unexpected string value %q", got)
	}
}

func TestAppendChunkMessageMergesRecentRole(t *testing.T) {
	session := &Session{}
	appendChunkMessage(session, "assistant", "hel", map[string]any{"a": 1})
	appendChunkMessage(session, "assistant", "lo", map[string]any{"a": 1})
	appendChunkMessage(session, "user", "world", nil)
	if len(session.Messages) != 2 {
		t.Fatalf("expected merged assistant message and user message, got %#v", session.Messages)
	}
	if session.Messages[0].Content != "hello" {
		t.Fatalf("expected merged content, got %#v", session.Messages[0])
	}
}

func TestAppendChunkMessageMergesByTurnAcrossInterleavedUpdates(t *testing.T) {
	session := &Session{}
	appendChunkMessage(session, "user", "hello", map[string]any{"turn_id": "turn-1"})
	appendChunkMessage(session, "system", "thinking", map[string]any{"turn_id": "turn-1"})
	appendChunkMessage(session, "assistant", "Part 1", map[string]any{"turn_id": "turn-1"})
	appendChunkMessage(session, "system", "tool", map[string]any{"turn_id": "turn-1"})
	appendChunkMessage(session, "assistant", " + part 2", map[string]any{"turn_id": "turn-1"})
	appendChunkMessage(session, "user", "hello", map[string]any{"turn_id": "turn-1"})

	if len(session.Messages) != 4 {
		t.Fatalf("expected user, two system, and assistant messages only, got %#v", session.Messages)
	}
	if session.Messages[0].Content != "hello" {
		t.Fatalf("expected single user message, got %#v", session.Messages[0])
	}
	if session.Messages[2].Content != "Part 1 + part 2" {
		t.Fatalf("expected merged assistant message, got %#v", session.Messages[2])
	}
}

func TestCloneMapSessionAndStringValue(t *testing.T) {
	original := &Session{
		ID:            "session-1",
		Messages:      []Message{{ID: "msg"}},
		ContextBlocks: []ContextBlock{{Key: "ctx"}},
		Approvals:     []Approval{{ID: "appr"}},
		Artifacts:     []Artifact{{ID: "art"}},
		Trace:         []Event{{ID: "evt"}},
		CurrentPlan:   []PlanEntry{{Content: "step"}},
		ProviderInfo:  map[string]any{"agent": "codex"},
	}
	cloned := cloneSession(original)
	cloned.Messages[0].ID = "changed"
	cloned.ProviderInfo["agent"] = "other"
	if original.Messages[0].ID != "msg" || original.ProviderInfo["agent"] != "codex" {
		t.Fatalf("expected deep clone, got %#v", original)
	}
	if cloneMap(nil) != nil {
		t.Fatal("expected nil cloneMap for nil input")
	}
	if got := stringValue(12); got != "" {
		t.Fatalf("expected empty string from non-string, got %q", got)
	}
}

func TestClientWriteMessageUsesFraming(t *testing.T) {
	var buf bytes.Buffer
	client := &acpClient{stdin: nopWriteCloser{Writer: &buf}}
	if err := client.writeMessage(map[string]any{"jsonrpc": "2.0", "id": 1}); err != nil {
		t.Fatalf("writeMessage failed: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Content-Length:")) || !bytes.Contains(buf.Bytes(), []byte(`"jsonrpc":"2.0"`)) {
		t.Fatalf("expected framed rpc payload, got %q", buf.String())
	}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestClientWriteMessageUsesJSONLTransport(t *testing.T) {
	var buf bytes.Buffer
	client := &acpClient{
		stdin:     nopWriteCloser{Writer: &buf},
		transport: rpcTransportJSONL,
	}
	if err := client.writeMessage(map[string]any{"jsonrpc": "2.0", "id": 1}); err != nil {
		t.Fatalf("writeMessage failed: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("Content-Length:")) {
		t.Fatalf("expected jsonl rpc payload, got %q", buf.String())
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) || !bytes.Contains(buf.Bytes(), []byte(`"jsonrpc":"2.0"`)) {
		t.Fatalf("expected newline-delimited json payload, got %q", buf.String())
	}
}
