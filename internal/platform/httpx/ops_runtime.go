package httpx

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

type operatorRunbook struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Subsystem       string   `json:"subsystem"`
	Symptom         string   `json:"symptom"`
	LikelyCauses    []string `json:"likely_causes,omitempty"`
	ImmediateChecks []string `json:"immediate_checks,omitempty"`
	SafeActions     []string `json:"safe_actions,omitempty"`
}

type traceStep struct {
	Kind            string         `json:"kind"`
	Status          string         `json:"status,omitempty"`
	Label           string         `json:"label"`
	OccurredAt      time.Time      `json:"occurred_at"`
	CorrelationID   string         `json:"correlation_id,omitempty"`
	FailureCategory string         `json:"failure_category,omitempty"`
	RunbookID       string         `json:"runbook_id,omitempty"`
	Data            map[string]any `json:"data,omitempty"`
}

func builtInRunbooks() map[string]operatorRunbook {
	return map[string]operatorRunbook{
		"runtime.dependencies": {
			ID:        "runtime.dependencies",
			Title:     "Dependency Health Checks",
			Subsystem: "platform",
			Symptom:   "Readiness is degraded because a required dependency is unavailable.",
			LikelyCauses: []string{
				"database connectivity failure",
				"dependency timeout",
				"misconfigured runtime credentials",
			},
			ImmediateChecks: []string{
				"verify /readyz dependency_error",
				"check database connectivity and pool saturation",
				"confirm environment secrets and endpoint reachability",
			},
			SafeActions: []string{
				"restore dependency connectivity",
				"reduce load or restart the affected dependency if needed",
			},
		},
		"runtime.jobs": {
			ID:        "runtime.jobs",
			Title:     "Job Runtime Failures",
			Subsystem: "jobs",
			Symptom:   "Queued or dead-letter jobs are accumulating or job handlers are failing repeatedly.",
			LikelyCauses: []string{
				"handler regression",
				"invalid job payload",
				"dependency unavailable during job execution",
			},
			ImmediateChecks: []string{
				"inspect /ops/jobs for failed and dead-letter items",
				"open the linked trace if correlation_id is available",
				"review handler-specific failure counters in /metrics",
			},
			SafeActions: []string{
				"requeue affected jobs after fixing the underlying failure",
			},
		},
		"runtime.outbox": {
			ID:        "runtime.outbox",
			Title:     "Outbox Dispatch Failures",
			Subsystem: "outbox",
			Symptom:   "Outbox deliveries are retrying, stalled, or dead-lettered.",
			LikelyCauses: []string{
				"missing handler or sink",
				"delivery sink failure",
				"missing referenced event",
			},
			ImmediateChecks: []string{
				"inspect /ops/outbox/deliveries and /ops/dead-letters",
				"check sink-level failures and attempt counts",
				"review the correlated domain event and audit trail",
			},
			SafeActions: []string{
				"retry the delivery or outbox record after restoring the sink",
			},
		},
		"runtime.scheduler": {
			ID:        "runtime.scheduler",
			Title:     "Scheduler Degradation",
			Subsystem: "scheduler",
			Symptom:   "Background scheduler runs are failing or lagging.",
			LikelyCauses: []string{
				"job runtime failure",
				"analytics/reporting dependency issue",
			},
			ImmediateChecks: []string{
				"inspect scheduled job queues and analytics consistency",
			},
			SafeActions: []string{
				"requeue or rerun the scheduled job after the dependency is healthy",
			},
		},
		"runtime.projections": {
			ID:        "runtime.projections",
			Title:     "Projection and Search Repair",
			Subsystem: "search",
			Symptom:   "Projection coverage or search runtime is stale, missing, or lagging.",
			LikelyCauses: []string{
				"missed refresh path",
				"projection rebuild required",
				"backend indexing failure",
			},
			ImmediateChecks: []string{
				"inspect /ops/projections/status and search consistency endpoints",
			},
			SafeActions: []string{
				"enqueue reconcile, repair, or rebuild actions through ops endpoints",
			},
		},
		"runtime.workflow": {
			ID:        "runtime.workflow",
			Title:     "Workflow and Policy Runtime Failures",
			Subsystem: "workflow",
			Symptom:   "Workflow actions are blocked by runtime validation or policy failures.",
			LikelyCauses: []string{
				"invalid workflow policy runtime",
				"assignment or SLA runtime mismatch",
			},
			ImmediateChecks: []string{
				"inspect workflow history and correlated audit/events",
				"check policy runtime diagnostics in admin validation endpoints",
			},
			SafeActions: []string{
				"fix the invalid policy/workflow runtime and retry the blocked action",
			},
		},
		"runtime.integrations": {
			ID:        "runtime.integrations",
			Title:     "Integration Delivery Failures",
			Subsystem: "integration",
			Symptom:   "Integration submissions are failing or dead-lettered.",
			LikelyCauses: []string{
				"external system unavailable",
				"adapter contract mismatch",
				"submission payload invalid",
			},
			ImmediateChecks: []string{
				"inspect /ops/integrations/deliveries and dead letters",
				"review submission attempts and correlated trace",
			},
			SafeActions: []string{
				"replay dead letters or retry submissions after correcting the adapter/runtime issue",
			},
		},
	}
}

