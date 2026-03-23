package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"orbyte/contracts"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/secretstore"
	"orbyte/internal/platform/shared"
)

type Service struct {
	repo        Repository
	adapters    map[string]Adapter
	obs         *observability.Service
	logger      *logging.Service
	policy      *policy.Service
	jobs        *jobs.Service
	secrets     *secretstore.Service
	retryPolicy RetryPolicy
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "integration validation failed"
	}
	return e.Issues[0].Message
}

const (
	JobProcessSubmission = "integration.process_submission"
	JobRetrySubmission   = "integration.retry_submission"

	submissionMetaKey = "_integration_runtime"
	deadLetterMetaKey = "_integration_dead_letter"
	attemptMetaKey    = "_integration_attempt"
)

func NewService(obs *observability.Service, logger *logging.Service) *Service {
	return NewServiceWithRepository(NewMemoryRepository(), obs, logger)
}

func NewServiceWithRepository(repo Repository, obs *observability.Service, logger *logging.Service) *Service {
	svc := &Service{
		repo:     repo,
		adapters: map[string]Adapter{},
		obs:      obs,
		logger:   logger,
		retryPolicy: RetryPolicy{
			MaxAttempts:         3,
			BackoffSeconds:      30,
			RetryableErrorCodes: []string{"timeout", "remote_5xx", "execution_error"},
		},
	}
	svc.RegisterAdapter("fake", FakeAdapter{})
	svc.RegisterAdapter("http", HTTPAdapter{Client: defaultHTTPClient()})
	now := time.Now().UTC()
	_ = svc.RegisterSystem(ExternalSystem{
		Key:         "fake_erp",
		Name:        "Fake ERP",
		Status:      "active",
		Adapter:     "fake",
		Connector:   "fake",
		Description: "Proof adapter for integration kernel flows.",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	_ = svc.RegisterSystem(ExternalSystem{
		Key:         "http_bridge",
		Name:        "HTTP Bridge",
		Status:      "inactive",
		Adapter:     "http",
		Connector:   "http",
		Description: "HTTP integration adapter boundary.",
		Settings:    map[string]any{"url": "", "method": "POST"},
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	_ = svc.RegisterContract(Contract{
		Key:         "document.submit",
		Name:        "Document Submit",
		Version:     1,
		Direction:   "outbound",
		Intent:      "command",
		SchemaRef:   contracts.IntegrationSchemaPath("document.submit", 1),
		Status:      "active",
		Description: "Default canonical contract for document submission.",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	_ = svc.RegisterEndpoint(Endpoint{
		Key:         "fake_erp.default",
		SystemKey:   "fake_erp",
		Name:        "Fake ERP Default",
		Direction:   "outbound",
		Mode:        "sync",
		Status:      "active",
		Connector:   "fake",
		Description: "Default outbound endpoint for fake ERP.",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	return svc
}

func (s *Service) AdapterDescriptors() []AdapterDescriptor {
	items := make([]AdapterDescriptor, 0, len(s.adapters))
	for _, adapter := range s.adapters {
		desc := adapter.Descriptor()
		if strings.TrimSpace(desc.ContractVersion) == "" {
			desc.ContractVersion = "2026-03-23"
		}
		if strings.TrimSpace(desc.Stability) == "" {
			desc.Stability = "stable"
		}
		items = append(items, desc)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) RetryPolicy() RetryPolicy {
	return s.retryPolicy
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: envDurationSeconds("APP_INTEGRATION_HTTP_TIMEOUT_SECONDS", 15*time.Second)}
}

func (s *Service) AttachPolicy(policySvc *policy.Service) {
	s.policy = policySvc
}

func (s *Service) AttachJobs(jobSvc *jobs.Service) {
	s.jobs = jobSvc
	if jobSvc == nil {
		return
	}
	jobSvc.RegisterHandler(JobProcessSubmission, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		id, _ := payload["submission_id"].(string)
		record, err := s.ProcessSubmission(id)
		if err != nil {
			return nil, err
		}
		return map[string]any{"submission_id": record.ID, "status": record.Status}, nil
	})
	jobSvc.RegisterHandler(JobRetrySubmission, func(_ context.Context, payload map[string]any) (map[string]any, error) {
		id, _ := payload["submission_id"].(string)
		record, err := s.RetrySubmission(id)
		if err != nil {
			return nil, err
		}
		return map[string]any{"submission_id": record.ID, "status": record.Status}, nil
	})
}

func (s *Service) AttachSecrets(secretSvc *secretstore.Service) {
	s.secrets = secretSvc
}

func (s *Service) EnqueueProcessSubmission(id string) (jobs.Job, error) {
	if s == nil || s.jobs == nil {
		return jobs.Job{}, fmt.Errorf("integration jobs are not configured")
	}
	return s.jobs.EnqueueUnique(JobProcessSubmission, map[string]any{"submission_id": strings.TrimSpace(id)}, JobProcessSubmission+":"+strings.TrimSpace(id))
}

func (s *Service) EnqueueRetrySubmission(id string) (jobs.Job, error) {
	if s == nil || s.jobs == nil {
		return jobs.Job{}, fmt.Errorf("integration jobs are not configured")
	}
	return s.jobs.EnqueueUnique(JobRetrySubmission, map[string]any{"submission_id": strings.TrimSpace(id)}, JobRetrySubmission+":"+strings.TrimSpace(id))
}

func (s *Service) RegisterAdapter(key string, adapter Adapter) {
	if strings.TrimSpace(key) == "" || adapter == nil {
		return
	}
	s.adapters[strings.TrimSpace(key)] = adapter
}

func (s *Service) ListAdapterDescriptors() []AdapterDescriptor {
	items := make([]AdapterDescriptor, 0, len(s.adapters))
	for key, adapter := range s.adapters {
		descriptor := adapter.Descriptor()
		if strings.TrimSpace(descriptor.Key) == "" {
			descriptor.Key = key
		}
		items = append(items, descriptor)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Service) RegisterSystem(system ExternalSystem) error {
	if strings.TrimSpace(system.Key) == "" || strings.TrimSpace(system.Adapter) == "" {
		return shared.Validation("integration system key and adapter are required")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return shared.Validation("integration adapter is not registered")
	}
	now := time.Now().UTC()
	if system.CreatedAt.IsZero() {
		system.CreatedAt = now
	}
	system.UpdatedAt = now
	if strings.TrimSpace(system.Connector) == "" {
		system.Connector = system.Adapter
	}
	if strings.TrimSpace(system.Status) == "" {
		system.Status = "active"
	}
	system.Settings = s.prepareStoredSettings("integration.system."+system.Key, nil, system.Settings, adapter.Descriptor())
	if system.Status == "active" {
		if issues := adapter.ValidateConfig(system, s.resolveSettings(system.Settings)); len(issues) > 0 {
			return ValidationError{Issues: issues}
		}
	}
	return s.repo.SaveSystem(system)
}

func (s *Service) RegisterEndpoint(endpoint Endpoint) error {
	if strings.TrimSpace(endpoint.Key) == "" || strings.TrimSpace(endpoint.SystemKey) == "" {
		return shared.Validation("integration endpoint key and system_key are required")
	}
	system, ok := s.repo.GetSystem(strings.TrimSpace(endpoint.SystemKey))
	if !ok {
		return shared.NotFound("integration system not found")
	}
	now := time.Now().UTC()
	if endpoint.CreatedAt.IsZero() {
		endpoint.CreatedAt = now
	}
	endpoint.UpdatedAt = now
	if strings.TrimSpace(endpoint.Connector) == "" {
		endpoint.Connector = firstNonEmptyString(system.Connector, system.Adapter)
	}
	if strings.TrimSpace(endpoint.Direction) == "" {
		endpoint.Direction = "outbound"
	}
	if strings.TrimSpace(endpoint.Mode) == "" {
		endpoint.Mode = "sync"
	}
	if strings.TrimSpace(endpoint.Status) == "" {
		endpoint.Status = "active"
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return shared.Validation("integration adapter is not registered")
	}
	endpoint.Settings = s.prepareStoredSettings("integration.endpoint."+endpoint.Key, nil, endpoint.Settings, adapter.Descriptor())
	if endpoint.Status == "active" {
		merged := mergeSettings(s.resolveSettings(system.Settings), s.resolveSettings(endpoint.Settings))
		if issues := append([]ValidationIssue{}, adapter.ValidateConfig(system, merged)...); len(issues) > 0 {
			return ValidationError{Issues: issues}
		}
	}
	return s.repo.SaveEndpoint(endpoint)
}

func (s *Service) RegisterContract(contract Contract) error {
	if strings.TrimSpace(contract.Key) == "" || contract.Version <= 0 {
		return shared.Validation("integration contract key and version are required")
	}
	if strings.TrimSpace(contract.SchemaRef) == "" {
		defaultRef := contracts.IntegrationSchemaPath(strings.TrimSpace(contract.Key), contract.Version)
		if contracts.Exists(defaultRef) {
			contract.SchemaRef = defaultRef
		}
	}
	if strings.TrimSpace(contract.SchemaRef) != "" {
		schema, err := contracts.Load(strings.TrimSpace(contract.SchemaRef))
		if err != nil {
			return shared.Validation("integration contract schema_ref could not be loaded")
		}
		if len(contract.Schema) == 0 {
			contract.Schema = schema
		}
	}
	if len(contract.Schema) == 0 {
		return shared.Validation("integration contract schema is required")
	}
	if err := contracts.ValidateIntegrationContract(contract.SchemaRef, strings.TrimSpace(contract.Key), contract.Version, contract.Schema); err != nil {
		return shared.Validation(err.Error())
	}
	now := time.Now().UTC()
	if contract.CreatedAt.IsZero() {
		contract.CreatedAt = now
	}
	contract.UpdatedAt = now
	if strings.TrimSpace(contract.Direction) == "" {
		contract.Direction = "outbound"
	}
	if strings.TrimSpace(contract.Intent) == "" {
		contract.Intent = "command"
	}
	if strings.TrimSpace(contract.Status) == "" {
		contract.Status = "active"
	}
	return s.repo.SaveContract(contract)
}

func (s *Service) RegisterMapping(mapping Mapping) error {
	if strings.TrimSpace(mapping.Key) == "" || strings.TrimSpace(mapping.SystemKey) == "" || strings.TrimSpace(mapping.ContractKey) == "" || mapping.ContractVersion <= 0 {
		return shared.Validation("integration mapping key, system_key, contract_key, and contract_version are required")
	}
	now := time.Now().UTC()
	if mapping.CreatedAt.IsZero() {
		mapping.CreatedAt = now
	}
	mapping.UpdatedAt = now
	if strings.TrimSpace(mapping.Direction) == "" {
		mapping.Direction = "outbound"
	}
	if strings.TrimSpace(mapping.Status) == "" {
		mapping.Status = "active"
	}
	return s.repo.SaveMapping(mapping)
}

func (s *Service) ListSystems() []ExternalSystem {
	return s.repo.ListSystems()
}

func (s *Service) ListEndpoints() []Endpoint {
	return s.repo.ListEndpoints()
}

func (s *Service) ListContracts() []Contract {
	return s.repo.ListContracts()
}

func (s *Service) ListMappings() []Mapping {
	return s.repo.ListMappings()
}

func (s *Service) ListSubmissions() []SubmissionRecord {
	items := s.repo.ListSubmissions()
	for i := range items {
		items[i] = decodeSubmissionRuntime(items[i])
	}
	return items
}

func (s *Service) ListSubmissionAttempts(submissionID string) []SubmissionAttempt {
	items := s.repo.ListSubmissionAttempts(strings.TrimSpace(submissionID))
	for i := range items {
		items[i] = decodeSubmissionAttemptRuntime(items[i])
	}
	return items
}

func (s *Service) ListDeadLetters() []DeadLetterRecord {
	items := s.repo.ListDeadLetters()
	for i := range items {
		items[i] = decodeDeadLetterRuntime(items[i])
	}
	return items
}

func (s *Service) GetSubmission(id string) (SubmissionRecord, bool) {
	record, ok := s.repo.GetSubmission(id)
	if !ok {
		return SubmissionRecord{}, false
	}
	return decodeSubmissionRuntime(record), true
}

func (s *Service) GetDeadLetter(id string) (DeadLetterRecord, bool) {
	item, ok := s.repo.GetDeadLetter(strings.TrimSpace(id))
	if !ok {
		return DeadLetterRecord{}, false
	}
	return decodeDeadLetterRuntime(item), true
}

func (s *Service) CreateSubmission(systemKey, operationType, documentID, correlationID string, payload map[string]any) (SubmissionRecord, error) {
	return s.CreateDelivery(SubmissionRecord{
		ExternalSystemKey: strings.TrimSpace(systemKey),
		OperationType:     strings.TrimSpace(operationType),
		DocumentID:        strings.TrimSpace(documentID),
		CorrelationID:     strings.TrimSpace(correlationID),
		Payload:           cloneMap(payload),
		Intent:            "command",
		Mode:              "sync",
		ContractKey:       "document.submit",
		ContractVersion:   1,
	})
}

func (s *Service) CreateDelivery(record SubmissionRecord) (SubmissionRecord, error) {
	system, endpoint, contract, adapter, settings, issues, err := s.preflight(record)
	if err != nil {
		return SubmissionRecord{}, err
	}
	if len(issues) > 0 {
		return SubmissionRecord{}, ValidationError{Issues: issues}
	}
	if strings.TrimSpace(record.IdempotencyKey) != "" {
		if existing, ok := s.repo.FindSubmissionByIdempotency(system.Key, strings.TrimSpace(record.EndpointKey), strings.TrimSpace(record.ContractKey), strings.TrimSpace(record.IdempotencyKey)); ok {
			return decodeSubmissionRuntime(existing), nil
		}
	}
	now := time.Now().UTC()
	record.ID = fmt.Sprintf("sub:%d", now.UnixNano())
	record.ExternalSystemKey = system.Key
	record.EndpointKey = endpoint.Key
	record.ContractKey = contract.Key
	record.ContractVersion = contract.Version
	record.Intent = firstNonEmptyString(strings.TrimSpace(record.Intent), contract.Intent)
	record.Mode = firstNonEmptyString(strings.TrimSpace(record.Mode), endpoint.Mode)
	record.Status = "queued"
	record.Payload = cloneMap(record.Payload)
	record.Result = map[string]any{}
	record.ValidationIssues = nil
	record.LastError = ""
	record.LastErrorCode = ""
	record.FailureCategory = ""
	record.NextRetryAt = time.Time{}
	record.Terminal = false
	record.CreatedAt = now
	record.UpdatedAt = now
	if err := s.repo.SaveSubmission(encodeSubmissionRuntime(record)); err != nil {
		return SubmissionRecord{}, err
	}
	_ = adapter
	_ = settings
	s.observeMetric("integration.submissions.queued.total")
	return record, nil
}

func (s *Service) ProcessSubmission(id string) (SubmissionRecord, error) {
	record, ok := s.GetSubmission(strings.TrimSpace(id))
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration submission not found")
	}
	system, _, contract, adapter, settings, issues, err := s.preflight(record)
	if err != nil {
		return SubmissionRecord{}, err
	}
	record.AttemptCount++
	record.Status = "processing"
	record.UpdatedAt = time.Now().UTC()
	record.ValidationIssues = nil
	if len(issues) > 0 {
		record = s.failSubmission(record, submissionFailure{
			Code:             "validation_error",
			Category:         "validation_error",
			Message:          "integration submission validation failed",
			Retryable:        false,
			ValidationIssues: issues,
		})
		return record, nil
	}
	if err := s.repo.SaveSubmission(encodeSubmissionRuntime(record)); err != nil {
		return SubmissionRecord{}, err
	}
	_ = contract
	result, execErr := adapter.Execute(applyResolvedSettings(system, settings), record)
	if execErr != nil {
		record = s.failSubmission(record, classifyExecutionFailure(execErr, s.retryPolicy))
		return record, nil
	}
	record.Status = "succeeded"
	record.LastError = ""
	record.LastErrorCode = ""
	record.FailureCategory = ""
	record.ExternalReference = result.ExternalReference
	record.Result = cloneMap(result.Result)
	record.ProcessedAt = time.Now().UTC()
	record.NextRetryAt = time.Time{}
	record.Terminal = false
	record.ValidationIssues = nil
	_ = s.repo.SaveSubmissionAttempt(encodeSubmissionAttemptRuntime(SubmissionAttempt{
		ID:           fmt.Sprintf("attempt:%s:%d", record.ID, record.AttemptCount),
		SubmissionID: record.ID,
		Attempt:      record.AttemptCount,
		Status:       "succeeded",
		Request:      submissionAttemptRequest(record),
		Response:     cloneMap(result.Result),
		OccurredAt:   record.ProcessedAt,
	}))
	if err := s.repo.SaveSubmission(encodeSubmissionRuntime(record)); err != nil {
		return SubmissionRecord{}, err
	}
	s.observeMetric("integration.submissions.succeeded.total")
	s.emitLog("integration.submission.succeeded", map[string]any{
		"submission_id": record.ID,
		"system_key":    record.ExternalSystemKey,
		"operation":     record.OperationType,
		"status":        record.Status,
	})
	return record, nil
}

func (s *Service) RetrySubmission(id string) (SubmissionRecord, error) {
	record, ok := s.GetSubmission(strings.TrimSpace(id))
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration submission not found")
	}
	record.Status = "queued"
	record.LastError = ""
	record.LastErrorCode = ""
	record.FailureCategory = ""
	record.NextRetryAt = time.Time{}
	record.Terminal = false
	record.ValidationIssues = nil
	record.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveSubmission(encodeSubmissionRuntime(record)); err != nil {
		return SubmissionRecord{}, err
	}
	return s.ProcessSubmission(record.ID)
}

func (s *Service) ReplayDeadLetter(id string) (SubmissionRecord, error) {
	dead, ok := s.GetDeadLetter(strings.TrimSpace(id))
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration dead letter not found")
	}
	dead.Status = "replayed"
	dead.UpdatedAt = time.Now().UTC()
	_ = s.repo.SaveDeadLetter(encodeDeadLetterRuntime(dead))
	return s.RetrySubmission(dead.SubmissionID)
}

func (s *Service) ValidateSystemConfig(key string) (ConnectorConfigView, error) {
	system, ok := s.repo.GetSystem(strings.TrimSpace(key))
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration system not found")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration adapter not found")
	}
	raw := cloneMap(system.Settings)
	resolved := s.resolveSettings(raw)
	return ConnectorConfigView{
		Raw:              redactSettings(adapter.Descriptor(), raw),
		Resolved:         redactSettings(adapter.Descriptor(), resolved),
		Effective:        redactSettings(adapter.Descriptor(), resolved),
		ValidationIssues: adapter.ValidateConfig(system, resolved),
	}, nil
}

func (s *Service) ValidateSystemSettings(key string, settings map[string]any) (ConnectorConfigView, error) {
	system, ok := s.repo.GetSystem(strings.TrimSpace(key))
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration system not found")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration adapter not found")
	}
	raw := s.prepareStoredSettings("integration.system."+system.Key, system.Settings, settings, adapter.Descriptor())
	resolved := s.resolveSettings(raw)
	view := ConnectorConfigView{
		Raw:              redactSettings(adapter.Descriptor(), raw),
		Resolved:         redactSettings(adapter.Descriptor(), resolved),
		Effective:        redactSettings(adapter.Descriptor(), resolved),
		ValidationIssues: adapter.ValidateConfig(system, resolved),
	}
	if len(view.ValidationIssues) > 0 {
		return view, ValidationError{Issues: view.ValidationIssues}
	}
	return view, nil
}

func (s *Service) ValidateEndpointConfig(key string) (ConnectorConfigView, error) {
	endpoint, ok := s.repo.GetEndpoint(strings.TrimSpace(key))
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration endpoint not found")
	}
	system, ok := s.repo.GetSystem(endpoint.SystemKey)
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration system not found")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration adapter not found")
	}
	raw := cloneMap(endpoint.Settings)
	resolved := mergeSettings(s.resolveSettings(system.Settings), s.resolveSettings(raw))
	return ConnectorConfigView{
		Raw:              redactSettings(adapter.Descriptor(), raw),
		Resolved:         redactSettings(adapter.Descriptor(), resolved),
		Effective:        redactSettings(adapter.Descriptor(), resolved),
		ValidationIssues: adapter.ValidateConfig(system, resolved),
	}, nil
}

