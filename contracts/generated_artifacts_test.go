package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"orbyte/internal/modules"
	"orbyte/internal/platform/contractartifacts"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/version"
)

func TestGeneratedArtifactsAreCurrent(t *testing.T) {
	generated, err := contractartifacts.Generate()
	if err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}

	assertJSONFile(t, filepath.Join("openapi", version.App, "openapi.json"), generated.OpenAPI)
	assertJSONFile(t, filepath.Join("openapi", "latest", "openapi.json"), generated.OpenAPI)
	assertJSONFile(t, filepath.Join("mcp", version.App, "catalog.json"), generated.MCP)
	assertJSONFile(t, filepath.Join("mcp", "latest", "catalog.json"), generated.MCP)
}

func TestGeneratedArtifactsCarryExpectedVersionMetadata(t *testing.T) {
	generated, err := contractartifacts.Generate()
	if err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}
	info, _ := generated.OpenAPI["info"].(map[string]any)
	if strings.TrimSpace(anyString(info["version"])) != version.App {
		t.Fatalf("unexpected openapi version: got %q want %q", anyString(info["version"]), version.App)
	}
	if strings.TrimSpace(anyString(generated.MCP["app_version"])) != version.App {
		t.Fatalf("unexpected mcp app_version: got %q want %q", anyString(generated.MCP["app_version"]), version.App)
	}
	if strings.TrimSpace(anyString(generated.MCP["contract_version"])) != mcp.ContractVersion {
		t.Fatalf("unexpected mcp contract_version: got %q want %q", anyString(generated.MCP["contract_version"]), mcp.ContractVersion)
	}
}

func TestGeneratedMCPCatalogIsSorted(t *testing.T) {
	generated, err := contractartifacts.Generate()
	if err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}
	assertSortedByField(t, generated.MCP["tools"], "name")
	assertSortedByField(t, generated.MCP["resources"], "uri")
	assertSortedByField(t, generated.MCP["apps"], "key")
}

func TestGeneratedArtifactsIncludeDefaultProfileContracts(t *testing.T) {
	generated, err := contractartifacts.Generate()
	if err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}

	manifests, err := modules.ForProfile("")
	if err != nil {
		t.Fatalf("resolve default profile manifests: %v", err)
	}
	foundClinicModule := false
	for _, manifest := range manifests {
		if manifest.Key == "clinic_registration" {
			foundClinicModule = true
			break
		}
	}
	if !foundClinicModule {
		t.Fatalf("default profile no longer includes clinic_registration; update this regression test")
	}

	openAPIRaw, err := json.Marshal(generated.OpenAPI)
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}
	openAPIText := string(openAPIRaw)
	for _, token := range []string{"clinic_registration", "patient_profile", "clinic.registration.worklist"} {
		if !strings.Contains(openAPIText, token) {
			t.Fatalf("generated openapi is missing default profile contract token %q", token)
		}
	}
}

func assertJSONFile(t *testing.T, path string, expected map[string]any) {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var actual map[string]any
	if err := json.Unmarshal(raw, &actual); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	expectedRaw, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal expected %s: %v", path, err)
	}
	var normalizedExpected map[string]any
	if err := json.Unmarshal(expectedRaw, &normalizedExpected); err != nil {
		t.Fatalf("normalize expected %s: %v", path, err)
	}
	if !reflect.DeepEqual(normalizedExpected, actual) {
		t.Fatalf("artifact %s is stale", path)
	}
}

func assertSortedByField(t *testing.T, items any, field string) {
	t.Helper()

	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal items: %v", err)
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	for i := 1; i < len(list); i++ {
		prev := strings.TrimSpace(anyString(list[i-1][field]))
		curr := strings.TrimSpace(anyString(list[i][field]))
		if prev > curr {
			t.Fatalf("items are not sorted by %s: %q before %q", field, prev, curr)
		}
	}
}

func anyString(value any) string {
	text, _ := value.(string)
	return text
}
