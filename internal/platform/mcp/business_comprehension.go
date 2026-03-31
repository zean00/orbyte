package mcp

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func (s *Server) businessTopologyMap(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.businessModuleInfos(actor, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	nodes := make([]map[string]any, 0, len(items))
	edges := make([]map[string]any, 0)
	for _, item := range items {
		nodes = append(nodes, map[string]any{
			"module_key":            item.Key,
			"name":                  item.Name,
			"description":           item.Description,
			"domain_family":         item.DomainFamily,
			"category":              item.Category,
			"role":                  item.Role,
			"enabled":               item.Enabled,
			"lifecycle_state":       item.LifecycleState,
			"business_capabilities": append([]string(nil), item.BusinessCapabilities...),
			"owned_document_types":  append([]string(nil), item.OwnedDocumentTypes...),
			"owned_model_keys":      append([]string(nil), item.OwnedModelKeys...),
			"dependents":            append([]string(nil), item.Dependents...),
			"status":                topologyNodeStatus(item),
		})
		for _, dep := range item.Dependencies {
			edges = append(edges, map[string]any{
				"source_module_key": item.Key,
				"target_module_key": dep.ModuleKey,
				"kind":              dep.Kind,
				"version_range":     dep.VersionRange,
				"optional":          dep.Kind == module.DependencyKindOptional,
				"status":            topologyEdgeStatus(dep),
			})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		left := anyString(edges[i]["source_module_key"]) + "->" + anyString(edges[i]["target_module_key"])
		right := anyString(edges[j]["source_module_key"]) + "->" + anyString(edges[j]["target_module_key"])
		return left < right
	})
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Loaded topology for %d enabled business modules.", len(nodes)),
		}},
		"structuredContent": map[string]any{
			"nodes": nodes,
			"edges": edges,
			"summary": map[string]any{
				"module_count":     len(nodes),
				"dependency_count": len(edges),
				"domain_families": uniqueStringCount(mapBusinessString(items, func(item businessModuleInfo) string {
					return item.DomainFamily
				})),
			},
		},
	}, true, nil
}

func (s *Server) businessTimelineGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	resourceKind := strings.TrimSpace(stringArg(arguments, "resource_kind"))
	switch resourceKind {
	case "document":
		if !allowsAll(actor.PermissionChecker, []string{"document.read"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		if s.documents == nil {
			return nil, false, nil
		}
		documentID := strings.TrimSpace(stringArg(arguments, "document_id"))
		if documentID == "" {
			return nil, true, shared.Validation("document_id is required")
		}
		record, err := s.documents.Get(documentID)
		if err != nil {
			return nil, true, err
		}
		documentEvents := s.auditEventsFor("document", record.Header.ID)
		history := []workflow.HistoryEvent(nil)
		if s.workflows != nil {
			history = s.workflows.ListHistory("document", record.Header.ID)
		}
		return map[string]any{
			"content": []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Loaded timeline for business document %s.", record.Header.ID),
			}},
			"structuredContent": map[string]any{
				"resource_kind":    "document",
				"record":           s.documentSummary(s.sanitizeDocumentRecord(actor, record), false),
				"audit_events":     documentEvents,
				"workflow_history": history,
			},
		}, true, nil
	case "model":
		if s.models == nil {
			return nil, false, nil
		}
		modelKey := strings.TrimSpace(stringArg(arguments, "model_key"))
		recordID := strings.TrimSpace(stringArg(arguments, "record_id"))
		if modelKey == "" || recordID == "" {
			return nil, true, shared.Validation("model_key and record_id are required")
		}
		def, ok := s.models.Definition(modelKey)
		if !ok {
			return nil, true, shared.NotFound("model definition not found")
		}
		if !allowsAll(actor.PermissionChecker, []string{def.ReadPermissionKey}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		record, err := s.models.Get(modelKey, recordID)
		if err != nil {
			return nil, true, err
		}
		sanitized := s.sanitizeModelRecord(actor, def, record)
		return map[string]any{
			"content": []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Loaded timeline for business model record %s.", recordID),
			}},
			"structuredContent": map[string]any{
				"resource_kind": "model",
				"record":        s.modelSummary(def, sanitized),
				"audit_events":  s.auditEventsFor("model:"+modelKey, recordID),
			},
		}, true, nil
	default:
		return nil, true, shared.Validation("resource_kind must be document or model")
	}
}

