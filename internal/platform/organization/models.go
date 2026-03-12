package organization

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Location struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Key            string    `json:"key"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	ParentID       string    `json:"parent_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ScopeContext struct {
	OrganizationID string   `json:"organization_id"`
	LocationID     string   `json:"location_id,omitempty"`
	AllowedScopes  []string `json:"allowed_scopes,omitempty"`
	Source         string   `json:"source"`
}
