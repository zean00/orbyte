package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func registerOpsRoutes(mux *http.ServeMux, ident *identity.Service, auditSvc *audit.Service, eventingSvc *eventing.Service, documentSvc *document.Service, searchSvc *search.Service, workflowSvc *workflow.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, obs *observability.Service, jobSvc *jobs.Service, health *runtimehealth.Tracker) {
	mux.HandleFunc("GET /ops/audit-events", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "audit.read", "", "audit.read"); !ok {
			return
		}
		filter, err := auditQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": auditSvc.Query(filter)})
	})

	mux.HandleFunc("GET /ops/audit-events/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "audit.read", "", "audit.read"); !ok {
			return
		}
		targetType, targetID, ok := opsAuditTimelinePath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("audit timeline not found"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"target_type": targetType,
			"target_id":   targetID,
			"items":       auditSvc.Query(audit.Query{TargetType: targetType, TargetID: targetID}),
		})
	})

	mux.HandleFunc("GET /ops/outbox", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "outbox.read", "", "outbox.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": eventingSvc.ListOutbox()})
	})

	mux.HandleFunc("GET /ops/outbox/deliveries", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "outbox.read", "", "outbox.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": eventingSvc.ListDeliveries()})
	})

	mux.HandleFunc("POST /ops/outbox/dispatch", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "outbox.dispatch", "", "outbox.dispatch"); !ok {
			return
		}
		count, err := eventingSvc.DispatchPending(100)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"dispatched": count})
	})

	mux.HandleFunc("POST /ops/outbox/", func(w http.ResponseWriter, r *http.Request) {
		outboxID, action, ok := opsOutboxActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("outbox action not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "outbox.dispatch", "", "outbox.dispatch"); !ok {
			return
		}
		switch action {
		case "retry":
			item, err := eventingSvc.RetryOutbox(outboxID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, item)
		default:
			respondError(w, shared.NotFound("outbox action not found"))
		}
	})

	mux.HandleFunc("POST /ops/outbox/deliveries/", func(w http.ResponseWriter, r *http.Request) {
		deliveryID, action, ok := opsOutboxDeliveryActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("outbox delivery action not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "outbox.dispatch", "", "outbox.dispatch"); !ok {
			return
		}
		switch action {
		case "retry":
			item, err := eventingSvc.RetryDelivery(deliveryID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, item)
		default:
			respondError(w, shared.NotFound("outbox delivery action not found"))
		}
	})

	mux.HandleFunc("GET /ops/domain-events", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "event.read", "", "event.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": eventingSvc.ListEvents()})
	})

	mux.HandleFunc("GET /ops/dead-letters", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "deadletter.read", "", "deadletter.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": eventingSvc.ListDeadLetters()})
	})

	mux.HandleFunc("GET /ops/stats", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		payload := map[string]any{"observability": obs.Snapshot()}
		if health != nil {
			payload["runtime_health"] = health.Snapshot(r.Context())
		}
		if jobSvc != nil {
			payload["jobs"] = map[string]any{
				"summary": jobSvc.Summary(),
				"items":   jobSvc.List(),
			}
		}
		payload["outbox"] = map[string]any{
			"items":      eventingSvc.ListOutbox(),
			"deliveries": eventingSvc.ListDeliveries(),
		}
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("POST /ops/jobs/", func(w http.ResponseWriter, r *http.Request) {
		jobID, action, ok := opsJobActionPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("job action not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		if jobSvc == nil {
			respondError(w, shared.Conflict("jobs are not configured"))
			return
		}
		switch action {
		case "requeue":
			item, err := jobSvc.Requeue(jobID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, item)
		default:
			respondError(w, shared.NotFound("job action not found"))
		}
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "metrics.read", "", "metrics.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		body := obs.RenderPrometheus()
		if health != nil {
			body += renderDBStatsMetrics(health.Snapshot(r.Context()))
		}
		_, _ = w.Write([]byte(body))
	})

	mux.HandleFunc("GET /ops/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, monitoringSvc.Summary())
	})

	mux.HandleFunc("GET /ops/analytics", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, analyticsSvc.Snapshot())
	})

	mux.HandleFunc("POST /ops/analytics/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.manage_reports", "", "analytics.manage_reports"); !ok {
			return
		}
		snapshot, err := analyticsSvc.CaptureSnapshot()
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, snapshot)
	})

	mux.HandleFunc("GET /ops/analytics/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.ListSnapshots()})
	})

	mux.HandleFunc("GET /ops/analytics/trends", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		query, err := analyticsQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.TrendsByQuery(query)})
	})

	mux.HandleFunc("GET /ops/analytics/query", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		query, err := analyticsQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.QuerySnapshots(query)})
	})

	mux.HandleFunc("GET /ops/analytics/breakdown/documents", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		query, err := analyticsQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		groupBy := r.URL.Query().Get("group_by")
		if groupBy == "" {
			groupBy = "document_type"
		}
		items, ok := analyticsSvc.Breakdown(query, groupBy)
		if !ok {
			respondJSON(w, http.StatusOK, map[string]any{"items": map[string]any{}})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"group_by": groupBy, "items": items})
	})

	mux.HandleFunc("GET /ops/analytics/rollups", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		granularity := r.URL.Query().Get("granularity")
		if granularity == "" {
			granularity = "daily"
		}
		limit := 30
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				respondError(w, shared.Validation("invalid limit"))
				return
			}
			limit = parsed
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.ListRollups(granularity, limit)})
	})

	mux.HandleFunc("GET /ops/analytics/rollups/breakdown/documents", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		granularity := r.URL.Query().Get("granularity")
		if granularity == "" {
			granularity = "daily"
		}
		groupBy := r.URL.Query().Get("group_by")
		if groupBy == "" {
			groupBy = "document_type"
		}
		limit := 30
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				respondError(w, shared.Validation("invalid limit"))
				return
			}
			limit = parsed
		}
		items, ok := analyticsSvc.RollupBreakdown(granularity, groupBy, limit)
		if !ok {
			respondJSON(w, http.StatusOK, map[string]any{"group_by": groupBy, "items": map[string]any{}})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"granularity": granularity, "group_by": groupBy, "items": items})
	})

	mux.HandleFunc("GET /ops/analytics/compare", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		query, err := analyticsQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		comparison, ok := analyticsSvc.Compare(query)
		if !ok {
			respondJSON(w, http.StatusOK, map[string]any{})
			return
		}
		respondJSON(w, http.StatusOK, comparison)
	})

	mux.HandleFunc("GET /ops/analytics/facts", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		query, err := analyticsQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		factQuery := analytics.FactQuery{From: query.From, To: query.To, LocationID: r.URL.Query().Get("location_id"), DocumentType: r.URL.Query().Get("document_type")}
		respondJSON(w, http.StatusOK, analyticsSvc.QueryFacts(factQuery))
	})

	mux.HandleFunc("GET /ops/analytics/reporting/documents", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		query, err := analyticsQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		dimension := r.URL.Query().Get("dimension")
		if dimension == "" {
			dimension = "document_type"
		}
		factQuery := analytics.FactQuery{From: query.From, To: query.To, LocationID: r.URL.Query().Get("location_id"), DocumentType: r.URL.Query().Get("document_type")}
		respondJSON(w, http.StatusOK, map[string]any{"dimension": dimension, "items": analyticsSvc.ReportingBreakdown(factQuery, dimension)})
	})

	mux.HandleFunc("GET /ops/analytics/reporting/documents/export", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		query, err := analyticsQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		dimension := r.URL.Query().Get("dimension")
		if dimension == "" {
			dimension = "document_type"
		}
		factQuery := analytics.FactQuery{From: query.From, To: query.To, LocationID: r.URL.Query().Get("location_id"), DocumentType: r.URL.Query().Get("document_type")}
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "csv"
		}
		var content []byte
		switch format {
		case "xlsx":
			content, err = analyticsSvc.ExportDocumentReportingXLSX(factQuery, dimension)
		case "pdf":
			content, err = analyticsSvc.ExportDocumentReportingPDF(factQuery, dimension)
		default:
			content, err = analyticsSvc.ExportDocumentReportingCSV(factQuery, dimension)
		}
		if err != nil {
			respondError(w, err)
			return
		}
		if format == "xlsx" {
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Header().Set("Content-Disposition", "attachment; filename=analytics_reporting_documents.xlsx")
		} else if format == "pdf" {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=analytics_reporting_documents.pdf")
		} else {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename=analytics_reporting_documents.csv")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	})

	mux.HandleFunc("POST /ops/analytics/reports", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.manage_reports", "", "analytics.manage_reports"); !ok {
			return
		}
		query := r.URL.Query()
		def, err := analyticsSvc.CreateReportDefinition(analytics.ReportDefinition{
			Name:            query.Get("name"),
			Dimension:       query.Get("dimension"),
			Format:          query.Get("format"),
			Window:          query.Get("window"),
			LocationID:      query.Get("location_id"),
			DocumentType:    query.Get("document_type"),
			DeliveryChannel: query.Get("delivery_channel"),
			DeliveryTarget:  query.Get("delivery_target"),
			Schedule:        query.Get("schedule"),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, def)
	})

	mux.HandleFunc("GET /ops/analytics/reports", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.ListReportDefinitions()})
	})

	mux.HandleFunc("GET /ops/analytics/report-runs", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.ListReportRuns()})
	})

	mux.HandleFunc("GET /ops/analytics/report-artifacts", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.ListReportArtifacts()})
	})

	mux.HandleFunc("GET /ops/analytics/report-deliveries", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.ListReportDeliveries()})
	})

	mux.HandleFunc("GET /ops/analytics/report-delivery-dead-letters", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": analyticsSvc.ListReportDeliveryDeadLetters()})
	})

	mux.HandleFunc("GET /ops/analytics/report-artifacts/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		artifactID := reportArtifactIDFromPath(r.URL.Path)
		if artifactID == "" {
			http.NotFound(w, r)
			return
		}
		artifact, ok := analyticsSvc.GetReportArtifact(artifactID)
		if !ok {
			respondError(w, shared.NotFound("report artifact not found"))
			return
		}
		w.Header().Set("Content-Type", artifact.ContentType)
		w.Header().Set("Content-Disposition", "attachment; filename="+artifact.FileName)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(artifact.Content)
	})

	mux.HandleFunc("POST /ops/analytics/report-artifacts/deliver", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.deliver_reports", "", "analytics.deliver_reports"); !ok {
			return
		}
		artifactID := r.URL.Query().Get("artifact_id")
		channel := r.URL.Query().Get("channel")
		recipient := r.URL.Query().Get("recipient")
		delivery, err := analyticsSvc.DeliverArtifact(artifactID, channel, recipient)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, delivery)
	})

	mux.HandleFunc("POST /ops/analytics/report-deliveries/retry", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.deliver_reports", "", "analytics.deliver_reports"); !ok {
			return
		}
		artifactID := r.URL.Query().Get("artifact_id")
		channel := r.URL.Query().Get("channel")
		recipient := r.URL.Query().Get("recipient")
		delivery, err := analyticsSvc.DeliverArtifact(artifactID, channel, recipient)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, delivery)
	})

	mux.HandleFunc("POST /ops/analytics/report-retention/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.manage_reports", "", "analytics.manage_reports"); !ok {
			return
		}
		before := r.URL.Query().Get("before")
		if before == "" {
			respondError(w, shared.Validation("before timestamp is required"))
			return
		}
		cutoff, err := time.Parse(time.RFC3339, before)
		if err != nil {
			respondError(w, shared.Validation("invalid before timestamp"))
			return
		}
		if err := analyticsSvc.CleanupReportData(cutoff); err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"cleaned_before": cutoff})
	})

	mux.HandleFunc("POST /ops/analytics/reports/run", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.manage_reports", "", "analytics.manage_reports"); !ok {
			return
		}
		reportID := r.URL.Query().Get("report_id")
		for _, def := range analyticsSvc.ListReportDefinitions() {
			if def.ID != reportID {
				continue
			}
			run, _, err := analyticsSvc.RunReport(def)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, run)
			return
		}
		respondError(w, shared.NotFound("report definition not found"))
	})

	mux.HandleFunc("GET /ops/projections/documents", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "projection.refresh"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": searchSvc.ListDocuments()})
	})

	mux.HandleFunc("GET /ops/consistency/projections", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, searchSvc.Consistency(documentSvc.List()))
	})

	mux.HandleFunc("GET /ops/projections/status", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": searchSvc.ProjectionStatuses(documentSvc.List())})
	})

	mux.HandleFunc("POST /ops/consistency/projections/document-summary/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "outbox.dispatch", "", "outbox.dispatch"); !ok {
			return
		}
		if jobSvc == nil {
			respondError(w, shared.Validation("job service is not configured"))
			return
		}
		documentID := r.URL.Query().Get("document_id")
		job, err := jobSvc.Enqueue(search.JobRebuildSummary, map[string]any{"document_id": documentID})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusAccepted, job)
	})

	mux.HandleFunc("GET /ops/consistency/analytics", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.read", "", "analytics.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, analyticsSvc.Consistency())
	})

	mux.HandleFunc("POST /ops/consistency/analytics/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "analytics.manage_reports", "", "analytics.manage_reports"); !ok {
			return
		}
		if jobSvc == nil {
			respondError(w, shared.Validation("job service is not configured"))
			return
		}
		job, err := jobSvc.Enqueue(analytics.JobRecomputeState, nil)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusAccepted, job)
	})

	mux.HandleFunc("GET /ops/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		if jobSvc == nil {
			respondError(w, shared.Validation("job service is not configured"))
			return
		}
		jobID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ops/jobs/"))
		job, ok := jobSvc.Get(jobID)
		if !ok || jobID == "" {
			respondError(w, shared.NotFound("job not found"))
			return
		}
		respondJSON(w, http.StatusOK, job)
	})

	mux.HandleFunc("GET /ops/workflow/tasks", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": workflowSvc.ListTasks()})
	})

	mux.HandleFunc("GET /ops/workflow/approvals", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": workflowSvc.ListApprovals()})
	})
}

