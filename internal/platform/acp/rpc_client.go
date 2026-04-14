package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type rpcTransport string

const (
	rpcTransportFramed rpcTransport = "framed"
	rpcTransportJSONL  rpcTransport = "jsonl"
)

type rpcResponse struct {
	ID     int64             `json:"id,omitempty"`
	Result json.RawMessage   `json:"result,omitempty"`
	Error  *rpcResponseError `json:"error,omitempty"`
}

type rpcResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcMessage struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      *int64            `json:"id,omitempty"`
	Method  string            `json:"method,omitempty"`
	Params  json.RawMessage   `json:"params,omitempty"`
	Result  json.RawMessage   `json:"result,omitempty"`
	Error   *rpcResponseError `json:"error,omitempty"`
}

type acpClient struct {
	cmd              *exec.Cmd
	stdin            io.WriteCloser
	stdout           io.ReadCloser
	stderr           io.ReadCloser
	pending          map[int64]chan rpcResponse
	mu               sync.Mutex
	nextID           int64
	onNotification   func(method string, params json.RawMessage)
	onRequest        func(id int64, method string, params json.RawMessage)
	closed           chan struct{}
	readLoopComplete chan struct{}
	writeMessageFn   func(message any) error
	transport        rpcTransport
	mcpServers       []map[string]any
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
	Models    struct {
		CurrentModelID  string `json:"currentModelId"`
		AvailableModels []struct {
			ModelID string `json:"modelId"`
			Name    string `json:"name"`
		} `json:"availableModels"`
	} `json:"models"`
}

func startACPClient(ctx context.Context, provider Provider, onNotification func(method string, params json.RawMessage), onRequest func(id int64, method string, params json.RawMessage)) (*acpClient, error) {
	if strings.TrimSpace(provider.Command) == "" {
		return nil, errors.New("acp provider command is required")
	}
	commandPath, err := resolveACPCommandPath(provider.Command)
	if err != nil {
		return nil, err
	}
	args, err := validateCommandArgs(provider.Args)
	if err != nil {
		return nil, err
	}
	env, err := sanitizeEnvironment(provider.Env)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, commandPath, args...)
	if provider.Cwd != "" {
		cmd.Dir = provider.Cwd
	} else if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}
	cmd.Env = env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	client := &acpClient{
		cmd:              cmd,
		stdin:            stdin,
		stdout:           stdout,
		stderr:           stderr,
		pending:          map[int64]chan rpcResponse{},
		onNotification:   onNotification,
		onRequest:        onRequest,
		closed:           make(chan struct{}),
		readLoopComplete: make(chan struct{}),
		transport:        detectRPCTransport(provider),
		mcpServers:       discoverMCPServers(provider),
	}
	go client.readLoop()
	go drainStderr(stderr)
	return client, nil
}