func (s *Service) ValidateEndpointSettings(key string, settings map[string]any) (ConnectorConfigView, error) {
	endpoint, ok := s.repo.GetEndpoint(strings.TrimSpace(key))
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration endpoint not found")
	}
	system, ok := s.repo.GetSystem(endpoint.SystemKey)
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration system not found")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return ConnectorConfigView{}, shared.NotFound("integration adapter not found")
	}
	raw := s.prepareStoredSettings("integration.endpoint."+endpoint.Key, endpoint.Settings, settings, adapter.Descriptor())
	resolved := mergeSettings(s.resolveSettings(system.Settings), s.resolveSettings(raw))
	view := ConnectorConfigView{
		Raw:              redactSettings(adapter.Descriptor(), raw),
		Resolved:         redactSettings(adapter.Descriptor(), resolved),
		Effective:        redactSettings(adapter.Descriptor(), resolved),
		ValidationIssues: adapter.ValidateConfig(system, resolved),
	}
	if len(view.ValidationIssues) > 0 {
		return view, ValidationError{Issues: view.ValidationIssues}
	}
	return view, nil
}

func (s *Service) UpdateSystemSettings(key string, settings map[string]any) (ExternalSystem, ConnectorConfigView, error) {
	system, ok := s.repo.GetSystem(strings.TrimSpace(key))
	if !ok {
		return ExternalSystem{}, ConnectorConfigView{}, shared.NotFound("integration system not found")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return ExternalSystem{}, ConnectorConfigView{}, shared.NotFound("integration adapter not found")
	}
	system.Settings = s.prepareStoredSettings("integration.system."+system.Key, system.Settings, settings, adapter.Descriptor())
	system.UpdatedAt = time.Now().UTC()
	view, err := s.ValidateSystemSettings(key, settings)
	if err != nil {
		return ExternalSystem{}, view, err
	}
	if err := s.repo.SaveSystem(system); err != nil {
		return ExternalSystem{}, ConnectorConfigView{}, err
	}
	view, _ = s.ValidateSystemConfig(system.Key)
	return system, view, nil
}

