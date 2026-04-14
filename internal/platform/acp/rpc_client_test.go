package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestResolveACPCommandPathRejectsRelativeCommandWithPathSeparators(t *testing.T) {
	for _, command := range []string{"./bin/whatever", "bin/helper", "..\\config\\tool"} {
		if _, err := resolveACPCommandPath(command); err == nil {
			t.Fatalf("expected relative command %q with path separators to be rejected", command)
		}
	}
}

func TestResolveACPCommandPathResolvesAllowedCommandFromPathOrLookPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test targets unix-style command discovery")
	}
	command := "sh"
	t.Setenv("APP_ACP_ALLOWED_COMMANDS", "sh")
	resolved, err := resolveACPCommandPath(command)
	if err != nil {
		t.Fatalf("resolveACPCommandPath failed: %v", err)
	}
	if resolved == "" {
		t.Fatal("expected resolved command path")
	}

	tmp := t.TempDir()
	tmpCommand := filepath.Join(tmp, "acp-helper")
	if err := os.WriteFile(tmpCommand, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write temp command failed: %v", err)
	}
	allowedBase := filepath.Base(tmpCommand)
	t.Setenv("APP_ACP_ALLOWED_COMMANDS", allowedBase)
	if _, err := resolveACPCommandPath(tmpCommand); err != nil {
		t.Fatalf("expected absolute allowed command path to be accepted: %v", err)
	}
}

func TestResolveACPCommandPathRejectsDisallowedCommandName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test targets unix-style command discovery")
	}
	command := "sh"
	t.Setenv("APP_ACP_ALLOWED_COMMANDS", "cat")
	if _, err := resolveACPCommandPath(command); err == nil {
		t.Fatal("expected disallowed command to be rejected")
	}
}

func TestResolveACPCommandPathRejectsMalformedCommand(t *testing.T) {
	_, err := resolveACPCommandPath("command\x00with-null")
	if err == nil {
		t.Fatal("expected null bytes in command name to fail")
	}
	_, err = resolveACPCommandPath("command\nline")
	if err == nil {
		t.Fatal("expected newline in command name to fail")
	}
}

func TestValidateCommandArgsRejectsMalformedArguments(t *testing.T) {
	_, err := validateCommandArgs([]string{"good", "bad\narg"})
	if err == nil {
		t.Fatal("expected malformed arg to fail")
	}
	result, err := validateCommandArgs([]string{"  good  ", " spaced "})
	if err != nil {
		t.Fatalf("validateCommandArgs failed: %v", err)
	}
	if result[0] != "good" || result[1] != "spaced" {
		t.Fatalf("expected trimmed args, got %#v", result)
	}
}

func TestSanitizeEnvironmentRejectsBadVariableName(t *testing.T) {
	_, err := sanitizeEnvironment(map[string]string{"bad-key": "1", "GOOD": "2"})
	if err == nil {
		t.Fatal("expected invalid environment variable key to fail")
	}
	values, err := sanitizeEnvironment(map[string]string{"GOOD": "2", "  AlsoOK": "3", "PATH": os.Getenv("PATH")})
	if err != nil {
		t.Fatalf("sanitizeEnvironment failed: %v", err)
	}
	found := false
	for _, entry := range values {
		if strings.HasPrefix(entry, "AlsoOK=") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected trimmed environment key to be included")
	}
}

func TestEnsureExecutableRejectsNonExecutableBinaryOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode bits checks are unix-specific")
	}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "not-exec")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}
	if err := ensureExecutable(path); err == nil {
		t.Fatal("expected non-executable file to fail")
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	if err := ensureExecutable(path); err != nil {
		t.Fatalf("expected executable file after chmod to pass: %v", err)
	}
}

func TestResolveACPCommandPathRejectsCommandWithoutPermissionOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix exec-bit checks are unix-specific")
	}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "acp-helper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatalf("write temp command failed: %v", err)
	}
	if _, err := resolveACPCommandPath(path); err == nil {
		t.Fatal("expected non-executable absolute command path to be rejected")
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Setenv("APP_ACP_ALLOWED_COMMANDS", "")
	if _, err := resolveACPCommandPath(path); err != nil {
		t.Fatalf("expected executable path to be accepted: %v", err)
	}
}

func TestResolveACPCommandPathRejectsNonExistent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test targets unix-style command discovery")
	}
	_, err := resolveACPCommandPath("nonexistent-command")
	if err == nil {
		t.Fatal("expected missing command to fail")
	}
}