func opsOutboxActionPath(path string) (string, string, bool) {
	trimmed := strings.TrimPrefix(path, "/ops/outbox/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[0] == "deliveries" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func opsOutboxDeliveryActionPath(path string) (string, string, bool) {
	trimmed := strings.TrimPrefix(path, "/ops/outbox/deliveries/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func opsJobActionPath(path string) (string, string, bool) {
	trimmed := strings.TrimPrefix(path, "/ops/jobs/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func opsAuditTimelinePath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "ops" || parts[1] != "audit-events" {
		return "", "", false
	}
	return parts[2], parts[3], parts[2] != "" && parts[3] != ""
}

func auditQueryFromRequest(r *http.Request) (audit.Query, error) {
	q := audit.Query{
		TargetType:       strings.TrimSpace(r.URL.Query().Get("target_type")),
		TargetID:         strings.TrimSpace(r.URL.Query().Get("target_id")),
		ActorID:          strings.TrimSpace(r.URL.Query().Get("actor_id")),
		ActorKind:        strings.TrimSpace(r.URL.Query().Get("actor_kind")),
		OnBehalfOfUserID: strings.TrimSpace(r.URL.Query().Get("on_behalf_of_user_id")),
		Action:           strings.TrimSpace(r.URL.Query().Get("action")),
		CorrelationID:    strings.TrimSpace(r.URL.Query().Get("correlation_id")),
		OrganizationID:   strings.TrimSpace(r.URL.Query().Get("organization_id")),
		LocationID:       strings.TrimSpace(r.URL.Query().Get("location_id")),
		OperatingUnitID:  strings.TrimSpace(r.URL.Query().Get("operating_unit_id")),
	}
	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return audit.Query{}, shared.Validation("invalid audit from timestamp")
		}
		q.OccurredFrom = parsed
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return audit.Query{}, shared.Validation("invalid audit to timestamp")
		}
		q.OccurredTo = parsed
	}
	return q, nil
}