func resolveACPCommandPath(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("acp provider command is required")
	}
	if strings.Contains(command, "\x00") || strings.ContainsAny(command, "\r\n") {
		return "", errors.New("acp provider command contains invalid characters")
	}
	if containsPathSeparator(command) && !filepath.IsAbs(command) {
		return "", fmt.Errorf("acp provider command must be an absolute path or a command name: %s", command)
	}
	if raw := strings.TrimSpace(os.Getenv("APP_ACP_ALLOWED_COMMANDS")); raw != "" {
		allowed := false
		for _, item := range strings.Split(raw, ",") {
			if strings.EqualFold(filepath.Base(command), strings.TrimSpace(item)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("acp provider command is not allowed: %s", filepath.Base(command))
		}
	}
	if filepath.IsAbs(command) {
		if err := ensureExecutable(command); err != nil {
			return "", err
		}
		return command, nil
	}
	resolved, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("acp provider command not found: %s", command)
	}
	if err := ensureExecutable(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func containsPathSeparator(value string) bool {
	return strings.ContainsAny(value, `/\`)
}

func validateCommandArgs(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}
	args := make([]string, len(values))
	for idx, arg := range values {
		if strings.Contains(arg, "\x00") || strings.ContainsAny(arg, "\r\n") {
			return nil, errors.New("acp provider argument contains invalid characters")
		}
		args[idx] = strings.TrimSpace(arg)
	}
	return args, nil
}

func sanitizeEnvironment(env map[string]string) ([]string, error) {
	base := os.Environ()
	for key, value := range env {
		key = strings.TrimSpace(key)
		if key == "" || !isSafeEnvVarName(key) {
			return nil, fmt.Errorf("acp provider environment key is invalid: %q", key)
		}
		base = append(base, key+"="+value)
	}
	return base, nil
}

func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("acp provider command is not executable: %s", path)
	}
	if runtime.GOOS == "windows" {
		if info.IsDir() {
			return fmt.Errorf("acp provider command is not executable: %s", path)
		}
		return nil
	}
	if info.IsDir() || info.Mode()&0111 == 0 {
		return fmt.Errorf("acp provider command is not executable: %s", path)
	}
	return nil
}

func isSafeEnvVarName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (c *acpClient) close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	<-c.readLoopComplete
	return c.cmd.Wait()
}

func (c *acpClient) initialize() (map[string]any, error) {
	params := map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]any{
				"readTextFile":  false,
				"writeTextFile": false,
			},
			"terminal": false,
		},
		"clientInfo": map[string]any{
			"name":    "orbyte",
			"title":   "Orbyte ACP Host",
			"version": "1.0.0",
		},
	}
	var result map[string]any
	if err := c.call("initialize", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *acpClient) newSession(cwd string) (newSessionResult, error) {
	var result newSessionResult
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	if !filepath.IsAbs(cwd) {
		if abs, err := filepath.Abs(cwd); err == nil {
			cwd = abs
		}
	}
	mcpServers := c.mcpServers
	if mcpServers == nil {
		mcpServers = []map[string]any{}
	}
	if err := c.call("session/new", map[string]any{"cwd": cwd, "mcpServers": mcpServers}, &result); err != nil {
		return newSessionResult{}, err
	}
	if strings.TrimSpace(result.SessionID) == "" {
		return newSessionResult{}, errors.New("acp agent returned empty session id")
	}
	return result, nil
}

func discoverMCPServers(provider Provider) []map[string]any {
	if servers := normalizeProvidedMCPServers(provider.MCPServers); len(servers) > 0 {
		return servers
	}
	configPath := opencodeConfigPath(provider)
	if strings.TrimSpace(configPath) == "" {
		return nil
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var payload struct {
		MCP map[string]json.RawMessage `json:"mcp"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	servers := make([]map[string]any, 0, len(payload.MCP))
	for key, rawServer := range payload.MCP {
		normalized, ok := normalizeMCPServer(key, rawServer)
		if ok {
			servers = append(servers, normalized)
		}
	}
	return servers
}

func normalizeProvidedMCPServers(items []map[string]any) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	servers := make([]map[string]any, 0, len(items))
	for _, item := range items {
		normalized, ok := normalizeMCPServer("", mustRawMessage(item))
		if ok {
			servers = append(servers, normalized)
		}
	}
	return servers
}

func mustRawMessage(payload map[string]any) json.RawMessage {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}

func normalizeMCPServer(name string, raw json.RawMessage) (map[string]any, bool) {
	trimmedName := strings.TrimSpace(name)
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}
	if trimmedName == "" {
		trimmedName = strings.TrimSpace(stringValue(payload["name"]))
	}
	if trimmedName == "" {
		return nil, false
	}
	if enabled, ok := payload["enabled"].(bool); ok && !enabled {
		return nil, false
	}
	serverType := strings.ToLower(strings.TrimSpace(stringValue(payload["type"])))
	url := strings.TrimSpace(stringValue(payload["url"]))
	switch serverType {
	case "remote", "http":
		if url == "" {
			return nil, false
		}
		server := map[string]any{
			"name":    trimmedName,
			"type":    "http",
			"url":     url,
			"headers": normalizeHeaderList(payload["headers"]),
		}
		if timeout, ok := normalizeNumericValue(payload["timeout"]); ok {
			server["timeout"] = timeout
		}
		if enabled, ok := payload["enabled"].(bool); ok {
			server["enabled"] = enabled
		}
		return server, true
	case "sse":
		if url == "" {
			return nil, false
		}
		server := map[string]any{
			"name":    trimmedName,
			"type":    "sse",
			"url":     url,
			"headers": normalizeHeaderList(payload["headers"]),
		}
		if timeout, ok := normalizeNumericValue(payload["timeout"]); ok {
			server["timeout"] = timeout
		}
		if enabled, ok := payload["enabled"].(bool); ok {
			server["enabled"] = enabled
		}
		return server, true
	case "local", "stdio", "command":
		command := strings.TrimSpace(stringValue(payload["command"]))
		if command == "" {
			return nil, false
		}
		server := map[string]any{
			"name":    trimmedName,
			"command": command,
			"args":    normalizeStringList(payload["args"]),
			"env":     normalizeEnvList(payload["env"]),
		}
		if enabled, ok := payload["enabled"].(bool); ok {
			server["enabled"] = enabled
		}
		return server, true
	default:
		return nil, false
	}
}

