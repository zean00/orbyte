package config

import (
	"time"

	"orbyte/internal/platform/i18n"
)

type Entry struct {
	Key         string         `json:"key"`
	ModuleKey   string         `json:"module_key,omitempty"`
	Category    string         `json:"category"`
	Scope       string         `json:"scope"`
	ScopeID     string         `json:"scope_id,omitempty"`
	Value       map[string]any `json:"value"`
	UpdatedAt   time.Time      `json:"updated_at"`
	UpdatedBy   string         `json:"updated_by"`
	Description string         `json:"description,omitempty"`
}

type Definition struct {
	Key             string             `json:"key"`
	ModuleKey       string             `json:"module_key"`
	Category        string             `json:"category"`
	DisplayName     string             `json:"display_name"`
	DisplayNameI18n i18n.LocalizedText `json:"display_name_i18n,omitempty"`
	Description     string             `json:"description,omitempty"`
	DescriptionI18n i18n.LocalizedText `json:"description_i18n,omitempty"`
	AllowedScopes   []string           `json:"allowed_scopes"`
	DefaultValue    map[string]any     `json:"default_value"`
	Fields          []FieldDefinition  `json:"fields"`
}

type FieldDefinition struct {
	Key             string             `json:"key"`
	Label           string             `json:"label"`
	LabelI18n       i18n.LocalizedText `json:"label_i18n,omitempty"`
	Type            string             `json:"type"`
	Required        bool               `json:"required"`
	Description     string             `json:"description,omitempty"`
	DescriptionI18n i18n.LocalizedText `json:"description_i18n,omitempty"`
	Enum            []string           `json:"enum,omitempty"`
	Sensitive       bool               `json:"sensitive,omitempty"`
}

type EffectiveValue struct {
	Key           string         `json:"key"`
	ModuleKey     string         `json:"module_key"`
	Scope         string         `json:"scope"`
	ScopeID       string         `json:"scope_id,omitempty"`
	Value         map[string]any `json:"value"`
	SourceScope   string         `json:"source_scope"`
	SourceScopeID string         `json:"source_scope_id,omitempty"`
	ResolvedAt    time.Time      `json:"resolved_at"`
}

type ValidationIssue struct {
	Key      string `json:"key"`
	Severity string `json:"severity"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

type ValidationReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}
