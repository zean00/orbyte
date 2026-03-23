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

type ValidationIssue struct {
	Code     string `json:"code"`
	Field    string `json:"field,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ConfigFieldDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Description string `json:"description,omitempty"`
}

type AdapterDescriptor struct {
	Key                 string                  `json:"key"`
	Name                string                  `json:"name"`
	ContractVersion     string                  `json:"contract_version,omitempty"`
	Stability           string                  `json:"stability,omitempty"`
	SupportedDirections []string                `json:"supported_directions,omitempty"`
	SupportedModes      []string                `json:"supported_modes,omitempty"`
	SupportsHealthCheck bool                    `json:"supports_health_check,omitempty"`
	SupportsRetry       bool                    `json:"supports_retry,omitempty"`
	SupportsDeadLetter  bool                    `json:"supports_dead_letter,omitempty"`
	SupportsReplay      bool                    `json:"supports_replay,omitempty"`
	SupportsSecrets     bool                    `json:"supports_secrets,omitempty"`
	SupportsIdempotency bool                    `json:"supports_idempotency,omitempty"`
	ConfigFields        []ConfigFieldDefinition `json:"config_fields,omitempty"`
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
	SchemaRef   string         `json:"schema_ref,omitempty"`
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

type RetryPolicy struct {
	MaxAttempts         int      `json:"max_attempts"`
	BackoffSeconds      int      `json:"backoff_seconds"`
	RetryableErrorCodes []string `json:"retryable_error_codes,omitempty"`
}

type SubmissionRecord struct {
	ID                string            `json:"id"`
	ExternalSystemKey string            `json:"external_system_key"`
	EndpointKey       string            `json:"endpoint_key,omitempty"`
	ContractKey       string            `json:"contract_key,omitempty"`
	ContractVersion   int               `json:"contract_version,omitempty"`
	Intent            string            `json:"intent,omitempty"`
	Mode              string            `json:"mode,omitempty"`
	IdempotencyKey    string            `json:"idempotency_key,omitempty"`
	OperationType     string            `json:"operation_type"`
	Status            string            `json:"status"`
	DocumentID        string            `json:"document_id,omitempty"`
	CorrelationID     string            `json:"correlation_id,omitempty"`
	ExternalReference string            `json:"external_reference,omitempty"`
	AttemptCount      int               `json:"attempt_count"`
	LastError         string            `json:"last_error,omitempty"`
	LastErrorCode     string            `json:"last_error_code,omitempty"`
	FailureCategory   string            `json:"failure_category,omitempty"`
	NextRetryAt       time.Time         `json:"next_retry_at,omitempty"`
	Terminal          bool              `json:"terminal,omitempty"`
	Payload           map[string]any    `json:"payload,omitempty"`
	Result            map[string]any    `json:"result,omitempty"`
	ValidationIssues  []ValidationIssue `json:"validation_issues,omitempty"`
	ProcessedAt       time.Time         `json:"processed_at,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type SubmissionAttempt struct {
	ID              string         `json:"id"`
	SubmissionID    string         `json:"submission_id"`
	Attempt         int            `json:"attempt"`
	Status          string         `json:"status"`
	ErrorCode       string         `json:"error_code,omitempty"`
	ErrorMessage    string         `json:"error_message,omitempty"`
	FailureCategory string         `json:"failure_category,omitempty"`
	Retryable       bool           `json:"retryable,omitempty"`
	Request         map[string]any `json:"request,omitempty"`
	Response        map[string]any `json:"response,omitempty"`
	OccurredAt      time.Time      `json:"occurred_at"`
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
	LastErrorCode     string         `json:"last_error_code,omitempty"`
	FailureCategory   string         `json:"failure_category,omitempty"`
	Payload           map[string]any `json:"payload,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type ConnectorConfigView struct {
	Raw              map[string]any    `json:"raw,omitempty"`
	Resolved         map[string]any    `json:"resolved,omitempty"`
	Effective        map[string]any    `json:"effective,omitempty"`
	ValidationIssues []ValidationIssue `json:"validation_issues,omitempty"`
}

type ConnectorHealth struct {
	SystemKey           string            `json:"system_key"`
	Status              string            `json:"status"`
	Adapter             string            `json:"adapter"`
	Connector           string            `json:"connector,omitempty"`
	LastSuccessAt       time.Time         `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time         `json:"last_failure_at,omitempty"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	OpenDeadLetters     int               `json:"open_dead_letters"`
	QueuedCount         int               `json:"queued_count"`
	FailedCount         int               `json:"failed_count"`
	OldestPendingAgeSec int64             `json:"oldest_pending_age_seconds"`
	ValidationIssues    []ValidationIssue `json:"validation_issues,omitempty"`
}

type AdapterResult struct {
	ExternalReference string         `json:"external_reference,omitempty"`
	Result            map[string]any `json:"result,omitempty"`
}

type Adapter interface {
	Descriptor() AdapterDescriptor
	ValidateConfig(system ExternalSystem, settings map[string]any) []ValidationIssue
	ValidateSubmission(system ExternalSystem, submission SubmissionRecord, settings map[string]any) []ValidationIssue
	HealthCheck(system ExternalSystem, settings map[string]any) error
	Execute(system ExternalSystem, submission SubmissionRecord) (AdapterResult, error)
}
