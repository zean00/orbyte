package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveFallsBackInOrder(t *testing.T) {
	text := LocalizedText{
		"en": "Request",
		"id": "Permintaan",
	}
	if got := Resolve("id-ID", text, "Fallback"); got != "Permintaan" {
		t.Fatalf("expected Indonesian translation, got %q", got)
	}
	if got := Resolve("fr", text, "Fallback"); got != "Request" {
		t.Fatalf("expected English fallback, got %q", got)
	}
	if got := Resolve("fr", nil, "Fallback"); got != "Fallback" {
		t.Fatalf("expected fallback text, got %q", got)
	}
}

func TestFromRequestUsesAcceptLanguage(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8")
	if got := FromRequest(req); got != "id" {
		t.Fatalf("expected id, got %q", got)
	}
}

func TestFromRequestPrefersLocaleCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	req.AddCookie(&http.Cookie{Name: LocaleCookieName, Value: "id"})
	if got := FromRequest(req); got != "id" {
		t.Fatalf("expected id from locale cookie, got %q", got)
	}
}

func TestSetCookieNormalizesLocale(t *testing.T) {
	rr := httptest.NewRecorder()
	SetCookie(rr, "id-ID")
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	if cookies[0].Name != LocaleCookieName {
		t.Fatalf("expected locale cookie, got %q", cookies[0].Name)
	}
	if cookies[0].Value != "id" {
		t.Fatalf("expected normalized locale value, got %q", cookies[0].Value)
	}
	if !cookies[0].HttpOnly {
		t.Fatal("expected locale cookie to be httpOnly")
	}
}
