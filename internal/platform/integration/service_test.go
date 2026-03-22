package integration

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/secretstore"
)

func TestCreateAndProcessSubmission(t *testing.T) {
	obs := observability.NewService()
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.queued.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.succeeded.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.failed.total", Type: "counter"})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.succeeded", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.failed", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	svc := NewService(obs, logging.NewService())

	record, err := svc.CreateSubmission("fake_erp", "submit_document", "doc-1", "corr-1", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("create submission failed: %v", err)
	}
	if record.Status != "queued" {
		t.Fatalf("expected queued submission, got %s", record.Status)
	}
	record, err = svc.ProcessSubmission(record.ID)
	if err != nil {
		t.Fatalf("process submission failed: %v", err)
	}
	if record.Status != "succeeded" || record.ExternalReference == "" {
		t.Fatalf("expected succeeded submission with external reference, got %+v", record)
	}
}

func TestRetrySubmissionAfterFailure(t *testing.T) {
	obs := observability.NewService()
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.queued.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.succeeded.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.failed.total", Type: "counter"})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.succeeded", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.failed", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	svc := NewService(obs, logging.NewService())

	record, err := svc.CreateSubmission("fake_erp", "submit_document", "doc-1", "corr-1", map[string]any{"force_fail": true})
	if err != nil {
		t.Fatalf("create submission failed: %v", err)
	}
	record, err = svc.ProcessSubmission(record.ID)
	if err != nil {
		t.Fatalf("process failure submission failed: %v", err)
	}
	if record.Status != "failed" {
		t.Fatalf("expected failed submission, got %s", record.Status)
	}
	record.Payload["force_fail"] = false
	if err := svc.repo.SaveSubmission(record); err != nil {
		t.Fatalf("save submission failed: %v", err)
	}
	record, err = svc.RetrySubmission(record.ID)
	if err != nil {
		t.Fatalf("retry submission failed: %v", err)
	}
	if record.Status != "succeeded" {
		t.Fatalf("expected retry to succeed, got %s", record.Status)
	}
}

func TestCreateSubmissionBlockedByPolicy(t *testing.T) {
	obs := observability.NewService()
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.queued.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.succeeded.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.failed.total", Type: "counter"})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.succeeded", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.failed", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{
		Key:           "integration.submission.preflight",
		Kind:          "integration",
		Target:        "integration_submission",
		RuleSchemaKey: "integration.submission.preflight",
		DefaultRule:   map[string]any{"blocked_operation_types": []string{"push_invoice"}, "required_system_status": "active"},
	}); err != nil {
		t.Fatalf("register policy hook failed: %v", err)
	}
	if err := policies.SetEvaluator("integration.submission.preflight", func(req policy.Request) policy.Decision {
		for _, blocked := range req.Rule["blocked_operation_types"].([]string) {
			if blocked == req.Inputs["operation_type"] {
				return policy.Decision{Allowed: false, Reason: "integration operation blocked by policy"}
			}
		}
		return policy.Decision{Allowed: true}
	}); err != nil {
		t.Fatalf("set policy evaluator failed: %v", err)
	}
	svc := NewService(obs, logging.NewService())
	svc.AttachPolicy(policies)

	if _, err := svc.CreateSubmission("fake_erp", "push_invoice", "doc-1", "corr-1", map[string]any{"foo": "bar"}); err == nil {
		t.Fatal("expected policy preflight to block submission")
	}
}

