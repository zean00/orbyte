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
var serviceWorkerJS = readEmbeddedAssetBytes("assets/sw.js")

// Legacy compatibility assets still served on old routes.
var legacyPlatformCSS = readEmbeddedAssetBytes("assets/platform.css")
var legacyUIShellInlineCSS = readEmbeddedAssetBytes("assets/ui-shell-inline.css")
var legacyUIShellBody = readEmbeddedAssetString("assets/ui-shell-body.html")
var legacyPlatformShellShared = readEmbeddedAssetString("assets/platform-shell-shared.js")
var legacyUIShellCore = readEmbeddedAssetString("assets/ui-shell-core.js")
var legacyUIShellOffline = readEmbeddedAssetString("assets/ui-shell-offline.js")
var legacyUIShellRoutes = readEmbeddedAssetString("assets/ui-shell-routes.js")
var legacyUIShellRuntime = readEmbeddedAssetString("assets/ui-shell-runtime.js")
var legacyUIServiceWorker = readEmbeddedAssetString("assets/ui-service-worker.js")
var legacyAdminConsoleBody = readEmbeddedAssetString("assets/admin-console-body.html")
var legacyAdminConsoleCore = readEmbeddedAssetString("assets/admin-console-core.js")
var legacyAdminConsoleRuntime = readEmbeddedAssetString("assets/admin-console-runtime.js")
var legacyAdminConsoleOperations = readEmbeddedAssetString("assets/admin-console-operations.js")
var legacyAdminConsoleGovernance = readEmbeddedAssetString("assets/admin-console-governance.js")

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

func readEmbeddedAssetBytes(path string) []byte {
	content, err := assetsFS.ReadFile(path)
	if err != nil {
		return nil
	}
	return content
}
