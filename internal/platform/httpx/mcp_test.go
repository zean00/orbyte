package httpx

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

func TestMCPRouteListsToolsAndCallsAnalyticsSnapshot(t *testing.T) {
	h := newTestHarness(t)

	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	rr := h.request("POST", "/mcp", reqBody, true)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(listResp.Result.Tools) == 0 || listResp.Result.Tools[0].Name != "analytics.snapshot.get" {
		t.Fatalf("expected analytics mcp tool, got %+v", listResp.Result.Tools)
	}

	reqBody, _ = json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "analytics.snapshot.get",
			"arguments": map[string]any{},
		},
	})
	rr = h.request("POST", "/mcp", reqBody, true)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var callResp struct {
		Result struct {
			Meta map[string]any `json:"_meta"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &callResp); err != nil {
		t.Fatalf("decode tool call response failed: %v", err)
	}
	appMeta, _ := callResp.Result.Meta["orbyte/app"].(map[string]any)
	if appMeta["resource_uri"] != "orbyte://apps/analytics.cockpit" {
		t.Fatalf("expected app resource uri, got %+v", appMeta)
	}
	if appMeta["stream_uri"] != "/mcp/events/analytics/snapshot" {
		t.Fatalf("expected app stream uri, got %+v", appMeta)
	}
}

func TestMCPAnalyticsSnapshotStreamSendsInitialAndLiveUpdates(t *testing.T) {
	h := newTestHarness(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/mcp/events/analytics/snapshot", nil).WithContext(ctx)
	req.AddCookie(h.cookie)
	req.AddCookie(h.csrf)
	rr := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		h.router.ServeHTTP(rr, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		body := rr.Body.String()
		if strings.Contains(body, "event: snapshot") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected initial snapshot event, got %q", body)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := h.analytics.CaptureSnapshot(); err != nil {
		t.Fatalf("capture live snapshot failed: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for {
		if strings.Count(rr.Body.String(), "event: snapshot") >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected live snapshot event, got %q", rr.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream handler did not exit after cancellation")
	}
}
