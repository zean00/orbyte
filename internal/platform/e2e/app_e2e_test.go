package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"orbyte/internal/modules"
	platformapp "orbyte/internal/platform/app"
	"orbyte/internal/platform/document"
)

type harness struct {
	app      *platformapp.App
	baseURL  *url.URL
	cancel   context.CancelFunc
	client   *http.Client
	server   *httptest.Server
	testingT *testing.T
}

type response struct {
	status int
	body   []byte
	header http.Header
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	t.Setenv("APP_JWT_SECRET", "e2e-test-secret")
	t.Setenv("APP_BOOTSTRAP_ADMIN_PASSWORD", "admin123!")
	t.Setenv("DATABASE_URL", "")

	app, err := platformapp.New(platformapp.Options{Profile: modules.ProfileAll})
	if err != nil {
		t.Fatalf("boot app failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.StartBackground(ctx)

	server := httptest.NewServer(app.Handler())
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url failed: %v", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar failed: %v", err)
	}
	h := &harness{
		app:      app,
		baseURL:  baseURL,
		cancel:   cancel,
		client:   &http.Client{Jar: jar},
		server:   server,
		testingT: t,
	}
	t.Cleanup(func() {
		server.Close()
		cancel()
		if err := app.Close(); err != nil {
			t.Fatalf("close app failed: %v", err)
		}
	})
	h.login("admin", "admin123!", "loc_hq")
	return h
}

func TestBlackBoxDocumentAuditAndProjectionFlow(t *testing.T) {
	h := newHarness(t)

	created := h.requestJSON(http.MethodPost, "/documents", map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload": map[string]any{
			"title": "E2E Audited Request",
		},
	})
	if created.status != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", created.status, string(created.body))
	}

	var record document.Record
	if err := json.Unmarshal(created.body, &record); err != nil {
		t.Fatalf("decode created document failed: %v", err)
	}
	if record.Header.ID == "" {
		t.Fatal("expected created document id")
	}

	h.waitFor(5*time.Second, func() error {
		result := h.requestJSON(http.MethodGet, "/ops/search/indexes/documents.requests.search/consistency", nil)
		if result.status != http.StatusOK {
			return fmt.Errorf("search consistency returned %d", result.status)
		}
		var payload struct {
			Status       string `json:"status"`
			MissingCount int    `json:"missing_count"`
		}
		if err := json.Unmarshal(result.body, &payload); err != nil {
			return err
		}
		if payload.Status != "ok" || payload.MissingCount != 0 {
			return fmt.Errorf("search consistency not healthy yet: status=%s missing=%d", payload.Status, payload.MissingCount)
		}
		return nil
	})

	h.waitFor(5*time.Second, func() error {
		result := h.requestJSON(http.MethodGet, "/ops/projections/status", nil)
		if result.status != http.StatusOK {
			return fmt.Errorf("projection status returned %d", result.status)
		}
		var payload struct {
			Coverage struct {
				Status       string `json:"status"`
				MissingCount int    `json:"missing_count"`
			} `json:"coverage"`
		}
		if err := json.Unmarshal(result.body, &payload); err != nil {
			return err
		}
		if payload.Coverage.Status != "ok" || payload.Coverage.MissingCount != 0 {
			return fmt.Errorf("projection coverage not healthy yet: status=%s missing=%d", payload.Coverage.Status, payload.Coverage.MissingCount)
		}
		return nil
	})

	updated := h.requestJSON(http.MethodPut, "/documents/"+record.Header.ID, map[string]any{
		"payload": map[string]any{
			"title": "E2E Audited Request V2",
		},
	})
	if updated.status != http.StatusOK {
		t.Fatalf("expected document update to succeed, got %d body=%s", updated.status, string(updated.body))
	}

	h.waitFor(5*time.Second, func() error {
		result := h.requestJSON(http.MethodGet, "/ops/audit-events?target_type=document&target_id="+url.QueryEscape(record.Header.ID)+"&action=document.update", nil)
		if result.status != http.StatusOK {
			return fmt.Errorf("audit query returned %d", result.status)
		}
		var payload struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(result.body, &payload); err != nil {
			return err
		}
		var sawUpdate bool
		for _, item := range payload.Items {
			action, _ := item["action"].(string)
			switch action {
			case "document.update":
				sawUpdate = true
			}
		}
		if !sawUpdate {
			return fmt.Errorf("audit events not complete yet")
		}
		return nil
	})

	timeline := h.requestJSON(http.MethodGet, "/ops/audit-events/document/"+record.Header.ID, nil)
	if timeline.status != http.StatusOK {
		t.Fatalf("expected audit timeline to succeed, got %d body=%s", timeline.status, string(timeline.body))
	}
	var timelinePayload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(timeline.body, &timelinePayload); err != nil {
		t.Fatalf("decode audit timeline failed: %v", err)
	}
	if len(timelinePayload.Items) == 0 {
		t.Fatal("expected document audit timeline events")
	}

	status := h.requestJSON(http.MethodGet, "/ops/projections/status", nil)
	if status.status != http.StatusOK {
		t.Fatalf("expected projection status route to succeed, got %d body=%s", status.status, string(status.body))
	}
	var statusPayload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(status.body, &statusPayload); err != nil {
		t.Fatalf("decode projection status failed: %v", err)
	}
	if len(statusPayload.Items) == 0 {
		t.Fatal("expected projection status items")
	}
}

