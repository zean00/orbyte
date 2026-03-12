package analytics

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clinic/internal/platform/application"
	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/observability"
	"clinic/internal/platform/search"
	"clinic/internal/platform/workflow"
)

func TestSnapshot(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	events.RegisterHandler("document.submitted", eventing.NewDocumentProjectionHandler(docs, searchSvc))
	actions := application.NewDocumentActions(docs, flows, nil, application.NewMemorySubmitStore(docs, flows, auditSvc, events))
	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	_, _ = actions.Submit(record.Header.ID, "user_admin", 1, record.Header.ETag)
	_, _ = events.DispatchPending(10)

	svc := NewService(docs, flows, events, searchSvc, auditSvc, obs)
	snap := svc.Snapshot()
	if snap.Documents.Created == 0 {
		t.Fatal("expected documents KPI")
	}
	if snap.Workflow.PendingApprovals == 0 {
		t.Fatal("expected workflow KPI")
	}
	if snap.Coverage.DocumentSummaries == 0 {
		t.Fatal("expected projection KPI")
	}
	if len(snap.Segments.ByDocumentType) == 0 {
		t.Fatal("expected document type segments")
	}
}

func TestCaptureSnapshotPersists(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(docs, flows, events, searchSvc, auditSvc, obs, repo)

	if _, err := svc.CaptureSnapshot(); err != nil {
		t.Fatalf("capture snapshot failed: %v", err)
	}
	if len(svc.ListSnapshots()) != 1 {
		t.Fatal("expected persisted snapshot")
	}
	if len(svc.Trends(10)) != 1 {
		t.Fatal("expected trend point")
	}
}

func TestQuerySnapshotsAndBreakdown(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(docs, flows, events, searchSvc, auditSvc, obs, repo)

	doc1, _ := docs.Create("generic_request", "org_default", "loc_a", "user_admin", map[string]any{"title": "a"})
	_ = docs.Save(doc1)
	if _, err := svc.CaptureSnapshot(); err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	items := svc.QuerySnapshots(Query{Window: "current_state", Limit: 1})
	if len(items) != 1 {
		t.Fatal("expected filtered snapshots")
	}
	breakdown, ok := svc.Breakdown(Query{Window: "current_state"}, "location")
	if !ok || len(breakdown) == 0 {
		t.Fatal("expected location breakdown")
	}
	if len(svc.ListRollups("daily", 10)) == 0 {
		_ = svc.RefreshRollups()
		if len(svc.ListRollups("daily", 10)) == 0 {
			t.Fatal("expected daily rollups")
		}
	}
	if breakdown, ok := svc.RollupBreakdown("daily", "document_type", 10); !ok || len(breakdown) == 0 {
		t.Fatal("expected rollup breakdown")
	}
	facts := svc.QueryFacts(FactQuery{})
	if facts.Documents == nil {
		t.Fatal("expected fact bundle")
	}
}

func TestCompareSnapshots(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(docs, flows, events, searchSvc, auditSvc, obs, repo)

	first, err := svc.CaptureSnapshot()
	if err != nil {
		t.Fatalf("capture first snapshot failed: %v", err)
	}
	record, _ := docs.Create("generic_request", "org_default", "loc_a", "user_admin", map[string]any{"title": "a"})
	record.Header.Status = "submitted"
	_ = docs.Save(record)
	second, err := svc.CaptureSnapshot()
	if err != nil {
		t.Fatalf("capture second snapshot failed: %v", err)
	}
	comparison, ok := svc.Compare(Query{From: first.GeneratedAt.Add(-time.Second), To: second.GeneratedAt.Add(time.Second)})
	if !ok {
		t.Fatal("expected comparison")
	}
	_ = comparison
}

func TestExportDocumentReportingCSV(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(docs, flows, events, searchSvc, auditSvc, obs, repo)

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "a"})
	_ = docs.Save(record)
	_, _ = svc.CaptureSnapshot()

	content, err := svc.ExportDocumentReportingCSV(FactQuery{}, "document_type")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "dimension_type,dimension_key,label") {
		t.Fatal("expected csv header")
	}
	if !strings.Contains(text, "document_type,generic_request") {
		t.Fatal("expected csv data row")
	}
}