func (s *Server) businessRelationshipsGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	recordPayload, ok, err := s.businessRecordGet(actor, arguments)
	if err != nil || !ok {
		return recordPayload, ok, err
	}
	relatedPayload, ok, err := s.businessRecordRelated(actor, arguments)
	if err != nil || !ok {
		return relatedPayload, ok, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: "Loaded business relationship context.",
		}},
		"structuredContent": map[string]any{
			"record":        recordPayload["structuredContent"],
			"relationships": relatedPayload["structuredContent"],
		},
	}, true, nil
}

func (s *Server) businessHealthSummary(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	moduleItems := s.businessModuleInfos(actor, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	summary := map[string]any{
		"modules": map[string]any{
			"enabled_count": len(moduleItems),
			"warning_count": countBusinessModules(moduleItems, func(item businessModuleInfo) bool {
				return strings.TrimSpace(item.LifecycleState) != "" && strings.TrimSpace(item.LifecycleState) != "stable"
			}),
			"without_summary":    countBusinessModules(moduleItems, func(item businessModuleInfo) bool { return strings.TrimSpace(item.Description) == "" }),
			"without_capability": countBusinessModules(moduleItems, func(item businessModuleInfo) bool { return len(item.BusinessCapabilities) == 0 }),
		},
	}
	if s.documents != nil && allowsAll(actor.PermissionChecker, []string{"document.list"}) {
		docCounts := map[string]int{}
		for _, item := range s.documents.List() {
			docCounts[firstNonEmpty(strings.TrimSpace(item.Header.Status), "unknown")]++
		}
		summary["documents"] = map[string]any{
			"count":     len(s.documents.List()),
			"by_status": docCounts,
		}
	}
	if s.models != nil && allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		modelCount := 0
		for _, def := range s.models.Definitions() {
			if allowsAll(actor.PermissionChecker, []string{def.ListPermissionKey}) {
				modelCount++
			}
		}
		summary["models"] = map[string]any{"visible_definition_count": modelCount}
	}
	if s.workflows != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		openTasks := 0
		for _, item := range s.workflows.ListTasks() {
			if strings.EqualFold(strings.TrimSpace(item.Status), "open") {
				openTasks++
			}
		}
		pendingApprovals := 0
		for _, item := range s.workflows.ListApprovals() {
			if strings.EqualFold(strings.TrimSpace(item.Status), "pending") {
				pendingApprovals++
			}
		}
		summary["workflow"] = map[string]any{
			"open_tasks":        openTasks,
			"pending_approvals": pendingApprovals,
		}
	}
	if s.audit != nil && allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		events := s.audit.List()
		summary["audit"] = map[string]any{
			"event_count": len(events),
		}
	}
	if s.analytics != nil && allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		snapshot, err := s.analyticsSnapshotPayload(actor)
		if err == nil {
			summary["analytics"] = map[string]any{
				"latest_snapshot_id": snapshot.ID,
				"generated_at":       snapshot.GeneratedAt,
				"documents":          snapshot.Documents,
				"workflow":           snapshot.Workflow,
				"coverage":           snapshot.Coverage,
			}
		}
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: "Loaded cross-domain business health summary.",
		}},
		"structuredContent": summary,
	}, true, nil
}

func (s *Server) businessAnalyticsKPISummary(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	_ = arguments
	if s == nil || s.analytics == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	snapshot, err := s.analyticsSnapshotPayload(actor)
	if err != nil {
		return nil, true, err
	}
	recent := s.analytics.ListRecent(5)
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: "Loaded analytics KPI summary for current business state.",
		}},
		"structuredContent": map[string]any{
			"current_snapshot": snapshot,
			"recent_snapshots": recent,
		},
	}, true, nil
}