func (s *Service) UpdateEndpointSettings(key string, settings map[string]any) (Endpoint, ConnectorConfigView, error) {
	endpoint, ok := s.repo.GetEndpoint(strings.TrimSpace(key))
	if !ok {
		return Endpoint{}, ConnectorConfigView{}, shared.NotFound("integration endpoint not found")
	}
	system, ok := s.repo.GetSystem(endpoint.SystemKey)
	if !ok {
		return Endpoint{}, ConnectorConfigView{}, shared.NotFound("integration system not found")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return Endpoint{}, ConnectorConfigView{}, shared.NotFound("integration adapter not found")
	}
	endpoint.Settings = s.prepareStoredSettings("integration.endpoint."+endpoint.Key, endpoint.Settings, settings, adapter.Descriptor())
	endpoint.UpdatedAt = time.Now().UTC()
	view, err := s.ValidateEndpointSettings(key, settings)
	if err != nil {
		return Endpoint{}, view, err
	}
	if err := s.repo.SaveEndpoint(endpoint); err != nil {
		return Endpoint{}, ConnectorConfigView{}, err
	}
	view, _ = s.ValidateEndpointConfig(endpoint.Key)
	return endpoint, view, nil
}

func (s *Service) HealthSummary() []ConnectorHealth {
	systems := s.repo.ListSystems()
	items := make([]ConnectorHealth, 0, len(systems))
	for _, system := range systems {
		items = append(items, s.systemHealth(system))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SystemKey < items[j].SystemKey })
	return items
}

