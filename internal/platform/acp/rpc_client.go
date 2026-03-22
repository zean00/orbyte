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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
}

func startACPClient(ctx context.Context, provider Provider, onNotification func(method string, params json.RawMessage), onRequest func(id int64, method string, params json.RawMessage)) (*acpClient, error) {
	if strings.TrimSpace(provider.Command) == "" {
		return nil, errors.New("acp provider command is required")
	}
	cmd := exec.CommandContext(ctx, provider.Command, provider.Args...)
	if provider.Cwd != "" {
		cmd.Dir = provider.Cwd
	} else if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}
	env := os.Environ()
	for key, value := range provider.Env {
		env = append(env, key+"="+value)
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
	}
	go client.readLoop()
	go drainStderr(stderr)
	return client, nil
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

func (c *acpClient) newSession(cwd string) (string, error) {
	var result struct {
		SessionID string `json:"sessionId"`
	}
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
	if err := c.call("session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}}, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.SessionID) == "" {
		return "", errors.New("acp agent returned empty session id")
	}
	return result.SessionID, nil
}

func (c *acpClient) prompt(sessionID string, content []map[string]any) error {
	return c.call("session/prompt", map[string]any{"sessionId": sessionID, "prompt": content}, nil)
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
