package acp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
	svc := newACPService(t, true, `[{"key":"codex","name":"Codex","description":"ACP","command":"/bin/echo"}]`)
	if !svc.Enabled() {
		t.Fatal("expected service enabled")
	}
	items := svc.Providers()
	if len(items) != 1 {
		t.Fatalf("expected one provider, got %d", len(items))
	}
	if items[0].Key != "codex" || !items[0].Available {
		t.Fatalf("unexpected provider info: %#v", items[0])
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

func TestHandleNotificationAndSessionUpdate(t *testing.T) {
	svc := NewService(config.NewService(), nil)
	svc.sessions["session-1"] = &Session{ID: "session-1"}

	svc.handleNotification("session-1", "session/update", json.RawMessage(`{"sessionId":"remote","update":{"sessionUpdate":"agent_message_chunk","content":{"text":"hello"}}}`))
	svc.handleNotification("session-1", "other/event", json.RawMessage(`{}`))
	svc.handleSessionUpdate("session-1", "plan", map[string]any{"text": "step 1"})
	svc.handleSessionUpdate("session-1", "user_message_chunk", map[string]any{"text": "user"})
	svc.handleSessionUpdate("session-1", "unknown", map[string]any{"text": "system"})

	session, _ := svc.GetSession("session-1")
	if len(session.Messages) < 3 {
		t.Fatalf("expected accumulated messages, got %#v", session.Messages)
	}
	if len(session.CurrentPlan) != 1 || session.CurrentPlan[0].Content != "step 1" {
		t.Fatalf("unexpected plan entries: %#v", session.CurrentPlan)
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
