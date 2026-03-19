package audit

import "time"

type Event struct {
	ID            string         `json:"id"`
	Action        string         `json:"action"`
	TargetType    string         `json:"target_type"`
	TargetID      string         `json:"target_id"`
	ActorID       string         `json:"actor_id"`
	ActorKind     string         `json:"actor_kind,omitempty"`
	FromState     string         `json:"from_state,omitempty"`
	ToState       string         `json:"to_state,omitempty"`
	OrganizationID string        `json:"organization_id,omitempty"`
	LocationID    string         `json:"location_id,omitempty"`
	OperatingUnitID string       `json:"operating_unit_id,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	OccurredAt    time.Time      `json:"occurred_at"`
	ChangeSummary map[string]any `json:"change_summary,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}

type Query struct {
	TargetType       string
	TargetID         string
	ActorID          string
	ActorKind        string
	Action           string
	CorrelationID    string
	OrganizationID   string
	LocationID       string
	OperatingUnitID  string
	OccurredFrom     time.Time
	OccurredTo       time.Time
}