func (s *Service) HealthForSystem(key string) (ConnectorHealth, error) {
	system, ok := s.repo.GetSystem(strings.TrimSpace(key))
	if !ok {
		return ConnectorHealth{}, shared.NotFound("integration system not found")
	}
	return s.systemHealth(system), nil
}

func (s *Service) systemHealth(system ExternalSystem) ConnectorHealth {
	health := ConnectorHealth{
		SystemKey: system.Key,
		Status:    "healthy",
		Adapter:   system.Adapter,
		Connector: system.Connector,
	}
	if system.Status != "active" {
		health.Status = "inactive"
	}
	if adapter, ok := s.adapters[system.Adapter]; ok {
		settings := s.resolveSettings(system.Settings)
		health.ValidationIssues = adapter.ValidateConfig(system, settings)
		if len(health.ValidationIssues) > 0 && health.Status == "healthy" {
			health.Status = "degraded"
		}
		if health.Status == "healthy" && adapter.Descriptor().SupportsHealthCheck {
			if err := adapter.HealthCheck(system, settings); err != nil {
				health.Status = "failed"
				health.ValidationIssues = append(health.ValidationIssues, ValidationIssue{Code: "health_check_failed", Severity: "error", Message: err.Error()})
			}
		}
	} else {
		health.Status = "failed"
		health.ValidationIssues = append(health.ValidationIssues, ValidationIssue{Code: "adapter_missing", Severity: "error", Message: "integration adapter is not registered"})
	}
	now := time.Now().UTC()
	for _, record := range s.ListSubmissions() {
		if record.ExternalSystemKey != system.Key {
			continue
		}
		switch record.Status {
		case "queued", "processing":
			health.QueuedCount++
			if !record.CreatedAt.IsZero() {
				age := int64(now.Sub(record.CreatedAt).Seconds())
				if age > health.OldestPendingAgeSec {
					health.OldestPendingAgeSec = age
				}
			}
		case "failed", "dead_letter":
			health.FailedCount++
		}
		if record.Status == "succeeded" && (health.LastSuccessAt.IsZero() || record.ProcessedAt.After(health.LastSuccessAt)) {
			health.LastSuccessAt = record.ProcessedAt
			health.ConsecutiveFailures = 0
		}
		if (record.Status == "failed" || record.Status == "dead_letter") && record.ProcessedAt.After(health.LastFailureAt) {
			health.LastFailureAt = record.ProcessedAt
		}
	}
	for _, dead := range s.ListDeadLetters() {
		if dead.ExternalSystemKey == system.Key && dead.Status == "open" {
			health.OpenDeadLetters++
		}
	}
	if health.OpenDeadLetters > 0 && health.Status == "healthy" {
		health.Status = "degraded"
	}
	if health.FailedCount > 0 && health.LastFailureAt.After(health.LastSuccessAt) {
		health.ConsecutiveFailures = health.FailedCount
		if health.Status == "healthy" {
			health.Status = "degraded"
		}
	}
	return health
}