func TestBlackBoxAdminControlPlaneFlow(t *testing.T) {
	h := newHarness(t)

	defs := h.requestJSON(http.MethodGet, "/admin/api/feature-flags/definitions", nil)
	if defs.status != http.StatusOK {
		t.Fatalf("expected feature-flag definitions to load, got %d body=%s", defs.status, string(defs.body))
	}

	ouID := "ou_e2e_ops"
	ou := h.requestJSON(http.MethodPut, "/admin/api/operating-units/"+ouID+"/value", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"key":             "e2e_ops",
		"name":            "E2E Ops",
		"status":          "active",
	})
	if ou.status != http.StatusOK {
		t.Fatalf("expected operating unit upsert to succeed, got %d body=%s", ou.status, string(ou.body))
	}

	update := h.requestJSON(http.MethodPut, "/admin/api/feature-flags/platform.admin_console/value", map[string]any{
		"scope":             "operating_unit",
		"scope_id":          ouID,
		"organization_id":   "org_default",
		"location_id":       "loc_hq",
		"operating_unit_id": ouID,
		"enabled":           false,
		"status":            "active",
	})
	if update.status != http.StatusOK {
		t.Fatalf("expected feature-flag update to succeed, got %d body=%s", update.status, string(update.body))
	}

	effective := h.requestJSON(http.MethodGet, "/admin/api/feature-flags/effective?location_id=loc_hq&operating_unit_id="+url.QueryEscape(ouID), nil)
	if effective.status != http.StatusOK {
		t.Fatalf("expected effective feature-flag read to succeed, got %d body=%s", effective.status, string(effective.body))
	}
	if !bytes.Contains(effective.body, []byte("platform.admin_console")) {
		t.Fatalf("expected effective flag payload to include platform.admin_console, got %s", string(effective.body))
	}

	principal := h.requestJSON(http.MethodPost, "/service-principals", map[string]any{
		"key":                     "e2e_projection_worker",
		"allowed_operation_types": []string{"projection.refresh"},
		"credential_ref":          "bootstrap://e2e-projection-worker",
	})
	if principal.status != http.StatusCreated {
		t.Fatalf("expected service principal create to succeed, got %d body=%s", principal.status, string(principal.body))
	}
	var createdPrincipal struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(principal.body, &createdPrincipal); err != nil {
		t.Fatalf("decode service principal failed: %v", err)
	}
	if createdPrincipal.ID == "" {
		t.Fatal("expected service principal id")
	}

	token := h.requestJSON(http.MethodPost, "/service-principals/"+createdPrincipal.ID+"/tokens", map[string]any{
		"ttl_seconds": 300,
	})
	if token.status != http.StatusOK {
		t.Fatalf("expected service principal token issuance to succeed, got %d body=%s", token.status, string(token.body))
	}

	statusUpdate := h.requestJSON(http.MethodPut, "/service-principals/"+createdPrincipal.ID+"/status", map[string]any{
		"status": "disabled",
	})
	if statusUpdate.status != http.StatusOK {
		t.Fatalf("expected service principal status update to succeed, got %d body=%s", statusUpdate.status, string(statusUpdate.body))
	}

	list := h.requestJSON(http.MethodGet, "/service-principals", nil)
	if list.status != http.StatusOK {
		t.Fatalf("expected service principal list to succeed, got %d body=%s", list.status, string(list.body))
	}
	if !bytes.Contains(list.body, []byte("e2e_projection_worker")) {
		t.Fatalf("expected service principal list to include created principal, got %s", string(list.body))
	}
}

