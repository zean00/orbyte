package httpx

import (
	"net/http"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerAdminShellRoutes(mux *http.ServeMux, ident *identity.Service) {
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		if err := authError(r); err != nil {
			http.Redirect(w, r, "/ui", http.StatusSeeOther)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok || p.kind != userPrincipal {
			http.Redirect(w, r, "/ui", http.StatusSeeOther)
			return
		}
		if !principalAllowsAll(ident, p, []string{"module.read"}) {
			respondError(w, shared.Forbidden("module.read is required"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(assetTemplateWithVersion(adminConsoleHTMLDocument())))
	})

	mux.HandleFunc("GET /admin/assets/platform.css", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(platformCSS)
	})

	mux.HandleFunc("GET /admin/assets/admin-console.js", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write([]byte(assetTemplateWithVersion(adminConsoleScriptTemplate())))
	})
}
