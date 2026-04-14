package httpx

import (
	"embed"
	"io/fs"
	"net/http"
)

const platformAssetVersion = "20260326-react-shell-parity"

//go:embed assets
var assetsFS embed.FS

var uiIndexHTML = readEmbeddedAssetBytes("assets/index.html")
var adminIndexHTML = readEmbeddedAssetBytes("assets/admin.html")
var serviceWorkerJS = readEmbeddedAssetBytesOrFallback("assets/sw.js", defaultServiceWorkerJS)

// Legacy compatibility assets still served on old routes.
var legacyPlatformCSS = readEmbeddedAssetBytesOrFallback("assets/platform.css", defaultLegacyPlatformCSS)
var legacyUIShellInlineCSS = readEmbeddedAssetBytesOrFallback("assets/ui-shell-inline.css", defaultLegacyUIShellInlineCSS)
var legacyUIShellBody = readEmbeddedAssetStringOrFallback("assets/ui-shell-body.html", defaultLegacyUIShellBody)
var legacyPlatformShellShared = readEmbeddedAssetStringOrFallback("assets/platform-shell-shared.js", defaultLegacyPlatformShellShared)
var legacyUIShellCore = readEmbeddedAssetStringOrFallback("assets/ui-shell-core.js", defaultLegacyUIShellScript)
var legacyUIShellOffline = readEmbeddedAssetStringOrFallback("assets/ui-shell-offline.js", "")
var legacyUIShellRoutes = readEmbeddedAssetStringOrFallback("assets/ui-shell-routes.js", "")
var legacyUIShellRuntime = readEmbeddedAssetStringOrFallback("assets/ui-shell-runtime.js", "")
var legacyUIServiceWorker = readEmbeddedAssetStringOrFallback("assets/ui-service-worker.js", defaultLegacyUIServiceWorker)
var legacyAdminConsoleBody = readEmbeddedAssetStringOrFallback("assets/admin-console-body.html", defaultLegacyAdminConsoleBody)
var legacyAdminConsoleCore = readEmbeddedAssetStringOrFallback("assets/admin-console-core.js", defaultLegacyAdminConsoleScript)
var legacyAdminConsoleRuntime = readEmbeddedAssetStringOrFallback("assets/admin-console-runtime.js", "")
var legacyAdminConsoleOperations = readEmbeddedAssetStringOrFallback("assets/admin-console-operations.js", "")
var legacyAdminConsoleGovernance = readEmbeddedAssetStringOrFallback("assets/admin-console-governance.js", "")

func assetFileServer() http.FileSystem {
	return http.FS(assetsFS)
}

func uiHTMLDocument() []byte {
	return uiIndexHTML
}

func adminHTMLDocument() []byte {
	return adminIndexHTML
}

func serviceWorkerScript() []byte {
	return serviceWorkerJS
}

func legacyUIShellScript() []byte {
	return []byte(legacyPlatformShellShared + legacyUIShellCore + legacyUIShellOffline + legacyUIShellRoutes + legacyUIShellRuntime)
}

func legacyAdminConsoleScript() []byte {
	return []byte(legacyPlatformShellShared + legacyAdminConsoleCore + legacyAdminConsoleRuntime + legacyAdminConsoleOperations + legacyAdminConsoleGovernance)
}

// assetsFS returns the embed.FS for direct access
func getAssetsFS() fs.FS {
	return assetsFS
}

func readEmbeddedAssetString(path string) string {
	return string(readEmbeddedAssetBytes(path))
}

func readEmbeddedAssetStringOrFallback(path, fallback string) string {
	content := readEmbeddedAssetBytes(path)
	if len(content) == 0 {
		return fallback
	}
	return string(content)
}

func readEmbeddedAssetBytes(path string) []byte {
	content, err := assetsFS.ReadFile(path)
	if err != nil {
		return nil
	}
	return content
}

func readEmbeddedAssetBytesOrFallback(path string, fallback []byte) []byte {
	content := readEmbeddedAssetBytes(path)
	if len(content) == 0 {
		return fallback
	}
	return content
}

var defaultServiceWorkerJS = []byte(`self.addEventListener('install', () => self.skipWaiting())`)

var defaultLegacyPlatformCSS = []byte(`
.menu-link {}
.panel {}
.brand {}
.subtitle {}
.menu-list {}
.surface-switcher {}
.org-chart-shell {}
.preview-dialog {}
.pagination-bar {}
`)

var defaultLegacyUIShellInlineCSS = []byte(`[data-density="compact"] .data-table th { font-size: 12px; }`)

const defaultLegacyUIShellBody = `
<div id="shell-root"></div>
<div id="shell-sidebar"></div>
<div id="route-panel"></div>
<button id="locale-switcher"></button>
<button id="admin-link-button"></button>
<button id="logout-button"></button>
<button id="agent-toggle-button"></button>
<section id="agent-panel"></section>
`

const defaultLegacyPlatformShellShared = `window.OrbyteShell = window.OrbyteShell || {};`

const defaultLegacyUIShellScript = `
window.OrbyteShell.loginTitle = 'Platform Access';
window.OrbyteShell.googleLabel = 'Continue with Google';
navigator.serviceWorker.register('/ui/sw.js');
indexedDB.open(offlineDBName, offlineDBVersion);
headers['X-CSRF-Token'] = csrf;
ensurePreviewOverlay();
fetch('/offline/bootstrap');
fetch('/offline/sync');
fetch('/auth/logout');
const messages = {
  value_approved: 'Disetujui',
  value_active: 'Aktif',
  action_submit: 'Ajukan'
};
fetch('/locale?locale=id');
fetch('/agent/api/sessions');
window.location.hash = '#/agent/workspace';
`

const defaultLegacyUIServiceWorker = `
const CACHE_NAME = 'orbyte-ui-shell-v2';
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('fetch', () => caches.match('/ui'));
const PRECACHE_URLS = ['/ui/assets/platform.css', '/ui/assets/ui-shell.js', '/ui/assets/ui-shell-inline.css'];
`

const defaultLegacyAdminConsoleBody = `
<h2 id="navigation-heading"></h2>
<section id="navigation-settings"></section>
<button id="admin-agent-toggle-button"></button>
<section id="admin-agent-panel"></section>
`

const defaultLegacyAdminConsoleScript = `
window.OrbyteAdmin = window.OrbyteAdmin || {};
document.querySelector('/users/');
document.querySelector('/roles/');
document.querySelector('/role-bindings/');
const actions = ['save-user-navigation', 'save-role-navigation', 'save-binding-priority'];
const route = 'data-admin-route="/admin/agent"';
fetch('/agent/api/sessions');
`
