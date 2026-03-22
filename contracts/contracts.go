package contracts

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed events/*.schema.json integration/*.schema.json
var filesystem embed.FS

type Issue struct {
	Field   string
	Code    string
	Message string
}

func Exists(path string) bool {
	_, err := filesystem.ReadFile(path)
	return err == nil
}

func Load(path string) (map[string]any, error) {
	raw, err := filesystem.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode contract %s: %w", path, err)
	}
	return payload, nil
}

func IntegrationSchemaPath(key string, version int) string {
	return filepath.ToSlash(filepath.Join("integration", fmt.Sprintf("%s.v%d.schema.json", key, version)))
}

func EventSchemaPath(eventType, version string) string {
	return filepath.ToSlash(filepath.Join("events", fmt.Sprintf("%s.%s.schema.json", eventType, version)))
}

func ValidateIntegrationContract(path, key string, version int, schema map[string]any) error {
	expectedPath := IntegrationSchemaPath(key, version)
	if trimmedPath := filepath.ToSlash(filepath.Clean(path)); strings.TrimSpace(path) != "" && trimmedPath != expectedPath {
		return fmt.Errorf("integration schema_ref %s does not match contract %s v%d", path, key, version)
	}
	if title, _ := schema["title"].(string); title != "" && title != fmt.Sprintf("%s.v%d", key, version) {
		return fmt.Errorf("integration schema title %s does not match contract %s v%d", title, key, version)
	}
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) == 0 {
		return nil
	}
	if expectedKey, ok := constString(properties["contract_key"]); ok && expectedKey != key {
		return fmt.Errorf("integration schema contract_key %s does not match contract %s", expectedKey, key)
	}
	if expectedVersion, ok := constInt(properties["contract_version"]); ok && expectedVersion != version {
		return fmt.Errorf("integration schema contract_version %d does not match contract v%d", expectedVersion, version)
	}
	return nil
}

func ValidateObject(schema map[string]any, payload map[string]any) []Issue {
	if len(schema) == 0 {
		return nil
	}
	return validateNode(schema, payload, "")
}

func ValidateEventSchema(path, eventType, schemaVersion string, envelope map[string]any) ([]Issue, error) {
	if strings.TrimSpace(path) == "" {
		path = EventSchemaPath(eventType, schemaVersion)
	}
	if !Exists(path) {
		return nil, nil
	}
	schema, err := Load(path)
	if err != nil {
		return nil, err
	}
	if title, _ := schema["title"].(string); title != "" && title != fmt.Sprintf("%s.%s", eventType, schemaVersion) {
		return nil, fmt.Errorf("event schema title %s does not match event %s %s", title, eventType, schemaVersion)
	}
	return ValidateObject(schema, envelope), nil
}

func validateNode(schema map[string]any, value any, field string) []Issue {
	issues := make([]Issue, 0)
	expectedType, _ := schema["type"].(string)
	if expectedType != "" && !valueMatchesType(value, expectedType) {
		issues = append(issues, Issue{Field: field, Code: "invalid_type", Message: fmt.Sprintf("expected %s", expectedType)})
		return issues
	}
	if constant, ok := schema["const"]; ok && !valuesEqual(value, constant) {
		issues = append(issues, Issue{Field: field, Code: "invalid_const", Message: "value does not match contract constant"})
	}
	properties, hasProperties := schema["properties"].(map[string]any)
	required, hasRequired := schema["required"].([]any)
	switch {
	case expectedType == "object" || expectedType == "" && (hasProperties || hasRequired):
		object, _ := value.(map[string]any)
		for _, raw := range required {
			key, _ := raw.(string)
			if strings.TrimSpace(key) == "" {
				continue
			}
			if _, ok := object[key]; !ok {
				issues = append(issues, Issue{Field: joinField(field, key), Code: "required", Message: "required field is missing"})
			}
		}
		for key, raw := range properties {
			childSchema, _ := raw.(map[string]any)
			if childSchema == nil {
				continue
			}
			childValue, ok := object[key]
			if !ok {
				continue
			}
			issues = append(issues, validateNode(childSchema, childValue, joinField(field, key))...)
		}
	case expectedType == "array" || expectedType == "" && schema["items"] != nil:
		items, _ := value.([]any)
		itemSchema, _ := schema["items"].(map[string]any)
		if itemSchema == nil {
			return issues
		}
		for index, item := range items {
			issues = append(issues, validateNode(itemSchema, item, fmt.Sprintf("%s[%d]", field, index))...)
		}
	}
	return issues
}

func joinField(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + "." + key
}

func valueMatchesType(value any, expected string) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64, json.Number:
			return true
		}
		return false
	case "integer":
		switch typed := value.(type) {
		case int, int32, int64:
			return true
		case float64:
			return typed == float64(int(typed))
		case json.Number:
			_, err := strconv.Atoi(typed.String())
			return err == nil
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return true
	}
}

func valuesEqual(left, right any) bool {
	switch typed := left.(type) {
	case json.Number:
		left = typed.String()
	}
	switch typed := right.(type) {
	case json.Number:
		right = typed.String()
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func constString(value any) (string, bool) {
	property, _ := value.(map[string]any)
	parsed, ok := property["const"]
	if !ok {
		return "", false
	}
	text, ok := parsed.(string)
	return text, ok
}

func constInt(value any) (int, bool) {
	property, _ := value.(map[string]any)
	parsed, ok := property["const"]
	if !ok {
		return 0, false
	}
	switch typed := parsed.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case json.Number:
		n, err := strconv.Atoi(typed.String())
		return n, err == nil
	default:
		return 0, false
	}
}