func TestCreateDeliveryReturnsStructuredValidationIssues(t *testing.T) {
	obs := observability.NewService()
	svc := NewService(obs, logging.NewService())
	if err := svc.RegisterContract(Contract{
		Key:       "strict.contract",
		Name:      "Strict Contract",
		Version:   1,
		Direction: "outbound",
		Intent:    "command",
		Status:    "active",
		Schema: map[string]any{
			"required": []any{"title"},
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
		},
	}); err != nil {
		t.Fatalf("register contract failed: %v", err)
	}
	if _, err := svc.CreateDelivery(SubmissionRecord{
		ExternalSystemKey: "fake_erp",
		ContractKey:       "strict.contract",
		ContractVersion:   1,
		OperationType:     "push_document",
		Payload:           map[string]any{"foo": "bar"},
	}); err == nil {
		t.Fatal("expected validation error")
	} else {
		var validationErr ValidationError
		if !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "url") && !errorAs(err, &validationErr) {
			t.Fatalf("expected structured validation error, got %v", err)
		}
	}
}

func TestCreateDeliveryDeduplicatesByIdempotencyKey(t *testing.T) {
	svc := NewService(observability.NewService(), logging.NewService())
	record1, err := svc.CreateDelivery(SubmissionRecord{
		ExternalSystemKey: "fake_erp",
		ContractKey:       "document.submit",
		ContractVersion:   1,
		OperationType:     "push_document",
		IdempotencyKey:    "idem-1",
		Payload:           map[string]any{"foo": "bar"},
	})
	if err != nil {
		t.Fatalf("create delivery failed: %v", err)
	}
	record2, err := svc.CreateDelivery(SubmissionRecord{
		ExternalSystemKey: "fake_erp",
		ContractKey:       "document.submit",
		ContractVersion:   1,
		OperationType:     "push_document",
		IdempotencyKey:    "idem-1",
		Payload:           map[string]any{"foo": "bar"},
	})
	if err != nil {
		t.Fatalf("second create delivery failed: %v", err)
	}
	if record1.ID != record2.ID {
		t.Fatalf("expected idempotent submission reuse, got %s and %s", record1.ID, record2.ID)
	}
}

func TestUpdateSystemSettingsStoresSecretReference(t *testing.T) {
	svc := NewService(observability.NewService(), logging.NewService())
	secrets := secretstore.NewService()
	svc.AttachSecrets(secrets)
	record, view, err := svc.UpdateSystemSettings("http_bridge", map[string]any{
		"url":          "https://example.test/submit",
		"bearer_token": "super-secret",
	})
	if err != nil {
		t.Fatalf("update system settings failed: %v", err)
	}
	raw := record.Settings["bearer_token"].(map[string]any)
	if _, ok := raw["secret_ref"].(string); !ok {
		t.Fatalf("expected secret_ref in stored settings, got %+v", record.Settings["bearer_token"])
	}
	if got := view.Resolved["bearer_token"]; got != "[redacted]" {
		t.Fatalf("expected redacted resolved token, got %+v", got)
	}
}

func TestRegisterSystemAndEndpointStoreSecretReferenceOnCreate(t *testing.T) {
	svc := NewService(observability.NewService(), logging.NewService())
	secrets := secretstore.NewService()
	svc.AttachSecrets(secrets)
	if err := svc.RegisterSystem(ExternalSystem{
		Key:      "http_create_secret",
		Name:     "HTTP Create Secret",
		Adapter:  "http",
		Settings: map[string]any{"url": "https://example.test/system", "bearer_token": "system-secret"},
	}); err != nil {
		t.Fatalf("register system failed: %v", err)
	}
	system, ok := svc.repo.GetSystem("http_create_secret")
	if !ok {
		t.Fatal("expected system to be stored")
	}
	rawSystem, ok := system.Settings["bearer_token"].(map[string]any)
	if !ok {
		t.Fatalf("expected map settings for created system token, got %+v", system.Settings["bearer_token"])
	}
	if _, ok := rawSystem["secret_ref"].(string); !ok {
		t.Fatalf("expected secret_ref in created system settings, got %+v", system.Settings["bearer_token"])
	}
	if err := svc.RegisterEndpoint(Endpoint{
		Key:       "http_create_secret.submit",
		SystemKey: "http_create_secret",
		Name:      "Submit",
		Settings:  map[string]any{"bearer_token": "endpoint-secret"},
	}); err != nil {
		t.Fatalf("register endpoint failed: %v", err)
	}
	endpoint, ok := svc.repo.GetEndpoint("http_create_secret.submit")
	if !ok {
		t.Fatal("expected endpoint to be stored")
	}
	rawEndpoint, ok := endpoint.Settings["bearer_token"].(map[string]any)
	if !ok {
		t.Fatalf("expected map settings for created endpoint token, got %+v", endpoint.Settings["bearer_token"])
	}
	if _, ok := rawEndpoint["secret_ref"].(string); !ok {
		t.Fatalf("expected secret_ref in created endpoint settings, got %+v", endpoint.Settings["bearer_token"])
	}
}