func normalizeHeaderList(value any) []map[string]string {
	switch headers := value.(type) {
	case map[string]any:
		if len(headers) == 0 {
			return []map[string]string{}
		}
		out := make([]map[string]string, 0, len(headers))
		for key, rawValue := range headers {
			name := strings.TrimSpace(key)
			headerValue := strings.TrimSpace(stringValue(rawValue))
			if name == "" || headerValue == "" {
				continue
			}
			out = append(out, map[string]string{
				"name":  name,
				"value": headerValue,
			})
		}
		return out
	case []any:
		out := make([]map[string]string, 0, len(headers))
		for _, rawHeader := range headers {
			item, ok := rawHeader.(map[string]any)
			if !ok {
				continue
			}
			name := strings.TrimSpace(stringValue(item["name"]))
			headerValue := strings.TrimSpace(stringValue(item["value"]))
			if name == "" || headerValue == "" {
				continue
			}
			out = append(out, map[string]string{
				"name":  name,
				"value": headerValue,
			})
		}
		return out
	default:
		return []map[string]string{}
	}
}

func normalizeStringList(value any) []string {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(stringValue(item)); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeEnvList(value any) []map[string]string {
	env, ok := value.(map[string]any)
	if !ok || len(env) == 0 {
		return []map[string]string{}
	}
	out := make([]map[string]string, 0, len(env))
	for key, rawValue := range env {
		name := strings.TrimSpace(key)
		envValue := strings.TrimSpace(stringValue(rawValue))
		if name == "" || envValue == "" {
			continue
		}
		out = append(out, map[string]string{
			"name":  name,
			"value": envValue,
		})
	}
	return out
}

func normalizeNumericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func opencodeConfigPath(provider Provider) string {
	if xdg := firstNonEmptyString(
		strings.TrimSpace(provider.Env["XDG_CONFIG_HOME"]),
		strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")),
	); xdg != "" {
		return filepath.Join(xdg, "opencode", "opencode.json")
	}
	if home := firstNonEmptyString(
		strings.TrimSpace(provider.Env["HOME"]),
		strings.TrimSpace(os.Getenv("HOME")),
	); home != "" {
		return filepath.Join(home, ".config", "opencode", "opencode.json")
	}
	return ""
}

func (c *acpClient) prompt(sessionID string, content []map[string]any) error {
	return c.call("session/prompt", map[string]any{"sessionId": sessionID, "prompt": content}, nil)
}

func (c *acpClient) setSessionMode(sessionID, modeID string) error {
	return c.call("session/set_mode", map[string]any{
		"sessionId": sessionID,
		"modeId":    modeID,
	}, nil)
}

func (c *acpClient) setSessionModel(sessionID, modelID string) (string, error) {
	var result map[string]any
	if err := c.call("session/set_model", map[string]any{
		"sessionId": sessionID,
		"modelId":   modelID,
	}, &result); err != nil {
		return "", err
	}
	return firstNonEmptyString(
		stringValue(nestedMap(nestedMap(result, "_meta"), "opencode")["modelId"]),
		modelID,
	), nil
}

func (c *acpClient) respond(id int64, result any, errResp *rpcResponseError) error {
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	if errResp != nil {
		msg["error"] = errResp
	} else {
		msg["result"] = result
	}
	return c.writeMessage(msg)
}

func (c *acpClient) call(method string, params any, out any) error {
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan rpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.writeMessage(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	resp, ok := <-ch
	if !ok {
		return errors.New("acp connection closed")
	}
	if resp.Error != nil {
		return fmt.Errorf("acp %s failed: %s", method, resp.Error.Message)
	}
	if out != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return err
		}
	}
	return nil
}