func (s *Server) businessExceptionSearch(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	items := make([]map[string]any, 0)
	if s.workflows != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		for _, task := range s.workflows.ListTasks() {
			if !strings.EqualFold(strings.TrimSpace(task.Status), "open") {
				continue
			}
			items = append(items, map[string]any{
				"kind":         "workflow_task",
				"status":       task.Status,
				"target_type":  task.TargetType,
				"target_id":    task.TargetID,
				"workflow_key": task.WorkflowKey,
				"title":        firstNonEmpty(task.TaskType, task.WorkflowKey, task.ID),
				"created_at":   task.CreatedAt,
				"due_at":       task.DueAt,
			})
		}
		for _, approval := range s.workflows.ListApprovals() {
			if !strings.EqualFold(strings.TrimSpace(approval.Status), "pending") {
				continue
			}
			items = append(items, map[string]any{
				"kind":         "workflow_approval",
				"status":       approval.Status,
				"target_type":  approval.TargetType,
				"target_id":    approval.TargetID,
				"workflow_key": approval.WorkflowKey,
				"title":        firstNonEmpty(approval.StageKey, approval.WorkflowKey, approval.ID),
				"created_at":   approval.RequestedAt,
			})
		}
	}
	if s.documents != nil && allowsAll(actor.PermissionChecker, []string{"document.list"}) {
		statusFilter := strings.ToLower(strings.TrimSpace(stringArg(arguments, "status")))
		for _, record := range s.documents.List() {
			status := strings.ToLower(strings.TrimSpace(record.Header.Status))
			if status == "approved" || status == "draft" {
				continue
			}
			if statusFilter != "" && status != statusFilter {
				continue
			}
			items = append(items, map[string]any{
				"kind":            "document_status",
				"status":          record.Header.Status,
				"target_type":     record.Header.Type,
				"target_id":       record.Header.ID,
				"title":           firstNonEmpty(record.Header.Number, stringValue(record.Body.Payload["name"]), record.Header.ID),
				"organization_id": record.Header.OrganizationID,
				"location_id":     record.Header.LocationID,
				"updated_at":      record.Header.UpdatedAt,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		left := anyTime(items[i]["created_at"], items[i]["updated_at"])
		right := anyTime(items[j]["created_at"], items[j]["updated_at"])
		return left.After(right)
	})
	items = paginateMaps(items, intArg(arguments, "page"), intArg(arguments, "page_size"))
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Found %d open business exceptions.", len(items)),
		}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) pricingPromotionAdvisorReview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.advisorReview(actor, arguments, advisorPackDefinition{
		Key:          "pricing.promotion.advisor",
		Title:        "Pricing and Promotion Advisor",
		Domain:       "pricing",
		Keywords:     []string{"pricing", "promotion", "discount", "price"},
		FollowUps:    []string{"business.capability.search", "business.document.search", "business.record.search", "business.document.draft.create"},
		DraftTargets: []string{"promotion_plan"},
	})
}

func (s *Server) taxStructureAdvisorReview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.advisorReview(actor, arguments, advisorPackDefinition{
		Key:       "tax.structure.advisor",
		Title:     "Tax Structure Advisor",
		Domain:    "tax",
		Keywords:  []string{"tax", "jurisdiction", "exemption", "registration"},
		FollowUps: []string{"business.capability.search", "business.record.search", "business.document.search", "business.document.draft.create"},
	})
}

func (s *Server) treasuryReconciliationAdvisorReview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.advisorReview(actor, arguments, advisorPackDefinition{
		Key:       "treasury.reconciliation.advisor",
		Title:     "Treasury and Reconciliation Advisor",
		Domain:    "treasury",
		Keywords:  []string{"treasury", "reconciliation", "bank", "settlement", "cash"},
		FollowUps: []string{"business.health.summary", "business.exception.search", "business.analytics.kpi.summary", "business.record.search"},
	})
}

func (s *Server) inventoryHealthAdvisorReview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.advisorReview(actor, arguments, advisorPackDefinition{
		Key:       "inventory.health.advisor",
		Title:     "Inventory Health Advisor",
		Domain:    "inventory",
		Keywords:  []string{"inventory", "warehouse", "stock", "production", "valuation"},
		FollowUps: []string{"business.health.summary", "business.record.search", "business.analytics.kpi.summary"},
	})
}

