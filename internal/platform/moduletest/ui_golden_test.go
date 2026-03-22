package moduletest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/module"
)

func TestGenericUIBootstrapAndRouteGoldens(t *testing.T) {
	h := NewHarness(t, sdkGoldenManifest())
	h.LoginAdmin()

	cases := []struct {
		name string
		path string
	}{
		{name: "bootstrap", path: "/ui/bootstrap"},
		{name: "list-route", path: "/ui/routes/resolve?path=/sdk/items"},
		{name: "detail-route", path: "/ui/routes/resolve?path=/sdk/items/detail"},
		{name: "form-route", path: "/ui/routes/resolve?path=/sdk/items/new"},
		{name: "queue-route", path: "/ui/routes/resolve?path=/sdk/queue&surface=worklist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := h.GetJSON(tc.path)
			normalized, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				t.Fatalf("marshal %s: %v", tc.name, err)
			}
			goldenPath := filepath.Join("testdata", tc.name+".golden.json")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, append(normalized, '\n'), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}
			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}
			if string(expected) != string(append(normalized, '\n')) {
				t.Fatalf("golden mismatch for %s\nexpected:\n%s\ngot:\n%s", tc.name, string(expected), string(normalized))
			}
		})
	}
}

func sdkGoldenManifest() module.Manifest {
	return module.Manifest{
		Key:                  "sdkgolden",
		Name:                 "SDK Golden",
		Version:              "1.0.0",
		KernelVersionRange:   ">=1.0.0,<2.0.0",
		RequiredCapabilities: []string{"generic_ui", "search_runtime"},
		DomainFamily:         "tools",
		OwnedDocumentTypes:   []string{"sdk_item"},
		Documents: []document.Definition{{
			Type:          "sdk_item",
			DisplayName:   "SDK Item",
			SchemaVersion: "v1",
		}},
		Frontend: module.FrontendDefinition{
			Menus: []module.MenuDefinition{
				{Key: "sdk.items.menu", Label: "SDK Items", ActionKey: "sdk.items.list"},
				{Key: "sdk.queue.menu", Label: "SDK Queue", ActionKey: "sdk.queue"},
			},
			Actions: []module.ActionDefinition{
				{Key: "sdk.items.list", Label: "SDK Items", RoutePath: "/sdk/items", ViewKey: "sdk.items.list", RenderMode: module.RenderModeGeneric},
				{Key: "sdk.items.detail", Label: "SDK Detail", RoutePath: "/sdk/items/detail", ViewKey: "sdk.items.detail", RenderMode: module.RenderModeGeneric},
				{Key: "sdk.items.form", Label: "SDK Form", RoutePath: "/sdk/items/new", ViewKey: "sdk.items.form", RenderMode: module.RenderModeGeneric},
				{Key: "sdk.queue", Label: "SDK Queue", RoutePath: "/sdk/queue", ViewKey: "sdk.queue", RenderMode: module.RenderModeGeneric, Surface: module.UISurfaceWorklist},
			},
			Views: []module.ViewDefinition{
				{Key: "sdk.items.list", Title: "SDK Items", Kind: "list", DocumentType: "sdk_item", Columns: []module.ColumnDefinition{{Key: "title", Label: "Title", Path: "payload.title"}}},
				{Key: "sdk.items.detail", Title: "SDK Item Detail", Kind: "detail", DocumentType: "sdk_item", Fields: []module.FieldDefinition{{Key: "title", Label: "Title", Path: "payload.title", Type: "string"}}},
				{Key: "sdk.items.form", Title: "New SDK Item", Kind: "form", DocumentType: "sdk_item", Fields: []module.FieldDefinition{{Key: "title", Label: "Title", Path: "payload.title", Type: "string"}}},
				{Key: "sdk.queue", Title: "SDK Queue", Kind: "queue", ProjectionKey: "documents.requests.search", Columns: []module.ColumnDefinition{{Key: "status", Label: "Status", Path: "status"}}},
			},
		},
	}
}
