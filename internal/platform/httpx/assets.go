package httpx

import (
	_ "embed"
	"strings"
)

const platformAssetVersion = "20260321-self-service-surface-v2"
const platformAssetVersionPlaceholder = "{{PLATFORM_ASSET_VERSION}}"

//go:embed assets/platform.css
var platformCSS []byte

//go:embed assets/ui-shell.html
var uiShellHTMLTemplate string

//go:embed assets/ui-service-worker.js
var uiServiceWorkerTemplate string

//go:embed assets/admin-console.html
var adminConsoleHTMLTemplate string

func assetTemplateWithVersion(template string) string {
	return strings.ReplaceAll(template, platformAssetVersionPlaceholder, platformAssetVersion)
}
