package httpx

import (
	"strings"
	"testing"
)

func TestUIShellRouteHelpers(t *testing.T) {
	if !isLocalDevHost("localhost:5173") || !isLocalDevHost("127.0.0.1:8080") || !isLocalDevHost("[::1]:3000") {
		t.Fatal("expected localhost variants to be detected as local")
	}
	if isLocalDevHost("example.com:443") {
		t.Fatal("expected non-local host to return false")
	}

	if got := getContentType("assets/app.js"); got != "application/javascript" {
		t.Fatalf("unexpected js content type: %q", got)
	}
	if got := getContentType("assets/app.css"); got != "text/css" {
		t.Fatalf("unexpected css content type: %q", got)
	}
	if got := getContentType("assets/file.unknown"); got != "application/octet-stream" {
		t.Fatalf("unexpected default content type: %q", got)
	}
}

func TestHTMLDocumentCompatibilityForHost(t *testing.T) {
	base := []byte("<html><body>" + vitePWARegisterScriptTag + firstModuleScriptTag + "main.js\"></script></body></html>")

	remote := string(htmlDocumentForHost(base, "example.com"))
	if !strings.Contains(remote, vitePWARegisterScriptTag) || strings.Contains(remote, "orbyte-localhost-sw-recovery-v1") {
		t.Fatalf("expected remote html to remain unchanged, got %q", remote)
	}

	local := string(htmlDocumentForHost(base, "localhost:8080"))
	if strings.Contains(local, vitePWARegisterScriptTag) {
		t.Fatalf("expected localhost pwa register script to be removed, got %q", local)
	}
	if !strings.Contains(local, "orbyte-localhost-sw-recovery-v1") {
		t.Fatalf("expected localhost recovery script to be injected, got %q", local)
	}

	compat := string(uiHTMLDocumentWithCompatibility("localhost:8080"))
	if !strings.Contains(compat, "legacy-compat") || !strings.Contains(compat, "/ui/assets/ui-shell.js") {
		t.Fatalf("expected compatibility markers in ui html, got %q", compat)
	}
}