func (s *Service) preflight(record SubmissionRecord) (ExternalSystem, Endpoint, Contract, Adapter, map[string]any, []ValidationIssue, error) {
	systemKey := strings.TrimSpace(record.ExternalSystemKey)
	system, ok := s.repo.GetSystem(systemKey)
	if !ok {
		return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.NotFound("integration system not found")
	}
	if system.Status != "active" {
		return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.Conflict("integration system is not active")
	}
	if strings.TrimSpace(record.OperationType) == "" {
		return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, []ValidationIssue{{Code: "required", Field: "operation_type", Severity: "error", Message: "operation_type is required"}}, nil
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.NotFound("integration adapter not found")
	}
	endpoint := Endpoint{}
	if strings.TrimSpace(record.EndpointKey) != "" {
		var found bool
		endpoint, found = s.repo.GetEndpoint(strings.TrimSpace(record.EndpointKey))
		if !found {
			return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.NotFound("integration endpoint not found")
		}
		if endpoint.SystemKey != system.Key {
			return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, []ValidationIssue{{Code: "endpoint_system_mismatch", Field: "endpoint_key", Severity: "error", Message: "integration endpoint does not belong to the selected system"}}, nil
		}
		if endpoint.Status != "active" {
			return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.Conflict("integration endpoint is not active")
		}
	} else {
		endpoint = Endpoint{SystemKey: system.Key, Direction: "outbound", Mode: firstNonEmptyString(record.Mode, "sync")}
	}
	contract := Contract{}
	if strings.TrimSpace(record.ContractKey) != "" {
		var found bool
		contract, found = s.repo.GetContract(strings.TrimSpace(record.ContractKey), record.ContractVersion)
		if !found {
			return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.NotFound("integration contract not found")
		}
		if contract.Status != "active" {
			return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.Conflict("integration contract is not active")
		}
	}
	direction := firstNonEmptyString(endpoint.Direction, firstNonEmptyString(contract.Direction, "outbound"))
	mode := firstNonEmptyString(endpoint.Mode, firstNonEmptyString(record.Mode, "sync"))
	issues := make([]ValidationIssue, 0)
	descriptor := adapter.Descriptor()
	if len(descriptor.SupportedDirections) > 0 && !containsString(descriptor.SupportedDirections, direction) {
		issues = append(issues, ValidationIssue{Code: "unsupported_direction", Field: "direction", Severity: "error", Message: "adapter does not support this direction"})
	}
	if len(descriptor.SupportedModes) > 0 && !containsString(descriptor.SupportedModes, mode) {
		issues = append(issues, ValidationIssue{Code: "unsupported_mode", Field: "mode", Severity: "error", Message: "adapter does not support this mode"})
	}
	settings := mergeSettings(s.resolveSettings(system.Settings), s.resolveSettings(endpoint.Settings))
	issues = append(issues, adapter.ValidateConfig(system, settings)...)
	issues = append(issues, validateContractPayload(contract, record.Payload)...)
	issues = append(issues, adapter.ValidateSubmission(system, record, settings)...)
	if s.policy != nil {
		decision := s.policy.Evaluate(policy.Request{
			HookKey: "integration.submission.preflight",
			Inputs: map[string]any{
				"system_key":     system.Key,
				"system_status":  system.Status,
				"endpoint_key":   strings.TrimSpace(record.EndpointKey),
				"contract_key":   strings.TrimSpace(record.ContractKey),
				"intent":         strings.TrimSpace(record.Intent),
				"mode":           strings.TrimSpace(record.Mode),
				"operation_type": strings.TrimSpace(record.OperationType),
				"document_id":    strings.TrimSpace(record.DocumentID),
			},
		})
		if !decision.Allowed {
			return ExternalSystem{}, Endpoint{}, Contract{}, nil, nil, nil, shared.Forbidden(firstNonEmptyString(decision.Reason, "integration submission blocked by policy"))
		}
	}
	return system, endpoint, contract, adapter, settings, issues, nil
}

