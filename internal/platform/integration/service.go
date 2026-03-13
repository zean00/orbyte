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

	"clinic/internal/platform/jobs"
	"clinic/internal/platform/logging"
	"clinic/internal/platform/observability"
	"clinic/internal/platform/policy"
	"clinic/internal/platform/shared"
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
		Description: "Proof adapter for integration kernel flows.",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	_ = svc.RegisterSystem(ExternalSystem{
		Key:         "http_bridge",
		Name:        "HTTP Bridge",
		Status:      "inactive",
		Adapter:     "http",
		Description: "HTTP integration adapter boundary.",
		Settings:    map[string]any{"url": "", "method": "POST"},
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
	if strings.TrimSpace(system.Status) == "" {
		system.Status = "active"
	}
	return s.repo.SaveSystem(system)
}

func (s *Service) ListSystems() []ExternalSystem {
	return s.repo.ListSystems()
}

func (s *Service) ListSubmissions() []SubmissionRecord {
	return s.repo.ListSubmissions()
}

func (s *Service) GetSubmission(id string) (SubmissionRecord, bool) {
	return s.repo.GetSubmission(id)
}

func (s *Service) CreateSubmission(systemKey, operationType, documentID, correlationID string, payload map[string]any) (SubmissionRecord, error) {
	system, ok := s.repo.GetSystem(strings.TrimSpace(systemKey))
	if !ok {
		return SubmissionRecord{}, shared.NotFound("integration system not found")
	}
	if system.Status != "active" {
		return SubmissionRecord{}, shared.Conflict("integration system is not active")
	}
	if strings.TrimSpace(operationType) == "" {
		return SubmissionRecord{}, shared.Validation("operation_type is required")
	}
	if s.policy != nil {
		decision := s.policy.Evaluate(policy.Request{
			HookKey: "integration.submission.preflight",
			Inputs: map[string]any{
				"system_key":     system.Key,
				"system_status":  system.Status,
				"operation_type": strings.TrimSpace(operationType),
				"document_id":    strings.TrimSpace(documentID),
			},
		})
		if !decision.Allowed {
			return SubmissionRecord{}, shared.Forbidden(firstNonEmptyString(decision.Reason, "integration submission blocked by policy"))
		}
	}
	now := time.Now().UTC()
	record := SubmissionRecord{
		ID:                fmt.Sprintf("sub:%d", now.UnixNano()),
		ExternalSystemKey: system.Key,
		OperationType:     strings.TrimSpace(operationType),
		Status:            "queued",
		DocumentID:        strings.TrimSpace(documentID),
		CorrelationID:     strings.TrimSpace(correlationID),
		Payload:           cloneMap(payload),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
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
		if record.AttemptCount >= 3 {
			record.Status = "dead_letter"
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
