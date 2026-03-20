package analytics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type RuntimeScope struct {
	ScopeType      string `json:"scope_type,omitempty"`
	ScopeID        string `json:"scope_id,omitempty"`
	OwnerUserID    string `json:"owner_user_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	LocationID     string `json:"location_id,omitempty"`
}

type QuerySpec struct {
	SourceKind   string            `json:"source_kind"`
	Window       string            `json:"window,omitempty"`
	From         time.Time         `json:"from,omitempty"`
	To           time.Time         `json:"to,omitempty"`
	LocationID   string            `json:"location_id,omitempty"`
	DocumentType string            `json:"document_type,omitempty"`
	GroupBy      string            `json:"group_by,omitempty"`
	Granularity  string            `json:"granularity,omitempty"`
	Measures     []string          `json:"measures,omitempty"`
	Filters      map[string]string `json:"filters,omitempty"`
	SortBy       string            `json:"sort_by,omitempty"`
	Desc         bool              `json:"desc,omitempty"`
	Limit        int               `json:"limit,omitempty"`
}

type ChartSeries struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

type ChartSpec struct {
	Type      string        `json:"type"`
	Title     string        `json:"title,omitempty"`
	X         []string      `json:"x,omitempty"`
	Series    []ChartSeries `json:"series,omitempty"`
	YFormat   string        `json:"y_format,omitempty"`
	Legend    bool          `json:"legend,omitempty"`
	Stacked   bool          `json:"stacked,omitempty"`
	TableCols []string      `json:"table_columns,omitempty"`
}

type QueryExecution struct {
	Spec      QuerySpec        `json:"spec"`
	Columns   []string         `json:"columns,omitempty"`
	Rows      []map[string]any `json:"rows,omitempty"`
	Summary   map[string]any   `json:"summary,omitempty"`
	Chart     ChartSpec        `json:"chart,omitempty"`
	Generated time.Time        `json:"generated_at"`
}

type SavedQuery struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description,omitempty"`
	Spec              QuerySpec `json:"spec"`
	VisualizationHint string    `json:"visualization_hint,omitempty"`
	RuntimeScope
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SavedMetric struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Spec        QuerySpec `json:"spec"`
	ChartType   string    `json:"chart_type,omitempty"`
	Format      string    `json:"format,omitempty"`
	RuntimeScope
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type DashboardWidget struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Kind          string    `json:"kind"`
	QueryID       string    `json:"query_id,omitempty"`
	MetricID      string    `json:"metric_id,omitempty"`
	InlineQuery   QuerySpec `json:"inline_query,omitempty"`
	ChartOverride string    `json:"chart_override,omitempty"`
}

