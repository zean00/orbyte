package httpx

import (
	"crypto/tls"
	"net/http/httptest"
	"strings"
	"testing"

	"orbyte/internal/platform/mcp"
)

func TestAssetHelpers(t *testing.T) {
	if len(uiHTMLDocument()) == 0 || !strings.Contains(string(uiHTMLDocument()), "<html") {
		t.Fatal("expected embedded ui html document")
	}
	if len(adminHTMLDocument()) == 0 || !strings.Contains(string(adminHTMLDocument()), "<html") {
		t.Fatal("expected embedded admin html document")
	}
	if len(serviceWorkerScript()) == 0 || !strings.Contains(string(serviceWorkerScript()), "self.") {
		t.Fatal("expected embedded service worker script")
	}
	if len(legacyUIShellScript()) == 0 || !strings.Contains(string(legacyUIShellScript()), "window.") {
		t.Fatal("expected legacy ui shell bundle")
	}
	if len(legacyAdminConsoleScript()) == 0 || !strings.Contains(string(legacyAdminConsoleScript()), "window.") {
		t.Fatal("expected legacy admin console bundle")
	}

	file, err := assetFileServer().Open("assets/index.html")
	if err != nil {
		t.Fatalf("expected embedded asset file to open: %v", err)
	}
	_ = file.Close()

	assets := getAssetsFS()
	if _, err := assets.Open("assets/admin.html"); err != nil {
		t.Fatalf("expected direct asset fs access: %v", err)
	}
}

func TestAdminOverviewHelperPayloads(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test/admin", nil)
	if got := requestBaseURL(req); got != "http://example.test" {
		t.Fatalf("unexpected base url: %q", got)
	}
	req.TLS = &tls.ConnectionState{}
	if got := requestBaseURL(req); got != "https://example.test" {
		t.Fatalf("unexpected tls base url: %q", got)
	}
	req.Host = ""
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := requestBaseURL(req); got != "https://127.0.0.1:8080" {
		t.Fatalf("unexpected forwarded/default base url: %q", got)
	}

	if got := portFromAddress(":8080"); got != "8080" {
		t.Fatalf("unexpected bare port parse: %q", got)
	}
	if got := portFromAddress("127.0.0.1:9090"); got != "9090" {
		t.Fatalf("unexpected host:port parse: %q", got)
	}
	if got := portFromAddress("https://example.test:7443/admin"); got != "7443" {
		t.Fatalf("unexpected url port parse: %q", got)
	}
	if got := portFromAddress("example.test"); got != "" {
		t.Fatalf("expected empty port for host without port, got %q", got)
	}

	states := parseToolStatesJSON(`{"tool.a":true,"tool.b":false}`)
	if !states["tool.a"] || states["tool.b"] {
		t.Fatalf("unexpected parsed tool states: %+v", states)
	}
	states = parseToolStatesJSON(map[string]any{" tool.c ": true, "tool.d": "invalid"})
	if !states["tool.c"] || len(states) != 1 {
		t.Fatalf("unexpected map parsed tool states: %+v", states)
	}
	if states := parseToolStatesJSON("{broken"); len(states) != 0 {
		t.Fatalf("expected invalid json to yield empty states, got %+v", states)
	}
	if len(toolInventoryPayload(nil)) != 0 || len(capabilityInventoryPayload(nil)) != 0 || len(resourceInventoryPayload(nil)) != 0 || len(appInventoryPayload(nil)) != 0 {
		t.Fatal("expected nil server payload helpers to return empty collections")
	}
	if got := compactToolCatalogPreviewPayload(nil); len(got) != 0 {
		t.Fatalf("expected nil server compact preview to be empty, got %+v", got)
	}
}

func TestAdminOverviewMCPPayloads(t *testing.T) {
	server := mcp.NewServer(mcp.ServerDeps{})

	tools := toolInventoryPayload(server)
	if tools == nil {
		t.Fatal("expected tool inventory payload slice")
	}

	capabilities := capabilityInventoryPayload(server)
	if capabilities == nil {
		t.Fatal("expected capability inventory payload slice")
	}

	preview := compactToolCatalogPreviewPayload(server)
	toolItems, ok := preview["tools"].([]mcp.ToolDescriptor)
	if !ok {
		t.Fatalf("expected compact preview tools, got %+v", preview["tools"])
	}
	catalog, ok := preview["catalog"].(map[string]any)
	if !ok || catalog["mode"] != "compact" {
		t.Fatalf("expected compact catalog payload, got %+v", preview["catalog"])
	}
	if _, ok := preview["groups"].([]map[string]any); !ok {
		t.Fatalf("expected compact preview groups, got %+v", preview["groups"])
	}
	if _, ok := preview["suggested_expansions"].([]mcp.ToolCapabilityDescriptor); !ok {
		t.Fatalf("expected compact preview suggested expansions, got %+v", preview["suggested_expansions"])
	}
	_ = toolItems

	if len(resourceInventoryPayload(server)) != 0 {
		t.Fatal("expected no module resources with empty server deps")
	}
	if len(appInventoryPayload(server)) != 0 {
		t.Fatal("expected no module apps with empty server deps")
	}
}
