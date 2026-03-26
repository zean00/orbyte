package httpx

import (
	_ "embed"
	"embed"
	"io/fs"
	"net/http"
)

const platformAssetVersion = "20260326-react-shell-parity"

//go:embed assets
var assetsFS embed.FS

//go:embed assets/index.html
var uiIndexHTML []byte

//go:embed assets/admin.html
var adminIndexHTML []byte

//go:embed assets/sw.js
var serviceWorkerJS []byte

// Legacy compatibility assets still served on old routes.
//
//go:embed assets/platform.css
var legacyPlatformCSS []byte

//go:embed assets/ui-shell-inline.css
var legacyUIShellInlineCSS []byte

//go:embed assets/ui-shell-body.html
var legacyUIShellBody string

//go:embed assets/platform-shell-shared.js
var legacyPlatformShellShared string

//go:embed assets/ui-shell-core.js
var legacyUIShellCore string

//go:embed assets/ui-shell-offline.js
var legacyUIShellOffline string

//go:embed assets/ui-shell-routes.js
var legacyUIShellRoutes string

//go:embed assets/ui-shell-runtime.js
var legacyUIShellRuntime string

//go:embed assets/ui-service-worker.js
var legacyUIServiceWorker string

//go:embed assets/admin-console-body.html
var legacyAdminConsoleBody string

//go:embed assets/admin-console-core.js
var legacyAdminConsoleCore string

//go:embed assets/admin-console-runtime.js
var legacyAdminConsoleRuntime string

//go:embed assets/admin-console-operations.js
var legacyAdminConsoleOperations string

//go:embed assets/admin-console-governance.js
var legacyAdminConsoleGovernance string

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