func framedPayload(payload string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
}

func TestReadRPCPayload(t *testing.T) {
	reader := bufio.NewReader(bytes.NewBufferString(framedPayload(`{"jsonrpc":"2.0","id":1}`)))
	payload, err := readRPCPayload(reader)
	if err != nil {
		t.Fatalf("readRPCPayload failed: %v", err)
	}
	if string(payload) != `{"jsonrpc":"2.0","id":1}` {
		t.Fatalf("unexpected payload: %s", string(payload))
	}

	reader = bufio.NewReader(bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":2}\n"))
	payload, err = readRPCPayload(reader)
	if err != nil {
		t.Fatalf("readRPCPayload jsonl failed: %v", err)
	}
	if string(payload) != `{"jsonrpc":"2.0","id":2}` {
		t.Fatalf("unexpected jsonl payload: %s", string(payload))
	}
}

func TestReadRPCPayloadErrors(t *testing.T) {
	reader := bufio.NewReader(bytes.NewBufferString("Content-Length: nope\r\n\r\n{}"))
	if _, err := readRPCPayload(reader); err == nil {
		t.Fatal("expected invalid content-length error")
	}
	reader = bufio.NewReader(bytes.NewBufferString("Header: value\r\n\r\n{}"))
	if _, err := readRPCPayload(reader); err == nil {
		t.Fatal("expected missing content-length error")
	}
}

