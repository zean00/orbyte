package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	got := normalizeHost("https://app.example.com:8443")
	if got != "app.example.com" {
		t.Fatalf("expected normalized host without scheme and port, got %q", got)
	}
	got = normalizeHost(" app.example.com ")
	if got != "app.example.com" {
		t.Fatalf("expected trimmed host, got %q", got)
	}
}

func TestShouldTrustForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Host = "app.example.com"
	req.RemoteAddr = "127.0.0.1:4444"
	if shouldTrustForwardedHeaders(req, []string{"https://app.example.com"}) != true {
		t.Fatal("expected trusted host from trusted proxy source to enable forwarded headers")
	}

	req.Host = "other.example.com"
	if shouldTrustForwardedHeaders(req, []string{"https://app.example.com"}) != false {
		t.Fatal("expected non-trusted host to disable forwarded headers")
	}
}

func TestShouldTrustForwardedHeadersRequiresTrustedOriginConfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Host = "app.example.com"
	req.RemoteAddr = "127.0.0.1:4444"
	if shouldTrustForwardedHeaders(req, nil) != false {
		t.Fatal("expected false when trusted origins are empty")
	}
}

func TestShouldTrustForwardedHeadersRequiresTrustedProxySource(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Host = "app.example.com"
	req.RemoteAddr = "198.51.100.10:4444"
	if shouldTrustForwardedHeaders(req, []string{"https://app.example.com"}) != false {
		t.Fatal("expected public remote source to disable forwarded headers")
	}
}

func TestExtractClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "198.51.100.10:4444"
	req.Host = "app.example.com"
	if got := extractClientIP(req, req.RemoteAddr, []string{}); got != "198.51.100.10" {
		t.Fatalf("expected remote host fallback, got %q", got)
	}

	req.RemoteAddr = "127.0.0.1:4444"
	req.Header.Set("X-Forwarded-For", "203.0.113.11, 203.0.113.12")
	if got := extractClientIP(req, req.RemoteAddr, []string{"https://app.example.com"}); got != "203.0.113.11" {
		t.Fatalf("expected first forwarded for entry, got %q", got)
	}

	req.Header.Del("X-Forwarded-For")
	req.Header.Set("X-Real-IP", "203.0.113.13")
	if got := extractClientIP(req, req.RemoteAddr, []string{"https://app.example.com"}); got != "203.0.113.13" {
		t.Fatalf("expected real ip header, got %q", got)
	}
}

func TestExtractClientIPDoesNotTrustHeadersWithoutTrustedOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "198.51.100.10:4444"
	req.Host = "other.example.com"
	req.Header.Set("X-Forwarded-For", "203.0.113.11")
	if got := extractClientIP(req, req.RemoteAddr, []string{"https://app.example.com"}); got != "198.51.100.10" {
		t.Fatalf("expected remote fallback when host is untrusted, got %q", got)
	}
}

func TestLoginLimitKeyUsesClientIPResolution(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "198.51.100.10:4444"
	req.Host = "app.example.com"
	got := loginLimitKey(req, "Admin User", nil)
	if got != "admin user|198.51.100.10" {
		t.Fatalf("unexpected login limiter key, got %q", got)
	}

	req.RemoteAddr = "127.0.0.1:4444"
	req.Header.Set("X-Forwarded-For", "203.0.113.11")
	got = loginLimitKey(req, "Admin User", []string{"https://app.example.com"})
	if !strings.HasPrefix(got, "admin user|") || !strings.Contains(got, "203.0.113.11") {
		t.Fatalf("expected forwarded address in trusted login limiter key, got %q", got)
	}
}
