package integration

import "time"

type ExternalSystem struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Adapter     string         `json:"adapter"`
	Connector   string         `json:"connector,omitempty"`
	Description string         `json:"description,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Endpoint struct {
	Key         string         `json:"key"`
	SystemKey   string         `json:"system_key"`
	Name        string         `json:"name"`
	Direction   string         `json:"direction"`
	Mode        string         `json:"mode"`
	Status      string         `json:"status"`
	Connector   string         `json:"connector"`
	Description string         `json:"description,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Contract struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Version     int            `json:"version"`
	Direction   string         `json:"direction"`
	Intent      string         `json:"intent"`
	Schema      map[string]any `json:"schema,omitempty"`
	Status      string         `json:"status"`
	Description string         `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Mapping struct {
	Key             string         `json:"key"`
	SystemKey       string         `json:"system_key"`
	EndpointKey     string         `json:"endpoint_key,omitempty"`
	ContractKey     string         `json:"contract_key"`
	ContractVersion int            `json:"contract_version"`
	Direction       string         `json:"direction"`
	Status          string         `json:"status"`
	Rule            map[string]any `json:"rule,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type SubmissionRecord struct {
	ID                string         `json:"id"`
	ExternalSystemKey string         `json:"external_system_key"`
	EndpointKey       string         `json:"endpoint_key,omitempty"`
	ContractKey       string         `json:"contract_key,omitempty"`
	ContractVersion   int            `json:"contract_version,omitempty"`
	Intent            string         `json:"intent,omitempty"`
	Mode              string         `json:"mode,omitempty"`
	IdempotencyKey    string         `json:"idempotency_key,omitempty"`
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

type SubmissionAttempt struct {
	ID           string         `json:"id"`
	SubmissionID string         `json:"submission_id"`
	Attempt      int            `json:"attempt"`
	Status       string         `json:"status"`
	ErrorCode    string         `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Request      map[string]any `json:"request,omitempty"`
	Response     map[string]any `json:"response,omitempty"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

type DeadLetterRecord struct {
	ID                string         `json:"id"`
	SubmissionID      string         `json:"submission_id"`
	ExternalSystemKey string         `json:"external_system_key"`
	EndpointKey       string         `json:"endpoint_key,omitempty"`
	ContractKey       string         `json:"contract_key,omitempty"`
	ContractVersion   int            `json:"contract_version,omitempty"`
	Intent            string         `json:"intent,omitempty"`
	Status            string         `json:"status"`
	LastError         string         `json:"last_error,omitempty"`
	Payload           map[string]any `json:"payload,omitempty"`
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