func runbooksForIDs(ids []string) []operatorRunbook {
	all := builtInRunbooks()
	items := make([]operatorRunbook, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if item, ok := all[id]; ok {
			items = append(items, item)
			seen[id] = struct{}{}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func filterAuditEvents(items []audit.Event, q map[string]string) []audit.Event {
	filtered := make([]audit.Event, 0, len(items))
	for _, item := range items {
		if q["request_id"] != "" && strings.TrimSpace(item.RequestID) != q["request_id"] {
			continue
		}
		if q["delegation_grant_id"] != "" && strings.TrimSpace(item.DelegationGrantID) != q["delegation_grant_id"] {
			continue
		}
		if q["from_state"] != "" && strings.TrimSpace(item.FromState) != q["from_state"] {
			continue
		}
		if q["to_state"] != "" && strings.TrimSpace(item.ToState) != q["to_state"] {
			continue
		}
		if q["metadata_key"] != "" {
			value, ok := item.Metadata[q["metadata_key"]]
			if !ok {
				continue
			}
			if q["metadata_value"] != "" && strings.TrimSpace(stringifyAny(value)) != q["metadata_value"] {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func buildAuditResponse(items []audit.Event) map[string]any {
	actions := map[string]int{}
	targetTypes := map[string]int{}
	actors := map[string]int{}
	for _, item := range items {
		actions[item.Action]++
		targetTypes[item.TargetType]++
		if item.ActorID != "" {
			actors[item.ActorID]++
		}
	}
	return map[string]any{
		"summary": map[string]any{
			"count": len(items),
		},
		"facets": map[string]any{
			"actions":      actions,
			"target_types": targetTypes,
			"actors":       actors,
		},
		"items": items,
	}
}

func summarizeJobs(items []jobs.Job) map[string]any {
	byName := map[string]map[string]int{}
	byStatus := map[string]int{}
	oldestQueuedAge := int64(0)
	now := time.Now().UTC()
	for _, item := range items {
		byStatus[item.Status]++
		if _, ok := byName[item.Name]; !ok {
			byName[item.Name] = map[string]int{}
		}
		byName[item.Name][item.Status]++
		if item.Status == jobs.StatusQueued && !item.CreatedAt.IsZero() {
			age := int64(now.Sub(item.CreatedAt).Seconds())
			if age > oldestQueuedAge {
				oldestQueuedAge = age
			}
		}
	}
	return map[string]any{
		"count":                     len(items),
		"by_status":                 byStatus,
		"by_name":                   byName,
		"oldest_queued_age_seconds": oldestQueuedAge,
	}
}

func summarizeOutbox(items []eventing.OutboxRecord) map[string]any {
	byStatus := map[string]int{}
	byType := map[string]int{}
	oldestPendingAge := int64(0)
	now := time.Now().UTC()
	for _, item := range items {
		byStatus[item.Status]++
		byType[item.EventType]++
		if item.Status == "pending" && !item.CreatedAt.IsZero() {
			age := int64(now.Sub(item.CreatedAt).Seconds())
			if age > oldestPendingAge {
				oldestPendingAge = age
			}
		}
	}
	return map[string]any{
		"count":                      len(items),
		"by_status":                  byStatus,
		"by_event_type":              byType,
		"oldest_pending_age_seconds": oldestPendingAge,
	}
}

func summarizeDeliveries(items []eventing.OutboxDeliveryRecord) map[string]any {
	byStatus := map[string]int{}
	bySink := map[string]int{}
	byFailureCategory := map[string]int{}
	for _, item := range items {
		byStatus[item.Status]++
		bySink[item.SinkName]++
		if category := deliveryFailureCategory(item); category != "" {
			byFailureCategory[category]++
		}
	}
	return map[string]any{
		"count":               len(items),
		"by_status":           byStatus,
		"by_sink":             bySink,
		"by_failure_category": byFailureCategory,
	}
}

func summarizeDeadLetters(items []eventing.DeadLetterRecord) map[string]any {
	bySink := map[string]int{}
	byReason := map[string]int{}
	for _, item := range items {
		bySink[item.SinkName]++
		reason := strings.TrimSpace(item.Reason)
		if reason == "" {
			reason = "unknown"
		}
		byReason[reason]++
	}
	return map[string]any{
		"count":     len(items),
		"by_sink":   bySink,
		"by_reason": byReason,
	}
}

func summarizeIntegrations(items []integration.SubmissionRecord) map[string]any {
	byStatus := map[string]int{}
	bySystem := map[string]int{}
	byFailureCategory := map[string]int{}
	for _, item := range items {
		byStatus[item.Status]++
		bySystem[item.ExternalSystemKey]++
		if category := integrationFailureCategory(item); category != "" {
			byFailureCategory[category]++
		}
	}
	return map[string]any{
		"count":               len(items),
		"by_status":           byStatus,
		"by_system":           bySystem,
		"by_failure_category": byFailureCategory,
	}
}

func decorateHealth(snapshot runtimehealth.Snapshot) map[string]any {
	return map[string]any{
		"snapshot": snapshot,
		"runbooks": runbooksForIDs(snapshot.RunbookIDs),
		"summary": map[string]any{
			"status":             snapshot.Status,
			"ready":              snapshot.Ready,
			"failure_categories": snapshot.FailureCategories,
		},
	}
}

func summarizeOfflineOutcomes(items []offline.SyncResultItem) map[string]any {
	byStatus := map[string]int{}
	byCode := map[string]int{}
	oldestRetryableAge := int64(0)
	now := time.Now().UTC()
	for _, item := range items {
		byStatus[item.Status]++
		if strings.TrimSpace(item.ErrorCode) != "" {
			byCode[item.ErrorCode]++
		}
		if item.Status == offline.StatusFailedRetryable && !item.ProcessedAt.IsZero() {
			age := int64(now.Sub(item.ProcessedAt).Seconds())
			if age > oldestRetryableAge {
				oldestRetryableAge = age
			}
		}
	}
	return map[string]any{
		"count":                        len(items),
		"by_status":                    byStatus,
		"by_error_code":                byCode,
		"oldest_retryable_age_seconds": oldestRetryableAge,
	}
}

func buildTrace(correlationID string, obs *observability.Service, auditSvc *audit.Service, eventingSvc *eventing.Service, workflowSvc *workflow.Service, jobSvc *jobs.Service, integrationSvc *integration.Service, offlineSvc *offline.Service) map[string]any {
	steps := make([]traceStep, 0)
	targetKeys := map[string]struct{}{}
	eventByID := map[string]eventing.Event{}
	for _, item := range auditSvc.Query(audit.Query{CorrelationID: correlationID}) {
		steps = append(steps, traceStep{
			Kind:          "audit",
			Status:        "recorded",
			Label:         item.Action,
			OccurredAt:    item.OccurredAt,
			CorrelationID: item.CorrelationID,
			Data: map[string]any{
				"id":          item.ID,
				"target_type": item.TargetType,
				"target_id":   item.TargetID,
				"actor_id":    item.ActorID,
			},
		})
		if item.TargetType != "" && item.TargetID != "" {
			targetKeys[item.TargetType+":"+item.TargetID] = struct{}{}
		}
	}
	for _, item := range eventingSvc.ListEvents() {
		if strings.TrimSpace(item.CorrelationID) != correlationID {
			continue
		}
		eventByID[item.ID] = item
		steps = append(steps, traceStep{
			Kind:          "domain_event",
			Status:        "recorded",
			Label:         item.Type,
			OccurredAt:    item.OccurredAt,
			CorrelationID: item.CorrelationID,
			Data: map[string]any{
				"id":             item.ID,
				"aggregate_type": item.AggregateType,
				"aggregate_id":   item.AggregateID,
			},
		})
		if item.AggregateType != "" && item.AggregateID != "" {
			targetKeys[item.AggregateType+":"+item.AggregateID] = struct{}{}
		}
	}
	if obs != nil {
		for _, item := range obs.QueryLogRecords("", correlationID) {
			steps = append(steps, traceStep{
				Kind:          "http",
				Status:        item.Severity,
				Label:         item.Key,
				OccurredAt:    item.OccurredAt,
				CorrelationID: item.Correlation,
				Data:          item.Fields,
			})
		}
	}
	if offlineSvc != nil {
		for _, item := range offlineSvc.RecentOutcomes(200) {
			if strings.TrimSpace(item.CorrelationID) != correlationID {
				continue
			}
			steps = append(steps, traceStep{
				Kind:            "offline_sync",
				Status:          item.Status,
				Label:           strings.TrimSpace(item.Kind) + "." + strings.TrimSpace(item.Operation),
				OccurredAt:      item.ProcessedAt,
				CorrelationID:   item.CorrelationID,
				FailureCategory: offlineFailureCategory(item),
				RunbookID:       offlineRunbookID(item),
				Data: map[string]any{
					"batch_id":         item.BatchID,
					"device_id":        item.DeviceID,
					"idempotency_key":  item.IdempotencyKey,
					"target_id":        item.TargetID,
					"error_code":       item.ErrorCode,
					"attempt_count":    item.AttemptCount,
					"resolution_count": len(item.Conflict.ResolutionOptions),
				},
			})
		}
	}
	submissionIDs := map[string]struct{}{}
	if integrationSvc != nil {
		for _, item := range integrationSvc.ListSubmissions() {
			if strings.TrimSpace(item.CorrelationID) != correlationID {
				continue
			}
			submissionIDs[item.ID] = struct{}{}
			steps = append(steps, traceStep{
				Kind:            "integration_submission",
				Status:          item.Status,
				Label:           item.OperationType,
				OccurredAt:      item.UpdatedAt,
				CorrelationID:   item.CorrelationID,
				FailureCategory: integrationFailureCategory(item),
				RunbookID:       "runtime.integrations",
				Data: map[string]any{
					"id":                  item.ID,
					"external_system_key": item.ExternalSystemKey,
					"document_id":         item.DocumentID,
				},
			})
			if item.DocumentID != "" {
				targetKeys["document:"+item.DocumentID] = struct{}{}
			}
		}
	}
	for _, item := range workflowSvc.ListHistory("", "") {
		if metadataCorrelation(item.Metadata) != correlationID && !traceTargetIncluded(targetKeys, item.TargetType, item.TargetID) {
			continue
		}
		steps = append(steps, traceStep{
			Kind:            "workflow_history",
			Status:          item.DecisionCode,
			Label:           item.Action,
			OccurredAt:      item.OccurredAt,
			CorrelationID:   metadataCorrelation(item.Metadata),
			FailureCategory: workflowFailureCategory(item),
			RunbookID:       workflowRunbookID(item),
			Data: map[string]any{
				"id":            item.ID,
				"target_type":   item.TargetType,
				"target_id":     item.TargetID,
				"from_state":    item.FromState,
				"to_state":      item.ToState,
				"decision_code": item.DecisionCode,
			},
		})
	}
	outboxIDs := map[string]struct{}{}
	for _, item := range eventingSvc.ListOutbox() {
		event, ok := eventByID[item.EventID]
		if !ok {
			continue
		}
		outboxIDs[item.ID] = struct{}{}
		steps = append(steps, traceStep{
			Kind:            "outbox",
			Status:          item.Status,
			Label:           item.EventType,
			OccurredAt:      item.CreatedAt,
			CorrelationID:   event.CorrelationID,
			FailureCategory: outboxFailureCategory(item),
			RunbookID:       "runtime.outbox",
			Data: map[string]any{
				"id":       item.ID,
				"event_id": item.EventID,
			},
		})
	}
	for _, item := range eventingSvc.ListDeliveries() {
		if _, ok := outboxIDs[item.OutboxID]; !ok {
			continue
		}
		event := eventByID[item.EventID]
		steps = append(steps, traceStep{
			Kind:            "delivery",
			Status:          item.Status,
			Label:           item.SinkName,
			OccurredAt:      firstTime(item.DispatchedAt, item.CreatedAt),
			CorrelationID:   event.CorrelationID,
			FailureCategory: deliveryFailureCategory(item),
			RunbookID:       "runtime.outbox",
			Data: map[string]any{
				"id":        item.ID,
				"outbox_id": item.OutboxID,
				"event_id":  item.EventID,
			},
		})
	}
	for _, item := range eventingSvc.ListDeadLetters() {
		event := eventByID[item.EventID]
		if event.ID == "" {
			continue
		}
		steps = append(steps, traceStep{
			Kind:            "dead_letter",
			Status:          "dead_letter",
			Label:           item.SinkName,
			OccurredAt:      item.CreatedAt,
			CorrelationID:   event.CorrelationID,
			FailureCategory: "dead_lettered",
			RunbookID:       "runtime.outbox",
			Data: map[string]any{
				"id":       item.ID,
				"reason":   item.Reason,
				"event_id": item.EventID,
			},
		})
	}
	if jobSvc != nil {
		for _, item := range jobSvc.List() {
			if !jobMatchesCorrelation(item, correlationID, submissionIDs) {
				continue
			}
			steps = append(steps, traceStep{
				Kind:            "job",
				Status:          item.Status,
				Label:           item.Name,
				OccurredAt:      firstTime(item.EndedAt, firstTime(item.StartedAt, item.CreatedAt)),
				CorrelationID:   jobCorrelationID(item),
				FailureCategory: jobFailureCategory(item),
				RunbookID:       jobRunbookID(item),
				Data: map[string]any{
					"id":            item.ID,
					"attempt_count": item.AttemptCount,
				},
			})
		}
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].OccurredAt.Before(steps[j].OccurredAt) })
	runbookIDs := make([]string, 0)
	seenRunbooks := map[string]struct{}{}
	for _, item := range steps {
		if item.RunbookID == "" {
			continue
		}
		if _, ok := seenRunbooks[item.RunbookID]; ok {
			continue
		}
		seenRunbooks[item.RunbookID] = struct{}{}
		runbookIDs = append(runbookIDs, item.RunbookID)
	}
	return map[string]any{
		"correlation_id": correlationID,
		"summary": map[string]any{
			"step_count": len(steps),
		},
		"runbooks": runbooksForIDs(runbookIDs),
		"items":    steps,
	}
}

func offlineFailureCategory(item offline.SyncResultItem) string {
	switch item.Status {
	case offline.StatusConflict:
		return "version_conflict"
	case offline.StatusForbidden:
		return "forbidden"
	case offline.StatusFailedTerminal:
		if strings.TrimSpace(item.ErrorCode) != "" {
			return strings.TrimSpace(item.ErrorCode)
		}
		return "validation_error"
	case offline.StatusFailedRetryable:
		return "dispatch_failure"
	default:
		return ""
	}
}

func offlineRunbookID(item offline.SyncResultItem) string {
	switch item.Status {
	case offline.StatusConflict:
		return "runtime.projections"
	case offline.StatusFailedRetryable, offline.StatusFailedTerminal, offline.StatusForbidden:
		return "runtime.dependencies"
	default:
		return ""
	}
}

func metadataCorrelation(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(stringifyAny(metadata["correlation_id"]))
}

func traceTargetIncluded(targets map[string]struct{}, targetType, targetID string) bool {
	if targetType == "" || targetID == "" {
		return false
	}
	_, ok := targets[targetType+":"+targetID]
	return ok
}

func jobCorrelationID(item jobs.Job) string {
	if correlation := strings.TrimSpace(stringifyAny(item.Payload["correlation_id"])); correlation != "" {
		return correlation
	}
	return strings.TrimSpace(stringifyAny(item.Result["correlation_id"]))
}

func jobMatchesCorrelation(item jobs.Job, correlationID string, submissionIDs map[string]struct{}) bool {
	if jobCorrelationID(item) == correlationID {
		return true
	}
	submissionID := strings.TrimSpace(stringifyAny(item.Payload["submission_id"]))
	if submissionID != "" {
		_, ok := submissionIDs[submissionID]
		return ok
	}
	return false
}

func stringifyAny(value any) string {
	switch current := value.(type) {
	case string:
		return current
	case int:
		return strconv.Itoa(current)
	case int64:
		return strconv.FormatInt(current, 10)
	case float64:
		return strconv.FormatFloat(current, 'f', -1, 64)
	default:
		return ""
	}
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func deliveryFailureCategory(item eventing.OutboxDeliveryRecord) string {
	if item.Status == "dead_letter" {
		return "dead_lettered"
	}
	if strings.TrimSpace(item.LastError) != "" {
		return "dispatch_failure"
	}
	return ""
}

func outboxFailureCategory(item eventing.OutboxRecord) string {
	if item.Status == "dead_letter" {
		return "dead_lettered"
	}
	if strings.TrimSpace(item.LastError) != "" {
		return "dispatch_failure"
	}
	return ""
}

func workflowFailureCategory(item workflow.HistoryEvent) string {
	if strings.Contains(strings.ToLower(item.DecisionReason), "policy") {
		return "workflow_runtime_invalid"
	}
	return ""
}

func workflowRunbookID(item workflow.HistoryEvent) string {
	if workflowFailureCategory(item) == "" {
		return ""
	}
	return "runtime.workflow"
}

func integrationFailureCategory(item integration.SubmissionRecord) string {
	if strings.TrimSpace(item.LastError) == "" {
		return ""
	}
	if item.Status == "dead_letter" {
		return "dead_lettered"
	}
	return "dispatch_failure"
}

func jobFailureCategory(item jobs.Job) string {
	switch item.Status {
	case jobs.StatusDeadLetter:
		return "dead_lettered"
	case jobs.StatusFailed:
		return "handler_failure"
	}
	if strings.TrimSpace(item.Error) != "" {
		return "handler_failure"
	}
	return ""
}

func jobRunbookID(item jobs.Job) string {
	if jobFailureCategory(item) == "" {
		return ""
	}
	return "runtime.jobs"
}

func projectionHealthSummary(searchSvc *search.Service) map[string]any {
	items := searchSvc.IndexRuntimes()
	unhealthy := 0
	maxLag := int64(0)
	for _, item := range items {
		if item.ConsistencyStatus != "" && item.ConsistencyStatus != "ok" && item.ConsistencyStatus != "unknown" {
			unhealthy++
		}
		if item.LastLagSeconds > maxLag {
			maxLag = item.LastLagSeconds
		}
	}
	return map[string]any{
		"count":                    len(items),
		"unhealthy_count":          unhealthy,
		"max_lag_seconds":          maxLag,
		"projection_runtime_items": items,
	}
}
