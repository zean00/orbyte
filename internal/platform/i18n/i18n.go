package i18n

import (
	"net/http"
	"strings"
	"time"
)

type LocalizedText map[string]string

var supportedLocales = []string{"en", "id"}

const LocaleCookieName = "orbyte_locale"

func SupportedLocales() []string {
	out := make([]string, len(supportedLocales))
	copy(out, supportedLocales)
	return out
}

func NormalizeLocale(locale string) string {
	value := strings.TrimSpace(strings.ToLower(locale))
	if value == "" {
		return "en"
	}
	value = strings.ReplaceAll(value, "_", "-")
	for _, candidate := range supportedLocales {
		if value == candidate || strings.HasPrefix(value, candidate+"-") {
			return candidate
		}
	}
	return "en"
}

func Resolve(locale string, localized LocalizedText, fallback string) string {
	if value := strings.TrimSpace(localized[NormalizeLocale(locale)]); value != "" {
		return value
	}
	if value := strings.TrimSpace(localized["en"]); value != "" {
		return value
	}
	if value := strings.TrimSpace(localized["id"]); value != "" {
		return value
	}
	return fallback
}

func FromRequest(r *http.Request) string {
	if r == nil {
		return "en"
	}
	if cookie, err := r.Cookie(LocaleCookieName); err == nil {
		if value := NormalizeLocale(cookie.Value); value != "" {
			return value
		}
	}
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		token := strings.TrimSpace(strings.Split(part, ";")[0])
		if token == "" {
			continue
		}
		return NormalizeLocale(token)
	}
	return "en"
}

func SetCookie(w http.ResponseWriter, locale string) {
	if w == nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     LocaleCookieName,
		Value:    NormalizeLocale(locale),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
	})
}
