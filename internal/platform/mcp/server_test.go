package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func TestHandleInitializeAndUnknownMethod(t *testing.T) {
	server := newTestServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{ID: 1, Method: "initialize"}, ActorContext{})
	if resp.Error != nil {
		t.Fatalf("initialize failed: %+v", resp.Error)
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected default jsonrpc version, got %q", resp.JSONRPC)
	}
	result := resp.Result.(map[string]any)
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("expected protocol version %q, got %+v", ProtocolVersion, result)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "does/not/exist"}, ActorContext{})
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected method not found error, got %+v", resp.Error)
	}
}

func TestServerListsToolsAndResourcesByPermission(t *testing.T) {
	server := newTestServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}
	payload := resp.Result.(map[string]any)
	tools := payload["tools"].([]ToolDescriptor)
	if len(tools) != 1 || tools[0].Name != "analytics.snapshot.get" {
		t.Fatalf("expected analytics mcp tool, got %+v", tools)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "resources/list"}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("resources/list failed: %+v", resp.Error)
	}
	resources := resp.Result.(map[string]any)["resources"].([]ResourceDescriptor)
	if len(resources) != 2 {
		t.Fatalf("expected analytics resources, got %+v", resources)
	}
}

func TestServerCallsAnalyticsToolAndReadsAppResource(t *testing.T) {
	server := newTestServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.get", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("tools/call failed: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	meta := result["_meta"].(map[string]any)["orbyte/app"].(map[string]any)
	if meta["resource_uri"] != "orbyte://apps/analytics.cockpit" {
		t.Fatalf("expected app resource uri, got %+v", meta)
	}
	if meta["stream_uri"] != "/mcp/events/analytics/snapshot" {
		t.Fatalf("expected app stream uri, got %+v", meta)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://apps/analytics.cockpit"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("resources/read failed: %+v", resp.Error)
	}
	contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(contents) != 1 || contents[0].MIMEType != "text/html" {
		t.Fatalf("expected html resource, got %+v", contents)
	}
	if contents[0].Text == "" {
		t.Fatal("expected app html")
	}
	if !strings.Contains(contents[0].Text, "/mcp/events/analytics/snapshot") {
		t.Fatalf("expected app html to include stream uri, got %q", contents[0].Text)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://analytics/snapshot/current"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("read analytics resource failed: %+v", resp.Error)
	}
	jsonContents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(jsonContents) != 1 || jsonContents[0].MIMEType != "application/json" {
		t.Fatalf("expected json resource, got %+v", jsonContents)
	}
	if !strings.Contains(jsonContents[0].Text, "\"generated_at\"") {
		t.Fatalf("expected analytics json payload, got %q", jsonContents[0].Text)
	}
}

func TestServerRejectsDisallowedAndUnsupportedContracts(t *testing.T) {
	server := newTestServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.get", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(string) bool { return false },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "not allowed") {
		t.Fatalf("expected disallowed tool error, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://missing"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "resource not found") {
		t.Fatalf("expected missing resource error, got %+v", resp.Error)
	}

	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "tool.missing", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "tool not found") {
		t.Fatalf("expected missing tool error, got %+v", resp.Error)
	}
}

func TestServerHandlesGenericViewAppsAndUnavailableServices(t *testing.T) {
	server := newGenericViewServer(t)
	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://apps/analytics.generic"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error != nil {
		t.Fatalf("generic app resource failed: %+v", resp.Error)
	}
	contents := resp.Result.(map[string]any)["contents"].([]ResourceContent)
	if len(contents) != 1 || !strings.Contains(contents[0].Text, "Generic MCP app bound to shared view definition.") {
		t.Fatalf("expected generic view app html, got %+v", contents)
	}

	unavailable := NewServer(nil, nil, "")
	resp = unavailable.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.get", "arguments": map[string]any{}}),
	}, ActorContext{})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "mcp tools are unavailable") {
		t.Fatalf("expected unavailable tools error, got %+v", resp.Error)
	}

	resp = unavailable.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://analytics/snapshot/current"}),
	}, ActorContext{})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "mcp resources are unavailable") {
		t.Fatalf("expected unavailable resources error, got %+v", resp.Error)
	}
}

func TestServerRejectsBrokenProvidersAndApps(t *testing.T) {
	server := newBrokenProviderServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://broken/provider"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "unsupported resource provider") {
		t.Fatalf("expected unsupported provider error, got %+v", resp.Error)
	}

	if _, err := server.renderApp(ActorContext{}, module.MCPAppDefinition{Key: "empty"}); err == nil || !strings.Contains(err.Error(), "app renderer is not configured") {
		t.Fatalf("expected app renderer error, got %v", err)
	}
}

func TestServerRejectsUnsupportedOperationsAndUnavailableAnalytics(t *testing.T) {
	server := newUnsupportedToolServer(t)

	resp := server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mustJSON(t, map[string]any{"name": "analytics.snapshot.unsupported", "arguments": map[string]any{}}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "unsupported tool operation") {
		t.Fatalf("expected unsupported tool operation error, got %+v", resp.Error)
	}

	server = NewServer(newTestModules(t), nil, "")
	resp = server.Handle(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  mustJSON(t, map[string]any{"uri": "orbyte://analytics/snapshot/current"}),
	}, ActorContext{
		PermissionChecker: func(permissionKey string) bool { return permissionKey == "analytics.read" },
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "analytics is unavailable") {
		t.Fatalf("expected unavailable analytics error, got %+v", resp.Error)
	}
}

