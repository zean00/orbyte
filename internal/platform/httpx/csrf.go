package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/runtimeconfig"
	"orbyte/internal/platform/shared"
)

func withCSRFProtection(next http.Handler, cfg *config.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requiresCSRFProtection(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal || p.authMethod != "cookie" {
			next.ServeHTTP(w, r)
			return
		}
		if err := validateTrustedOrigin(r, cfg.AuthPolicy().TrustedOrigins); err != nil {
			respondError(w, err)
			return
		}
		if err := validateCSRF(r, p.sessionID); err != nil {
			respondError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requiresCSRFProtection(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func validateCSRF(r *http.Request, sessionID string) error {
	if sessionID == "" {
		return shared.Unauthorized("authentication required")
	}
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return shared.Forbidden("csrf token is required")
	}
	headerToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	if headerToken == "" {
		return shared.Forbidden("csrf token is required")
	}
	expected, err := issueCSRFToken(sessionID)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(cookie.Value), []byte(expected)) || !hmac.Equal([]byte(headerToken), []byte(expected)) {
		return shared.Forbidden("invalid csrf token")
	}
	return nil
}

func validateTrustedOrigin(r *http.Request, trustedOrigins []string) error {
	if len(trustedOrigins) == 0 {
		return nil
	}
	candidate := strings.TrimSpace(r.Header.Get("Origin"))
	if candidate == "" {
		if referrer := strings.TrimSpace(r.Header.Get("Referer")); referrer != "" {
			parsed, err := url.Parse(referrer)
			if err != nil {
				return shared.Forbidden("invalid referer")
			}
			candidate = parsed.Scheme + "://" + parsed.Host
		}
	}
	if candidate == "" {
		return shared.Forbidden("trusted origin is required")
	}
	for _, trusted := range trustedOrigins {
		if candidate == strings.TrimSpace(trusted) {
			return nil
		}
	}
	return shared.Forbidden("untrusted origin")
}

func issueCSRFToken(sessionID string) (string, error) {
	secret := []byte(runtimeconfig.Current().JWTSecret())
	if len(secret) == 0 {
		return "", shared.Unauthorized("APP_JWT_SECRET is required")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("csrf:" + sessionID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func buildCSRFCookie(sessionID string) (*http.Cookie, error) {
	token, err := issueCSRFToken(sessionID)
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   runtimeconfig.Current().CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	}, nil
}

func clearedCSRFCookie() *http.Cookie {
	return &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   runtimeconfig.Current().CookieSecure(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
	}
}
