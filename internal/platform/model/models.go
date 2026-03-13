package model

import "time"

type Definition struct {
	Key                 string               `json:"key"`
	DisplayName         string               `json:"display_name"`
	OwnerModuleKey      string               `json:"owner_module_key,omitempty"`
	Version             string               `json:"version"`
	CreatePermissionKey string               `json:"create_permission_key,omitempty"`
	ListPermissionKey   string               `json:"list_permission_key,omitempty"`
	ReadPermissionKey   string               `json:"read_permission_key,omitempty"`
	UpdatePermissionKey string               `json:"update_permission_key,omitempty"`
	DefaultSort         string               `json:"default_sort,omitempty"`
	Fields              []FieldDefinition    `json:"fields,omitempty"`
	Relations           []RelationDefinition `json:"relations,omitempty"`
}

type FieldDefinition struct {
	Key                string   `json:"key"`
	Label              string   `json:"label"`
	Type               string   `json:"type"`
	Required           bool     `json:"required,omitempty"`
	ReadOnly           bool     `json:"read_only,omitempty"`
	Indexed            bool     `json:"indexed,omitempty"`
	Sensitive          bool     `json:"sensitive,omitempty"`
	SecurityClass      string   `json:"security_class,omitempty"`
	DefaultMask        string   `json:"default_mask,omitempty"`
	SearchVisible      *bool    `json:"search_visible,omitempty"`
	ExportVisible      *bool    `json:"export_visible,omitempty"`
	ReadPermissionKey  string   `json:"read_permission_key,omitempty"`
	WritePermissionKey string   `json:"write_permission_key,omitempty"`
	DefaultValue       any      `json:"default_value,omitempty"`
	DefaultRuleKey     string   `json:"default_rule_key,omitempty"`
	ComputeRuleKey     string   `json:"compute_rule_key,omitempty"`
	ConstraintRuleKeys []string `json:"constraint_rule_keys,omitempty"`
}

type Record struct {
	ModelKey  string         `json:"model_key"`
	ID        string         `json:"id"`
	Version   int            `json:"version"`
	Values    map[string]any `json:"values"`
	CreatedBy string         `json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedBy string         `json:"updated_by"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Query struct {
	Filters  map[string]string `json:"filters,omitempty"`
	SortKey  string            `json:"sort_key,omitempty"`
	Desc     bool              `json:"desc,omitempty"`
	Page     int               `json:"page,omitempty"`
	PageSize int               `json:"page_size,omitempty"`
}

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type RelationDefinition struct {
	Key            string `json:"key"`
	Type           string `json:"type"`
	TargetModelKey string `json:"target_model_key"`
	ForeignKey     string `json:"foreign_key"`
}

type ChildMutation struct {
	Operation       string                     `json:"operation,omitempty"`
	ID              string                     `json:"id,omitempty"`
	ExpectedVersion int                        `json:"expected_version,omitempty"`
	Values          map[string]any             `json:"values,omitempty"`
	Relations       map[string][]ChildMutation `json:"relations,omitempty"`
}

type CompositeMutation struct {
	ExpectedVersion int                        `json:"expected_version,omitempty"`
	Values          map[string]any             `json:"values,omitempty"`
	Relations       map[string][]ChildMutation `json:"relations,omitempty"`
}

type RuleInput struct {
	ModelKey string         `json:"model_key"`
	FieldKey string         `json:"field_key,omitempty"`
	Values   map[string]any `json:"values,omitempty"`
	Existing map[string]any `json:"existing,omitempty"`
	ActorID  string         `json:"actor_id,omitempty"`
}

type DefaultEvaluator func(RuleInput) (any, error)
type ComputeEvaluator func(RuleInput) (any, error)
type ConstraintEvaluator func(RuleInput) error
