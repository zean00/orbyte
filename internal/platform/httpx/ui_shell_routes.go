package httpx

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
)

const localhostRegisterSWShim = `if ('serviceWorker' in navigator) {
  window.addEventListener('load', async () => {
    const registrations = await navigator.serviceWorker.getRegistrations()
    await Promise.all(registrations.map((registration) => registration.unregister()))
    if ('caches' in window) {
      const cacheNames = await caches.keys()
      await Promise.all(cacheNames.map((cacheName) => caches.delete(cacheName)))
    }
  })
}`

const vitePWARegisterScriptTag = `<script id="vite-plugin-pwa:register-sw" src="/registerSW.js"></script>`
const firstModuleScriptTag = `<script type="module" crossorigin src="/assets/`

const localhostRecoveryScript = `<script>
(() => {
  if (!('serviceWorker' in navigator) || !window.sessionStorage) return
  const recoveryKey = 'orbyte-localhost-sw-recovery-v1'
  if (sessionStorage.getItem(recoveryKey)) return
  sessionStorage.setItem(recoveryKey, '1')
  window.addEventListener('load', () => {
    void (async () => {
      const registrations = await navigator.serviceWorker.getRegistrations()
      await Promise.all(registrations.map((registration) => registration.unregister()))
      if ('caches' in window) {
        const cacheNames = await caches.keys()
        await Promise.all(cacheNames.map((cacheName) => caches.delete(cacheName)))
      }
      window.location.replace(window.location.href)
    })().catch(() => {})
  }, { once: true })
})()
</script>
`

const localhostServiceWorkerShim = `self.addEventListener('install', () => {
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    await self.registration.unregister()
    const clients = await self.clients.matchAll({ type: 'window' })
    await Promise.all(clients.map((client) => client.navigate(client.url)))
  })())
})`

func registerUIShellRoutes(mux *http.ServeMux) {
	// Serve UI shell index.html for SPA routing (catchall for /ui/*)
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(uiHTMLDocumentWithCompatibility(r.Host))
	})

	// SPA fallback: serve index.html for any /ui/* path that isn't a known asset
	mux.HandleFunc("GET /ui/", func(w http.ResponseWriter, r *http.Request) {
		// Skip if this looks like a file request (has extension)
		if strings.Contains(r.URL.Path, ".") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(uiHTMLDocumentWithCompatibility(r.Host))
	})

	mux.HandleFunc("GET /ui/assets/platform.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(legacyPlatformCSS)
	})

	mux.HandleFunc("GET /ui/assets/ui-shell-inline.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(legacyUIShellInlineCSS)
	})

	mux.HandleFunc("GET /ui/assets/ui-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(legacyUIShellScript())
	})

	// Serve UI shell assets from Vite build output
	// The embed.FS root is the assets/ directory, so:
	// - Vite output is at assets/assets/ (main-BoBfFkGm.js, etc.)
	// - favicon.svg is at assets/favicon.svg
	// - manifest.webmanifest is at assets/manifest.webmanifest
	mux.HandleFunc("GET /ui/assets/", func(w http.ResponseWriter, r *http.Request) {
		// URL: /ui/assets/main-BoBfFkGm.js -> embed path: assets/assets/main-BoBfFkGm.js
		filename := strings.TrimPrefix(r.URL.Path, "/ui/assets/")
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

	// Also serve assets at /assets/ prefix (used by index.html)
	// Also serves admin assets from assets/ directory (admin-*.js, AdminShell-*.js, etc.)
	mux.HandleFunc("GET /assets/", func(w http.ResponseWriter, r *http.Request) {
		// URL: /assets/main-BoBfFkGm.js -> embed path: assets/assets/main-BoBfFkGm.js
		filename := strings.TrimPrefix(r.URL.Path, "/assets/")
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

	// Serve manifest at /manifest.webmanifest
	mux.Handle("GET /manifest.webmanifest", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, err := assetsFS.Open("assets/manifest.webmanifest")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "application/manifest+json")
		io.Copy(w, file)
	}))

	mux.Handle("GET /ui/manifest.webmanifest", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		respondJSON(w, http.StatusOK, map[string]any{
			"name":             "Orbyte Platform UI",
			"short_name":       "Orbyte UI",
			"start_url":        "/ui",
			"display":          "standalone",
			"background_color": "#f3efe7",
			"theme_color":      "#1f6f5f",
			"description":      "Manifest-driven offline-capable platform shell.",
		})
	}))

	// Serve service worker registration
	mux.HandleFunc("GET /registerSW.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-store")
		if isLocalDevHost(r.Host) {
			_, _ = w.Write([]byte(localhostRegisterSWShim))
			return
		}
		file, err := assetsFS.Open("assets/registerSW.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		io.Copy(w, file)
	})

	// Serve favicon
	mux.HandleFunc("GET /favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		file, err := assetsFS.Open("assets/favicon.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "image/svg+xml")
		io.Copy(w, file)
	})

	// Serve service worker at /ui/sw.js and /sw.js
	mux.HandleFunc("GET /ui/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(legacyUIServiceWorker))
	})

	// Also serve at /sw.js (for root-registered service workers)
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if isLocalDevHost(r.Host) {
			_, _ = w.Write([]byte(localhostServiceWorkerShim))
			return
		}
		_, _ = w.Write(serviceWorkerScript())
	})

	// Older root-scoped Workbox service workers import their helper from /workbox-*.js.
	// Keep that path alive so browsers with an existing worker do not break after updates.
	mux.HandleFunc("GET /{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := r.PathValue("filename")
		if !strings.HasPrefix(filename, "workbox-") || !strings.HasSuffix(filename, ".js") {
			http.NotFound(w, r)
			return
		}

		file, err := assetsFS.Open("assets/" + filename)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		_, _ = io.Copy(w, file)
	})
}

