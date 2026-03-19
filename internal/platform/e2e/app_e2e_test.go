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