func TestCallAndReadLoopDispatch(t *testing.T) {
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	defer stdoutR.Close()
	defer stdinR.Close()

	var notifications []string
	var requests []string
	client := &acpClient{
		stdin:            stdinW,
		stdout:           stdoutR,
		pending:          map[int64]chan rpcResponse{},
		closed:           make(chan struct{}),
		readLoopComplete: make(chan struct{}),
		onNotification: func(method string, params json.RawMessage) {
			notifications = append(notifications, method)
		},
		onRequest: func(id int64, method string, params json.RawMessage) {
			requests = append(requests, fmt.Sprintf("%d:%s", id, method))
		},
	}
	go client.readLoop()

	done := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stdinR)
		payload, err := readRPCPayload(reader)
		if err != nil {
			done <- err
			return
		}
		if !bytes.Contains(payload, []byte(`"method":"initialize"`)) {
			done <- fmt.Errorf("unexpected outbound request %q", string(payload))
			return
		}
		if _, err := io.WriteString(stdoutW, framedPayload(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1}}`)); err != nil {
			done <- err
			return
		}
		if _, err := io.WriteString(stdoutW, framedPayload(`{"jsonrpc":"2.0","method":"tick","params":{"ok":true}}`)); err != nil {
			done <- err
			return
		}
		if _, err := io.WriteString(stdoutW, framedPayload(`{"jsonrpc":"2.0","id":9,"method":"client/request","params":{"kind":"write"}}`)); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	var result map[string]any
	if err := client.call("initialize", map[string]any{"hello": "world"}, &result); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if result["protocolVersion"] != float64(1) {
		t.Fatalf("unexpected call result: %#v", result)
	}
	if err := <-done; err != nil {
		t.Fatalf("io goroutine failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	close(client.closed)
	stdoutW.Close()
	<-client.readLoopComplete
	if len(notifications) != 1 || notifications[0] != "tick" {
		t.Fatalf("unexpected notifications: %#v", notifications)
	}
	if len(requests) != 1 || requests[0] != "9:client/request" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
}

func TestClosePendingAndClose(t *testing.T) {
	ch := make(chan rpcResponse)
	client := &acpClient{pending: map[int64]chan rpcResponse{1: ch}}
	client.closePending()
	if len(client.pending) != 0 {
		t.Fatalf("expected cleared pending, got %#v", client.pending)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel")
		}
	default:
		t.Fatal("expected pending channel closed")
	}

	if runtime.GOOS == "windows" {
		t.Skip("shell test uses /bin/sh")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cl, err := startACPClient(ctx, Provider{
		Key:     "shell",
		Name:    "shell",
		Command: "/bin/sh",
		Args:    []string{"-lc", "cat"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("startACPClient failed: %v", err)
	}
	if err := cl.close(); err != nil && !strings.Contains(err.Error(), "killed") {
		t.Fatalf("close failed: %v", err)
	}
}

func TestInitializeNewSessionAndPromptHelpers(t *testing.T) {
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		id := int64(message["id"].(float64))
		method := message["method"].(string)
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		switch method {
		case "initialize":
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"protocolVersion":1,"agentInfo":{"name":"test"}}`)}
		case "session/new":
			params := message["params"].(map[string]any)
			if !filepath.IsAbs(params["cwd"].(string)) {
				t.Fatalf("expected absolute cwd, got %#v", params)
			}
			got, _ := params["mcpServers"].([]any)
			if len(got) != 1 {
				t.Fatalf("expected discovered mcp servers, got %#v", params["mcpServers"])
			}
			server, _ := got[0].(map[string]any)
			if server["name"] != "orbyte" || server["type"] != "http" || server["url"] != "http://127.0.0.1:18110/mcp" {
				t.Fatalf("unexpected normalized mcp server %#v", got[0])
			}
			if server["timeout"] != float64(120000) {
				t.Fatalf("expected normalized timeout, got %#v", server["timeout"])
			}
			headers, _ := server["headers"].([]any)
			if len(headers) != 1 {
				t.Fatalf("expected one normalized header, got %#v", server["headers"])
			}
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"sessionId":"remote-1","models":{"currentModelId":"opencode/big-pickle","availableModels":[{"modelId":"opencode/big-pickle","name":"Big Pickle"},{"modelId":"opencode/gpt-5-nano","name":"GPT-5 Nano"}]}}`)}
		case "session/prompt":
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{}`)}
		case "session/set_mode":
			params := message["params"].(map[string]any)
			if params["sessionId"] != "remote-1" || params["modeId"] != "plan" {
				t.Fatalf("unexpected set_mode params: %#v", params)
			}
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{}`)}
		case "session/set_model":
			params := message["params"].(map[string]any)
			if params["sessionId"] != "remote-1" || params["modelId"] != "opencode/minimax-m2.5-free" {
				t.Fatalf("unexpected set_model params: %#v", params)
			}
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"_meta":{"opencode":{"modelId":"opencode/minimax-m2.5-free"}}}`)}
		default:
			t.Fatalf("unexpected method: %s", method)
		}
		close(ch)
		return nil
	})
	client.mcpServers = []map[string]any{{
		"name":    "orbyte",
		"type":    "http",
		"url":     "http://127.0.0.1:18110/mcp",
		"timeout": float64(120000),
		"headers": []map[string]string{{
			"name":  "Authorization",
			"value": "Bearer test",
		}},
	}}

	result, err := client.initialize()
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	if result["protocolVersion"] != float64(1) {
		t.Fatalf("unexpected initialize result: %#v", result)
	}
	sessionInfo, err := client.newSession(".")
	if err != nil {
		t.Fatalf("newSession failed: %v", err)
	}
	if sessionInfo.SessionID != "remote-1" {
		t.Fatalf("unexpected session id: %q", sessionInfo.SessionID)
	}
	if sessionInfo.Models.CurrentModelID != "opencode/big-pickle" {
		t.Fatalf("unexpected current model: %#v", sessionInfo.Models)
	}
	if len(sessionInfo.Models.AvailableModels) != 2 {
		t.Fatalf("unexpected available models: %#v", sessionInfo.Models.AvailableModels)
	}
	modelID, err := client.setSessionModel("remote-1", "opencode/minimax-m2.5-free")
	if err != nil {
		t.Fatalf("setSessionModel failed: %v", err)
	}
	if modelID != "opencode/minimax-m2.5-free" {
		t.Fatalf("unexpected selected model: %q", modelID)
	}
	if err := client.setSessionMode("remote-1", "plan"); err != nil {
		t.Fatalf("setSessionMode failed: %v", err)
	}
	if err := client.prompt("remote-1", []map[string]any{{"type": "text", "text": "hello"}}); err != nil {
		t.Fatalf("prompt failed: %v", err)
	}
}