type submissionFailure struct {
	Code             string
	Category         string
	Message          string
	Retryable        bool
	ValidationIssues []ValidationIssue
}

func (s *Service) failSubmission(record SubmissionRecord, failure submissionFailure) SubmissionRecord {
	record.LastError = failure.Message
	record.LastErrorCode = failure.Code
	record.FailureCategory = failure.Category
	record.ProcessedAt = time.Now().UTC()
	record.ValidationIssues = append([]ValidationIssue(nil), failure.ValidationIssues...)
	record.Terminal = !failure.Retryable
	record.NextRetryAt = time.Time{}
	if failure.Retryable && record.AttemptCount < maxInt(s.retryPolicy.MaxAttempts, 1) {
		record.Status = "failed"
		record.NextRetryAt = record.ProcessedAt.Add(time.Duration(maxInt(s.retryPolicy.BackoffSeconds, 1)) * time.Second)
		record.Terminal = false
	} else {
		record.Status = "dead_letter"
		record.Terminal = true
		dead := DeadLetterRecord{
			ID:                fmt.Sprintf("dl:%s", record.ID),
			SubmissionID:      record.ID,
			ExternalSystemKey: record.ExternalSystemKey,
			EndpointKey:       record.EndpointKey,
			ContractKey:       record.ContractKey,
			ContractVersion:   record.ContractVersion,
			Intent:            record.Intent,
			Status:            "open",
			LastError:         record.LastError,
			LastErrorCode:     record.LastErrorCode,
			FailureCategory:   record.FailureCategory,
			Payload:           cloneMap(record.Payload),
			CreatedAt:         record.CreatedAt,
			UpdatedAt:         record.ProcessedAt,
		}
		_ = s.repo.SaveDeadLetter(encodeDeadLetterRuntime(dead))
	}
	_ = s.repo.SaveSubmissionAttempt(encodeSubmissionAttemptRuntime(SubmissionAttempt{
		ID:              fmt.Sprintf("attempt:%s:%d", record.ID, record.AttemptCount),
		SubmissionID:    record.ID,
		Attempt:         record.AttemptCount,
		Status:          record.Status,
		ErrorCode:       record.LastErrorCode,
		ErrorMessage:    record.LastError,
		FailureCategory: record.FailureCategory,
		Retryable:       failure.Retryable,
		Request:         submissionAttemptRequest(record),
		Response: map[string]any{
			"validation_issues": record.ValidationIssues,
		},
		OccurredAt: record.ProcessedAt,
	}))
	_ = s.repo.SaveSubmission(encodeSubmissionRuntime(record))
	s.observeMetric("integration.submissions.failed.total")
	s.emitLog("integration.submission.failed", map[string]any{
		"submission_id": record.ID,
		"system_key":    record.ExternalSystemKey,
		"operation":     record.OperationType,
		"status":        record.Status,
	})
	return record
}

func (s *Service) observeMetric(key string) {
	if s.obs == nil {
		return
	}
	_ = s.obs.RecordMetric(key, map[string]string{}, 1)
}

func (s *Service) emitLog(key string, fields map[string]any) {
	if s.obs != nil {
		_ = s.obs.EmitLogEvent(key, fields)
	}
	if s.logger != nil {
		s.logger.Info(key, fields)
	}
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneIssues(items []ValidationIssue) []ValidationIssue {
	if len(items) == 0 {
		return nil
	}
	out := make([]ValidationIssue, len(items))
	copy(out, items)
	return out
}

func submissionAttemptRequest(record SubmissionRecord) map[string]any {
	return map[string]any{
		"system_key":       record.ExternalSystemKey,
		"endpoint_key":     record.EndpointKey,
		"contract_key":     record.ContractKey,
		"contract_version": record.ContractVersion,
		"intent":           record.Intent,
		"mode":             record.Mode,
		"operation_type":   record.OperationType,
		"correlation_id":   record.CorrelationID,
		"payload":          cloneMap(record.Payload),
	}
}

func classifyIntegrationError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "timeout"):
		return "timeout"
	case strings.Contains(message, "status 4"):
		return "remote_4xx"
	case strings.Contains(message, "status 5"):
		return "remote_5xx"
	case strings.Contains(message, "validation"):
		return "validation_error"
	case strings.Contains(message, "config"):
		return "config_error"
	default:
		return "execution_error"
	}
}

func classifyExecutionFailure(err error, policy RetryPolicy) submissionFailure {
	code := classifyIntegrationError(err)
	failure := submissionFailure{
		Code:     code,
		Category: code,
		Message:  err.Error(),
	}
	failure.Retryable = containsString(policy.RetryableErrorCodes, code)
	if !failure.Retryable && code == "remote_4xx" {
		failure.Category = "validation_error"
	}
	return failure
}