func TestHealthSummaryReflectsValidationAndDeadLetters(t *testing.T) {
	svc := NewService(observability.NewService(), logging.NewService())
	_, _, _ = svc.UpdateSystemSettings("http_bridge", map[string]any{"url": ""})
	health := svc.HealthSummary()
	var httpBridge ConnectorHealth
	for _, item := range health {
		if item.SystemKey == "http_bridge" {
			httpBridge = item
			break
		}
	}
	if httpBridge.Status == "" {
		t.Fatal("expected http bridge health item")
	}
	if httpBridge.Status != "inactive" && httpBridge.Status != "degraded" && httpBridge.Status != "failed" {
		t.Fatalf("unexpected health status: %+v", httpBridge)
	}
}

func TestHTTPAdapterExecuteSuccessAndFailure(t *testing.T) {
	adapter := HTTPAdapter{
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Authorization") != "Bearer secret" {
				t.Fatalf("expected bearer token header, got %+v", req.Header)
			}
			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), `"foo":"bar"`) {
				t.Fatalf("expected request body to contain payload, got %s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"external_reference":"ext-1","accepted":true}`)),
			}, nil
		})},
	}
	result, err := adapter.Execute(ExternalSystem{
		Key:      "http_bridge",
		Adapter:  "http",
		Settings: map[string]any{"url": "https://example.test/submit", "method": "post", "bearer_token": "secret"},
	}, SubmissionRecord{ID: "sub-1", Payload: map[string]any{"foo": "bar"}})
	if err != nil {
		t.Fatalf("expected http adapter success, got %v", err)
	}
	if result.ExternalReference != "ext-1" {
		t.Fatalf("expected decoded external reference, got %+v", result)
	}

	adapter = HTTPAdapter{
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`boom`)),
			}, nil
		})},
	}
	if _, err := adapter.Execute(ExternalSystem{
		Key:      "http_bridge",
		Adapter:  "http",
		Settings: map[string]any{"url": "https://example.test/submit"},
	}, SubmissionRecord{ID: "sub-2", Payload: map[string]any{"foo": "bar"}}); err == nil {
		t.Fatal("expected http adapter status failure")
	}
}

func TestDefaultHTTPClientUsesConfiguredTimeout(t *testing.T) {
	old := os.Getenv("APP_INTEGRATION_HTTP_TIMEOUT_SECONDS")
	defer os.Setenv("APP_INTEGRATION_HTTP_TIMEOUT_SECONDS", old)
	t.Setenv("APP_INTEGRATION_HTTP_TIMEOUT_SECONDS", "3")
	client := defaultHTTPClient()
	if client.Timeout != 3*time.Second {
		t.Fatalf("expected configured timeout, got %s", client.Timeout)
	}
}