func TestNewSessionSendsEmptyMCPServerArrayWhenDiscoveryIsEmpty(t *testing.T) {
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		id := int64(message["id"].(float64))
		method := message["method"].(string)
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		switch method {
		case "session/new":
			params := message["params"].(map[string]any)
			got, ok := params["mcpServers"].([]any)
			if !ok {
				t.Fatalf("expected mcpServers array, got %#v", params["mcpServers"])
			}
			if len(got) != 0 {
				t.Fatalf("expected empty mcpServers array, got %#v", got)
			}
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"sessionId":"remote-empty","models":{"currentModelId":"opencode/big-pickle","availableModels":[]}}`)}
		default:
			t.Fatalf("unexpected method: %s", method)
		}
		close(ch)
		return nil
	})

	sessionInfo, err := client.newSession(".")
	if err != nil {
		t.Fatalf("newSession failed: %v", err)
	}
	if sessionInfo.SessionID != "remote-empty" {
		t.Fatalf("unexpected session id: %q", sessionInfo.SessionID)
	}
}

func TestDiscoverMCPServersFromProviderEnv(t *testing.T) {
	temp := t.TempDir()
	configDir := filepath.Join(temp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"mcp":{"orbyte":{"type":"remote","url":"http://127.0.0.1:18110/mcp","timeout":120000,"headers":{"Authorization":"Bearer test"}},"other":{"type":"local","command":"tool","args":["serve"],"env":{"MODE":"dev"}},"disabled":{"enabled":false,"type":"remote","url":"http://127.0.0.1:18110/mcp"}}}`), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	got := discoverMCPServers(Provider{Env: map[string]string{"HOME": temp}})
	if len(got) != 2 {
		t.Fatalf("expected 2 mcp servers, got %#v", got)
	}
	if got[0]["name"] != "orbyte" && got[1]["name"] != "orbyte" {
		t.Fatalf("expected remote mcp server to be normalized, got %#v", got)
	}
	for _, item := range got {
		if item["name"] == "orbyte" && item["timeout"] != float64(120000) {
			t.Fatalf("expected discovered timeout on remote server, got %#v", item["timeout"])
		}
	}
}

func TestDiscoverMCPServersFallsBackToProcessHome(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("HOME", temp)
	t.Setenv("XDG_CONFIG_HOME", "")
	configDir := filepath.Join(temp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"mcp":{"orbyte":{"type":"remote","url":"http://127.0.0.1:18110/mcp"}}}`), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	got := discoverMCPServers(Provider{})
	if len(got) != 1 || got[0]["name"] != "orbyte" {
		t.Fatalf("expected process-home mcp server, got %#v", got)
	}
}

func TestDiscoverMCPServersPrefersProviderConfig(t *testing.T) {
	got := discoverMCPServers(Provider{
		MCPServers: []map[string]any{{
			"name":    "orbyte-inline",
			"type":    "http",
			"url":     "http://127.0.0.1:18110/mcp",
			"enabled": true,
			"timeout": 120000,
			"headers": []map[string]any{{
				"name":  "Authorization",
				"value": "Bearer inline",
			}},
		}},
		Env: map[string]string{
			"HOME": t.TempDir(),
		},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 inline mcp server, got %#v", got)
	}
	if got[0]["name"] != "orbyte-inline" || got[0]["type"] != "http" || got[0]["url"] != "http://127.0.0.1:18110/mcp" {
		t.Fatalf("unexpected inline mcp server %#v", got[0])
	}
	if got[0]["timeout"] != float64(120000) {
		t.Fatalf("unexpected inline timeout %#v", got[0]["timeout"])
	}
	headers, _ := got[0]["headers"].([]map[string]string)
	if len(headers) != 1 || headers[0]["name"] != "Authorization" || headers[0]["value"] != "Bearer inline" {
		t.Fatalf("unexpected inline headers %#v", got[0]["headers"])
	}
}

func TestCallReturnsRemoteErrorAndConnectionClosed(t *testing.T) {
	client := testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		id := int64(message["id"].(float64))
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		ch <- rpcResponse{ID: id, Error: &rpcResponseError{Code: -1, Message: "nope"}}
		close(ch)
		return nil
	})
	if err := client.call("initialize", nil, nil); err == nil {
		t.Fatal("expected remote error")
	}

	client = testClientWithResponses(t, func(message map[string]any, c *acpClient) error {
		id := int64(message["id"].(float64))
		c.mu.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		close(ch)
		return nil
	})
	if err := client.call("initialize", nil, nil); err == nil {
		t.Fatal("expected connection closed error")
	}
}

func TestDetectRPCTransport(t *testing.T) {
	if got := detectRPCTransport(Provider{Command: "/home/user/.opencode/bin/opencode", Args: []string{"acp"}}); got != rpcTransportJSONL {
		t.Fatalf("expected opencode transport jsonl, got %q", got)
	}
	if got := detectRPCTransport(Provider{Command: "/bin/echo", Transport: "jsonl"}); got != rpcTransportJSONL {
		t.Fatalf("expected explicit jsonl transport, got %q", got)
	}
	if got := detectRPCTransport(Provider{Command: "/bin/echo"}); got != rpcTransportFramed {
		t.Fatalf("expected default framed transport, got %q", got)
	}
}