func TestBlackBoxWorkflowPolicyRuntimeValidation(t *testing.T) {
	h := newHarness(t)

	created := h.requestJSON(http.MethodPost, "/documents", map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload": map[string]any{
			"title": "Policy Runtime Validation",
		},
	})
	if created.status != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", created.status, string(created.body))
	}

	var record document.Record
	if err := json.Unmarshal(created.body, &record); err != nil {
		t.Fatalf("decode created document failed: %v", err)
	}

	update := h.requestJSON(http.MethodPut, "/admin/api/security/policy-hooks/documents.workflow.transition/rego", map[string]any{
		"scope":  "deployment",
		"source": "package orbyte.policy.documents.workflow.transition\n\nimport rego.v1\n\ndecision := true",
	})
	if update.status != http.StatusOK {
		t.Fatalf("expected rego policy update to succeed, got %d body=%s", update.status, string(update.body))
	}
	if !bytes.Contains(update.body, []byte(`"eval_valid":false`)) {
		t.Fatalf("expected runtime response to report invalid evaluation shape, got %s", string(update.body))
	}

	validate := h.requestJSON(http.MethodPost, "/admin/api/workflows/generic_request_flow/versions/1/validate", nil)
	if validate.status != http.StatusOK {
		t.Fatalf("expected workflow validate to succeed, got %d body=%s", validate.status, string(validate.body))
	}
	if !bytes.Contains(validate.body, []byte(`"valid":false`)) || !bytes.Contains(validate.body, []byte("documents.workflow.transition")) {
		t.Fatalf("expected workflow validation to include runtime issue, got %s", string(validate.body))
	}

	submit := h.requestJSON(http.MethodPost, "/documents/"+record.Header.ID+"/actions", map[string]any{
		"action": "submit",
	})
	if submit.status != http.StatusForbidden {
		t.Fatalf("expected submit to fail closed on invalid policy runtime, got %d body=%s", submit.status, string(submit.body))
	}
	if !bytes.Contains(submit.body, []byte("workflow policy runtime invalid for documents.workflow.transition")) {
		t.Fatalf("expected submit failure to identify the invalid hook, got %s", string(submit.body))
	}
}

func TestBlackBoxEngagementFlow(t *testing.T) {
	h := newHarness(t)

	programKey := fmt.Sprintf("e2e_loyalty_%d", time.Now().UTC().UnixNano())

	createProgram := h.mcpCall("engagement.program.create", map[string]any{
		"program_key":   programKey,
		"name":          "E2E Loyalty",
		"subject_type":  "user",
		"confirm_apply": true,
	})
	if createProgram.status != http.StatusOK {
		t.Fatalf("expected engagement program create to succeed, got %d body=%s", createProgram.status, string(createProgram.body))
	}

	createVersion := h.mcpCall("engagement.program.version.create", map[string]any{
		"program_key":   programKey,
		"confirm_apply": true,
	})
	if createVersion.status != http.StatusOK {
		t.Fatalf("expected engagement version create to succeed, got %d body=%s", createVersion.status, string(createVersion.body))
	}
	version := h.mcpStructuredValue(createVersion, "version")
	versionNumber, ok := version.(float64)
	if !ok || int(versionNumber) <= 0 {
		t.Fatalf("expected engagement version number, got %#v", version)
	}

	saveVersion := h.mcpCall("engagement.program.version.save", map[string]any{
		"program_key":   programKey,
		"version":       int(versionNumber),
		"confirm_apply": true,
		"rules": []map[string]any{
			{
				"key":                "submit_points",
				"action":             "credit_points",
				"source_event_types": []string{"document.submitted"},
				"subject_source":     "actor_id",
				"account_key":        "points",
				"fixed_amount":       10,
			},
			{
				"key":                "bronze_tier",
				"action":             "set_tier",
				"source_event_types": []string{"document.submitted"},
				"subject_source":     "actor_id",
				"account_key":        "points",
				"threshold":          10,
				"tier_key":           "bronze",
			},
		},
	})
	if saveVersion.status != http.StatusOK {
		t.Fatalf("expected engagement version save to succeed, got %d body=%s", saveVersion.status, string(saveVersion.body))
	}

	publishVersion := h.mcpCall("engagement.program.version.publish", map[string]any{
		"program_key":   programKey,
		"version":       int(versionNumber),
		"confirm_apply": true,
	})
	if publishVersion.status != http.StatusOK {
		t.Fatalf("expected engagement version publish to succeed, got %d body=%s", publishVersion.status, string(publishVersion.body))
	}

	created := h.requestJSON(http.MethodPost, "/documents", map[string]any{
		"type":            "generic_request",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"payload": map[string]any{
			"title": "Engagement E2E Request",
		},
	})
	if created.status != http.StatusCreated {
		t.Fatalf("expected document create to succeed, got %d body=%s", created.status, string(created.body))
	}

	var record document.Record
	if err := json.Unmarshal(created.body, &record); err != nil {
		t.Fatalf("decode created document failed: %v", err)
	}

	submit := h.requestJSON(http.MethodPost, "/documents/"+record.Header.ID+"/actions", map[string]any{
		"action":           "submit",
		"expected_version": record.Header.Version,
		"expected_etag":    record.Header.ETag,
	})
	if submit.status != http.StatusOK {
		t.Fatalf("expected submit to succeed, got %d body=%s", submit.status, string(submit.body))
	}

	h.waitFor(5*time.Second, func() error {
		balance := h.mcpCall("engagement.balance.get", map[string]any{
			"program_key": programKey,
			"subject_id":  "user_admin",
			"account_key": "points",
		})
		if balance.status != http.StatusOK {
			return fmt.Errorf("balance call returned %d", balance.status)
		}
		var payload map[string]any
		if err := json.Unmarshal(balance.body, &payload); err != nil {
			return err
		}
		if rpcErr, ok := payload["error"]; ok && rpcErr != nil {
			return fmt.Errorf("balance not ready yet: %v", rpcErr)
		}
		result, _ := payload["result"].(map[string]any)
		structured, _ := result["structuredContent"].(map[string]any)
		value := structured["balance"]
		number, ok := value.(float64)
		if !ok || int(number) != 10 {
			return fmt.Errorf("balance not ready yet: %#v", value)
		}
		return nil
	})

	qualification := h.mcpCall("engagement.qualification.get", map[string]any{
		"program_key": programKey,
		"subject_id":  "user_admin",
	})
	if qualification.status != http.StatusOK {
		t.Fatalf("expected qualification read to succeed, got %d body=%s", qualification.status, string(qualification.body))
	}
	if tier, _ := h.mcpStructuredValue(qualification, "tier_key").(string); tier != "bronze" {
		t.Fatalf("expected bronze tier, got %#v", h.mcpStructuredValue(qualification, "tier_key"))
	}

	journal := h.mcpCall("engagement.journal.list", map[string]any{
		"program_key": programKey,
		"subject_id":  "user_admin",
		"account_key": "points",
	})
	if journal.status != http.StatusOK {
		t.Fatalf("expected journal list to succeed, got %d body=%s", journal.status, string(journal.body))
	}
	items := h.mcpStructuredItems(journal)
	if len(items) != 1 {
		t.Fatalf("expected one journal entry, got %d body=%s", len(items), string(journal.body))
	}
}