func (c *acpClient) writeMessage(message any) error {
	if c.writeMessageFn != nil {
		return c.writeMessageFn(message)
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if c.transport == rpcTransportJSONL {
		if _, err := c.stdin.Write(payload); err != nil {
			return err
		}
		_, err = c.stdin.Write([]byte{'\n'})
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := io.WriteString(c.stdin, header); err != nil {
		return err
	}
	_, err = c.stdin.Write(payload)
	return err
}

func (c *acpClient) readLoop() {
	defer close(c.readLoopComplete)
	reader := bufio.NewReader(c.stdout)
	for {
		select {
		case <-c.closed:
			c.closePending()
			return
		default:
		}
		payload, err := readRPCPayload(reader)
		if err != nil {
			c.closePending()
			return
		}
		var msg rpcMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}
		switch {
		case msg.ID != nil && msg.Method == "":
			c.mu.Lock()
			ch := c.pending[*msg.ID]
			delete(c.pending, *msg.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- rpcResponse{ID: *msg.ID, Result: msg.Result, Error: msg.Error}
				close(ch)
			}
		case msg.ID != nil && msg.Method != "":
			if c.onRequest != nil {
				c.onRequest(*msg.ID, msg.Method, msg.Params)
			} else {
				_ = c.respond(*msg.ID, nil, &rpcResponseError{Code: -32601, Message: "method not supported"})
			}
		case msg.Method != "":
			if c.onNotification != nil {
				c.onNotification(msg.Method, msg.Params)
			}
		}
	}
}

func (c *acpClient) closePending() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		delete(c.pending, id)
		close(ch)
	}
}

func readRPCPayload(reader *bufio.Reader) ([]byte, error) {
	for {
		peek, err := reader.Peek(1)
		if err != nil {
			return nil, err
		}
		switch peek[0] {
		case ' ', '\t', '\r', '\n':
			if _, err := reader.ReadByte(); err != nil {
				return nil, err
			}
			continue
		case '{', '[':
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if errors.Is(err, io.EOF) && len(line) > 0 {
					return bytes.TrimSpace(line), nil
				}
				return nil, err
			}
			return bytes.TrimSpace(line), nil
		}
		break
	}
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			raw := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(line), "content-length:"))
			contentLength, err = strconv.Atoi(raw)
			if err != nil {
				return nil, err
			}
		}
	}
	if contentLength <= 0 {
		return nil, errors.New("missing content length")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf), nil
}

func drainStderr(r io.Reader) {
	if r == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r)
}

func detectRPCTransport(provider Provider) rpcTransport {
	switch strings.ToLower(strings.TrimSpace(provider.Transport)) {
	case string(rpcTransportJSONL):
		return rpcTransportJSONL
	case "", "auto":
	default:
		return rpcTransportFramed
	}
	command := strings.ToLower(filepath.Base(strings.TrimSpace(provider.Command)))
	if command == "opencode" && len(provider.Args) > 0 && strings.EqualFold(strings.TrimSpace(provider.Args[0]), "acp") {
		return rpcTransportJSONL
	}
	return rpcTransportFramed
}
