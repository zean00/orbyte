package httpx

import "net/http"

func registerUIShellRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(assetTemplateWithVersion(uiShellHTMLDocument())))
	})

	mux.HandleFunc("GET /ui/assets/platform.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(platformCSS)
	})

	mux.HandleFunc("GET /ui/assets/ui-shell-inline.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(assetTemplateWithVersion(uiShellInlineStyleTemplate())))
	})

	mux.HandleFunc("GET /ui/assets/ui-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(assetTemplateWithVersion(uiShellScriptTemplate())))
	})

	mux.HandleFunc("GET /ui/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"name":             "Orbyte Platform UI",
			"short_name":       "Orbyte UI",
			"start_url":        "/ui",
			"display":          "standalone",
			"background_color": "#f3efe7",
			"theme_color":      "#1f6f5f",
			"description":      "Manifest-driven offline-capable platform shell.",
		})
	})

	mux.HandleFunc("GET /ui/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(assetTemplateWithVersion(uiServiceWorkerTemplate)))
	})
}
