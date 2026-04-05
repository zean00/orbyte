package analytics

import (
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func newAuthoringTestService() *Service {
	docs := document.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	return NewServiceWithRepository(docs, flows, events, searchSvc, auditSvc, obs, NewMemoryRepository())
}

func assertConflictError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected conflict error")
	}
	typed, ok := err.(shared.Error)
	if !ok || typed.Kind != shared.KindConflict {
		t.Fatalf("expected conflict error, got %T %v", err, err)
	}
}

func TestDashboardAuthoringDefaultsAndCRUD(t *testing.T) {
	svc := newAuthoringTestService()

	saved, err := svc.SaveDashboard(Dashboard{
		Name:    "Ops",
		Widgets: []DashboardWidget{{Title: "Open tasks", Kind: "metric"}},
	})
	if err != nil {
		t.Fatalf("save dashboard failed: %v", err)
	}
	if saved.ID == "" || !strings.HasPrefix(saved.ID, "analytics-dashboard:") {
		t.Fatalf("expected generated dashboard id, got %+v", saved)
	}
	if saved.Visibility != "private" || saved.ScopeType != "deployment" || saved.Status != "active" {
		t.Fatalf("expected defaults applied, got %+v", saved)
	}
	if saved.Widgets[0].ID == "" {
		t.Fatalf("expected generated widget id, got %+v", saved.Widgets)
	}
	if _, ok := svc.Dashboard(" " + saved.ID + " "); !ok {
		t.Fatalf("expected dashboard lookup by trimmed id")
	}
	if len(svc.Dashboards()) != 1 {
		t.Fatalf("expected dashboard list to include saved item")
	}
	if err := svc.DeleteDashboard(" " + saved.ID + " "); err != nil {
		t.Fatalf("delete dashboard failed: %v", err)
	}
	if _, ok := svc.Dashboard(saved.ID); ok {
		t.Fatalf("expected deleted dashboard to be removed")
	}

	if _, err := svc.SaveDashboard(Dashboard{}); err == nil {
		t.Fatal("expected validation error for empty dashboard name")
	}

	nilRepo := &Service{}
	_, err = nilRepo.SaveDashboard(Dashboard{Name: "x"})
	assertConflictError(t, err)
}

