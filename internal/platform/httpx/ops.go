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
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func registerOpsRoutes(mux *http.ServeMux, ident *identity.Service, auditSvc *audit.Service, eventingSvc *eventing.Service, offlineSvc *offline.Service, documentSvc *document.Service, searchSvc *search.Service, workflowSvc *workflow.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, obs *observability.Service, integrationSvc *integration.Service, jobSvc *jobs.Service, health *runtimehealth.Tracker) {
	mux.HandleFunc("GET /ops/audit-events", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "audit.read", "", "audit.read"); !ok {
			return
		}
		filter, err := auditQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		items := auditSvc.Query(filter)
		items = filterAuditEvents(items, map[string]string{
			"request_id":          strings.TrimSpace(r.URL.Query().Get("request_id")),
			"delegation_grant_id": strings.TrimSpace(r.URL.Query().Get("delegation_grant_id")),
			"from_state":          strings.TrimSpace(r.URL.Query().Get("from_state")),
			"to_state":            strings.TrimSpace(r.URL.Query().Get("to_state")),
			"metadata_key":        strings.TrimSpace(r.URL.Query().Get("metadata_key")),
			"metadata_value":      strings.TrimSpace(r.URL.Query().Get("metadata_value")),
		})
		respondJSON(w, http.StatusOK, buildAuditResponse(items))
	})

	mux.HandleFunc("GET /ops/audit-events/correlation/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "audit.read", "", "audit.read"); !ok {
			return
		}
		correlationID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ops/audit-events/correlation/"))
		if correlationID == "" {
			respondError(w, shared.NotFound("audit correlation route not found"))
			return
		}
		respondJSON(w, http.StatusOK, buildAuditResponse(auditSvc.Query(audit.Query{CorrelationID: correlationID})))
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
			"trace":       buildAuditResponse(auditSvc.Query(audit.Query{TargetType: targetType, TargetID: targetID})),
			"items":       auditSvc.Query(audit.Query{TargetType: targetType, TargetID: targetID}),
		})
	})

	mux.HandleFunc("GET /ops/health", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		snapshot := runtimehealth.Snapshot{Status: "healthy", Live: true, Ready: true, DependencyOK: true}
		if health != nil {
			snapshot = health.Snapshot(r.Context())
		}
		respondJSON(w, http.StatusOK, decorateHealth(snapshot))
	})

	mux.HandleFunc("GET /ops/offline/sync", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		if offlineSvc == nil {
			respondJSON(w, http.StatusOK, map[string]any{"summary": map[string]any{}, "batches": []any{}, "recent_items": []any{}})
			return
		}
		batches := offlineSvc.RecentBatches(50)
		items := offlineSvc.RecentOutcomes(200)
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		actorFilter := strings.TrimSpace(r.URL.Query().Get("actor_id"))
		deviceFilter := strings.TrimSpace(r.URL.Query().Get("device_id"))
		if statusFilter != "" || actorFilter != "" || deviceFilter != "" {
			filteredItems := make([]offline.SyncResultItem, 0, len(items))
			for _, item := range items {
				if statusFilter != "" && item.Status != statusFilter {
					continue
				}
				if deviceFilter != "" && item.DeviceID != deviceFilter {
					continue
				}
				if actorFilter != "" {
					matched := false
					for _, batch := range batches {
						if batch.ID == item.BatchID && batch.ActorID == actorFilter {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
				filteredItems = append(filteredItems, item)
			}
			items = filteredItems
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"summary":      offlineSvc.SyncSummary(),
			"batches":      batches,
			"recent_items": items,
		})
	})

	mux.HandleFunc("GET /ops/offline/sync/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		if offlineSvc == nil {
			respondError(w, shared.Validation("offline service is not configured"))
			return
		}
		batchID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ops/offline/sync/"))
		if batchID == "" {
			respondError(w, shared.NotFound("offline sync batch not found"))
			return
		}
		var batch offline.SyncBatch
		found := false
		for _, item := range offlineSvc.RecentBatches(200) {
			if item.ID == batchID {
				batch = item
				found = true
				break
			}
		}
		if !found {
			respondError(w, shared.NotFound("offline sync batch not found"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"batch": batch,
			"items": offlineSvc.BatchOutcomes(batchID),
		})
	})

	mux.HandleFunc("GET /ops/offline/conflicts", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		if offlineSvc == nil {
			respondJSON(w, http.StatusOK, map[string]any{"summary": map[string]any{}, "items": []any{}})
			return
		}
		items := make([]offline.SyncResultItem, 0)
		for _, item := range offlineSvc.RecentOutcomes(200) {
			if item.Status == offline.StatusConflict {
				items = append(items, item)
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"summary": summarizeOfflineOutcomes(items),
			"items":   items,
		})
	})

	mux.HandleFunc("GET /ops/outbox", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "outbox.read", "", "outbox.read"); !ok {
			return
		}
		items := eventingSvc.ListOutbox()
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		eventTypeFilter := strings.TrimSpace(r.URL.Query().Get("event_type"))
		filtered := make([]eventing.OutboxRecord, 0, len(items))
		for _, item := range items {
			if statusFilter != "" && item.Status != statusFilter {
				continue
			}
			if eventTypeFilter != "" && item.EventType != eventTypeFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		respondJSON(w, http.StatusOK, map[string]any{"summary": summarizeOutbox(filtered), "items": filtered})
	})

	mux.HandleFunc("GET /ops/outbox/deliveries", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "outbox.read", "", "outbox.read"); !ok {
			return
		}
		items := eventingSvc.ListDeliveries()
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		sinkFilter := strings.TrimSpace(r.URL.Query().Get("sink"))
		filtered := make([]eventing.OutboxDeliveryRecord, 0, len(items))
		for _, item := range items {
			if statusFilter != "" && item.Status != statusFilter {
				continue
			}
			if sinkFilter != "" && item.SinkName != sinkFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		respondJSON(w, http.StatusOK, map[string]any{"summary": summarizeDeliveries(filtered), "items": filtered})
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
		items := eventingSvc.ListDeadLetters()
		sinkFilter := strings.TrimSpace(r.URL.Query().Get("sink"))
		filtered := make([]eventing.DeadLetterRecord, 0, len(items))
		for _, item := range items {
			if sinkFilter != "" && item.SinkName != sinkFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		respondJSON(w, http.StatusOK, map[string]any{"summary": summarizeDeadLetters(filtered), "items": filtered})
	})

	mux.HandleFunc("GET /ops/integrations/deliveries", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		items := integrationSvc.ListSubmissions()
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		systemFilter := strings.TrimSpace(r.URL.Query().Get("system_key"))
		correlationFilter := strings.TrimSpace(r.URL.Query().Get("correlation_id"))
		filtered := make([]integration.SubmissionRecord, 0, len(items))
		for _, item := range items {
			if statusFilter != "" && item.Status != statusFilter {
				continue
			}
			if systemFilter != "" && item.ExternalSystemKey != systemFilter {
				continue
			}
			if correlationFilter != "" && item.CorrelationID != correlationFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		respondJSON(w, http.StatusOK, map[string]any{"summary": summarizeIntegrations(filtered), "items": filtered})
	})

	mux.HandleFunc("GET /ops/integrations/health", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.HealthSummary()})
	})

	mux.HandleFunc("GET /ops/integrations/deliveries/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/ops/integrations/deliveries/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 1 && strings.TrimSpace(parts[0]) != "" {
			item, ok := integrationSvc.GetSubmission(parts[0])
			if !ok {
				respondError(w, shared.NotFound("integration delivery not found"))
				return
			}
			respondJSON(w, http.StatusOK, item)
			return
		}
		if len(parts) != 2 || parts[1] != "attempts" || strings.TrimSpace(parts[0]) == "" {
			respondError(w, shared.NotFound("integration delivery route not found"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListSubmissionAttempts(parts[0])})
	})

	mux.HandleFunc("GET /ops/integrations/dead-letters", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": integrationSvc.ListDeadLetters()})
	})

	mux.HandleFunc("GET /ops/integrations/dead-letters/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		deadLetterID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ops/integrations/dead-letters/"))
		if deadLetterID == "" || strings.Contains(deadLetterID, "/") {
			respondError(w, shared.NotFound("integration dead letter route not found"))
			return
		}
		item, ok := integrationSvc.GetDeadLetter(deadLetterID)
		if !ok {
			respondError(w, shared.NotFound("integration dead letter not found"))
			return
		}
		respondJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("POST /ops/integrations/dead-letters/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/ops/integrations/dead-letters/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 || parts[1] != "replay" || strings.TrimSpace(parts[0]) == "" {
			respondError(w, shared.NotFound("integration dead letter route not found"))
			return
		}
		item, err := integrationSvc.ReplayDeadLetter(parts[0])
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET /ops/stats", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		payload := map[string]any{"observability": obs.Snapshot()}
		if health != nil {
			payload["runtime_health"] = health.Snapshot(r.Context())
			payload["health"] = decorateHealth(health.Snapshot(r.Context()))
		}
		if jobSvc != nil {
			jobItems := jobSvc.List()
			payload["jobs"] = map[string]any{
				"summary":         jobSvc.Summary(),
				"runtime_summary": summarizeJobs(jobItems),
				"items":           jobItems,
			}
		}
		outboxItems := eventingSvc.ListOutbox()
		deliveryItems := eventingSvc.ListDeliveries()
		deadLetterItems := eventingSvc.ListDeadLetters()
		payload["outbox"] = map[string]any{
			"summary":             summarizeOutbox(outboxItems),
			"items":               outboxItems,
			"deliveries":          deliveryItems,
			"delivery_summary":    summarizeDeliveries(deliveryItems),
			"dead_letter_summary": summarizeDeadLetters(deadLetterItems),
		}
		submissions := integrationSvc.ListSubmissions()
		payload["integrations"] = map[string]any{
			"systems":      integrationSvc.ListSystems(),
			"health":       integrationSvc.HealthSummary(),
			"deliveries":   submissions,
			"summary":      summarizeIntegrations(submissions),
			"dead_letters": integrationSvc.ListDeadLetters(),
		}
		payload["projections"] = projectionHealthSummary(searchSvc)
		if offlineSvc != nil {
			payload["offline"] = map[string]any{
				"summary":      offlineSvc.SyncSummary(),
				"batches":      offlineSvc.RecentBatches(20),
				"recent_items": offlineSvc.RecentOutcomes(50),
			}
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
			snapshot := health.Snapshot(r.Context())
			body += renderDBStatsMetrics(snapshot)
			body += renderRuntimeHealthMetrics(snapshot)
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
		report, err := searchSvc.ProjectionConsistencyReport()
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, report)
	})

	mux.HandleFunc("GET /ops/projections/status", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		report, err := searchSvc.ProjectionConsistencyReport()
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items":         searchSvc.ProjectionStatuses(documentSvc.List()),
			"runtime_items": searchSvc.IndexRuntimes(),
			"coverage":      report,
		})
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

	mux.HandleFunc("GET /ops/jobs", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		if jobSvc == nil {
			respondError(w, shared.Validation("job service is not configured"))
			return
		}
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		nameFilter := strings.TrimSpace(r.URL.Query().Get("name"))
		correlationFilter := strings.TrimSpace(r.URL.Query().Get("correlation_id"))
		items := jobSvc.List()
		filtered := make([]jobs.Job, 0, len(items))
		for _, item := range items {
			if statusFilter != "" && item.Status != statusFilter {
				continue
			}
			if nameFilter != "" && item.Name != nameFilter {
				continue
			}
			if correlationFilter != "" && jobCorrelationID(item) != correlationFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		respondJSON(w, http.StatusOK, map[string]any{"summary": summarizeJobs(filtered), "items": filtered})
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

	mux.HandleFunc("GET /ops/traces/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		correlationID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ops/traces/"))
		if correlationID == "" {
			respondError(w, shared.NotFound("trace not found"))
			return
		}
		respondJSON(w, http.StatusOK, buildTrace(correlationID, obs, auditSvc, eventingSvc, workflowSvc, jobSvc, integrationSvc, offlineSvc))
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

	mux.HandleFunc("GET /ops/workflow/history", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "monitoring.read", "", "monitoring.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items": workflowSvc.ListHistory(strings.TrimSpace(r.URL.Query().Get("target_type")), strings.TrimSpace(r.URL.Query().Get("target_id"))),
		})
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

func renderRuntimeHealthMetrics(snapshot runtimehealth.Snapshot) string {
	ready := "0"
	if snapshot.Ready {
		ready = "1"
	}
	lines := []string{
		"runtime_ready " + ready,
		"runtime_degraded_subsystems " + strconv.Itoa(len(snapshot.FailureCategories)),
	}
	for _, item := range snapshot.Subsystems {
		if item.Status != "degraded" {
			continue
		}
		lines = append(lines, "runtime_subsystem_"+strings.ReplaceAll(item.Name, "-", "_")+"_degraded 1")
	}
	return strings.Join(lines, "\n") + "\n"
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
