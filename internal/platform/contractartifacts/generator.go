package contractartifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"orbyte/internal/modules"
	"orbyte/internal/platform/app"
	"orbyte/internal/platform/httpx"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/runtimeconfig"
	"orbyte/internal/platform/version"
)

type Generated struct {
	OpenAPI map[string]any
	MCP     map[string]any
}

func Generate() (Generated, error) {
	restore := applyGenerationEnv()
	defer restore()

	runtime := runtimeconfig.Current()
	profile := runtime.DomainProfile()
	manifests, err := modules.ForProfile(profile)
	if err != nil {
		return Generated{}, err
	}
	application, err := app.New(app.Options{
		Profile:           profile,
		BusinessManifests: manifests,
	})
	if err != nil {
		return Generated{}, err
	}
	defer func() {
		_ = application.Close()
	}()

	openAPI := httpx.OpenAPIDocument(application.Config, application.Modules, application.Models, application.Documents, application.Search)
	catalog, err := application.MCP.Catalog(mcp.ActorContext{
		EndpointScope: mcp.EndpointScopeAll,
		PermissionChecker: func(string) bool {
			return true
		},
	})
	if err != nil {
		return Generated{}, err
	}
	catalog["app_version"] = version.App
	generated := Generated{OpenAPI: openAPI, MCP: catalog}
	if err := validateGenerated(generated); err != nil {
		return Generated{}, err
	}
	return generated, nil
}

func Write(root string, generated Generated) error {
	files := map[string]map[string]any{
		filepath.Join(root, "contracts", "openapi", version.App, "openapi.json"): generated.OpenAPI,
		filepath.Join(root, "contracts", "openapi", "latest", "openapi.json"):    generated.OpenAPI,
		filepath.Join(root, "contracts", "mcp", version.App, "catalog.json"):     generated.MCP,
		filepath.Join(root, "contracts", "mcp", "latest", "catalog.json"):        generated.MCP,
	}
	for path, payload := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", path, err)
		}
		raw = append(raw, '\n')
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func applyGenerationEnv() func() {
	pairs := map[string]string{
		"APP_AUTH_DEV_MODE":            "true",
		"APP_ENV":                      "development",
		"APP_JWT_SECRET":               "contract-artifacts-secret",
		"APP_BOOTSTRAP_ADMIN_PASSWORD": "admin123!",
		"DATABASE_URL":                 "",
	}
	originals := make(map[string]*string, len(pairs))
	for key, value := range pairs {
		if current, ok := os.LookupEnv(key); ok {
			copyValue := current
			originals[key] = &copyValue
		} else {
			originals[key] = nil
		}
		_ = os.Setenv(key, value)
	}
	return func() {
		for key, value := range originals {
			if value == nil {
				_ = os.Unsetenv(key)
				continue
			}
			_ = os.Setenv(key, *value)
		}
	}
}

func validateGenerated(g Generated) error {
	if err := validateOpenAPI(g.OpenAPI); err != nil {
		return err
	}
	if err := validateMCPCatalog(g.MCP); err != nil {
		return err
	}
	return nil
}

func validateOpenAPI(doc map[string]any) error {
	if strings.TrimSpace(anyString(doc["openapi"])) == "" {
		return fmt.Errorf("openapi artifact is missing openapi version")
	}
	info, _ := doc["info"].(map[string]any)
	if strings.TrimSpace(anyString(info["version"])) != version.App {
		return fmt.Errorf("openapi artifact version %q does not match app version %q", anyString(info["version"]), version.App)
	}
	return nil
}

func validateMCPCatalog(catalog map[string]any) error {
	if strings.TrimSpace(anyString(catalog["app_version"])) != version.App {
		return fmt.Errorf("mcp catalog app_version %q does not match app version %q", anyString(catalog["app_version"]), version.App)
	}
	if strings.TrimSpace(anyString(catalog["contract_version"])) != mcp.ContractVersion {
		return fmt.Errorf("mcp catalog contract_version %q does not match runtime contract version %q", anyString(catalog["contract_version"]), mcp.ContractVersion)
	}
	if err := validateSortedStringField(catalog["tools"], "name"); err != nil {
		return fmt.Errorf("mcp catalog tools: %w", err)
	}
	if err := validateSortedStringField(catalog["resources"], "uri"); err != nil {
		return fmt.Errorf("mcp catalog resources: %w", err)
	}
	if err := validateSortedStringField(catalog["apps"], "key"); err != nil {
		return fmt.Errorf("mcp catalog apps: %w", err)
	}
	return nil
}

func validateSortedStringField(items any, field string) error {
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal items: %w", err)
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("decode items: %w", err)
	}
	for i := 1; i < len(list); i++ {
		prevValue := strings.TrimSpace(anyString(list[i-1][field]))
		currValue := strings.TrimSpace(anyString(list[i][field]))
		if prevValue == "" || currValue == "" {
			return fmt.Errorf("missing %s at position %d", field, i)
		}
		if prevValue > currValue {
			return fmt.Errorf("items are not sorted by %s: %q before %q", field, prevValue, currValue)
		}
	}
	return nil
}

func anyString(value any) string {
	text, _ := value.(string)
	return text
}
