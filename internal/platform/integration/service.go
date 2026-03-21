package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
)

type Service struct {
	repo     Repository
	adapters map[string]Adapter
	obs      *observability.Service
	logger   *logging.Service
	policy   *policy.Service
	jobs     *jobs.Service
}

const (
	JobProcessSubmission = "integration.process_submission"
	JobRetrySubmission   = "integration.retry_submission"
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

func (s *Service) RegisterSystem(system ExternalSystem) error {
	if strings.TrimSpace(system.Key) == "" || strings.TrimSpace(system.Adapter) == "" {
		return shared.Validation("integration system key and adapter are required")
	}
	if _, ok := s.adapters[system.Adapter]; !ok {
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
	return s.repo.SaveEndpoint(endpoint)
}

func (s *Service) RegisterContract(contract Contract) error {
	if strings.TrimSpace(contract.Key) == "" || contract.Version <= 0 {
		return shared.Validation("integration contract key and version are required")
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
	return s.repo.ListSubmissions()
}

func (s *Service) ListSubmissionAttempts(submissionID string) []SubmissionAttempt {
	return s.repo.ListSubmissionAttempts(strings.TrimSpace(submissionID))
}

func (s *Service) ListDeadLetters() []DeadLetterRecord {
	return s.repo.ListDeadLetters()
}

func (s *Service) GetSubmission(id string) (SubmissionRecord, bool) {
	return s.repo.GetSubmission(id)
}

func (s *Service) GetDeadLetter(id string) (DeadLetterRecord, bool) {
	return s.repo.GetDeadLetter(strings.TrimSpace(id))
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
	systemKey := strings.TrimSpace(record.ExternalSystemKey)
	system, ok := s.repo.GetSystem(systemKey)
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration system not found")
	}
	if system.Status != "active" {
		return SubmissionRecord{}, shared.Conflict("integration system is not active")
	}
	if strings.TrimSpace(record.OperationType) == "" {
		return SubmissionRecord{}, shared.Validation("operation_type is required")
	}
	if strings.TrimSpace(record.Intent) == "" {
		record.Intent = "command"
	}
	if strings.TrimSpace(record.Mode) == "" {
		record.Mode = "sync"
	}
	if strings.TrimSpace(record.CorrelationID) == "" {
		record.CorrelationID = fmt.Sprintf("integration:%d", time.Now().UTC().UnixNano())
	}
	if record.ContractKey != "" {
		contract, ok := s.repo.GetContract(strings.TrimSpace(record.ContractKey), record.ContractVersion)
		if !ok {
			return SubmissionRecord{}, shared.NotFound("integration contract not found")
		}
		if contract.Status != "active" {
			return SubmissionRecord{}, shared.Conflict("integration contract is not active")
		}
		record.Intent = firstNonEmptyString(strings.TrimSpace(record.Intent), contract.Intent)
	}
	if record.EndpointKey != "" {
		endpoint, ok := s.repo.GetEndpoint(strings.TrimSpace(record.EndpointKey))
		if !ok {
			return SubmissionRecord{}, shared.NotFound("integration endpoint not found")
		}
		if endpoint.Status != "active" {
			return SubmissionRecord{}, shared.Conflict("integration endpoint is not active")
		}
		record.Mode = firstNonEmptyString(strings.TrimSpace(record.Mode), endpoint.Mode)
	}
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
			return SubmissionRecord{}, shared.Forbidden(firstNonEmptyString(decision.Reason, "integration submission blocked by policy"))
		}
	}
	now := time.Now().UTC()
	record.ID = fmt.Sprintf("sub:%d", now.UnixNano())
	record.ExternalSystemKey = system.Key
	record.Status = "queued"
	record.Payload = cloneMap(record.Payload)
	record.CreatedAt = now
	record.UpdatedAt = now
	if err := s.repo.SaveSubmission(record); err != nil {
		return SubmissionRecord{}, err
	}
	s.observeMetric("integration.submissions.queued.total")
	return record, nil
}

