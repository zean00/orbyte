package httpx

import (
	"net/http"
	"strings"

	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
)

func registerLocaleRoutes(mux *http.ServeMux, ident *identity.Service) {
	mux.HandleFunc("GET /locale", func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimSpace(r.URL.Query().Get("locale"))
		current := localeFromRequest(r, ident)
		if requested != "" {
			if p, ok := currentPrincipal(r); ok && p.kind == userPrincipal && p.userID != "" {
				current = i18n.NormalizeLocale(requested)
				if ident != nil {
					if _, err := ident.SetUserPreferredLocale(p.userID, current); err != nil {
						respondError(w, err)
						return
					}
				}
			} else {
				current = i18n.NormalizeLocale(requested)
				i18n.SetCookie(w, current)
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"locale":            current,
			"supported_locales": i18n.SupportedLocales(),
		})
	})
}

func localeFromRequest(r *http.Request, ident *identity.Service) string {
	if ident != nil {
		if p, ok := currentPrincipal(r); ok && p.kind == userPrincipal && p.userID != "" {
			if locale := ident.PreferredLocale(p.userID); locale != "" {
				return locale
			}
		}
	}
	return i18n.FromRequest(r)
}