func renderDBStatsMetrics(snapshot runtimehealth.Snapshot) string {
	if snapshot.Database == nil {
		return ""
	}
	db := snapshot.Database
	return strings.Join([]string{
		"db_pool_max_open_connections " + strconv.Itoa(db.MaxOpenConnections),
		"db_pool_open_connections " + strconv.Itoa(db.OpenConnections),
		"db_pool_in_use_connections " + strconv.Itoa(db.InUse),
		"db_pool_idle_connections " + strconv.Itoa(db.Idle),
		"db_pool_wait_count " + strconv.FormatInt(db.WaitCount, 10),
		"db_pool_wait_duration_millis " + strconv.FormatInt(db.WaitDurationMillis, 10),
		"db_pool_max_idle_closed " + strconv.FormatInt(db.MaxIdleClosed, 10),
		"db_pool_max_idle_time_closed " + strconv.FormatInt(db.MaxIdleTimeClosed, 10),
		"db_pool_max_lifetime_closed " + strconv.FormatInt(db.MaxLifetimeClosed, 10),
	}, "\n") + "\n"
}

func reportArtifactIDFromPath(path string) string {
	const prefix = "/ops/analytics/report-artifacts/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return ""
	}
	return path[len(prefix):]
}

func analyticsQueryFromRequest(r *http.Request) (analytics.Query, error) {
	q := analytics.Query{Window: r.URL.Query().Get("window")}
	if from := r.URL.Query().Get("from"); from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return analytics.Query{}, shared.Validation("invalid from timestamp")
		}
		q.From = parsed
	}
	if to := r.URL.Query().Get("to"); to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return analytics.Query{}, shared.Validation("invalid to timestamp")
		}
		q.To = parsed
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil {
			return analytics.Query{}, shared.Validation("invalid limit")
		}
		q.Limit = parsed
	}
	return q, nil
}
