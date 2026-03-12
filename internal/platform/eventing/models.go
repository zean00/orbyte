package eventing

import "time"

type Event struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Version        int            `json:"version"`
	SchemaVersion  string         `json:"schema_version,omitempty"`
	AggregateType  string         `json:"aggregate_type"`
	AggregateID    string         `json:"aggregate_id"`
	ActorID        string         `json:"actor_id,omitempty"`
	CorrelationID  string         `json:"correlation_id,omitempty"`
	OrganizationID string         `json:"organization_id,omitempty"`
	LocationID     string         `json:"location_id,omitempty"`
	ModuleKey      string         `json:"module_key,omitempty"`
	OccurredAt     time.Time      `json:"occurred_at"`
	Payload        map[string]any `json:"payload,omitempty"`
}

type OutboxRecord struct {
	ID           string    `json:"id"`
	EventID      string    `json:"event_id"`
	EventType    string    `json:"event_type"`
	Status       string    `json:"status"`
	AttemptCount int       `json:"attempt_count"`
	LastError    string    `json:"last_error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	DispatchedAt time.Time `json:"dispatched_at,omitempty"`
}

type DeadLetterRecord struct {
	ID           string    `json:"id"`
	OutboxID     string    `json:"outbox_id"`
	EventID      string    `json:"event_id"`
	EventType    string    `json:"event_type"`
	SinkName     string    `json:"sink_name,omitempty"`
	Reason       string    `json:"reason"`
	AttemptCount int       `json:"attempt_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type OutboxDeliveryRecord struct {
	ID           string    `json:"id"`
	OutboxID     string    `json:"outbox_id"`
	EventID      string    `json:"event_id"`
	EventType    string    `json:"event_type"`
	SinkName     string    `json:"sink_name"`
	Status       string    `json:"status"`
	AttemptCount int       `json:"attempt_count"`
	LastError    string    `json:"last_error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	DispatchedAt time.Time `json:"dispatched_at,omitempty"`
}