func (s *Service) ProcessSubmission(id string) (SubmissionRecord, error) {
	record, ok := s.repo.GetSubmission(strings.TrimSpace(id))
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration submission not found")
	}
	system, ok := s.repo.GetSystem(record.ExternalSystemKey)
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration system not found")
	}
	adapter, ok := s.adapters[system.Adapter]
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration adapter not found")
	}
	record.Status = "processing"
	record.AttemptCount++
	record.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveSubmission(record); err != nil {
		return SubmissionRecord{}, err
	}
	result, err := adapter.Execute(system, record)
	record.UpdatedAt = time.Now().UTC()
	if err != nil {
		record.LastError = err.Error()
		record.ProcessedAt = time.Now().UTC()
		_ = s.repo.SaveSubmissionAttempt(SubmissionAttempt{
			ID:           fmt.Sprintf("attempt:%s:%d", record.ID, record.AttemptCount),
			SubmissionID: record.ID,
			Attempt:      record.AttemptCount,
			Status:       "failed",
			ErrorCode:    classifyIntegrationError(err),
			ErrorMessage: err.Error(),
			Request:      submissionAttemptRequest(record),
			OccurredAt:   record.ProcessedAt,
		})
		if record.AttemptCount >= 3 {
			record.Status = "dead_letter"
			_ = s.repo.SaveDeadLetter(DeadLetterRecord{
				ID:                fmt.Sprintf("dl:%s", record.ID),
				SubmissionID:      record.ID,
				ExternalSystemKey: record.ExternalSystemKey,
				EndpointKey:       record.EndpointKey,
				ContractKey:       record.ContractKey,
				ContractVersion:   record.ContractVersion,
				Intent:            record.Intent,
				Status:            "open",
				LastError:         record.LastError,
				Payload:           cloneMap(record.Payload),
				CreatedAt:         record.CreatedAt,
				UpdatedAt:         record.UpdatedAt,
			})
		} else {
			record.Status = "failed"
		}
		_ = s.repo.SaveSubmission(record)
		s.observeMetric("integration.submissions.failed.total")
		s.emitLog("integration.submission.failed", map[string]any{
			"submission_id": record.ID,
			"system_key":    record.ExternalSystemKey,
			"operation":     record.OperationType,
			"status":        record.Status,
		})
		return record, nil
	}
	record.Status = "succeeded"
	record.LastError = ""
	record.ExternalReference = result.ExternalReference
	record.Result = cloneMap(result.Result)
	record.ProcessedAt = time.Now().UTC()
	_ = s.repo.SaveSubmissionAttempt(SubmissionAttempt{
		ID:           fmt.Sprintf("attempt:%s:%d", record.ID, record.AttemptCount),
		SubmissionID: record.ID,
		Attempt:      record.AttemptCount,
		Status:       "succeeded",
		Request:      submissionAttemptRequest(record),
		Response:     cloneMap(result.Result),
		OccurredAt:   record.ProcessedAt,
	})
	if err := s.repo.SaveSubmission(record); err != nil {
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
	record, ok := s.repo.GetSubmission(strings.TrimSpace(id))
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration submission not found")
	}
	record.Status = "queued"
	record.LastError = ""
	record.UpdatedAt = time.Now().UTC()
	if err := s.repo.SaveSubmission(record); err != nil {
		return SubmissionRecord{}, err
	}
	return s.ProcessSubmission(record.ID)
}

func (s *Service) ReplayDeadLetter(id string) (SubmissionRecord, error) {
	dead, ok := s.repo.GetDeadLetter(strings.TrimSpace(id))
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration dead letter not found")
	}
	return s.RetrySubmission(dead.SubmissionID)
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
	default:
		return "execution_error"
	}
}

type FakeAdapter struct{}

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

func (a HTTPAdapter) Execute(system ExternalSystem, submission SubmissionRecord) (AdapterResult, error) {
	if a.Client == nil {
		a.Client = defaultHTTPClient()
	}
	target, _ := system.Settings["url"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return AdapterResult{}, fmt.Errorf("http adapter target url is required")
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