func (h *harness) login(username, password, locationID string) {
	h.testingT.Helper()
	result := h.requestJSON(http.MethodPost, "/auth/login", map[string]any{
		"username":    username,
		"password":    password,
		"location_id": locationID,
	})
	if result.status != http.StatusOK {
		h.testingT.Fatalf("login failed: %d body=%s", result.status, string(result.body))
	}
	if h.csrfToken() == "" {
		h.testingT.Fatal("expected csrf cookie after login")
	}
}

func (h *harness) requestJSON(method, path string, payload any) response {
	h.testingT.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			h.testingT.Fatalf("marshal request failed: %v", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, body)
	if err != nil {
		h.testingT.Fatalf("build request failed: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requiresCSRF(method) {
		if token := h.csrfToken(); token != "" {
			req.Header.Set("X-CSRF-Token", token)
			req.Header.Set("Origin", h.server.URL)
		}
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.testingT.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		h.testingT.Fatalf("read response failed: %v", err)
	}
	return response{status: resp.StatusCode, body: raw, header: resp.Header.Clone()}
}

func (h *harness) mcpCall(name string, arguments map[string]any) response {
	h.testingT.Helper()
	return h.requestJSON(http.MethodPost, "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	})
}

func (h *harness) mcpStructuredValue(resp response, key string) any {
	h.testingT.Helper()
	var payload map[string]any
	if err := json.Unmarshal(resp.body, &payload); err != nil {
		h.testingT.Fatalf("decode mcp response failed: %v body=%s", err, string(resp.body))
	}
	if rpcErr, ok := payload["error"]; ok && rpcErr != nil {
		h.testingT.Fatalf("unexpected mcp error: %v body=%s", rpcErr, string(resp.body))
	}
	result, _ := payload["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	return structured[key]
}

func (h *harness) mcpStructuredItems(resp response) []map[string]any {
	h.testingT.Helper()
	var payload map[string]any
	if err := json.Unmarshal(resp.body, &payload); err != nil {
		h.testingT.Fatalf("decode mcp response failed: %v body=%s", err, string(resp.body))
	}
	if rpcErr, ok := payload["error"]; ok && rpcErr != nil {
		h.testingT.Fatalf("unexpected mcp error: %v body=%s", rpcErr, string(resp.body))
	}
	result, _ := payload["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	rawItems, _ := structured["items"].([]any)
	items := make([]map[string]any, 0, len(rawItems))
	for _, item := range rawItems {
		if mapped, ok := item.(map[string]any); ok {
			items = append(items, mapped)
		}
	}
	return items
}

func (h *harness) csrfToken() string {
	for _, cookie := range h.client.Jar.Cookies(h.baseURL) {
		if cookie.Name == "orbyte_csrf" {
			return cookie.Value
		}
	}
	return ""
}

func (h *harness) waitFor(timeout time.Duration, check func() error) {
	h.testingT.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := check(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.testingT.Fatalf("condition not met within %s: %v", timeout, lastErr)
}

func requiresCSRF(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}
