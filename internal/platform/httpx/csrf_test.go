package httpx

import (
	"testing"
	"time"
)

func TestBuildCSRFCookieSetsHttpOnlyFlag(t *testing.T) {
	t.Setenv("APP_JWT_SECRET", "test-secret")
	cookie, err := buildCSRFCookie("session-id")
	if err != nil {
		t.Fatalf("buildCSRFCookie failed: %v", err)
	}
	if cookie.Name != csrfCookieName || cookie.Path != "/" {
		t.Fatalf("unexpected cookie path/name: %#v", cookie)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected csrf cookie to be HttpOnly")
	}
}

func TestClearedCSRFCookieSetsExpiringFlags(t *testing.T) {
	cookie := clearedCSRFCookie()
	if !cookie.HttpOnly {
		t.Fatal("expected cleared csrf cookie to be HttpOnly")
	}
	if cookie.Value != "" || cookie.MaxAge != -1 {
		t.Fatalf("expected cleared csrf cookie to reset value and expire immediately, got %+v", cookie)
	}
	if cookie.Expires.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("expected expired timestamp on cleared csrf cookie, got %v", cookie.Expires)
	}
}
