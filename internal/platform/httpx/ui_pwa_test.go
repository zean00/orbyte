package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestUIShellRegistersPWAAndOfflineRuntime(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/ui", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, fragment := range []string{
		`rel="manifest" href="/ui/manifest.webmanifest"`,
		`navigator.serviceWorker.register('/ui/sw.js'`,
		`indexedDB.open(offlineDBName, offlineDBVersion)`,
		`/offline/bootstrap`,
		`/offline/sync`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected UI shell to contain %q", fragment)
		}
	}
}

func TestUIManifestAndServiceWorkerRoutes(t *testing.T) {
	h := newTestHarness(t)

	manifest := h.request(http.MethodGet, "/ui/manifest.webmanifest", nil, false)
	if manifest.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", manifest.Code, manifest.Body.String())
	}
	var manifestPayload map[string]any
	if err := json.Unmarshal(manifest.Body.Bytes(), &manifestPayload); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifestPayload["start_url"] != "/ui" {
		t.Fatalf("expected start_url=/ui, got %+v", manifestPayload["start_url"])
	}

	sw := h.request(http.MethodGet, "/ui/sw.js", nil, false)
	if sw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", sw.Code, sw.Body.String())
	}
	swBody := sw.Body.String()
	if !strings.Contains(swBody, "CACHE_NAME = 'orbyte-ui-shell-v1'") {
		t.Fatal("expected service worker cache name")
	}
	if !strings.Contains(swBody, "caches.match('/ui')") {
		t.Fatal("expected service worker offline fallback")
	}
}