func TestHelpers(t *testing.T) {
	errResp := errorResponse("id-1", http.StatusForbidden, errors.New("denied"))
	if errResp.Error == nil || errResp.Error.Code != -32003 {
		t.Fatalf("expected forbidden code, got %+v", errResp.Error)
	}
	if allowsAll(func(permissionKey string) bool { return permissionKey != "deny" }, []string{"ok", "deny"}) {
		t.Fatal("expected permission check to fail")
	}
	if !allowsAll(func(permissionKey string) bool { return permissionKey == "ok" }, []string{"", "ok"}) {
		t.Fatal("expected blank permissions to be ignored")
	}
	cloned := cloneMap(map[string]any{"a": 1})
	cloned["a"] = 2
	if original := cloneMap(nil); original != nil {
		t.Fatalf("expected nil clone for nil map, got %+v", original)
	}
	if firstNonEmpty("", " value ", "fallback") != "value" {
		t.Fatalf("expected trimmed first non-empty value")
	}
	if escaped := escapeHTML(`a&<>"'`); escaped != "a&amp;&lt;&gt;&quot;&#39;" {
		t.Fatalf("unexpected html escape output %q", escaped)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	documents := document.NewService()
	flows := workflow.NewService()
	eventingSvc := eventing.NewService()
	searchSvc := search.NewService()
	obsSvc := observability.NewService()
	analyticsSvc := analytics.NewService(documents, flows, eventingSvc, searchSvc, audit.NewService(), obsSvc)
	return NewServer(modules, analyticsSvc, "/mcp/events/analytics/snapshot")
}

func newGenericViewServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	if err := modules.Register(module.Manifest{
		Key: "analytics.generic",
		Frontend: module.FrontendDefinition{
			Views: []module.ViewDefinition{{
				Key: "analytics.generic.view", Title: "Analytics Generic", Kind: "dashboard",
			}},
		},
		MCP: module.MCPDefinition{
			Resources: []module.MCPResourceDefinition{{
				Key: "analytics.generic.app", Title: "Analytics Generic App", URI: "orbyte://apps/analytics.generic", MIMEType: "text/html", Provider: "mcp.app", RequiredPermissions: []string{"analytics.read"}, AppKey: "analytics.generic",
			}},
			Apps: []module.MCPAppDefinition{{
				Key: "analytics.generic", Title: "Analytics Generic", ResourceKey: "analytics.generic.app", ViewKey: "analytics.generic.view", RequiredPermissions: []string{"analytics.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register generic manifest failed: %v", err)
	}
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), "/mcp/events/analytics/snapshot")
}

func newBrokenProviderServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	if err := modules.Register(module.Manifest{
		Key: "analytics.broken",
		MCP: module.MCPDefinition{
			Resources: []module.MCPResourceDefinition{
				{Key: "broken.provider", Title: "Broken Provider", URI: "orbyte://broken/provider", MIMEType: "application/json", Provider: "unsupported.provider", RequiredPermissions: []string{"analytics.read"}},
			},
		},
	}, "system"); err != nil {
		t.Fatalf("register broken manifest failed: %v", err)
	}
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), "")
}

func newUnsupportedToolServer(t *testing.T) *Server {
	t.Helper()
	modules := newTestModules(t)
	if err := modules.Register(module.Manifest{
		Key: "analytics.unsupported",
		MCP: module.MCPDefinition{
			Tools: []module.MCPToolDefinition{{
				Key: "analytics.snapshot.unsupported", Title: "Unsupported", Operation: "unsupported.operation", RequiredPermissions: []string{"analytics.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register unsupported tool manifest failed: %v", err)
	}
	return NewServer(modules, analytics.NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService()), "")
}

func newTestModules(t *testing.T) *module.Service {
	t.Helper()
	modules := module.NewService()
	if err := modules.Register(module.Manifest{
		Key: "analytics",
		Frontend: module.FrontendDefinition{
			CustomEntries: []module.CustomEntryDefinition{{
				Key: "analytics.cockpit", Title: "Analytics Cockpit", RoutePath: "/analytics/cockpit", BundleKey: "analytics-cockpit", ComponentExport: "render",
			}},
		},
		Bundles: []module.BundleDefinition{{Key: "analytics-cockpit", Script: "console.log('analytics')"}},
		MCP: module.MCPDefinition{
			Tools: []module.MCPToolDefinition{{
				Key: "analytics.snapshot.get", Title: "Get Analytics Snapshot", Operation: "analytics.snapshot.get", RequiredPermissions: []string{"analytics.read"}, AppKey: "analytics.cockpit",
			}},
			Resources: []module.MCPResourceDefinition{
				{Key: "analytics.snapshot.current", Title: "Current Analytics Snapshot", URI: "orbyte://analytics/snapshot/current", MIMEType: "application/json", Provider: "analytics.snapshot.current", RequiredPermissions: []string{"analytics.read"}},
				{Key: "analytics.cockpit.app", Title: "Analytics Cockpit App", URI: "orbyte://apps/analytics.cockpit", MIMEType: "text/html", Provider: "mcp.app", RequiredPermissions: []string{"analytics.read"}, AppKey: "analytics.cockpit"},
			},
			Apps: []module.MCPAppDefinition{{
				Key: "analytics.cockpit", Title: "Analytics Cockpit", ResourceKey: "analytics.cockpit.app", CustomEntryKey: "analytics.cockpit", RequiredPermissions: []string{"analytics.read"},
			}},
		},
	}, "system"); err != nil {
		t.Fatalf("register manifest failed: %v", err)
	}
	return modules
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	buf, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal params failed: %v", err)
	}
	return buf
}