func TestDashboardsForSurfaceIncludesLegacyDashboards(t *testing.T) {
	svc := newAuthoringTestService()
	now := time.Now().UTC()
	legacy := Dashboard{
		ID:        "dashboard:legacy",
		Name:      "Legacy Dashboard",
		Surface:   "",
		IsDefault: true,
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now,
	}
	if err := svc.repo.SaveDashboard(legacy); err != nil {
		t.Fatalf("save legacy dashboard failed: %v", err)
	}
	if _, err := svc.SaveDashboard(Dashboard{
		ID:        "dashboard:dashboard",
		Name:      "Dedicated Dashboard",
		Surface:   "dashboard",
		IsDefault: true,
		Status:    "active",
		UpdatedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("save surface dashboard failed: %v", err)
	}

	items := svc.DashboardsForSurface("dashboard")
	if len(items) != 2 {
		t.Fatalf("expected legacy and explicit dashboards for dashboard surface, got %+v", items)
	}

	if _, ok := svc.EffectiveDashboard("dashboard", "", "", nil); !ok {
		t.Fatal("expected effective dashboard lookup to consider legacy dashboard surfaces")
	}
}

func TestSavedQueryAndMetricAuthoringDefaultsCRUDAndValidation(t *testing.T) {
	svc := newAuthoringTestService()

	query, err := svc.SaveSavedQuery(SavedQuery{
		Name: "Daily trend",
		Spec: QuerySpec{SourceKind: "trend", Limit: 250},
	})
	if err != nil {
		t.Fatalf("save saved query failed: %v", err)
	}
	if query.ID == "" || !strings.HasPrefix(query.ID, "analytics-query:") {
		t.Fatalf("expected generated query id, got %+v", query)
	}
	if query.ScopeType != "user" || query.Status != "active" || query.Spec.Limit != 200 {
		t.Fatalf("expected query defaults and clamped limit, got %+v", query)
	}
	if len(query.Spec.Measures) != 2 || query.Spec.Measures[0] != "submitted_documents" {
		t.Fatalf("expected trend default measures, got %+v", query.Spec.Measures)
	}
	if _, ok := svc.SavedQuery(" " + query.ID + " "); !ok {
		t.Fatalf("expected saved query lookup by trimmed id")
	}
	if err := svc.DeleteSavedQuery(" " + query.ID + " "); err != nil {
		t.Fatalf("delete saved query failed: %v", err)
	}
	if _, ok := svc.SavedQuery(query.ID); ok {
		t.Fatal("expected saved query to be deleted")
	}
	if _, err := svc.SaveSavedQuery(SavedQuery{Name: "Bad", Spec: QuerySpec{SourceKind: "breakdown", Measures: []string{"pending_approvals"}}}); err == nil {
		t.Fatal("expected invalid breakdown measure error")
	}

	metric, err := svc.SaveSavedMetric(SavedMetric{
		Name: "Submitted Docs",
		Spec: QuerySpec{SourceKind: "snapshot", Measures: []string{"submitted"}},
	})
	if err != nil {
		t.Fatalf("save saved metric failed: %v", err)
	}
	if metric.ID == "" || !strings.HasPrefix(metric.ID, "analytics-metric:") {
		t.Fatalf("expected generated metric id, got %+v", metric)
	}
	if metric.Key != "submitted_docs" || metric.ScopeType != "user" || metric.Status != "active" {
		t.Fatalf("expected metric defaults, got %+v", metric)
	}
	if _, ok := svc.SavedMetric(" " + metric.ID + " "); !ok {
		t.Fatalf("expected saved metric lookup by trimmed id")
	}
	if err := svc.DeleteSavedMetric(" " + metric.ID + " "); err != nil {
		t.Fatalf("delete saved metric failed: %v", err)
	}
	if _, ok := svc.SavedMetric(metric.ID); ok {
		t.Fatal("expected saved metric to be deleted")
	}
	if _, err := svc.SaveSavedMetric(SavedMetric{}); err == nil {
		t.Fatal("expected validation error for empty metric name")
	}

	nilRepo := &Service{}
	_, err = nilRepo.SaveSavedQuery(SavedQuery{Name: "x", Spec: QuerySpec{SourceKind: "snapshot"}})
	assertConflictError(t, err)
	_, err = nilRepo.SaveSavedMetric(SavedMetric{Name: "x", Spec: QuerySpec{SourceKind: "snapshot"}})
	assertConflictError(t, err)
}

func TestReportDefinitionSaveOrUpdateBranches(t *testing.T) {
	svc := newAuthoringTestService()

	created, err := svc.SaveOrUpdateReportDefinition(ReportDefinition{Name: "Ops report"})
	if err != nil {
		t.Fatalf("create via save-or-update failed: %v", err)
	}
	if created.ID == "" || created.Dimension != "document_type" || created.Format != "csv" || created.Window != "current_state" || created.Schedule != "daily" || !created.Enabled {
		t.Fatalf("expected created report defaults, got %+v", created)
	}

	updated, err := svc.SaveOrUpdateReportDefinition(ReportDefinition{ID: created.ID})
	if err != nil {
		t.Fatalf("update existing report failed: %v", err)
	}
	if updated.Name != " report" {
		t.Fatalf("expected update-time default report name, got %+v", updated)
	}
	if updated.NextRunAt.IsZero() {
		t.Fatalf("expected update-time next run")
	}
	if fetched, ok := svc.ReportDefinition(created.ID); !ok || fetched.ID != created.ID {
		t.Fatalf("expected report definition lookup to return updated report")
	}
	if err := svc.DeleteReportDefinition(" " + created.ID + " "); err != nil {
		t.Fatalf("delete report definition failed: %v", err)
	}
	if _, ok := svc.ReportDefinition(created.ID); ok {
		t.Fatal("expected report definition to be deleted")
	}

	nilRepo := &Service{}
	_, err = nilRepo.SaveOrUpdateReportDefinition(ReportDefinition{ID: "report:1", Name: "x"})
	assertConflictError(t, err)
}

func TestNormalizeQuerySpecAndHelpers(t *testing.T) {
	spec, err := normalizeQuerySpec(QuerySpec{})
	if err != nil {
		t.Fatalf("normalize default spec failed: %v", err)
	}
	if spec.SourceKind != "snapshot" || spec.Window != "current_state" || spec.Limit != 20 {
		t.Fatalf("expected default query spec, got %+v", spec)
	}
	if len(spec.Measures) != 3 || spec.Measures[0] != "created" {
		t.Fatalf("expected snapshot default measures, got %+v", spec.Measures)
	}

	breakdown, err := normalizeQuerySpec(QuerySpec{SourceKind: "breakdown"})
	if err != nil {
		t.Fatalf("normalize breakdown spec failed: %v", err)
	}
	if breakdown.GroupBy != "document_type" || breakdown.Measures[0] != "submitted" {
		t.Fatalf("expected breakdown defaults, got %+v", breakdown)
	}

	if _, err := normalizeQuerySpec(QuerySpec{SourceKind: "nope"}); err == nil {
		t.Fatal("expected invalid source kind error")
	}
	if err := validateMeasures("trend", []string{"created"}); err == nil {
		t.Fatal("expected invalid trend measure error")
	}
	if firstMeasure([]string{" ", "approved"}) != "approved" {
		t.Fatal("expected first non-empty measure")
	}
	row := snapshotMetricRow(Snapshot{GeneratedAt: time.Unix(10, 0).UTC(), Documents: DocumentKPI{Created: 2, Submitted: 1}}, []string{"submitted"})
	if len(row) != 2 || row["submitted"] != 1 {
		t.Fatalf("expected filtered snapshot metric row, got %+v", row)
	}
	if got := sortedKeys(map[string]any{"b": 2, "a": 1}); got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected sorted keys, got %+v", got)
	}
	if toFloat(int64(4)) != 4 || toFloat("x") != 0 {
		t.Fatalf("unexpected toFloat behavior")
	}
	if stringValue(42) != "42" {
		t.Fatalf("unexpected stringValue behavior")
	}
}

