package integration

import "time"

type ExternalSystem struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Adapter     string         `json:"adapter"`
	Description string         `json:"description,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type SubmissionRecord struct {
	ID                string         `json:"id"`
	ExternalSystemKey string         `json:"external_system_key"`
	OperationType     string         `json:"operation_type"`
	Status            string         `json:"status"`
	DocumentID        string         `json:"document_id,omitempty"`
	CorrelationID     string         `json:"correlation_id,omitempty"`
	ExternalReference string         `json:"external_reference,omitempty"`
	AttemptCount      int            `json:"attempt_count"`
	LastError         string         `json:"last_error,omitempty"`
	Payload           map[string]any `json:"payload,omitempty"`
	Result            map[string]any `json:"result,omitempty"`
	ProcessedAt       time.Time      `json:"processed_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type AdapterResult struct {
	ExternalReference string         `json:"external_reference,omitempty"`
	Result            map[string]any `json:"result,omitempty"`
}

type Adapter interface {
	Execute(system ExternalSystem, submission SubmissionRecord) (AdapterResult, error)
}
