package mcp

import (
	"testing"
)

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

func TestNewServerInitializesBuiltInToolRegistry(t *testing.T) {
	server := newTestServer(t)

	registry := server.mustBuiltInToolRegistrations()
	index := server.mustBuiltInToolRegistrationIndex()
	if len(registry) == 0 {
		t.Fatal("expected built-in tool registrations")
	}
	if len(registry) != len(index) {
		t.Fatalf("expected registration index size to match registry, got %d vs %d", len(index), len(registry))
	}
	for _, reg := range registry {
		if reg.definition.name == "" {
			t.Fatal("expected registration name")
		}
		if _, ok := index[reg.definition.name]; !ok {
			t.Fatalf("expected index entry for %s", reg.definition.name)
		}
	}
}

func TestNewServerInitializesBuiltInResourceRegistry(t *testing.T) {
	server := newTestServer(t)

	registry := server.mustBuiltInResourceRegistrations()
	index := server.mustBuiltInResourceIndex()
	if len(registry) == 0 {
		t.Fatal("expected built-in resource registrations")
	}
	if len(registry) != len(index) {
		t.Fatalf("expected resource index size to match registry, got %d vs %d", len(index), len(registry))
	}
	for i, reg := range registry {
		if reg.descriptor.URI == "" {
			t.Fatal("expected resource uri")
		}
		if reg.reader == nil {
			t.Fatalf("expected resource reader for %s", reg.descriptor.URI)
		}
		if _, ok := index[reg.descriptor.URI]; !ok {
			t.Fatalf("expected resource index entry for %s", reg.descriptor.URI)
		}
		if i > 0 && registry[i-1].descriptor.URI > reg.descriptor.URI {
			t.Fatalf("expected resource registry to be sorted, got %s before %s", registry[i-1].descriptor.URI, reg.descriptor.URI)
		}
	}
}
