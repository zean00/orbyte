package httpx

import (
	_ "embed"
	"strings"
)

const platformAssetVersion = "20260323-shell-auth-session-v2"
const platformAssetVersionPlaceholder = "{{PLATFORM_ASSET_VERSION}}"

//go:embed assets/platform.css
var platformCSS []byte

//go:embed assets/ui-shell-body.html
var uiShellBodyTemplate string

//go:embed assets/ui-shell-inline.css
var uiShellInlineCSSTemplate string

//go:embed assets/platform-shell-shared.js
var platformShellSharedTemplate string

//go:embed assets/ui-shell-core.js
var uiShellCoreTemplate string

//go:embed assets/ui-shell-offline.js
var uiShellOfflineTemplate string

//go:embed assets/ui-shell-routes.js
var uiShellRoutesTemplate string

//go:embed assets/ui-shell-runtime.js
var uiShellRuntimeTemplate string

//go:embed assets/ui-service-worker.js
var uiServiceWorkerTemplate string

//go:embed assets/admin-console-body.html
var adminConsoleBodyTemplate string

//go:embed assets/admin-console-core.js
var adminConsoleCoreTemplate string

//go:embed assets/admin-console-runtime.js
var adminConsoleRuntimeTemplate string

//go:embed assets/admin-console-operations.js
var adminConsoleOperationsTemplate string

//go:embed assets/admin-console-governance.js
var adminConsoleGovernanceTemplate string

func assetTemplateWithVersion(template string) string {
	return strings.ReplaceAll(template, platformAssetVersionPlaceholder, platformAssetVersion)
}

func uiShellInlineStyleTemplate() string {
	return uiShellInlineCSSTemplate
}

func uiShellScriptTemplate() string {
	return strings.Join([]string{
		platformShellSharedTemplate,
		uiShellCoreTemplate,
		uiShellOfflineTemplate,
		uiShellRoutesTemplate,
		uiShellRuntimeTemplate,
	}, "")
}

func uiShellHTMLDocument() string {
	return strings.Join([]string{
		"<!doctype html>",
		"<html lang=\"en\">",
		"<head>",
		"  <meta charset=\"utf-8\">",
		"  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
		"  <meta name=\"theme-color\" content=\"#1f6f5f\">",
		"  <link rel=\"manifest\" href=\"/ui/manifest.webmanifest\">",
		"  <title>Orbyte Platform UI</title>",
		"  <link rel=\"stylesheet\" href=\"/ui/assets/platform.css?v={{PLATFORM_ASSET_VERSION}}\">",
		"  <link rel=\"stylesheet\" href=\"/ui/assets/ui-shell-inline.css?v={{PLATFORM_ASSET_VERSION}}\">",
		"</head>",
		"<body>",
		strings.TrimSuffix(uiShellBodyTemplate, "\n"),
		"  <script src=\"/ui/assets/ui-shell.js?v={{PLATFORM_ASSET_VERSION}}\"></script>",
		"</body>",
		"</html>",
	}, "\n")
}

func adminConsoleScriptTemplate() string {
	return strings.Join([]string{
		platformShellSharedTemplate,
		adminConsoleCoreTemplate,
		adminConsoleRuntimeTemplate,
		adminConsoleOperationsTemplate,
		adminConsoleGovernanceTemplate,
	}, "")
}

func adminConsoleHTMLDocument() string {
	return strings.Join([]string{
		"<!doctype html>",
		"<html lang=\"en\">",
		"<head>",
		"  <meta charset=\"utf-8\">",
		"  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
		"  <title>Orbyte Admin</title>",
		"  <link rel=\"stylesheet\" href=\"/admin/assets/platform.css?v={{PLATFORM_ASSET_VERSION}}\">",
		"</head>",
		"<body>",
		strings.TrimSuffix(adminConsoleBodyTemplate, "\n"),
		"  <script src=\"/admin/assets/admin-console.js?v={{PLATFORM_ASSET_VERSION}}\"></script>",
		"</body>",
		"</html>",
	}, "\n")
}