func TestExecuteQuerySpecAcrossSourceKinds(t *testing.T) {
	svc := newAuthoringTestService()
	docs := svc.documents

	first, _ := docs.Create("generic_request", "org_default", "loc_a", "user_admin", map[string]any{"title": "a"})
	first.Header.Status = "submitted"
	_ = docs.Save(first)
	second, _ := docs.Create("generic_request", "org_default", "loc_b", "user_admin", map[string]any{"title": "b"})
	second.Header.Status = "approved"
	_ = docs.Save(second)

	if _, err := svc.CaptureSnapshot(); err != nil {
		t.Fatalf("capture snapshot failed: %v", err)
	}
	third, _ := docs.Create("generic_request", "org_default", "loc_a", "user_admin", map[string]any{"title": "c"})
	third.Header.Status = "submitted"
	_ = docs.Save(third)
	if _, err := svc.CaptureSnapshot(); err != nil {
		t.Fatalf("capture second snapshot failed: %v", err)
	}

	snapshotExec, err := svc.ExecuteQuerySpec(QuerySpec{SourceKind: "snapshot", Measures: []string{"submitted"}})
	if err != nil {
		t.Fatalf("execute snapshot query failed: %v", err)
	}
	if snapshotExec.Chart.Type != "metric" || len(snapshotExec.Rows) != 1 || snapshotExec.Rows[0]["submitted"] == nil {
		t.Fatalf("expected metric snapshot execution, got %+v", snapshotExec)
	}

	trendExec, err := svc.ExecuteQuerySpec(QuerySpec{SourceKind: "trend", Limit: 1})
	if err != nil {
		t.Fatalf("execute trend query failed: %v", err)
	}
	if trendExec.Chart.Type != "line" || len(trendExec.Rows) != 1 || len(trendExec.Chart.Series) != 2 {
		t.Fatalf("expected line trend execution, got %+v", trendExec)
	}

	breakdownExec, err := svc.ExecuteQuerySpec(QuerySpec{SourceKind: "breakdown", GroupBy: "location"})
	if err != nil {
		t.Fatalf("execute breakdown query failed: %v", err)
	}
	if breakdownExec.Chart.Type != "bar" || len(breakdownExec.Rows) == 0 {
		t.Fatalf("expected bar breakdown execution, got %+v", breakdownExec)
	}

	reportingExec, err := svc.ExecuteQuerySpec(QuerySpec{SourceKind: "reporting_breakdown", GroupBy: "location", LocationID: "loc_a"})
	if err != nil {
		t.Fatalf("execute reporting breakdown failed: %v", err)
	}
	if reportingExec.Chart.Type != "bar" || len(reportingExec.Rows) == 0 {
		t.Fatalf("expected reporting breakdown execution, got %+v", reportingExec)
	}

	if _, err := svc.ExecuteQuerySpec(QuerySpec{SourceKind: "breakdown", GroupBy: "invalid"}); err == nil {
		t.Fatal("expected invalid breakdown group to fail")
	}
	if _, err := svc.ExecuteQuerySpec(QuerySpec{SourceKind: "invalid"}); err == nil {
		t.Fatal("expected invalid source kind to fail")
	}
}
