package reference

import (
	"time"

	"orbyte/internal/platform/i18n"
)

type TypeDefinition struct {
	Key             string             `json:"key"`
	DisplayName     string             `json:"display_name"`
	DisplayNameI18n i18n.LocalizedText `json:"display_name_i18n,omitempty"`
	OwnerModuleKey  string             `json:"owner_module_key,omitempty"`
	ValueType       string             `json:"value_type,omitempty"`
	AllowedScopes   []string           `json:"allowed_scopes,omitempty"`
}

type Record struct {
	TypeKey         string             `json:"type_key"`
	Key             string             `json:"key"`
	DisplayName     string             `json:"display_name"`
	DisplayNameI18n i18n.LocalizedText `json:"display_name_i18n,omitempty"`
	Scope           string             `json:"scope,omitempty"`
	ScopeID         string             `json:"scope_id,omitempty"`
	Status          string             `json:"status,omitempty"`
	Value           map[string]any     `json:"value,omitempty"`
	ExternalCodes   map[string]string  `json:"external_codes,omitempty"`
	EffectiveFrom   time.Time          `json:"effective_from,omitempty"`
	EffectiveTo     time.Time          `json:"effective_to,omitempty"`
	UpdatedAt       time.Time          `json:"updated_at"`
	UpdatedBy       string             `json:"updated_by,omitempty"`
}

type ResolvedSet struct {
	TypeKey      string    `json:"type_key"`
	Scope        string    `json:"scope"`
	ScopeID      string    `json:"scope_id,omitempty"`
	ResolvedAt   time.Time `json:"resolved_at"`
	SourceScopes []string  `json:"source_scopes,omitempty"`
	Items        []Record  `json:"items"`
}
