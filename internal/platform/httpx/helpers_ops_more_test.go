package httpx

import (
	"testing"
	"time"

	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

func TestOpsRuntimeAndOfflineHelpers(t *testing.T) {
	now := time.Now().UTC()

	offlineSummary := summarizeOfflineOutcomes([]offline.SyncResultItem{
		{Status: offline.StatusAccepted, ErrorCode: "", ProcessedAt: now.Add(-10 * time.Second)},
		{Status: offline.StatusFailedRetryable, ErrorCode: "sync_retryable", ProcessedAt: now.Add(-30 * time.Second)},
		{Status: offline.StatusFailedRetryable, ErrorCode: "sync_retryable", ProcessedAt: now.Add(-5 * time.Second)},
	})
	if offlineSummary["count"] != 3 {
		t.Fatalf("unexpected offline summary count: %+v", offlineSummary)
	}
	byStatus, _ := offlineSummary["by_status"].(map[string]int)
	if byStatus[offline.StatusFailedRetryable] != 2 {
		t.Fatalf("unexpected offline by_status summary: %+v", offlineSummary)
	}
	if offlineSummary["oldest_retryable_age_seconds"].(int64) <= 0 {
		t.Fatalf("expected retryable age summary, got %+v", offlineSummary)
	}

	integrationSummary := summarizeIntegrations([]integration.SubmissionRecord{
		{ID: "sub-1", ExternalSystemKey: "erp", Status: "failed", LastError: "boom"},
		{ID: "sub-2", ExternalSystemKey: "erp", Status: "dead_letter", LastError: "dead"},
		{ID: "sub-3", ExternalSystemKey: "crm", Status: "accepted"},
	})
	bySystem, _ := integrationSummary["by_system"].(map[string]int)
	if bySystem["erp"] != 2 || bySystem["crm"] != 1 {
		t.Fatalf("unexpected integration summary: %+v", integrationSummary)
	}

	if got := offlineFailureCategory(offline.SyncResultItem{Status: offline.StatusConflict}); got != "version_conflict" {
		t.Fatalf("unexpected offline conflict category: %q", got)
	}
	if got := offlineFailureCategory(offline.SyncResultItem{Status: offline.StatusFailedTerminal, ErrorCode: "bad_request"}); got != "bad_request" {
		t.Fatalf("unexpected offline terminal category: %q", got)
	}
	if got := offlineRunbookID(offline.SyncResultItem{Status: offline.StatusForbidden}); got != "runtime.dependencies" {
		t.Fatalf("unexpected offline runbook: %q", got)
	}
	if got := integrationFailureCategory(integration.SubmissionRecord{Status: "dead_letter", LastError: "x"}); got != "dead_lettered" {
		t.Fatalf("unexpected integration failure category: %q", got)
	}
	if got := jobFailureCategory(jobs.Job{Status: jobs.StatusFailed}); got != "handler_failure" {
		t.Fatalf("unexpected job failure category: %q", got)
	}
	if got := jobFailureCategory(jobs.Job{Status: jobs.StatusDeadLetter}); got != "dead_lettered" {
		t.Fatalf("unexpected dead-letter job category: %q", got)
	}
	if got := jobRunbookID(jobs.Job{Status: jobs.StatusFailed}); got != "runtime.jobs" {
		t.Fatalf("unexpected job runbook: %q", got)
	}
	if got := workflowFailureCategory(workflow.HistoryEvent{DecisionReason: "Denied by policy runtime"}); got != "workflow_runtime_invalid" {
		t.Fatalf("unexpected workflow failure category: %q", got)
	}
	if got := workflowRunbookID(workflow.HistoryEvent{DecisionReason: "Denied by policy runtime"}); got != "runtime.workflow" {
		t.Fatalf("unexpected workflow runbook: %q", got)
	}
	if got := deliveryFailureCategory(eventing.OutboxDeliveryRecord{Status: "dead_letter"}); got != "dead_lettered" {
		t.Fatalf("unexpected delivery failure category: %q", got)
	}
	if got := outboxFailureCategory(eventing.OutboxRecord{Status: "failed", LastError: "boom"}); got != "dispatch_failure" {
		t.Fatalf("unexpected outbox failure category: %q", got)
	}
	if !traceTargetIncluded(map[string]struct{}{"document:doc-1": {}}, "document", "doc-1") {
		t.Fatal("expected trace target inclusion")
	}
	if traceTargetIncluded(map[string]struct{}{}, "document", "doc-1") {
		t.Fatal("expected missing trace target to fail")
	}
	if got := metadataCorrelation(map[string]any{"correlation_id": int64(42)}); got != "42" {
		t.Fatalf("unexpected metadata correlation: %q", got)
	}
	if got := jobCorrelationID(jobs.Job{Payload: map[string]any{"correlation_id": "c-1"}}); got != "c-1" {
		t.Fatalf("unexpected payload job correlation id: %q", got)
	}
	if !jobMatchesCorrelation(jobs.Job{Result: map[string]any{"correlation_id": "c-2"}}, "c-2", nil) {
		t.Fatal("expected result correlation match")
	}
	if !jobMatchesCorrelation(jobs.Job{Payload: map[string]any{"submission_id": "sub-1"}}, "ignored", map[string]struct{}{"sub-1": {}}) {
		t.Fatal("expected submission id match")
	}
	if got := stringifyAny(float64(12.5)); got != "12.5" {
		t.Fatalf("unexpected stringifyAny float result: %q", got)
	}
	if got := firstTime(time.Time{}, now); !got.Equal(now) {
		t.Fatalf("unexpected firstTime result: %v", got)
	}

	retryable := offlineFailureFromError(assertErr{}, offline.SyncResultItem{})
	if retryable.Status != offline.StatusFailedRetryable || retryable.ErrorCode != "sync_retryable" || retryable.AttemptCount != 1 {
		t.Fatalf("unexpected retryable offline failure: %+v", retryable)
	}
	conflict := offlineConflictFromError(shared.Conflict("version mismatch"), offline.SyncResultItem{}, map[string]any{"version": 1}, map[string]any{"version": 2})
	if conflict.Status != offline.StatusConflict || conflict.ErrorCode != "version_conflict" || len(conflict.Conflict.ResolutionOptions) == 0 {
		t.Fatalf("unexpected offline conflict result: %+v", conflict)
	}
	terminal := offlineFailureFromPlatformError(shared.Error{Kind: shared.KindValidation, Message: "bad payload"}, offline.SyncResultItem{})
	if terminal.Status != offline.StatusFailedTerminal || terminal.ErrorCode != "validation_error" {
		t.Fatalf("unexpected offline terminal result: %+v", terminal)
	}
	forbidden := offlineFailureFromPlatformError(shared.Error{Kind: shared.KindForbidden, Message: "nope"}, offline.SyncResultItem{})
	if forbidden.Status != offline.StatusForbidden || forbidden.ErrorCode != "forbidden" {
		t.Fatalf("unexpected offline forbidden result: %+v", forbidden)
	}
	bootstrapToken := offlineBootstrapToken(offline.Bootstrap{SchemaVersion: "v1", References: []module.OfflineReferenceDefinition{{TypeKey: "roles", Title: "Roles"}}})
	if len(bootstrapToken) != 16 {
		t.Fatalf("expected stable bootstrap token length, got %q", bootstrapToken)
	}
	query := decodeOfflineProjectionQuery(map[string]any{"query": "hello", "limit": 3})
	if query.Query != "hello" || query.Limit != 3 {
		t.Fatalf("unexpected offline projection query decode: %+v", query)
	}
}

func TestDocumentFlowAndTemplateHelpers(t *testing.T) {
	flow := module.DocumentFlowDefinition{
		Key: "onboarding",
		Steps: []module.DocumentFlowStepDefinition{
			{Key: "start", Documents: []module.DocumentFlowDocumentDefinition{{Key: "request"}, {Key: "party", PrimaryOutput: true}}},
		},
	}
	if got := primaryDocumentKey(flow); got != "party" {
		t.Fatalf("unexpected primary document key: %q", got)
	}

	step := module.DocumentFlowStepDefinition{
		NextStepKey: "fallback",
		NextRules: []module.DocumentFlowBranchRule{
			{Path: "flags.approved", Truthy: true, NextStepKey: "approved"},
			{Path: "status", Equals: "rejected", NextStepKey: "rejected"},
			{Path: "status", In: []string{"pending", "queued"}, NextStepKey: "queue"},
		},
	}
	if got := resolveNextStepKey(step, map[string]any{"flags": map[string]any{"approved": true}}); got != "approved" {
		t.Fatalf("unexpected truthy next step: %q", got)
	}
	if got := resolveNextStepKey(step, map[string]any{"status": "rejected"}); got != "rejected" {
		t.Fatalf("unexpected equals next step: %q", got)
	}
	if got := resolveNextStepKey(step, map[string]any{"status": "queued"}); got != "queue" {
		t.Fatalf("unexpected in-list next step: %q", got)
	}
	if got := resolveNextStepKey(step, map[string]any{"status": "draft"}); got != "fallback" {
		t.Fatalf("unexpected fallback next step: %q", got)
	}
	if got := resolveFlowContextPath(map[string]any{"a": map[string]any{"b": "value"}}, "a.b"); got != "value" {
		t.Fatalf("unexpected flow context path result: %v", got)
	}
	if got := stringifyValue(123); got != "123" {
		t.Fatalf("unexpected stringifyValue result: %q", got)
	}
	if !isTruthy("yes") || isTruthy("false") || isTruthy(nil) {
		t.Fatalf("unexpected truthy evaluation")
	}

	filtered := filterValidationIssues([]templateoutput.ValidationIssue{
		{Severity: "error", Message: "bad"},
		{Severity: "warning", Message: "warn"},
	}, "error")
	if len(filtered) != 1 || filtered[0].Message != "bad" {
		t.Fatalf("unexpected filtered validation issues: %+v", filtered)
	}
	if key, version, action, ok := templateVersionPath("/admin/api/templates/invoice/actions/publish"); !ok || key != "invoice" || version != "" || action != "publish" {
		t.Fatalf("unexpected template action path parse: key=%q version=%q action=%q ok=%v", key, version, action, ok)
	}
	if key, version, action, ok := templateVersionPath("/admin/api/templates/invoice/versions/v2/render"); !ok || key != "invoice" || version != "v2" || action != "render" {
		t.Fatalf("unexpected template version path parse: key=%q version=%q action=%q ok=%v", key, version, action, ok)
	}
	if _, _, _, ok := templateVersionPath("/admin/api/templates/invoice/render"); ok {
		t.Fatal("expected malformed template path to fail")
	}
}