func isLocalDevHost(host string) bool {
	normalized := strings.TrimSpace(host)
	if normalized == "" {
		return false
	}
	if parsedHost, _, err := net.SplitHostPort(normalized); err == nil {
		normalized = parsedHost
	} else if strings.HasPrefix(normalized, "[") && strings.HasSuffix(normalized, "]") {
		normalized = strings.TrimPrefix(strings.TrimSuffix(normalized, "]"), "[")
	} else if strings.Count(normalized, ":") == 1 {
		if idx := strings.Index(normalized, ":"); idx >= 0 {
			normalized = normalized[:idx]
		}
	}
	normalized = strings.TrimPrefix(strings.TrimSuffix(normalized, "]"), "[")
	return normalized == "localhost" || normalized == "127.0.0.1" || normalized == "::1"
}

func getContentType(filePath string) string {
	ext := path.Ext(filePath)
	switch ext {
	case ".js":
		return "application/javascript"
	case ".css":
		return "text/css"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	case ".html":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

func htmlDocumentForHost(document []byte, host string) []byte {
	if !isLocalDevHost(host) {
		return document
	}
	clean := bytes.ReplaceAll(document, []byte(vitePWARegisterScriptTag), nil)
	return bytes.Replace(clean, []byte(firstModuleScriptTag), []byte(localhostRecoveryScript+firstModuleScriptTag), 1)
}

func uiHTMLDocumentWithCompatibility(host string) []byte {
	extra := "\n<!-- legacy-compat: <link rel=\"manifest\" href=\"/ui/manifest.webmanifest\"> -->" +
		"\n<!-- legacy-compat: <link rel=\"stylesheet\" href=\"/ui/assets/platform.css?v=" + platformAssetVersion + "\"> -->" +
		"\n<!-- legacy-compat: <link rel=\"stylesheet\" href=\"/ui/assets/ui-shell-inline.css?v=" + platformAssetVersion + "\"> -->" +
		"\n<!-- legacy-compat: <script src=\"/ui/assets/ui-shell.js?v=" + platformAssetVersion + "\"></script> -->" +
		"\n<!-- legacy-compat: " + strings.ReplaceAll(strings.TrimSpace(legacyUIShellBody), "--", "__") + " -->"
	base := htmlDocumentForHost(uiHTMLDocument(), host)
	return bytes.Replace(base, []byte("</body>"), []byte(extra+"\n</body>"), 1)
}
