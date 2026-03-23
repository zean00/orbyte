package mcp

import "testing"

func TestBuildBuiltInToolRegistrationsRejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	_, err := buildBuiltInToolRegistrations(
		[]builtInTool{{name: "duplicate.tool", permission: "ops.read"}, {name: "duplicate.tool", permission: "ops.read"}},
		map[string]builtInToolHandler{
			"duplicate.tool": func(*Server, ActorContext, map[string]any) (map[string]any, bool, error) { return nil, true, nil },
		},
	)
	if err == nil {
		t.Fatal("expected duplicate definition error")
	}
}

func TestBuildBuiltInToolRegistrationsRejectsMissingPermission(t *testing.T) {
	t.Parallel()

	_, err := buildBuiltInToolRegistrations(
		[]builtInTool{{name: "missing.permission"}},
		map[string]builtInToolHandler{
			"missing.permission": func(*Server, ActorContext, map[string]any) (map[string]any, bool, error) { return nil, true, nil },
		},
	)
	if err == nil {
		t.Fatal("expected missing permission error")
	}
}

func TestBuildBuiltInToolRegistrationsRejectsMissingHandler(t *testing.T) {
	t.Parallel()

	_, err := buildBuiltInToolRegistrations(
		[]builtInTool{{name: "missing.handler", permission: "ops.read"}},
		map[string]builtInToolHandler{},
	)
	if err == nil {
		t.Fatal("expected missing handler error")
	}
}
