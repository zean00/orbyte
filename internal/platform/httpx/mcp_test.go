package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
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
	found := false
	for _, tool := range listResp.Result.Tools {
		if tool.Name == "analytics.snapshot.get" {
			found = true
			break
		}
	}
	if !found {
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
	if appMeta["stream_uri"] != "/mcp/analytics/events/analytics/snapshot" {
		t.Fatalf("expected app stream uri, got %+v", appMeta)
	}
}

func TestMCPScopedAnalyticsRouteFiltersSurface(t *testing.T) {
	h := newTestHarness(t)

	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	rr := h.request("POST", "/mcp/analytics", reqBody, true)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Result struct {
			Tools []struct {
				Name  string `json:"name"`
				Scope string `json:"scope"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	foundAnalytics := false
	for _, tool := range listResp.Result.Tools {
		if tool.Name == "template.definition.list" {
			t.Fatalf("did not expect template tool on scoped analytics endpoint: %+v", listResp.Result.Tools)
		}
		if tool.Scope != "analytics" {
			t.Fatalf("expected analytics scope, got %+v", tool)
		}
		if tool.Name == "analytics.snapshot.get" {
			foundAnalytics = true
		}
	}
	if !foundAnalytics {
		t.Fatalf("expected analytics tool on scoped endpoint, got %+v", listResp.Result.Tools)
	}

	reqBody, _ = json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "template.definition.list",
			"arguments": map[string]any{},
		},
	})
	rr = h.request("POST", "/mcp/analytics", reqBody, true)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "not available on this endpoint") {
		t.Fatalf("expected scoped endpoint rejection, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMCPRouteListsWorkflowToolsAndRejectsPublishWithoutConfirmation(t *testing.T) {
	h := newTestHarness(t)
	if err := h.cfg.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 false,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}

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
	found := false
	for _, tool := range listResp.Result.Tools {
		if tool.Name == "workflow.definition.list" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected workflow mcp tool, got %+v", listResp.Result.Tools)
	}

	reqBody, _ = json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "workflow.draft.publish",
			"arguments": map[string]any{
				"workflow_key": "generic_request_flow",
				"version":      1,
			},
		},
	})
	rr = h.request("POST", "/mcp", reqBody, true)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "confirm_publish") {
		t.Fatalf("expected publish confirmation rejection, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMCPRouteAppliesGovernancePolicyMetadataAndBlocksMutation(t *testing.T) {
	h := newTestHarness(t)
	if err := h.cfg.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 true,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        "[]",
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}

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
				Name        string `json:"name"`
				PolicyState string `json:"policyState"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	foundBlocked := false
	for _, tool := range listResp.Result.Tools {
		if tool.Name == "module.enable" {
			foundBlocked = true
			if tool.PolicyState != "blocked" {
				t.Fatalf("expected blocked policy state, got %+v", tool)
			}
		}
	}
	if !foundBlocked {
		t.Fatalf("expected module.enable in tools/list, got %+v", listResp.Result.Tools)
	}

	reqBody, _ = json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "module.enable",
			"arguments": map[string]any{
				"module_key":    "analytics",
				"confirm_apply": true,
			},
		},
	})
	rr = h.request("POST", "/mcp", reqBody, true)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "blocked by policy") {
		t.Fatalf("expected governance policy rejection, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMCPRouteAuditsCallTimeGovernanceStateFromArguments(t *testing.T) {
	h := newTestHarness(t)
	if err := h.cfg.Save(config.Entry{
		Key:   "platform.mcp",
		Scope: "deployment",
		Value: map[string]any{
			"enabled":                            true,
			"governance_enabled":                 true,
			"default_action_mode":                "draft_only",
			"tool_states_json":                   "{}",
			"blocked_action_classes_json":        "[]",
			"blocked_tool_keys_json":             "[]",
			"blocked_document_types_json":        `["promotion_plan"]`,
			"allowed_submit_document_types_json": "[]",
			"domain_policy_overrides_json":       "{}",
		},
	}); err != nil {
		t.Fatalf("save platform.mcp config failed: %v", err)
	}

	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "business.document.draft.create",
			"arguments": map[string]any{
				"document_type": "promotion_plan",
				"confirm_apply": true,
				"payload":       map[string]any{"name": "Spring Promo"},
			},
		},
	})
	rr := h.request("POST", "/mcp", reqBody, true)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "blocked by policy") {
		t.Fatalf("expected governance policy rejection, got %d body=%s", rr.Code, rr.Body.String())
	}

	found := false
	for _, event := range h.audit.Query(audit.Query{TargetType: "mcp_tool", TargetID: "business.document.draft.create"}) {
		if state, _ := event.Metadata["policy_state"].(string); state == "blocked" {
			reason, _ := event.Metadata["policy_reason"].(string)
			if strings.Contains(reason, "promotion_plan") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected MCP audit event to record execution-time blocked policy state for document_type")
	}
}

