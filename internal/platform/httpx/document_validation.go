package httpx

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"orbyte/internal/platform/module"
	"orbyte/internal/platform/shared"
)

func validateDocumentPayloadForType(modules *module.Service, documentType string, payload map[string]any) error {
	fields := documentValidationFieldsForType(modules, documentType)
	return validateDocumentPayloadFields(fields, payload)
}

func validateDocumentPayloadFields(fields []module.FieldDefinition, payload map[string]any) error {
	for _, field := range fields {
		path := validationPayloadPath(field.Path)
		if path == "" || field.ReadOnly {
			continue
		}
		if err := validateDocumentField(field, resolveValidationPath(payload, path), path); err != nil {
			return err
		}
	}
	return nil
}

func documentValidationFieldsForType(modules *module.Service, documentType string) []module.FieldDefinition {
	if modules == nil || strings.TrimSpace(documentType) == "" {
		return nil
	}
	merged := map[string]module.FieldDefinition{}
	order := make([]string, 0)

	for _, view := range modules.Views() {
		if view.Kind != "form" || view.DocumentType != documentType {
			continue
		}
		for _, field := range collectModuleFields(view.Fields, view.Sections, view.Tabs) {
			path := validationPayloadPath(field.Path)
			if path == "" || field.ReadOnly {
				continue
			}
			current, ok := merged[path]
			if !ok {
				merged[path] = field
				order = append(order, path)
				continue
			}
			merged[path] = mergeValidationField(current, field)
		}
	}

	result := make([]module.FieldDefinition, 0, len(order))
	for _, path := range order {
		result = append(result, merged[path])
	}
	return result
}

func collectModuleFields(fields []module.FieldDefinition, sections []module.SectionDefinition, tabs []module.TabDefinition) []module.FieldDefinition {
	items := append([]module.FieldDefinition{}, fields...)
	for _, section := range sections {
		items = append(items, section.Fields...)
	}
	for _, tab := range tabs {
		for _, section := range tab.Sections {
			items = append(items, section.Fields...)
		}
	}
	return items
}

func mergeValidationField(current, incoming module.FieldDefinition) module.FieldDefinition {
	merged := current
	if merged.Label == "" {
		merged.Label = incoming.Label
	}
	if len(merged.LabelI18n) == 0 && len(incoming.LabelI18n) > 0 {
		merged.LabelI18n = incoming.LabelI18n
	}
	merged.Required = merged.Required || incoming.Required
	if incoming.MinLength > merged.MinLength {
		merged.MinLength = incoming.MinLength
	}
	if merged.MaxLength == 0 || (incoming.MaxLength > 0 && incoming.MaxLength < merged.MaxLength) {
		merged.MaxLength = incoming.MaxLength
	}
	if strings.TrimSpace(merged.Pattern) == "" && strings.TrimSpace(incoming.Pattern) != "" {
		merged.Pattern = incoming.Pattern
	}
	if merged.MinValue == nil || (incoming.MinValue != nil && *incoming.MinValue > *merged.MinValue) {
		merged.MinValue = incoming.MinValue
	}
	if merged.MaxValue == nil || (incoming.MaxValue != nil && *incoming.MaxValue < *merged.MaxValue) {
		merged.MaxValue = incoming.MaxValue
	}
	if len(merged.Options) == 0 && len(incoming.Options) > 0 {
		merged.Options = append([]string(nil), incoming.Options...)
	}
	return merged
}

func validationPayloadPath(path string) string {
	trimmed := strings.TrimSpace(path)
	switch {
	case strings.HasPrefix(trimmed, "body.payload."):
		return strings.TrimPrefix(trimmed, "body.payload.")
	case strings.HasPrefix(trimmed, "payload."):
		return strings.TrimPrefix(trimmed, "payload.")
	default:
		return ""
	}
}

func resolveValidationPath(payload map[string]any, path string) any {
	current := any(payload)
	for _, part := range strings.Split(path, ".") {
		if currentMap, ok := current.(map[string]any); ok {
			current = currentMap[part]
			continue
		}
		return nil
	}
	return current
}

func validateDocumentField(field module.FieldDefinition, value any, path string) error {
	label := strings.TrimSpace(field.Label)
	if label == "" {
		label = humanize(path)
	}
	if field.Required && isValidationEmpty(value, field.Type) {
		return shared.Validation(label + " is required")
	}
	if isValidationEmpty(value, field.Type) {
		return nil
	}

	if len(field.Options) > 0 {
		asString := strings.TrimSpace(fmt.Sprint(value))
		if !slices.Contains(field.Options, asString) {
			return shared.Validation(label + " must be one of: " + strings.Join(field.Options, ", "))
		}
	}

	asString := strings.TrimSpace(fmt.Sprint(value))
	if field.MinLength > 0 && len([]rune(asString)) < field.MinLength {
		return shared.Validation(fmt.Sprintf("%s must be at least %d characters", label, field.MinLength))
	}
	if field.MaxLength > 0 && len([]rune(asString)) > field.MaxLength {
		return shared.Validation(fmt.Sprintf("%s must be at most %d characters", label, field.MaxLength))
	}
	if strings.TrimSpace(field.Pattern) != "" {
		matched, err := regexp.MatchString(field.Pattern, asString)
		if err != nil {
			return shared.Validation("invalid validation pattern for " + label)
		}
		if !matched {
			return shared.Validation(label + " has an invalid format")
		}
	}

	if field.MinValue != nil || field.MaxValue != nil {
		number, ok := toValidationNumber(value)
		if !ok {
			return shared.Validation(label + " must be a number")
		}
		if field.MinValue != nil && number < *field.MinValue {
			return shared.Validation(fmt.Sprintf("%s must be at least %s", label, formatValidationNumber(*field.MinValue)))
		}
		if field.MaxValue != nil && number > *field.MaxValue {
			return shared.Validation(fmt.Sprintf("%s must be at most %s", label, formatValidationNumber(*field.MaxValue)))
		}
	}
	return nil
}

func isValidationEmpty(value any, fieldType string) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	case bool:
		return false
	}
	if strings.EqualFold(fieldType, "bool") {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(value)) == ""
}

func toValidationNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func formatValidationNumber(value float64) string {
	if math.Mod(value, 1) == 0 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%g", value)
}

func humanize(value string) string {
	cleaned := strings.ReplaceAll(value, ".", " ")
	cleaned = strings.ReplaceAll(cleaned, "_", " ")
	cleaned = strings.ReplaceAll(cleaned, "-", " ")
	parts := strings.Fields(cleaned)
	for index := range parts {
		if parts[index] == "" {
			continue
		}
		parts[index] = strings.ToUpper(parts[index][:1]) + strings.ToLower(parts[index][1:])
	}
	return strings.Join(parts, " ")
}