func TestAttachJobsQueuesSubmissionProcessing(t *testing.T) {
	obs := observability.NewService()
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.queued.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.succeeded.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.failed.total", Type: "counter"})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.succeeded", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{Key: "integration.submission.failed", RequiredFields: []string{"submission_id", "system_key", "operation", "status"}})
	svc := NewService(obs, logging.NewService())
	jobSvc := jobs.NewService()
	svc.AttachJobs(jobSvc)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobSvc.Start(ctx)
	defer jobSvc.Stop()

	record, err := svc.CreateSubmission("fake_erp", "submit_document", "doc-1", "corr-1", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("create submission failed: %v", err)
	}
	job, err := svc.EnqueueProcessSubmission(record.ID)
	if err != nil {
		t.Fatalf("enqueue process submission failed: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stored, ok := svc.GetSubmission(record.ID)
		if ok && stored.Status == "succeeded" {
			return
		}
		if queued, ok := jobSvc.Get(job.ID); ok && queued.Status == jobs.StatusDeadLetter {
			t.Fatalf("expected queued integration job to succeed, got %+v", queued)
		}
		time.Sleep(10 * time.Millisecond)
	}
	stored, _ := svc.GetSubmission(record.ID)
	t.Fatalf("expected async integration processing to succeed, got %+v", stored)
}

func TestCreateDeliveryTracksContractAndAttempts(t *testing.T) {
	obs := observability.NewService()
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.queued.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.succeeded.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.failed.total", Type: "counter"})
	svc := NewService(obs, logging.NewService())

	record, err := svc.CreateDelivery(SubmissionRecord{
		ExternalSystemKey: "fake_erp",
		EndpointKey:       "fake_erp.default",
		ContractKey:       "document.submit",
		ContractVersion:   1,
		Intent:            "command",
		Mode:              "sync",
		OperationType:     "submit_document",
		DocumentID:        "doc-1",
		Payload:           map[string]any{"foo": "bar"},
	})
	if err != nil {
		t.Fatalf("create delivery failed: %v", err)
	}
	if record.ContractKey != "document.submit" || record.EndpointKey != "fake_erp.default" {
		t.Fatalf("expected delivery metadata, got %+v", record)
	}
	record, err = svc.ProcessSubmission(record.ID)
	if err != nil {
		t.Fatalf("process delivery failed: %v", err)
	}
	attempts := svc.ListSubmissionAttempts(record.ID)
	if len(attempts) != 1 || attempts[0].Status != "succeeded" {
		t.Fatalf("expected succeeded attempt, got %+v", attempts)
	}
}

func TestDeadLetterReplay(t *testing.T) {
	obs := observability.NewService()
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.queued.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.succeeded.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "integration.submissions.failed.total", Type: "counter"})
	svc := NewService(obs, logging.NewService())

	record, err := svc.CreateDelivery(SubmissionRecord{
		ExternalSystemKey: "fake_erp",
		EndpointKey:       "fake_erp.default",
		ContractKey:       "document.submit",
		ContractVersion:   1,
		Intent:            "command",
		Mode:              "sync",
		OperationType:     "submit_document",
		DocumentID:        "doc-1",
		Payload:           map[string]any{"force_fail": true},
	})
	if err != nil {
		t.Fatalf("create delivery failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		record, err = svc.ProcessSubmission(record.ID)
		if err != nil {
			t.Fatalf("process submission failed: %v", err)
		}
	}
	if record.Status != "dead_letter" {
		t.Fatalf("expected dead letter status, got %+v", record)
	}
	letters := svc.ListDeadLetters()
	if len(letters) != 1 {
		t.Fatalf("expected one dead letter, got %+v", letters)
	}
	record.Payload["force_fail"] = false
	if err := svc.repo.SaveSubmission(record); err != nil {
		t.Fatalf("save updated payload failed: %v", err)
	}
	replayed, err := svc.ReplayDeadLetter(letters[0].ID)
	if err != nil {
		t.Fatalf("replay dead letter failed: %v", err)
	}
	if replayed.Status != "succeeded" {
		t.Fatalf("expected replay success, got %+v", replayed)
	}
}

func errorAs(err error, target any) bool {
	switch typed := target.(type) {
	case *ValidationError:
		value, ok := err.(ValidationError)
		if ok {
			*typed = value
			return true
		}
	}
	return false
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
