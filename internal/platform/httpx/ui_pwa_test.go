package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"orbyte/internal/platform/i18n"
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
		`rel="stylesheet" href="/ui/assets/platform.css?v=` + platformAssetVersion + `"`,
		`navigator.serviceWorker.register('/ui/sw.js'`,
		`indexedDB.open(offlineDBName, offlineDBVersion)`,
		`headers['X-CSRF-Token'] = csrf`,
		`id="shell-root"`,
		`id="shell-sidebar"`,
		`id="route-panel"`,
		`ensurePreviewOverlay()`,
		`id="locale-switcher"`,
		`id="admin-link-button"`,
		`id="logout-button"`,
		`/offline/bootstrap`,
		`/offline/sync`,
		`/auth/logout`,
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
	if !strings.Contains(swBody, "CACHE_NAME = 'orbyte-ui-shell-v2'") {
		t.Fatal("expected service worker cache name")
	}
	if !strings.Contains(swBody, "caches.match('/ui')") {
		t.Fatal("expected service worker offline fallback")
	}
	if !strings.Contains(swBody, "/ui/assets/platform.css") {
		t.Fatal("expected stylesheet in service worker cache list")
	}

	css := h.request(http.MethodGet, "/ui/assets/platform.css", nil, false)
	if css.Code != http.StatusOK {
		t.Fatalf("expected 200 from stylesheet route, got %d body=%s", css.Code, css.Body.String())
	}
	if !strings.Contains(css.Body.String(), ".menu-link") {
		t.Fatal("expected generated platform stylesheet")
	}
	for _, selector := range []string{
		".panel",
		".brand",
		".subtitle",
		".menu-list",
		".surface-switcher",
		".org-chart-shell",
		".preview-dialog",
		".pagination-bar",
	} {
		if !strings.Contains(css.Body.String(), selector) {
			t.Fatalf("expected stylesheet compatibility selector %q", selector)
		}
	}
}

func TestLocalePreferencePersistsThroughBootstrap(t *testing.T) {
	h := newTestHarness(t)

	localeReq := httptest.NewRequest(http.MethodGet, "/locale?locale=id-ID", nil)
	localeReq.RemoteAddr = "192.0.2.10:1234"
	localeRR := httptest.NewRecorder()
	h.router.ServeHTTP(localeRR, localeReq)
	if localeRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from locale route, got %d body=%s", localeRR.Code, localeRR.Body.String())
	}
	localeCookie := findCookieByName(localeRR.Result().Cookies(), i18n.LocaleCookieName)
	if localeCookie == nil {
		t.Fatal("expected locale cookie to be set")
	}
	if localeCookie.Value != "id" {
		t.Fatalf("expected normalized locale cookie, got %q", localeCookie.Value)
	}

	bootstrapReq := httptest.NewRequest(http.MethodGet, "/ui/bootstrap", nil)
	bootstrapReq.RemoteAddr = "192.0.2.10:1234"
	bootstrapReq.AddCookie(h.cookie)
	bootstrapReq.AddCookie(localeCookie)
	bootstrapRR := httptest.NewRecorder()
	h.router.ServeHTTP(bootstrapRR, bootstrapReq)
	if bootstrapRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from bootstrap, got %d body=%s", bootstrapRR.Code, bootstrapRR.Body.String())
	}
	var bootstrap map[string]any
	if err := json.Unmarshal(bootstrapRR.Body.Bytes(), &bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if bootstrap["locale"] != "id" {
		t.Fatalf("expected locale=id from bootstrap, got %+v", bootstrap["locale"])
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/admin/api/bootstrap", nil)
	adminReq.RemoteAddr = "192.0.2.10:1234"
	adminReq.AddCookie(h.cookie)
	adminReq.AddCookie(localeCookie)
	adminRR := httptest.NewRecorder()
	h.router.ServeHTTP(adminRR, adminReq)
	if adminRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from admin bootstrap, got %d body=%s", adminRR.Code, adminRR.Body.String())
	}
	var adminBootstrap map[string]any
	if err := json.Unmarshal(adminRR.Body.Bytes(), &adminBootstrap); err != nil {
		t.Fatalf("decode admin bootstrap: %v", err)
	}
	if adminBootstrap["locale"] != "id" {
		t.Fatalf("expected admin locale=id, got %+v", adminBootstrap["locale"])
	}
}

func TestUIShellIncludesIndonesianValueTranslations(t *testing.T) {
	h := newTestHarness(t)

	rr := h.request(http.MethodGet, "/ui", nil, false)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	for _, fragment := range []string{
		`value_approved: 'Disetujui'`,
		`value_active: 'Aktif'`,
		`action_submit: 'Ajukan'`,
		`/locale?locale=`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected UI shell to contain %q", fragment)
		}
	}
}

func TestAuthenticatedLocalePreferencePersistsPerUser(t *testing.T) {
	h := newTestHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/locale?locale=id", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(h.cookie)
	// Deliberately send a conflicting anonymous cookie; authenticated user preference should win.
	req.AddCookie(&http.Cookie{Name: i18n.LocaleCookieName, Value: "en"})
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from authenticated locale route, got %d body=%s", rr.Code, rr.Body.String())
	}
	if locale := h.ident.PreferredLocale("user_admin"); locale != "id" {
		t.Fatalf("expected persisted user locale=id, got %q", locale)
	}

	bootstrapReq := httptest.NewRequest(http.MethodGet, "/ui/bootstrap", nil)
	bootstrapReq.RemoteAddr = "192.0.2.10:1234"
	bootstrapReq.AddCookie(h.cookie)
	bootstrapReq.AddCookie(&http.Cookie{Name: i18n.LocaleCookieName, Value: "en"})
	bootstrapRR := httptest.NewRecorder()
	h.router.ServeHTTP(bootstrapRR, bootstrapReq)
	if bootstrapRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from bootstrap, got %d body=%s", bootstrapRR.Code, bootstrapRR.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(bootstrapRR.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if payload["locale"] != "id" {
		t.Fatalf("expected authenticated bootstrap to prefer per-user locale, got %+v", payload["locale"])
	}
}
