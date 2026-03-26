package httpx

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerAdminShellRoutes(mux *http.ServeMux, ident *identity.Service) {
	serveAdminShell := func(w http.ResponseWriter, r *http.Request) {
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
		_, _ = w.Write(adminHTMLDocumentWithCompatibility(r.Host))
	}

	// Serve admin console index.html
	mux.HandleFunc("GET /admin", serveAdminShell)

	// SPA fallback for nested admin routes like /admin/modules.
	mux.HandleFunc("GET /admin/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ".") {
			http.NotFound(w, r)
			return
		}
		serveAdminShell(w, r)
	})

	mux.HandleFunc("GET /admin/assets/platform.css", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(legacyPlatformCSS)
	})

	// Serve admin console assets from Vite build output
	// URL: /admin/assets/admin-BJhT79fJ.js -> embed path: assets/assets/admin-BJhT79fJ.js
	mux.HandleFunc("GET /admin/assets/", func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/admin/assets/")
		embedPath := "assets/assets/" + filename

		file, err := assetsFS.Open(embedPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		contentType := getContentType(embedPath)
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=31536000")

		io.Copy(w, file)
		_ = stat
	})

	mux.HandleFunc("GET /admin/assets/admin-console.js", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(legacyAdminConsoleScript())
	})

	// Also serve admin assets at /assets/ prefix for admin.html
	// This is handled by ui_shell_routes.go which has the main /assets/ handler
	// The admin assets are in the same assets/ directory as main assets
	// Admin-specific assets (admin-*.js, AdminShell-*.js, etc.) are served alongside main assets
}

func adminHTMLDocumentWithCompatibility(host string) []byte {
	extra := "\n<!-- legacy-compat: <script src=\"/admin/assets/admin-console.js?v=" + platformAssetVersion + "\"></script> -->" +
		"\n<!-- legacy-compat: " + strings.ReplaceAll(strings.TrimSpace(legacyAdminConsoleBody), "--", "__") + " -->"
	base := htmlDocumentForHost(adminHTMLDocument(), host)
	return bytes.Replace(base, []byte("</body>"), []byte(extra+"\n</body>"), 1)
}
