package audit

import "time"

type Event struct {
	ID                string         `json:"id"`
	Action            string         `json:"action"`
	TargetType        string         `json:"target_type"`
	TargetID          string         `json:"target_id"`
	ActorID           string         `json:"actor_id"`
	ActorKind         string         `json:"actor_kind,omitempty"`
	OnBehalfOfUserID  string         `json:"on_behalf_of_user_id,omitempty"`
	DelegationGrantID string         `json:"delegation_grant_id,omitempty"`
	FromState         string         `json:"from_state,omitempty"`
	ToState           string         `json:"to_state,omitempty"`
	OrganizationID    string         `json:"organization_id,omitempty"`
	LocationID        string         `json:"location_id,omitempty"`
	OperatingUnitID   string         `json:"operating_unit_id,omitempty"`
	RequestID         string         `json:"request_id,omitempty"`
	OccurredAt        time.Time      `json:"occurred_at"`
	ChangeSummary     map[string]any `json:"change_summary,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CorrelationID     string         `json:"correlation_id,omitempty"`
}

type Query struct {
	TargetType        string
	TargetID          string
	ActorID           string
	ActorKind         string
	OnBehalfOfUserID  string
	Action            string
	CorrelationID     string
	RequestID         string
	DelegationGrantID string
	FromState         string
	ToState           string
	OrganizationID    string
	LocationID        string
	OperatingUnitID   string
	OccurredFrom      time.Time
	OccurredTo        time.Time
	Text              string
	MetadataKey       string
	MetadataValue     string
	Sort              string
	Direction         string
	Page              int
	PageSize          int
}

type SearchResult struct {
	Items   []Event        `json:"items"`
	Total   int            `json:"total"`
	Facets  map[string]any `json:"facets,omitempty"`
	Summary map[string]any `json:"summary,omitempty"`
}