func TestMCPAnalyticsSnapshotStreamSendsInitialAndLiveUpdates(t *testing.T) {
	h := newTestHarness(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/mcp/analytics/events/analytics/snapshot", nil).WithContext(ctx)
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

func TestMCPRouteServicePrincipalDelegation(t *testing.T) {
	h := newTestHarness(t)
	principal, err := h.ident.UpsertServicePrincipal(identity.ServicePrincipal{Key: "mcp_agent", Status: "active"})
	if err != nil {
		t.Fatalf("create service principal failed: %v", err)
	}
	token, err := identity.NewTokenManagerFromEnv().IssueServicePrincipalToken(principal, time.Hour)
	if err != nil {
		t.Fatalf("issue service principal token failed: %v", err)
	}

	grantBody, _ := json.Marshal(map[string]any{
		"delegate_kind":           "agent",
		"delegate_id":             principal.ID,
		"location_id":             "loc_hq",
		"allowed_permission_keys": []string{"analytics.read"},
		"expires_at":              time.Now().UTC().Add(time.Hour),
	})
	grantRR := h.request(http.MethodPost, "/me/delegations/outgoing", grantBody, true)
	if grantRR.Code != http.StatusCreated {
		t.Fatalf("expected agent delegation grant create to succeed, got %d body=%s", grantRR.Code, grantRR.Body.String())
	}
	var grant identity.DelegationGrant
	if err := json.Unmarshal(grantRR.Body.Bytes(), &grant); err != nil {
		t.Fatalf("decode agent delegation grant failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/service-principals/me/delegations/incoming", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected service principal incoming grants, got %d body=%s", rr.Code, rr.Body.String())
	}

	acceptReq := httptest.NewRequest(http.MethodPost, "/service-principals/me/delegations/incoming/"+grant.ID+"/accept", nil)
	acceptReq.Header.Set("Authorization", "Bearer "+token)
	acceptRR := httptest.NewRecorder()
	h.router.ServeHTTP(acceptRR, acceptReq)
	if acceptRR.Code != http.StatusOK {
		t.Fatalf("expected service principal accept to succeed, got %d body=%s", acceptRR.Code, acceptRR.Body.String())
	}

	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "analytics.snapshot.get",
			"arguments": map[string]any{},
		},
	})
	callReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(reqBody)))
	callReq.Header.Set("Authorization", "Bearer "+token)
	callReq.Header.Set(mcpDelegationGrantHeader, grant.ID)
	callRR := httptest.NewRecorder()
	h.router.ServeHTTP(callRR, callReq)
	if callRR.Code != http.StatusOK {
		t.Fatalf("expected delegated MCP call to succeed, got %d body=%s", callRR.Code, callRR.Body.String())
	}
	var resp struct {
		Result map[string]any `json:"result"`
		Error  map[string]any `json:"error"`
	}
	if err := json.Unmarshal(callRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode delegated MCP response failed: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("expected successful delegated MCP response, got %+v", resp.Error)
	}

	found := false
	for _, event := range h.audit.Query(audit.Query{TargetType: "mcp_tool", TargetID: "analytics.snapshot.get", OnBehalfOfUserID: "user_admin"}) {
		if event.ActorID == principal.ID && event.ActorKind == "service" && event.DelegationGrantID == grant.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected MCP audit event with service actor on behalf of user_admin")
	}
}
