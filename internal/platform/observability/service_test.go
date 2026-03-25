package observability

import (
	"strings"
	"testing"
	"time"
)

func TestServiceRecordsMetricsLogsEventsAndStatuses(t *testing.T) {
	svc := NewService()
	svc.RegisterMetricDefinition(MetricDefinition{Key: "http.requests.total", Type: "counter", Labels: []string{"status"}, ModuleKey: "platform"})
	svc.RegisterMetricDefinition(MetricDefinition{Key: "http.latency", Type: "timing", Labels: []string{"status"}, ModuleKey: "platform"})
	svc.RegisterLogEventDefinition(LogEventDefinition{Key: "http.request", RequiredFields: []string{"path"}, ModuleKey: "platform"})
	svc.RegisterDomainEventDefinition(DomainEventDefinition{Type: "document.created", CorrelationRequired: true, ModuleKey: "documents"})

	if err := svc.RecordMetric("http.requests.total", map[string]string{"status": "200"}, 2); err != nil {
		t.Fatalf("record metric failed: %v", err)
	}
	if err := svc.ObserveMetric("http.latency", map[string]string{"status": "200"}, 150*time.Millisecond); err != nil {
		t.Fatalf("observe metric failed: %v", err)
	}
	if err := svc.EmitLogEvent("http.request", map[string]any{"path": "/healthz"}); err != nil {
		t.Fatalf("emit log failed: %v", err)
	}
	if err := svc.RecordDomainEvent("document.created", true); err != nil {
		t.Fatalf("record domain event failed: %v", err)
	}

	snap := svc.Snapshot()
	if snap.Counters["http.requests.total"] != 2 {
		t.Fatalf("expected counter snapshot, got %+v", snap.Counters)
	}
	if snap.Timings["http.latency"].Count != 1 {
		t.Fatalf("expected timing snapshot, got %+v", snap.Timings)
	}

	statuses := svc.ContractStatuses()
	if len(statuses) < 4 {
		t.Fatalf("expected contract statuses, got %+v", statuses)
	}

	rendered := svc.RenderPrometheus()
	if !strings.Contains(rendered, "http_requests_total 2") {
		t.Fatalf("expected prometheus counter, got %s", rendered)
	}
	if !strings.Contains(rendered, "http_latency_count 1") {
		t.Fatalf("expected prometheus timing, got %s", rendered)
	}
}

func TestServiceValidationFailuresAreTracked(t *testing.T) {
	svc := NewService()
	svc.RegisterMetricDefinition(MetricDefinition{Key: "jobs.total", Type: "counter", Labels: []string{"queue"}, ModuleKey: "jobs"})
	svc.RegisterMetricDefinition(MetricDefinition{Key: "jobs.latency", Type: "counter", Labels: []string{"queue"}, ModuleKey: "jobs"})
	svc.RegisterLogEventDefinition(LogEventDefinition{Key: "jobs.failed", RequiredFields: []string{"job_id"}, ModuleKey: "jobs"})
	svc.RegisterDomainEventDefinition(DomainEventDefinition{Type: "jobs.failed", CorrelationRequired: true, ModuleKey: "jobs"})

	if err := svc.RecordMetric("jobs.total", map[string]string{}, 1); err == nil {
		t.Fatal("expected missing label error")
	}
	if err := svc.ObserveMetric("jobs.latency", map[string]string{"queue": "default"}, time.Second); err == nil {
		t.Fatal("expected wrong metric type error")
	}
	if err := svc.EmitLogEvent("jobs.failed", map[string]any{}); err == nil {
		t.Fatal("expected missing log field error")
	}
	if err := svc.RecordDomainEvent("jobs.failed", false); err == nil {
		t.Fatal("expected missing correlation error")
	}

	statuses := svc.ContractStatuses()
	byKey := map[string]ContractStatus{}
	for _, status := range statuses {
		byKey[status.Key] = status
	}
	if byKey["jobs.total"].ValidationFailures == 0 {
		t.Fatalf("expected metric failure status, got %+v", byKey["jobs.total"])
	}
	if byKey["jobs.failed"].ValidationFailures == 0 {
		t.Fatalf("expected event/log failure status, got %+v", byKey["jobs.failed"])
	}
}

func TestRenderPrometheusPreservesHistogramLabels(t *testing.T) {
	svc := NewService()

	svc.ObserveHistogram("http.request.duration.seconds", 0.25, map[string]string{
		"method":      "GET",
		"route":       "health",
		"status_code": "200",
	})
	svc.ObserveHistogram("http.request.duration.seconds", 0.50, map[string]string{
		"method":      "POST",
		"route":       "documents",
		"status_code": "201",
	})

	rendered := svc.RenderPrometheus()
	if !strings.Contains(rendered, `http_request_duration_seconds_count{method="GET",route="health",status_code="200"} 1`) {
		t.Fatalf("expected GET histogram series, got %s", rendered)
	}
	if !strings.Contains(rendered, `http_request_duration_seconds_count{method="POST",route="documents",status_code="201"} 1`) {
		t.Fatalf("expected POST histogram series, got %s", rendered)
	}
	if !strings.Contains(rendered, `http_request_duration_seconds_bucket{le="+Inf",method="GET",route="health",status_code="200"} 1`) {
		t.Fatalf("expected +Inf bucket for labeled histogram, got %s", rendered)
	}
}
