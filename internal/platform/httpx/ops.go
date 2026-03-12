package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"clinic/internal/platform/analytics"
	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/identity"
	"clinic/internal/platform/jobs"
	"clinic/internal/platform/monitoring"
	"clinic/internal/platform/observability"
	"clinic/internal/platform/search"
	"clinic/internal/platform/shared"
	"clinic/internal/platform/workflow"
)

func registerOpsRoutes(mux *http.ServeMux, ident *identity.Service, auditSvc *audit.Service, eventingSvc *eventing.Service, documentSvc *document.Service, searchSvc *search.Service, workflowSvc *workflow.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, obs *observability.Service, jobSvc *jobs.Service) {
	mux.HandleFunc("GET /ops/audit-events", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "audit.read", "", "audit.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": auditSvc.List()})
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
		respondJSON(w, http.StatusOK, obs.Snapshot())
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "metrics.read", "", "metrics.read"); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(obs.RenderPrometheus()))
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

	mux.HandleFunc("POST /ops/consistency/projections/document-summary/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "outbox.dispatch", "", "outbox.dispatch"); !ok {
			return
		}
		if jobSvc == nil {
			respondError(w, shared.Validation("job service is not configured"))
			return
		}
		documentID := r.URL.Query().Get("document_id")
		job := jobSvc.Enqueue("projection.rebuild.document_summary", func() (map[string]any, error) {
			if documentID != "" {
				record, err := documentSvc.Get(documentID)
				if err != nil {
					return nil, err
				}
				searchSvc.RebuildDocument(record)
				return map[string]any{"rebuilt_document_id": documentID}, nil
			}
			records := documentSvc.List()
			searchSvc.RebuildAll(records)
			return map[string]any{"rebuilt": "document_summary", "count": len(records)}, nil
		})
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
		job := jobSvc.Enqueue("analytics.recompute.current_state", func() (map[string]any, error) {
			snapshot, err := analyticsSvc.RecomputeCurrentState()
			if err != nil {
				return nil, err
			}
			return map[string]any{"snapshot": snapshot}, nil
		})
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