func TestExportDocumentReportingXLSXAndReports(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(docs, flows, events, searchSvc, auditSvc, obs, repo)

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "a"})
	_ = docs.Save(record)
	_, _ = svc.CaptureSnapshot()

	xlsx, err := svc.ExportDocumentReportingXLSX(FactQuery{}, "document_type")
	if err != nil {
		t.Fatalf("xlsx export failed: %v", err)
	}
	if len(xlsx) == 0 {
		t.Fatal("expected xlsx bytes")
	}
	pdf, err := svc.ExportDocumentReportingPDF(FactQuery{}, "document_type")
	if err != nil {
		t.Fatalf("pdf export failed: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("expected pdf bytes")
	}
	def, err := svc.CreateReportDefinition(ReportDefinition{Name: "Daily documents", Dimension: "document_type", Format: "xlsx", Schedule: "daily"})
	if err != nil {
		t.Fatalf("create report definition failed: %v", err)
	}
	if len(svc.ListReportDefinitions()) != 1 {
		t.Fatal("expected report definition")
	}
	run, _, err := svc.RunReport(def)
	if err != nil {
		t.Fatalf("run report failed: %v", err)
	}
	if run.RowCount == 0 {
		t.Fatal("expected report rows")
	}
	if len(svc.ListReportRuns()) != 1 {
		t.Fatal("expected report run")
	}
	if len(svc.ListReportArtifacts()) != 1 {
		t.Fatal("expected report artifact")
	}
	artifact, ok := svc.GetReportArtifact(run.ArtifactID)
	if !ok || len(artifact.Content) == 0 {
		t.Fatal("expected downloadable report artifact")
	}
	delivery, err := svc.DeliverArtifact(run.ArtifactID, "download", "")
	if err != nil {
		t.Fatalf("deliver artifact failed: %v", err)
	}
	if delivery.Status != "delivered" {
		t.Fatalf("expected delivered status, got %s", delivery.Status)
	}
	if len(svc.ListReportDeliveries()) != 1 {
		t.Fatal("expected report delivery")
	}
	tempFile := filepath.Join(t.TempDir(), "report.csv")
	filesystemDelivery, err := svc.DeliverArtifact(run.ArtifactID, "filesystem", tempFile)
	if err != nil {
		t.Fatalf("filesystem delivery failed: %v", err)
	}
	if filesystemDelivery.Status != "delivered" {
		t.Fatalf("expected filesystem delivered status")
	}
	if _, err := os.Stat(tempFile); err != nil {
		t.Fatalf("expected written file: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	webhookDelivery, err := svc.DeliverArtifact(run.ArtifactID, "webhook", server.URL)
	if err != nil {
		t.Fatalf("webhook delivery failed: %v", err)
	}
	if webhookDelivery.Status != "delivered" {
		t.Fatalf("expected webhook delivered status")
	}
	emailDelivery, err := svc.DeliverArtifact(run.ArtifactID, "email", "user@example.com")
	if err != nil {
		t.Fatalf("email delivery failed: %v", err)
	}
	if emailDelivery.Status != "delivered" {
		t.Fatalf("expected email delivered status")
	}
	objectTarget := filepath.Join("bucket-a", "reports", "report.csv")
	objectDelivery, err := svc.DeliverArtifact(run.ArtifactID, "object_store", objectTarget)
	if err != nil {
		t.Fatalf("object_store delivery failed: %v", err)
	}
	if objectDelivery.Status != "delivered" {
		t.Fatalf("expected object_store delivered status")
	}
	_, err = svc.DeliverArtifact(run.ArtifactID, "filesystem", "")
	if err == nil {
		t.Fatal("expected filesystem delivery failure")
	}
	_, _ = svc.DeliverArtifact(run.ArtifactID, "filesystem", "")
	_, err = svc.DeliverArtifact(run.ArtifactID, "filesystem", "")
	if err == nil {
		t.Fatal("expected dead-lettering failure on repeated attempts")
	}
	if len(svc.ListReportDeliveryDeadLetters()) == 0 {
		t.Fatal("expected report delivery dead letter")
	}
	def.NextRunAt = time.Now().UTC().Add(-time.Minute)
	_ = repo.UpdateReportDefinition(def)
	if err := svc.RunDueReports(time.Now().UTC()); err != nil {
		t.Fatalf("run due reports failed: %v", err)
	}
	if len(svc.ListReportRuns()) < 2 {
		t.Fatal("expected scheduled report run")
	}
	cutoff := time.Now().UTC().Add(time.Hour)
	if err := svc.CleanupReportData(cutoff); err != nil {
		t.Fatalf("cleanup report data failed: %v", err)
	}
	if len(svc.ListReportArtifacts()) != 0 {
		t.Fatal("expected artifacts cleaned up")
	}
}
