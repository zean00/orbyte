package idempotency

import "time"

type Record struct {
	Key          string         `json:"key"`
	Operation    string         `json:"operation"`
	ActorID      string         `json:"actor_id,omitempty"`
	RequestHash  string         `json:"request_hash"`
	Status       string         `json:"status"`
	ResponseCode int            `json:"response_code"`
	Response     map[string]any `json:"response,omitempty"`
	Error        string         `json:"error,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Outcome struct {
	StatusCode int            `json:"status_code"`
	Response   map[string]any `json:"response,omitempty"`
}
