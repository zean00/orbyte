package audit

import "time"

type Event struct {
	ID            string         `json:"id"`
	Action        string         `json:"action"`
	TargetType    string         `json:"target_type"`
	TargetID      string         `json:"target_id"`
	ActorID       string         `json:"actor_id"`
	FromState     string         `json:"from_state,omitempty"`
	ToState       string         `json:"to_state,omitempty"`
	OccurredAt    time.Time      `json:"occurred_at"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}
