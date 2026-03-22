package featureflags

import "time"

type Definition struct {
	Key           string    `json:"key"`
	ModuleKey     string    `json:"module_key"`
	Description   string    `json:"description,omitempty"`
	AllowedScopes []string  `json:"allowed_scopes,omitempty"`
	DefaultState  bool      `json:"default_state"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Value struct {
	FlagKey       string    `json:"flag_key"`
	Scope         string    `json:"scope"`
	ScopeID       string    `json:"scope_id,omitempty"`
	Enabled       bool      `json:"enabled"`
	Status        string    `json:"status"`
	UpdatedAt     time.Time `json:"updated_at"`
	UpdatedBy     string    `json:"updated_by"`
	EffectiveFrom time.Time `json:"effective_from,omitempty"`
	EffectiveTo   time.Time `json:"effective_to,omitempty"`
}

type EffectiveValue struct {
	Key             string    `json:"key"`
	Enabled         bool      `json:"enabled"`
	SourceScope     string    `json:"source_scope"`
	SourceScopeID   string    `json:"source_scope_id,omitempty"`
	OperatingUnitID string    `json:"operating_unit_id,omitempty"`
	ResolvedAt      time.Time `json:"resolved_at"`
}

type TargetingView struct {
	Definition Definition     `json:"definition"`
	Values     []Value        `json:"values"`
	Effective  EffectiveValue `json:"effective"`
}