func validateContractPayload(contract Contract, payload map[string]any) []ValidationIssue {
	if len(contract.Schema) == 0 {
		return nil
	}
	schemaIssues := contracts.ValidateObject(contract.Schema, payload)
	issues := make([]ValidationIssue, 0, len(schemaIssues))
	for _, issue := range schemaIssues {
		issues = append(issues, ValidationIssue{
			Code:     firstNonEmptyString(strings.TrimSpace(issue.Code), "validation_error"),
			Field:    strings.TrimSpace(issue.Field),
			Severity: "error",
			Message:  firstNonEmptyString(strings.TrimSpace(issue.Message), "contract validation failed"),
		})
	}
	return issues
}

func mergeSettings(base, overrides map[string]any) map[string]any {
	merged := cloneMap(base)
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

func containsString(items []string, candidate string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func secretRefFromValue(raw any) string {
	switch value := raw.(type) {
	case map[string]any:
		if ref, _ := value["secret_ref"].(string); strings.TrimSpace(ref) != "" {
			return strings.TrimSpace(ref)
		}
	}
	return ""
}

func (s *Service) prepareStoredSettings(prefix string, existing, settings map[string]any, descriptor AdapterDescriptor) map[string]any {
	stored := cloneMap(existing)
	for key, raw := range settings {
		if ref := secretRefFromValue(raw); ref != "" {
			stored[key] = map[string]any{"secret_ref": ref}
			continue
		}
		text, ok := raw.(string)
		if !ok {
			stored[key] = raw
			continue
		}
		if !isSensitiveConfigField(prefix, key, s, descriptor) {
			stored[key] = text
			continue
		}
		if s.secrets == nil {
			stored[key] = text
			continue
		}
		secret, err := s.secrets.Upsert(prefix+":"+key, secretRefFromValue(stored[key]), text)
		if err != nil {
			stored[key] = text
			continue
		}
		stored[key] = map[string]any{"secret_ref": secret.Ref}
	}
	return stored
}

func isSensitiveConfigField(prefix, key string, svc *Service, descriptor AdapterDescriptor) bool {
	for _, field := range descriptor.ConfigFields {
		if field.Key == key {
			return field.Sensitive
		}
	}
	parts := strings.Split(prefix, ".")
	if len(parts) < 3 || svc == nil {
		return false
	}
	var system ExternalSystem
	if parts[1] == "system" {
		item, ok := svc.repo.GetSystem(parts[2])
		if !ok {
			return false
		}
		system = item
	} else {
		endpoint, ok := svc.repo.GetEndpoint(parts[2])
		if !ok {
			return false
		}
		item, ok := svc.repo.GetSystem(endpoint.SystemKey)
		if !ok {
			return false
		}
		system = item
	}
	adapter, ok := svc.adapters[system.Adapter]
	if !ok {
		return false
	}
	for _, field := range adapter.Descriptor().ConfigFields {
		if field.Key == key {
			return field.Sensitive
		}
	}
	return false
}

func (s *Service) resolveSettings(settings map[string]any) map[string]any {
	resolved := cloneMap(settings)
	for key, raw := range resolved {
		ref := secretRefFromValue(raw)
		if ref == "" || s.secrets == nil {
			continue
		}
		if secret, ok := s.secrets.Resolve(ref); ok {
			resolved[key] = secret
		}
	}
	return resolved
}

func redactSettings(descriptor AdapterDescriptor, settings map[string]any) map[string]any {
	redacted := cloneMap(settings)
	sensitive := map[string]struct{}{}
	for _, field := range descriptor.ConfigFields {
		if field.Sensitive {
			sensitive[field.Key] = struct{}{}
		}
	}
	for key := range sensitive {
		if raw, ok := redacted[key]; ok {
			if ref := secretRefFromValue(raw); ref != "" {
				redacted[key] = map[string]any{"secret_ref": ref, "value": "[redacted]"}
			} else {
				redacted[key] = "[redacted]"
			}
		}
	}
	return redacted
}

func applyResolvedSettings(system ExternalSystem, settings map[string]any) ExternalSystem {
	system.Settings = cloneMap(settings)
	return system
}

func encodeSubmissionRuntime(record SubmissionRecord) SubmissionRecord {
	record.Result = cloneMap(record.Result)
	record.Result[submissionMetaKey] = map[string]any{
		"last_error_code":   record.LastErrorCode,
		"failure_category":  record.FailureCategory,
		"next_retry_at":     record.NextRetryAt,
		"terminal":          record.Terminal,
		"validation_issues": record.ValidationIssues,
	}
	return record
}

func decodeSubmissionRuntime(record SubmissionRecord) SubmissionRecord {
	meta, _ := record.Result[submissionMetaKey].(map[string]any)
	record.Result = cloneMap(record.Result)
	delete(record.Result, submissionMetaKey)
	if meta == nil {
		return record
	}
	record.LastErrorCode = strings.TrimSpace(stringifyAny(meta["last_error_code"]))
	record.FailureCategory = strings.TrimSpace(stringifyAny(meta["failure_category"]))
	if next := parseTimeAny(meta["next_retry_at"]); !next.IsZero() {
		record.NextRetryAt = next
	}
	record.Terminal = boolFromAny(meta["terminal"])
	record.ValidationIssues = issuesFromAny(meta["validation_issues"])
	return record
}

func encodeDeadLetterRuntime(record DeadLetterRecord) DeadLetterRecord {
	record.Payload = cloneMap(record.Payload)
	record.Payload[deadLetterMetaKey] = map[string]any{
		"last_error_code":  record.LastErrorCode,
		"failure_category": record.FailureCategory,
	}
	return record
}

func decodeDeadLetterRuntime(record DeadLetterRecord) DeadLetterRecord {
	meta, _ := record.Payload[deadLetterMetaKey].(map[string]any)
	record.Payload = cloneMap(record.Payload)
	delete(record.Payload, deadLetterMetaKey)
	if meta == nil {
		return record
	}
	record.LastErrorCode = strings.TrimSpace(stringifyAny(meta["last_error_code"]))
	record.FailureCategory = strings.TrimSpace(stringifyAny(meta["failure_category"]))
	return record
}

func encodeSubmissionAttemptRuntime(attempt SubmissionAttempt) SubmissionAttempt {
	attempt.Response = cloneMap(attempt.Response)
	attempt.Response[attemptMetaKey] = map[string]any{
		"failure_category": attempt.FailureCategory,
		"retryable":        attempt.Retryable,
	}
	return attempt
}

func decodeSubmissionAttemptRuntime(attempt SubmissionAttempt) SubmissionAttempt {
	meta, _ := attempt.Response[attemptMetaKey].(map[string]any)
	attempt.Response = cloneMap(attempt.Response)
	delete(attempt.Response, attemptMetaKey)
	if meta == nil {
		return attempt
	}
	attempt.FailureCategory = strings.TrimSpace(stringifyAny(meta["failure_category"]))
	attempt.Retryable = boolFromAny(meta["retryable"])
	return attempt
}

func issuesFromAny(raw any) []ValidationIssue {
	if raw == nil {
		return nil
	}
	encoded, _ := json.Marshal(raw)
	var items []ValidationIssue
	_ = json.Unmarshal(encoded, &items)
	return items
}

func parseTimeAny(raw any) time.Time {
	switch value := raw.(type) {
	case time.Time:
		return value
	case string:
		parsed, _ := time.Parse(time.RFC3339Nano, value)
		return parsed
	}
	return time.Time{}
}

func boolFromAny(raw any) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	}
	return false
}

func stringifyAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		encoded, _ := json.Marshal(value)
		return string(encoded)
	}
}

type FakeAdapter struct{}

func (FakeAdapter) Descriptor() AdapterDescriptor {
	return AdapterDescriptor{
		Key:                 "fake",
		Name:                "Fake Adapter",
		ContractVersion:     "2026-03-23",
		Stability:           "stable",
		SupportedDirections: []string{"outbound"},
		SupportedModes:      []string{"sync"},
		SupportsHealthCheck: true,
		SupportsRetry:       true,
		SupportsDeadLetter:  true,
		SupportsReplay:      true,
		SupportsIdempotency: true,
	}
}

func (FakeAdapter) ValidateConfig(system ExternalSystem, settings map[string]any) []ValidationIssue {
	return nil
}

func (FakeAdapter) ValidateSubmission(system ExternalSystem, submission SubmissionRecord, settings map[string]any) []ValidationIssue {
	return nil
}

func (FakeAdapter) HealthCheck(system ExternalSystem, settings map[string]any) error {
	return nil
}

func (FakeAdapter) Execute(system ExternalSystem, submission SubmissionRecord) (AdapterResult, error) {
	if fail, _ := submission.Payload["force_fail"].(bool); fail {
		return AdapterResult{}, fmt.Errorf("forced fake adapter failure")
	}
	return AdapterResult{
		ExternalReference: system.Key + ":" + submission.ID,
		Result: map[string]any{
			"accepted": true,
			"system":   system.Key,
		},
	}, nil
}

type HTTPAdapter struct {
	Client *http.Client
}

func (a HTTPAdapter) Descriptor() AdapterDescriptor {
	return AdapterDescriptor{
		Key:                 "http",
		Name:                "HTTP Adapter",
		ContractVersion:     "2026-03-23",
		Stability:           "stable",
		SupportedDirections: []string{"outbound"},
		SupportedModes:      []string{"sync"},
		SupportsHealthCheck: true,
		SupportsRetry:       true,
		SupportsDeadLetter:  true,
		SupportsReplay:      true,
		SupportsSecrets:     true,
		SupportsIdempotency: true,
		ConfigFields: []ConfigFieldDefinition{
			{Key: "url", Label: "URL", Type: "string", Required: true},
			{Key: "method", Label: "Method", Type: "string"},
			{Key: "bearer_token", Label: "Bearer Token", Type: "string", Sensitive: true},
		},
	}
}

func (a HTTPAdapter) ValidateConfig(system ExternalSystem, settings map[string]any) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	url, _ := settings["url"].(string)
	if strings.TrimSpace(url) == "" {
		issues = append(issues, ValidationIssue{Code: "required", Field: "url", Severity: "error", Message: "http adapter target url is required"})
	}
	return issues
}

func (a HTTPAdapter) ValidateSubmission(system ExternalSystem, submission SubmissionRecord, settings map[string]any) []ValidationIssue {
	return nil
}

func (a HTTPAdapter) HealthCheck(system ExternalSystem, settings map[string]any) error {
	if issues := a.ValidateConfig(system, settings); len(issues) > 0 {
		return fmt.Errorf("%s", issues[0].Message)
	}
	return nil
}

func (a HTTPAdapter) Execute(system ExternalSystem, submission SubmissionRecord) (AdapterResult, error) {
	if a.Client == nil {
		a.Client = defaultHTTPClient()
	}
	target, _ := system.Settings["url"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return AdapterResult{}, fmt.Errorf("http adapter config error: target url is required")
	}
	method, _ := system.Settings["method"].(string)
	if strings.TrimSpace(method) == "" {
		method = http.MethodPost
	}
	body, err := json.Marshal(submission.Payload)
	if err != nil {
		return AdapterResult{}, err
	}
	req, err := http.NewRequest(strings.ToUpper(method), target, bytes.NewReader(body))
	if err != nil {
		return AdapterResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token, _ := system.Settings["bearer_token"].(string); strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	resp, err := a.Client.Do(req)
	if err != nil {
		return AdapterResult{}, err
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return AdapterResult{}, fmt.Errorf("http adapter status %d", resp.StatusCode)
	}
	result := map[string]any{"status_code": resp.StatusCode, "body": string(responseBody)}
	if len(responseBody) > 0 {
		var decoded map[string]any
		if json.Unmarshal(responseBody, &decoded) == nil {
			result = decoded
			result["status_code"] = resp.StatusCode
		}
	}
	return AdapterResult{
		ExternalReference: firstNonEmptyString(result["external_reference"], system.Key+":"+submission.ID),
		Result:            result,
	}, nil
}

func firstNonEmptyString(value any, fallback string) string {
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	return fallback
}

func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}