func (s *Server) partyMasterAdvisorReview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	return s.advisorReview(actor, arguments, advisorPackDefinition{
		Key:       "party.master.advisor",
		Title:     "Party Master Advisor",
		Domain:    "party",
		Keywords:  []string{"party", "customer", "vendor", "contact", "address"},
		FollowUps: []string{"business.capability.search", "business.record.search", "business.health.summary", "business.document.draft.create"},
	})
}

type advisorPackDefinition struct {
	Key          string
	Title        string
	Domain       string
	Keywords     []string
	FollowUps    []string
	DraftTargets []string
}

func (s *Server) advisorReview(actor ActorContext, arguments map[string]any, def advisorPackDefinition) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	modules := s.businessModuleInfos(actor, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	matched := make([]businessModuleInfo, 0)
	for _, item := range modules {
		if moduleMatchesKeywords(item, def.Keywords) {
			matched = append(matched, item)
		}
	}
	recommendations := advisorRecommendations(def, matched)
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Prepared a %s review across %d relevant modules.", def.Title, len(matched)),
		}},
		"structuredContent": map[string]any{
			"advisor_key":       def.Key,
			"title":             def.Title,
			"business_domain":   def.Domain,
			"relevant_modules":  matched,
			"recommended_tools": append([]string(nil), def.FollowUps...),
			"draft_targets":     append([]string(nil), def.DraftTargets...),
			"recommendations":   recommendations,
		},
	}, true, nil
}

func topologyNodeStatus(item businessModuleInfo) string {
	switch {
	case !item.Enabled:
		return "disabled"
	case strings.TrimSpace(item.LifecycleState) != "" && strings.TrimSpace(item.LifecycleState) != "stable":
		return "warning"
	default:
		return "healthy"
	}
}

func topologyEdgeStatus(dep module.DependencyRequirement) string {
	if dep.Kind == module.DependencyKindOptional {
		return "optional"
	}
	return "ok"
}

func mapBusinessString(items []businessModuleInfo, fn func(businessModuleInfo) string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fn(item))
	}
	return out
}

func uniqueStringCount(items []string) int {
	seen := map[string]struct{}{}
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		seen[strings.TrimSpace(item)] = struct{}{}
	}
	return len(seen)
}

func countBusinessModules(items []businessModuleInfo, fn func(businessModuleInfo) bool) int {
	count := 0
	for _, item := range items {
		if fn(item) {
			count++
		}
	}
	return count
}

func (s *Server) auditEventsFor(targetType, targetID string) []audit.Event {
	if s == nil || s.audit == nil {
		return nil
	}
	return s.audit.Query(audit.Query{TargetType: strings.TrimSpace(targetType), TargetID: strings.TrimSpace(targetID)})
}

func moduleMatchesKeywords(item businessModuleInfo, keywords []string) bool {
	for _, keyword := range keywords {
		if businessModuleMatches(item, keyword) {
			return true
		}
	}
	return false
}

func advisorRecommendations(def advisorPackDefinition, modules []businessModuleInfo) []string {
	recommendations := []string{}
	if len(modules) == 0 {
		recommendations = append(recommendations, "No strongly matched modules were found. Start with business.module.list and business.capability.search to confirm capability coverage.")
	} else {
		recommendations = append(recommendations, fmt.Sprintf("Review %d relevant modules and compare their business capabilities, owned documents, and shared dependencies.", len(modules)))
	}
	recommendations = append(recommendations, "Inspect current transaction and configuration state before proposing changes.")
	if len(def.DraftTargets) > 0 {
		recommendations = append(recommendations, "Prepare only draft artifacts first and require explicit confirmation before creation.")
	}
	recommendations = append(recommendations, "Use business.health.summary and business.exception.search to identify gaps, backlogs, and unresolved workflow pressure.")
	return recommendations
}

func paginateMaps(items []map[string]any, page, pageSize int) []map[string]any {
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []map[string]any{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func anyTime(values ...any) time.Time {
	for _, value := range values {
		switch typed := value.(type) {
		case time.Time:
			if !typed.IsZero() {
				return typed
			}
		}
	}
	return time.Time{}
}
