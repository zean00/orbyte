package integration

import (
	"errors"
	"testing"
	"time"

	"orbyte/internal/platform/logging"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/secretstore"
)

func TestIntegrationHelperClassifiersAndRuntimeDecoders(t *testing.T) {
	if got := classifyIntegrationError(nil); got != "" {
		t.Fatalf("expected empty code for nil error, got %q", got)
	}
	if got := classifyIntegrationError(errors.New("request timeout")); got != "timeout" {
		t.Fatalf("unexpected timeout classification: %q", got)
	}
	if got := classifyIntegrationError(errors.New("upstream status 404")); got != "remote_4xx" {
		t.Fatalf("unexpected 4xx classification: %q", got)
	}
	if got := classifyIntegrationError(errors.New("upstream status 503")); got != "remote_5xx" {
		t.Fatalf("unexpected 5xx classification: %q", got)
	}
	if got := classifyIntegrationError(errors.New("validation failed")); got != "validation_error" {
		t.Fatalf("unexpected validation classification: %q", got)
	}
	if got := classifyIntegrationError(errors.New("config missing")); got != "config_error" {
		t.Fatalf("unexpected config classification: %q", got)
	}

	failure := classifyExecutionFailure(errors.New("upstream status 404"), RetryPolicy{RetryableErrorCodes: []string{"timeout", "remote_5xx"}})
	if failure.Retryable || failure.Category != "validation_error" {
		t.Fatalf("expected non-retryable 4xx to map to validation category, got %+v", failure)
	}

	at := time.Now().UTC()
	record := decodeSubmissionRuntime(encodeSubmissionRuntime(SubmissionRecord{
		Result:           map[string]any{"ok": true},
		LastErrorCode:    "timeout",
		FailureCategory:  "timeout",
		NextRetryAt:      at,
		Terminal:         true,
		ValidationIssues: []ValidationIssue{{Code: "required", Field: "url", Message: "missing", Severity: "error"}},
	}))
	if record.LastErrorCode != "timeout" || !record.NextRetryAt.Equal(at) || !record.Terminal || len(record.ValidationIssues) != 1 {
		t.Fatalf("unexpected decoded submission runtime: %+v", record)
	}

	attempt := decodeSubmissionAttemptRuntime(encodeSubmissionAttemptRuntime(SubmissionAttempt{
		Response:        map[string]any{"status_code": 200},
		FailureCategory: "execution_error",
		Retryable:       true,
	}))
	if attempt.FailureCategory != "execution_error" || !attempt.Retryable {
		t.Fatalf("unexpected decoded attempt runtime: %+v", attempt)
	}

	deadLetter := decodeDeadLetterRuntime(encodeDeadLetterRuntime(DeadLetterRecord{
		Payload:         map[string]any{"payload": "value"},
		LastErrorCode:   "remote_5xx",
		FailureCategory: "remote_5xx",
	}))
	if deadLetter.LastErrorCode != "remote_5xx" || deadLetter.FailureCategory != "remote_5xx" {
		t.Fatalf("unexpected decoded dead letter runtime: %+v", deadLetter)
	}

	if got := stringifyAny(map[string]any{"a": 1}); got == "" {
		t.Fatal("expected stringifyAny to encode non-string values")
	}
	if !boolFromAny("true") || boolFromAny("false") {
		t.Fatal("expected boolFromAny string handling")
	}
	if parseTimeAny(at.Format(time.RFC3339Nano)).IsZero() {
		t.Fatal("expected parseTimeAny to parse RFC3339 strings")
	}
	if got := cloneIssues([]ValidationIssue{{Code: "required"}}); len(got) != 1 || got[0].Code != "required" {
		t.Fatalf("unexpected cloned issues: %+v", got)
	}
}

func TestIntegrationSensitiveSettingsAndHTTPAdapterHelpers(t *testing.T) {
	svc := NewService(observability.NewService(), logging.NewService())
	secrets := secretstore.NewService()
	svc.AttachSecrets(secrets)

	systemRecord, _, err := svc.UpdateSystemSettings("http_bridge", map[string]any{
		"url":          "https://example.test",
		"method":       "POST",
		"bearer_token": "top-secret",
	})
	if err != nil {
		t.Fatalf("update system settings failed: %v", err)
	}
	if !isSensitiveConfigField("integration.system.http_bridge", "bearer_token", svc, AdapterDescriptor{}) {
		t.Fatal("expected bearer_token to resolve as sensitive from system adapter descriptor")
	}

	if err := svc.RegisterEndpoint(Endpoint{
		Key:       "http_bridge.default",
		SystemKey: "http_bridge",
		Status:    "inactive",
	}); err != nil {
		t.Fatalf("register endpoint failed: %v", err)
	}
	if !isSensitiveConfigField("integration.endpoint.http_bridge.default", "bearer_token", svc, AdapterDescriptor{}) {
		t.Fatal("expected bearer_token to resolve as sensitive through endpoint system lookup")
	}
	if isSensitiveConfigField("integration.system.http_bridge", "url", svc, AdapterDescriptor{}) {
		t.Fatal("expected url to be non-sensitive")
	}

	ref := secretRefFromValue(systemRecord.Settings["bearer_token"])
	if ref == "" {
		t.Fatalf("expected stored secret reference, got %+v", systemRecord.Settings["bearer_token"])
	}
	resolved := svc.resolveSettings(systemRecord.Settings)
	if got, _ := resolved["bearer_token"].(string); got != "top-secret" {
		t.Fatalf("expected resolved secret value, got %+v", resolved["bearer_token"])
	}
	redacted := redactSettings(svc.adapters["http"].Descriptor(), systemRecord.Settings)
	if got, _ := redacted["bearer_token"].(map[string]any); got["value"] != "[redacted]" {
		t.Fatalf("expected redacted secret value, got %+v", redacted["bearer_token"])
	}

	adapter := HTTPAdapter{}
	if issues := adapter.ValidateSubmission(ExternalSystem{}, SubmissionRecord{}, nil); issues != nil {
		t.Fatalf("expected validate submission to be nil, got %+v", issues)
	}
	if err := adapter.HealthCheck(ExternalSystem{}, map[string]any{}); err == nil {
		t.Fatal("expected health check to fail without url")
	}
	if err := adapter.HealthCheck(ExternalSystem{}, map[string]any{"url": "https://example.test"}); err != nil {
		t.Fatalf("expected health check to pass with url, got %v", err)
	}
}
