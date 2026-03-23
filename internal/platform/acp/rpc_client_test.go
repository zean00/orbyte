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
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{"sessionId":"remote-1"}`)}
		case "session/prompt":
			ch <- rpcResponse{ID: id, Result: json.RawMessage(`{}`)}
		default:
			t.Fatalf("unexpected method: %s", method)
		}
		close(ch)
		return nil
	})

	result, err := client.initialize()
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	if result["protocolVersion"] != float64(1) {
		t.Fatalf("unexpected initialize result: %#v", result)
	}
	sessionID, err := client.newSession(".")
	if err != nil {
		t.Fatalf("newSession failed: %v", err)
	}
	if sessionID != "remote-1" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
	if err := client.prompt("remote-1", []map[string]any{{"type": "text", "text": "hello"}}); err != nil {
		t.Fatalf("prompt failed: %v", err)
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