type Dashboard struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Visibility  string            `json:"visibility,omitempty"`
	Layout      map[string]any    `json:"layout,omitempty"`
	Widgets     []DashboardWidget `json:"widgets,omitempty"`
	RuntimeScope
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) SaveDashboard(item Dashboard) (Dashboard, error) {
	if strings.TrimSpace(item.Name) == "" {
		return Dashboard{}, shared.Validation("dashboard name is required")
	}
	now := time.Now().UTC()
	if strings.TrimSpace(item.ID) == "" {
		item.ID = fmt.Sprintf("analytics-dashboard:%d", now.UnixNano())
	}
	if strings.TrimSpace(item.Visibility) == "" {
		item.Visibility = "private"
	}
	if strings.TrimSpace(item.ScopeType) == "" {
		item.ScopeType = "user"
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "active"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	for i := range item.Widgets {
		if strings.TrimSpace(item.Widgets[i].ID) == "" {
			item.Widgets[i].ID = fmt.Sprintf("%s-widget-%d", item.ID, i+1)
		}
	}
	if s.repo == nil {
		return Dashboard{}, shared.Conflict("analytics repository is not configured")
	}
	return item, s.repo.SaveDashboard(item)
}

func (s *Service) Dashboards() []Dashboard {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ListDashboards()
}

func (s *Service) Dashboard(id string) (Dashboard, bool) {
	if s == nil || s.repo == nil {
		return Dashboard{}, false
	}
	return s.repo.GetDashboard(strings.TrimSpace(id))
}

func (s *Service) DeleteDashboard(id string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.DeleteDashboard(strings.TrimSpace(id))
}

func (s *Service) SaveSavedQuery(item SavedQuery) (SavedQuery, error) {
	if strings.TrimSpace(item.Name) == "" {
		return SavedQuery{}, shared.Validation("query name is required")
	}
	spec, err := normalizeQuerySpec(item.Spec)
	if err != nil {
		return SavedQuery{}, err
	}
	now := time.Now().UTC()
	if strings.TrimSpace(item.ID) == "" {
		item.ID = fmt.Sprintf("analytics-query:%d", now.UnixNano())
	}
	if strings.TrimSpace(item.ScopeType) == "" {
		item.ScopeType = "user"
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "active"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.Spec = spec
	item.UpdatedAt = now
	if s.repo == nil {
		return SavedQuery{}, shared.Conflict("analytics repository is not configured")
	}
	return item, s.repo.SaveSavedQuery(item)
}

func (s *Service) SavedQueries() []SavedQuery {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ListSavedQueries()
}

func (s *Service) SavedQuery(id string) (SavedQuery, bool) {
	if s == nil || s.repo == nil {
		return SavedQuery{}, false
	}
	return s.repo.GetSavedQuery(strings.TrimSpace(id))
}

func (s *Service) DeleteSavedQuery(id string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.DeleteSavedQuery(strings.TrimSpace(id))
}

func (s *Service) SaveSavedMetric(item SavedMetric) (SavedMetric, error) {
	if strings.TrimSpace(item.Name) == "" {
		return SavedMetric{}, shared.Validation("metric name is required")
	}
	spec, err := normalizeQuerySpec(item.Spec)
	if err != nil {
		return SavedMetric{}, err
	}
	now := time.Now().UTC()
	if strings.TrimSpace(item.ID) == "" {
		item.ID = fmt.Sprintf("analytics-metric:%d", now.UnixNano())
	}
	if strings.TrimSpace(item.Key) == "" {
		item.Key = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(item.Name), " ", "_"))
	}
	if strings.TrimSpace(item.ScopeType) == "" {
		item.ScopeType = "user"
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "active"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.Spec = spec
	item.UpdatedAt = now
	if s.repo == nil {
		return SavedMetric{}, shared.Conflict("analytics repository is not configured")
	}
	return item, s.repo.SaveSavedMetric(item)
}

func (s *Service) SavedMetrics() []SavedMetric {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ListSavedMetrics()
}

func (s *Service) SavedMetric(id string) (SavedMetric, bool) {
	if s == nil || s.repo == nil {
		return SavedMetric{}, false
	}
	return s.repo.GetSavedMetric(strings.TrimSpace(id))
}

func (s *Service) DeleteSavedMetric(id string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.DeleteSavedMetric(strings.TrimSpace(id))
}

func (s *Service) SaveOrUpdateReportDefinition(def ReportDefinition) (ReportDefinition, error) {
	if strings.TrimSpace(def.ID) == "" {
		return s.CreateReportDefinition(def)
	}
	found := false
	for _, item := range s.ListReportDefinitions() {
		if item.ID == def.ID {
			found = true
			break
		}
	}
	if !found {
		return s.CreateReportDefinition(def)
	}
	if strings.TrimSpace(def.Name) == "" {
		def.Name = def.Dimension + " report"
	}
	if strings.TrimSpace(def.Dimension) == "" {
		def.Dimension = "document_type"
	}
	if strings.TrimSpace(def.Format) == "" {
		def.Format = "csv"
	}
	if strings.TrimSpace(def.Window) == "" {
		def.Window = "current_state"
	}
	if strings.TrimSpace(def.Schedule) == "" {
		def.Schedule = "daily"
	}
	if def.NextRunAt.IsZero() {
		def.NextRunAt = nextRun(def.Schedule, time.Now().UTC())
	}
	if s.repo == nil {
		return ReportDefinition{}, shared.Conflict("analytics repository is not configured")
	}
	return def, s.repo.UpdateReportDefinition(def)
}

func (s *Service) ReportDefinition(id string) (ReportDefinition, bool) {
	for _, item := range s.ListReportDefinitions() {
		if item.ID == strings.TrimSpace(id) {
			return item, true
		}
	}
	return ReportDefinition{}, false
}

func (s *Service) DeleteReportDefinition(id string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.DeleteReportDefinition(strings.TrimSpace(id))
}

func (s *Service) ExecuteQuerySpec(spec QuerySpec) (QueryExecution, error) {
	normalized, err := normalizeQuerySpec(spec)
	if err != nil {
		return QueryExecution{}, err
	}
	result := QueryExecution{Spec: normalized, Generated: time.Now().UTC(), Summary: map[string]any{}}
	switch normalized.SourceKind {
	case "snapshot":
		snapshot, ok := s.LatestSnapshot(Query{Window: firstNonEmpty(normalized.Window, "current_state"), From: normalized.From, To: normalized.To})
		if !ok {
			snapshot, err = s.CaptureSnapshot()
			if err != nil {
				return QueryExecution{}, err
			}
		}
		row := snapshotMetricRow(snapshot, normalized.Measures)
		result.Columns = sortedKeys(row)
		result.Rows = []map[string]any{row}
		result.Summary = map[string]any{"row_count": 1}
	case "trend":
		points := s.TrendsByQuery(Query{Window: firstNonEmpty(normalized.Window, "current_state"), From: normalized.From, To: normalized.To, Limit: normalized.Limit})
		result.Columns = []string{"generated_at", "submitted_documents", "approved_documents", "pending_approvals", "dead_letters"}
		result.Rows = make([]map[string]any, 0, len(points))
		for _, point := range points {
			result.Rows = append(result.Rows, map[string]any{
				"generated_at":        point.GeneratedAt.Format(time.RFC3339),
				"submitted_documents": point.SubmittedDocuments,
				"approved_documents":  point.ApprovedDocuments,
				"pending_approvals":   point.PendingApprovals,
				"dead_letters":        point.DeadLetters,
			})
		}
		result.Summary = map[string]any{"row_count": len(result.Rows)}
	case "breakdown":
		groupBy := firstNonEmpty(normalized.GroupBy, "document_type")
		items, ok := s.Breakdown(Query{Window: firstNonEmpty(normalized.Window, "current_state"), From: normalized.From, To: normalized.To}, groupBy)
		if !ok {
			return QueryExecution{}, shared.Validation("unsupported breakdown group_by")
		}
		result.Columns = []string{groupBy, "created", "draft", "submitted", "approved", "rejected", "cancelled"}
		keys := make([]string, 0, len(items))
		for key := range items {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			kpi := items[key]
			result.Rows = append(result.Rows, map[string]any{
				groupBy:     key,
				"created":   kpi.Created,
				"draft":     kpi.Draft,
				"submitted": kpi.Submitted,
				"approved":  kpi.Approved,
				"rejected":  kpi.Rejected,
				"cancelled": kpi.Cancelled,
			})
		}
		result.Summary = map[string]any{"row_count": len(result.Rows)}
	case "reporting_breakdown":
		dimension := firstNonEmpty(normalized.GroupBy, "document_type")
		rows := s.ReportingBreakdown(FactQuery{From: normalized.From, To: normalized.To, LocationID: normalized.LocationID, DocumentType: normalized.DocumentType}, dimension)
		result.Columns = []string{"dimension_type", "dimension_key", "label", "created", "draft", "submitted", "approved", "rejected", "cancelled"}
		for _, row := range rows {
			result.Rows = append(result.Rows, map[string]any{
				"dimension_type": row.DimensionType,
				"dimension_key":  row.DimensionKey,
				"label":          row.Label,
				"created":        row.Created,
				"draft":          row.Draft,
				"submitted":      row.Submitted,
				"approved":       row.Approved,
				"rejected":       row.Rejected,
				"cancelled":      row.Cancelled,
			})
		}
		result.Summary = map[string]any{"row_count": len(result.Rows)}
	default:
		return QueryExecution{}, shared.Validation("unsupported analytics query source_kind")
	}
	result.Chart = buildChartSpec(normalized, result)
	return result, nil
}

func buildChartSpec(spec QuerySpec, result QueryExecution) ChartSpec {
	chartType := "table"
	if len(result.Rows) == 1 && (spec.SourceKind == "snapshot") {
		chartType = "metric"
	} else if spec.SourceKind == "trend" {
		chartType = "line"
	} else if spec.SourceKind == "breakdown" || spec.SourceKind == "reporting_breakdown" {
		chartType = "bar"
	}
	chart := ChartSpec{Type: chartType, Legend: true, TableCols: append([]string(nil), result.Columns...)}
	switch chartType {
	case "metric":
		if len(result.Rows) > 0 {
			row := result.Rows[0]
			measure := firstNonEmpty(firstMeasure(spec.Measures), "created")
			chart.Title = strings.ReplaceAll(measure, "_", " ")
			chart.Series = []ChartSeries{{Name: measure, Values: []float64{toFloat(row[measure])}}}
		}
	case "line":
		chart.X = make([]string, 0, len(result.Rows))
		measures := []string{"submitted_documents", "approved_documents"}
		for _, measure := range measures {
			series := ChartSeries{Name: measure}
			for _, row := range result.Rows {
				if measure == measures[0] {
					chart.X = append(chart.X, stringValue(row["generated_at"]))
				}
				series.Values = append(series.Values, toFloat(row[measure]))
			}
			chart.Series = append(chart.Series, series)
		}
	case "bar":
		labelKey := "dimension_key"
		if spec.SourceKind == "breakdown" {
			labelKey = firstNonEmpty(spec.GroupBy, "document_type")
		}
		chart.X = make([]string, 0, len(result.Rows))
		measure := firstNonEmpty(firstMeasure(spec.Measures), "submitted")
		series := ChartSeries{Name: measure}
		for _, row := range result.Rows {
			chart.X = append(chart.X, stringValue(row[labelKey]))
			series.Values = append(series.Values, toFloat(row[measure]))
		}
		chart.Series = []ChartSeries{series}
	}
	return chart
}

func normalizeQuerySpec(spec QuerySpec) (QuerySpec, error) {
	spec.SourceKind = strings.TrimSpace(spec.SourceKind)
	if spec.SourceKind == "" {
		spec.SourceKind = "snapshot"
	}
	switch spec.SourceKind {
	case "snapshot", "trend", "breakdown", "reporting_breakdown":
	default:
		return QuerySpec{}, shared.Validation("analytics query source_kind is invalid")
	}
	if spec.Window == "" {
		spec.Window = "current_state"
	}
	if spec.GroupBy == "" && (spec.SourceKind == "breakdown" || spec.SourceKind == "reporting_breakdown") {
		spec.GroupBy = "document_type"
	}
	if spec.Limit <= 0 {
		spec.Limit = 20
	}
	if spec.Limit > 200 {
		spec.Limit = 200
	}
	if len(spec.Measures) == 0 {
		switch spec.SourceKind {
		case "snapshot":
			spec.Measures = []string{"created", "submitted", "approved"}
		case "trend":
			spec.Measures = []string{"submitted_documents", "approved_documents"}
		default:
			spec.Measures = []string{"submitted"}
		}
	}
	if err := validateMeasures(spec.SourceKind, spec.Measures); err != nil {
		return QuerySpec{}, err
	}
	return spec, nil
}

func validateMeasures(sourceKind string, measures []string) error {
	allowed := map[string]bool{}
	switch sourceKind {
	case "snapshot":
		for _, key := range []string{"created", "draft", "submitted", "approved", "rejected", "cancelled", "pending_approvals", "dead_letters"} {
			allowed[key] = true
		}
	case "trend":
		for _, key := range []string{"submitted_documents", "approved_documents", "pending_approvals", "dead_letters"} {
			allowed[key] = true
		}
	default:
		for _, key := range []string{"created", "draft", "submitted", "approved", "rejected", "cancelled"} {
			allowed[key] = true
		}
	}
	for _, measure := range measures {
		if !allowed[strings.TrimSpace(measure)] {
			return shared.Validation("analytics query measure is not supported")
		}
	}
	return nil
}

func snapshotMetricRow(snapshot Snapshot, measures []string) map[string]any {
	row := map[string]any{
		"generated_at":      snapshot.GeneratedAt.Format(time.RFC3339),
		"created":           snapshot.Documents.Created,
		"draft":             snapshot.Documents.Draft,
		"submitted":         snapshot.Documents.Submitted,
		"approved":          snapshot.Documents.Approved,
		"rejected":          snapshot.Documents.Rejected,
		"cancelled":         snapshot.Documents.Cancelled,
		"pending_approvals": snapshot.Workflow.PendingApprovals,
		"dead_letters":      snapshot.Reliability.OutboxDeadLetters,
	}
	if len(measures) == 0 {
		return row
	}
	filtered := map[string]any{"generated_at": row["generated_at"]}
	for _, measure := range measures {
		if value, ok := row[measure]; ok {
			filtered[measure] = value
		}
	}
	return filtered
}

func firstMeasure(items []string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func sortedKeys(input map[string]any) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toFloat(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	default:
		return 0
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}
