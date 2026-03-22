package moduletest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"orbyte/internal/platform/app"
	"orbyte/internal/platform/module"
)

type Harness struct {
	t      testing.TB
	app    *app.App
	server *httptest.Server
	client *http.Client
}

func NewHarness(t testing.TB, manifests ...module.Manifest) *Harness {
	t.Helper()
	t.Setenv("APP_AUTH_DEV_MODE", "true")
	application, err := app.New(app.Options{BusinessManifests: manifests})
	if err != nil {
		t.Fatalf("build app: %v", err)
	}
	server := httptest.NewServer(application.Handler())
	t.Cleanup(func() {
		server.Close()
		_ = application.Close()
	})
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &Harness{
		t:      t,
		app:    application,
		server: server,
		client: &http.Client{Jar: jar},
	}
}

func (h *Harness) LoginAdmin() {
	h.t.Helper()
	payload := map[string]any{
		"username":    "admin",
		"password":    "admin123!",
		"location_id": "loc_hq",
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		h.t.Fatalf("login status=%d body=%s", resp.StatusCode, string(data))
	}
}

func (h *Harness) GetJSON(path string) map[string]any {
	h.t.Helper()
	resp, err := h.client.Get(h.server.URL + path)
	if err != nil {
		h.t.Fatalf("get %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		h.t.Fatalf("get %s status=%d body=%s", path, resp.StatusCode, string(data))
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		h.t.Fatalf("decode %s: %v", path, err)
	}
	return payload
}
