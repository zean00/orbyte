package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
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

func (s *Server) businessAnalyticsOverview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	domain := strings.TrimSpace(stringArg(arguments, "domain"))
	payload, err := s.analyticsOverviewStructured(actor, arguments, domain)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: "Loaded a cross-domain analytical business overview.",
		}},
		"structuredContent": payload,
	}, true, nil
}

func (s *Server) businessAnalyticsDomainSummary(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	domain := strings.TrimSpace(stringArg(arguments, "domain"))
	if domain == "" {
		return nil, true, shared.Validation("domain is required")
	}
	payload, err := s.analyticsOverviewStructured(actor, arguments, domain)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Loaded an analytical summary for the %s domain.", domain),
		}},
		"structuredContent": payload,
	}, true, nil
}

func (s *Server) businessAnalyticsTrend(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.analytics == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	query := analyticsQueryFromArguments(arguments)
	snapshots := s.analytics.QuerySnapshots(query)
	bucket := firstNonEmpty(strings.TrimSpace(stringArg(arguments, "bucket")), "day")
	series := buildTrendSeries(snapshots, bucket)
	coverage := analyticsCoverage(arguments, "")
	coverage["included_domains"] = []string{"analytics", "operations"}
	coverage["missing_domains"] = []string{}
	coverage["permission_filtered"] = []string{}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Loaded %d analytical trend points.", len(series)),
		}},
		"structuredContent": map[string]any{
			"summary": []map[string]any{{
				"key":    "trend_points",
				"label":  "Trend Points",
				"value":  len(series),
				"domain": "analytics",
				"status": "info",
			}},
			"segments":   []map[string]any{},
			"anomalies":  []map[string]any{},
			"exceptions": []map[string]any{},
			"drilldowns": []map[string]any{
				analyticsDrilldownItem(
					"trend_recent_exceptions",
					"Recent open exceptions",
					"business.exception.search",
					map[string]any{"page_size": 25},
				),
			},
			"scope":    analyticsScope(arguments, ""),
			"sources":  []string{"analytics.snapshot"},
			"coverage": coverage,
			"series":   series,
		},
	}, true, nil
}

func (s *Server) businessAnalyticsAnomalySearch(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	payload, err := s.analyticsOverviewStructured(actor, arguments, strings.TrimSpace(stringArg(arguments, "domain")))
	if err != nil {
		return nil, true, err
	}
	anomalies, _ := payload["anomalies"].([]map[string]any)
	items := paginateMaps(anomalies, intArg(arguments, "page"), intArg(arguments, "page_size"))
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Found %d analytical anomaly signals.", len(items)),
		}},
		"structuredContent": map[string]any{
			"items":      items,
			"scope":      payload["scope"],
			"sources":    payload["sources"],
			"coverage":   payload["coverage"],
			"drilldowns": payload["drilldowns"],
		},
	}, true, nil
}

func (s *Server) businessAnalyticsExceptionCluster(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.collectBusinessExceptionItems(actor, arguments)
	groupBy := firstNonEmpty(strings.TrimSpace(stringArg(arguments, "group_by")), "kind")
	clusters := clusterBusinessExceptions(items, groupBy)
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Clustered %d open business exceptions by %s.", len(items), groupBy),
		}},
		"structuredContent": map[string]any{
			"items":   clusters,
			"scope":   analyticsScope(arguments, ""),
			"sources": []string{"workflow", "documents"},
			"coverage": map[string]any{
				"requested_domain":    "",
				"included_domains":    []string{"workflow", "operations"},
				"missing_domains":     []string{},
				"permission_filtered": []string{},
			},
		},
	}, true, nil
}

func (s *Server) businessAnalyticsDrilldown(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"module.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	handle := strings.TrimSpace(stringArg(arguments, "handle"))
	if handle == "" {
		return nil, true, shared.Validation("handle is required")
	}
	resolved, err := decodeAnalyticsDrilldown(handle)
	if err != nil {
		return nil, true, shared.Validation("handle is invalid")
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Resolved analytical drilldown to %s.", firstNonEmpty(anyString(resolved["target_tool"]), "unknown")),
		}},
		"structuredContent": resolved,
	}, true, nil
}

func (s *Server) businessExceptionSearch(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	items := s.collectBusinessExceptionItems(actor, arguments)
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

func (s *Server) analyticsOverviewStructured(actor ActorContext, arguments map[string]any, forcedDomain string) (map[string]any, error) {
	domain := firstNonEmpty(strings.TrimSpace(forcedDomain), strings.TrimSpace(stringArg(arguments, "domain")))
	scope := analyticsScope(arguments, domain)
	sources := []string{}
	includedDomains := []string{}
	missingDomains := []string{}
	permissionFiltered := []string{}
	summary := make([]map[string]any, 0)
	segments := make([]map[string]any, 0)

	moduleItems := s.businessModuleInfos(actor, stringArg(arguments, "organization_id"), stringArg(arguments, "location_id"))
	summary = append(summary,
		analyticsSummaryCard("enabled_modules", "Enabled Modules", len(moduleItems), "cross-domain", "healthy"),
		analyticsSummaryCard("module_warnings", "Module Warnings", countBusinessModules(moduleItems, func(item businessModuleInfo) bool {
			return strings.TrimSpace(item.LifecycleState) != "" && strings.TrimSpace(item.LifecycleState) != "stable"
		}), "cross-domain", "warning"),
	)
	sources = append(sources, "module_service")
	includedDomains = append(includedDomains, "cross-domain")

	if s.documents != nil && allowsAll(actor.PermissionChecker, []string{"document.list"}) && analyticsDomainIncludes(domain, "operations", "cross-domain", "finance") {
		drafts, submitted, rejected := 0, 0, 0
		for _, record := range s.documents.List() {
			switch strings.ToLower(strings.TrimSpace(record.Header.Status)) {
			case "draft":
				drafts++
			case "submitted":
				submitted++
			case "rejected":
				rejected++
			}
		}
		summary = append(summary,
			analyticsSummaryCard("draft_documents", "Draft Documents", drafts, "operations", summaryStatusFromCount(drafts, 0, 10)),
			analyticsSummaryCard("submitted_documents", "Submitted Documents", submitted, "operations", summaryStatusFromCount(submitted, 0, 5)),
			analyticsSummaryCard("rejected_documents", "Rejected Documents", rejected, "operations", summaryStatusFromCount(rejected, 0, 0)),
		)
		sources = append(sources, "document_service")
		includedDomains = append(includedDomains, "operations")
	} else if analyticsDomainIncludes(domain, "operations", "cross-domain", "finance") {
		permissionFiltered = append(permissionFiltered, "documents")
	}

	if s.workflows != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) && analyticsDomainIncludes(domain, "workflow", "operations", "cross-domain") {
		openTasks, pendingApprovals := 0, 0
		for _, item := range s.workflows.ListTasks() {
			if strings.EqualFold(strings.TrimSpace(item.Status), "open") {
				openTasks++
			}
		}
		for _, item := range s.workflows.ListApprovals() {
			if strings.EqualFold(strings.TrimSpace(item.Status), "pending") {
				pendingApprovals++
			}
		}
		summary = append(summary,
			analyticsSummaryCard("open_tasks", "Open Tasks", openTasks, "workflow", summaryStatusFromCount(openTasks, 0, 10)),
			analyticsSummaryCard("pending_approvals", "Pending Approvals", pendingApprovals, "workflow", summaryStatusFromCount(pendingApprovals, 0, 5)),
		)
		sources = append(sources, "workflow_service")
		includedDomains = append(includedDomains, "workflow")
	} else if analyticsDomainIncludes(domain, "workflow", "operations", "cross-domain") {
		permissionFiltered = append(permissionFiltered, "workflow")
	}

	if s.audit != nil && allowsAll(actor.PermissionChecker, []string{"module.read"}) && analyticsDomainIncludes(domain, "cross-domain", "operations") {
		summary = append(summary,
			analyticsSummaryCard("audit_events", "Audit Events", len(s.audit.List()), "cross-domain", "info"),
		)
		sources = append(sources, "audit_service")
	}

	if s.analytics != nil && allowsAll(actor.PermissionChecker, []string{"analytics.read"}) && analyticsDomainIncludes(domain, "analytics", "cross-domain", "operations") {
		snapshot, err := s.analyticsSnapshotPayload(actor)
		if err == nil {
			compare, _ := s.analytics.Compare(analyticsQueryFromArguments(arguments))
			summary = append(summary,
				analyticsSummaryCardWithDelta("approved_documents_latest", "Approved Documents", snapshot.Documents.Approved, compare.Delta.ApprovedDocuments, "analytics", summaryStatusFromCount(snapshot.Documents.Approved, 0, 0)),
				analyticsSummaryCardWithDelta("pending_approvals_latest", "Pending Approvals", snapshot.Workflow.PendingApprovals, compare.Delta.PendingApprovals, "analytics", summaryStatusFromCount(snapshot.Workflow.PendingApprovals, 0, 0)),
				analyticsSummaryCardWithDelta("dead_letters_latest", "Dead Letters", snapshot.Reliability.OutboxDeadLetters, compare.Delta.DeadLetters, "analytics", summaryStatusFromCount(snapshot.Reliability.OutboxDeadLetters, 0, 0)),
			)
			segments = append(segments,
				analyticsSegment("document_type", snapshot.Segments.ByDocumentType),
				analyticsSegment("location", snapshot.Segments.ByLocation),
			)
			sources = append(sources, "analytics.snapshot")
			includedDomains = append(includedDomains, "analytics")
		} else {
			missingDomains = append(missingDomains, "analytics")
		}
	} else if analyticsDomainIncludes(domain, "analytics", "cross-domain", "operations") {
		permissionFiltered = append(permissionFiltered, "analytics")
	}

	anomalies := analyticsAnomaliesFromSummary(summary, moduleItems)
	exceptionClusters := clusterBusinessExceptions(s.collectBusinessExceptionItems(actor, arguments), "kind")
	drilldowns := analyticsOverviewDrilldowns(summary, anomalies)

	return map[string]any{
		"summary":    summary,
		"segments":   segments,
		"anomalies":  anomalies,
		"exceptions": exceptionClusters,
		"drilldowns": drilldowns,
		"scope":      scope,
		"sources":    uniqueNonEmptyStrings(sources),
		"coverage": map[string]any{
			"requested_domain":    domain,
			"included_domains":    uniqueNonEmptyStrings(includedDomains),
			"missing_domains":     uniqueNonEmptyStrings(missingDomains),
			"permission_filtered": uniqueNonEmptyStrings(permissionFiltered),
		},
	}, nil
}

func (s *Server) collectBusinessExceptionItems(actor ActorContext, arguments map[string]any) []map[string]any {
	items := make([]map[string]any, 0)
	statusFilter := strings.ToLower(strings.TrimSpace(stringArg(arguments, "status")))
	statusFilters := uniqueNonEmptyStrings(stringSliceArg(arguments, "statuses"))
	if s.workflows != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		for _, task := range s.workflows.ListTasks() {
			status := strings.ToLower(strings.TrimSpace(task.Status))
			if status != "open" {
				continue
			}
			if statusFilter != "" && status != statusFilter {
				continue
			}
			if len(statusFilters) > 0 && !containsString(statusFilters, status) {
				continue
			}
			items = append(items, map[string]any{
				"kind":         "workflow_task",
				"domain":       "workflow",
				"severity":     severityFromAge(task.CreatedAt),
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
			status := strings.ToLower(strings.TrimSpace(approval.Status))
			if status != "pending" {
				continue
			}
			if statusFilter != "" && status != statusFilter {
				continue
			}
			if len(statusFilters) > 0 && !containsString(statusFilters, status) {
				continue
			}
			items = append(items, map[string]any{
				"kind":         "workflow_approval",
				"domain":       "workflow",
				"severity":     severityFromAge(approval.RequestedAt),
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
		for _, record := range s.documents.List() {
			status := strings.ToLower(strings.TrimSpace(record.Header.Status))
			if status == "approved" || status == "draft" {
				continue
			}
			if statusFilter != "" && status != statusFilter {
				continue
			}
			if len(statusFilters) > 0 && !containsString(statusFilters, status) {
				continue
			}
			items = append(items, map[string]any{
				"kind":            "document_status",
				"domain":          "operations",
				"severity":        summaryStatusFromCount(1, 0, 0),
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
	return items
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

func analyticsScope(arguments map[string]any, domain string) map[string]any {
	return map[string]any{
		"organization_id":   strings.TrimSpace(stringArg(arguments, "organization_id")),
		"location_id":       strings.TrimSpace(stringArg(arguments, "location_id")),
		"operating_unit_id": strings.TrimSpace(stringArg(arguments, "operating_unit_id")),
		"date_from":         strings.TrimSpace(stringArg(arguments, "date_from")),
		"date_to":           strings.TrimSpace(stringArg(arguments, "date_to")),
		"domain":            strings.TrimSpace(domain),
	}
}

func analyticsQueryFromArguments(arguments map[string]any) analytics.Query {
	query := analytics.Query{Limit: intArg(arguments, "limit")}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if from := parseDateArg(arguments, "date_from"); !from.IsZero() {
		query.From = from
	}
	if to := parseDateArg(arguments, "date_to"); !to.IsZero() {
		query.To = to
	}
	if query.From.IsZero() && query.To.IsZero() {
		query.Window = "current_state"
	}
	return query
}

func parseDateArg(arguments map[string]any, key string) time.Time {
	raw := strings.TrimSpace(stringArg(arguments, key))
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func analyticsSummaryCard(key, label string, value int, domain, status string) map[string]any {
	return map[string]any{
		"key":    key,
		"label":  label,
		"value":  value,
		"domain": domain,
		"status": status,
	}
}

func analyticsSummaryCardWithDelta(key, label string, value, delta int, domain, status string) map[string]any {
	item := analyticsSummaryCard(key, label, value, domain, status)
	item["delta"] = delta
	return item
}

func analyticsSegment(groupBy string, source map[string]analytics.DocumentKPI) map[string]any {
	items := make([]map[string]any, 0, len(source))
	for key, value := range source {
		items = append(items, map[string]any{
			"key":       key,
			"label":     key,
			"submitted": value.Submitted,
			"approved":  value.Approved,
			"rejected":  value.Rejected,
			"draft":     value.Draft,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		left := anyInt(items[i]["submitted"]) + anyInt(items[i]["approved"]) + anyInt(items[i]["rejected"])
		right := anyInt(items[j]["submitted"]) + anyInt(items[j]["approved"]) + anyInt(items[j]["rejected"])
		if left == right {
			return anyString(items[i]["label"]) < anyString(items[j]["label"])
		}
		return left > right
	})
	if len(items) > 5 {
		items = items[:5]
	}
	return map[string]any{
		"group_by": groupBy,
		"items":    items,
	}
}

func analyticsAnomaliesFromSummary(summary []map[string]any, modules []businessModuleInfo) []map[string]any {
	items := make([]map[string]any, 0)
	for _, card := range summary {
		value := anyInt(card["value"])
		key := anyString(card["key"])
		status := anyString(card["status"])
		if key == "enabled_modules" || key == "audit_events" {
			continue
		}
		if value <= 0 && status != "warning" && status != "critical" {
			continue
		}
		if status == "healthy" || status == "info" {
			continue
		}
		items = append(items, map[string]any{
			"key":       key,
			"title":     anyString(card["label"]),
			"severity":  normalizeSeverity(status),
			"domain":    anyString(card["domain"]),
			"value":     value,
			"reason":    analyticsAnomalyReason(key, value),
			"drilldown": analyticsDrilldownItem(key, anyString(card["label"]), analyticsDrilldownTargetForKey(key), analyticsDrilldownArgsForKey(key)),
		})
	}
	if withoutSummary := countBusinessModules(modules, func(item businessModuleInfo) bool { return strings.TrimSpace(item.Description) == "" }); withoutSummary > 0 {
		items = append(items, map[string]any{
			"key":      "module_descriptions_missing",
			"title":    "Modules Missing Summary",
			"severity": summaryStatusFromCount(withoutSummary, 0, 0),
			"domain":   "cross-domain",
			"value":    withoutSummary,
			"reason":   "Some enabled modules are missing business summaries, which weakens agent comprehension coverage.",
			"drilldown": analyticsDrilldownItem(
				"module_descriptions_missing",
				"Review module topology",
				"business.topology.map",
				map[string]any{},
			),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		left := severityRank(anyString(items[i]["severity"]))
		right := severityRank(anyString(items[j]["severity"]))
		if left == right {
			return anyString(items[i]["title"]) < anyString(items[j]["title"])
		}
		return left > right
	})
	return items
}

func analyticsAnomalyReason(key string, value int) string {
	switch key {
	case "rejected_documents":
		return fmt.Sprintf("%d rejected documents require investigation or rework.", value)
	case "submitted_documents", "pending_approvals_latest", "pending_approvals":
		return fmt.Sprintf("%d items are waiting in approval flow and may indicate operational backlog.", value)
	case "open_tasks":
		return fmt.Sprintf("%d workflow tasks remain open.", value)
	case "dead_letters_latest":
		return fmt.Sprintf("%d dead-letter events indicate delivery or reliability issues.", value)
	default:
		return fmt.Sprintf("%d signals require follow-up investigation.", value)
	}
}

func analyticsOverviewDrilldowns(summary []map[string]any, anomalies []map[string]any) []map[string]any {
	items := []map[string]any{
		analyticsDrilldownItem("open_exceptions", "Open Exceptions", "business.exception.search", map[string]any{"page_size": 25}),
		analyticsDrilldownItem("document_submitted", "Submitted Documents", "business.document.search", map[string]any{"status": "submitted", "page_size": 25}),
	}
	for _, item := range anomalies {
		if drilldown, ok := item["drilldown"].(map[string]any); ok {
			items = append(items, drilldown)
		}
	}
	return uniqueDrilldowns(items)
}

func analyticsDrilldownItem(kind, label, targetTool string, targetArguments map[string]any) map[string]any {
	handle := encodeAnalyticsDrilldown(map[string]any{
		"drilldown_kind":   kind,
		"label":            label,
		"target_tool":      targetTool,
		"target_arguments": targetArguments,
	})
	return map[string]any{
		"drilldown_kind":   kind,
		"label":            label,
		"target_tool":      targetTool,
		"target_arguments": cloneMap(targetArguments),
		"handle":           handle,
	}
}

func encodeAnalyticsDrilldown(payload map[string]any) string {
	buf, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func decodeAnalyticsDrilldown(handle string) (map[string]any, error) {
	buf, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(handle))
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if err := json.Unmarshal(buf, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func analyticsDrilldownTargetForKey(key string) string {
	switch key {
	case "rejected_documents":
		return "business.document.search"
	case "open_tasks", "pending_approvals", "pending_approvals_latest":
		return "business.exception.search"
	case "dead_letters_latest":
		return "business.analytics.kpi.summary"
	default:
		return "business.health.summary"
	}
}

func analyticsDrilldownArgsForKey(key string) map[string]any {
	switch key {
	case "rejected_documents":
		return map[string]any{"status": "rejected", "page_size": 25}
	case "open_tasks":
		return map[string]any{"status": "open", "page_size": 25}
	case "pending_approvals", "pending_approvals_latest":
		return map[string]any{"status": "pending", "page_size": 25}
	default:
		return map[string]any{}
	}
}

func analyticsDomainIncludes(selected string, domains ...string) bool {
	trimmed := strings.TrimSpace(selected)
	if trimmed == "" {
		return true
	}
	for _, domain := range domains {
		if strings.EqualFold(strings.TrimSpace(domain), trimmed) {
			return true
		}
	}
	return false
}

func analyticsCoverage(arguments map[string]any, domain string) map[string]any {
	return map[string]any{
		"requested_domain":    strings.TrimSpace(domain),
		"included_domains":    []string{},
		"missing_domains":     []string{},
		"permission_filtered": []string{},
	}
}

func summaryStatusFromCount(value, warningThreshold, criticalThreshold int) string {
	switch {
	case criticalThreshold >= 0 && value > criticalThreshold:
		return "critical"
	case warningThreshold >= 0 && value > warningThreshold:
		return "warning"
	case value > 0:
		return "warning"
	default:
		return "healthy"
	}
}

func normalizeSeverity(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "critical", "high":
		return "high"
	case "warning", "medium":
		return "medium"
	default:
		return "low"
	}
}

func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func severityFromAge(createdAt time.Time) string {
	if createdAt.IsZero() {
		return "medium"
	}
	age := time.Since(createdAt)
	switch {
	case age > 7*24*time.Hour:
		return "high"
	case age > 48*time.Hour:
		return "medium"
	default:
		return "low"
	}
}

func clusterBusinessExceptions(items []map[string]any, groupBy string) []map[string]any {
	buckets := map[string][]map[string]any{}
	for _, item := range items {
		key := firstNonEmpty(anyString(item[groupBy]), anyString(item["kind"]), "unknown")
		buckets[key] = append(buckets[key], item)
	}
	clusters := make([]map[string]any, 0, len(buckets))
	for key, group := range buckets {
		severity := "low"
		statuses := make([]string, 0)
		for _, item := range group {
			if severityRank(anyString(item["severity"])) > severityRank(severity) {
				severity = anyString(item["severity"])
			}
			statuses = append(statuses, strings.ToLower(strings.TrimSpace(anyString(item["status"]))))
		}
		statuses = uniqueNonEmptyStrings(statuses)
		drilldownArgs := map[string]any{"page_size": 25}
		if len(statuses) == 1 {
			drilldownArgs["status"] = statuses[0]
		} else if len(statuses) > 1 {
			drilldownArgs["statuses"] = append([]string(nil), statuses...)
		}
		clusters = append(clusters, map[string]any{
			"group_by":  groupBy,
			"group_key": key,
			"count":     len(group),
			"severity":  severity,
			"domain":    anyString(group[0]["domain"]),
			"statuses":  append([]string(nil), statuses...),
			"items":     truncateMaps(group, 3),
			"drilldown": analyticsDrilldownItem("exception_cluster_"+key, "Inspect "+key+" exceptions", "business.exception.search", drilldownArgs),
		})
	}
	sort.Slice(clusters, func(i, j int) bool {
		leftCount, rightCount := anyInt(clusters[i]["count"]), anyInt(clusters[j]["count"])
		if leftCount == rightCount {
			return anyString(clusters[i]["group_key"]) < anyString(clusters[j]["group_key"])
		}
		return leftCount > rightCount
	})
	return clusters
}

func truncateMaps(items []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(items) <= limit {
		return append([]map[string]any(nil), items...)
	}
	return append([]map[string]any(nil), items[:limit]...)
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func uniqueDrilldowns(items []map[string]any) []map[string]any {
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		handle := strings.TrimSpace(anyString(item["handle"]))
		if handle == "" {
			continue
		}
		if _, ok := seen[handle]; ok {
			continue
		}
		seen[handle] = struct{}{}
		out = append(out, item)
	}
	return out
}

func anyInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func buildTrendSeries(snapshots []analytics.Snapshot, bucket string) []map[string]any {
	type aggregated struct {
		label              string
		start              time.Time
		generatedAt        time.Time
		submittedDocuments int
		approvedDocuments  int
		pendingApprovals   int
		deadLetters        int
	}
	buckets := map[string]*aggregated{}
	for _, snapshot := range snapshots {
		label, start := analyticsBucketLabel(snapshot.GeneratedAt, bucket)
		current, ok := buckets[label]
		if !ok {
			current = &aggregated{label: label, start: start}
			buckets[label] = current
		}
		if current.generatedAt.IsZero() || snapshot.GeneratedAt.After(current.generatedAt) {
			current.generatedAt = snapshot.GeneratedAt
			current.submittedDocuments = snapshot.Documents.Submitted
			current.approvedDocuments = snapshot.Documents.Approved
			current.pendingApprovals = snapshot.Workflow.PendingApprovals
			current.deadLetters = snapshot.Reliability.OutboxDeadLetters
		}
	}
	items := make([]*aggregated, 0, len(buckets))
	for _, item := range buckets {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].start.Before(items[j].start) })
	series := make([]map[string]any, 0, len(items))
	for _, item := range items {
		series = append(series, map[string]any{
			"bucket":              item.label,
			"submitted_documents": item.submittedDocuments,
			"approved_documents":  item.approvedDocuments,
			"pending_approvals":   item.pendingApprovals,
			"dead_letters":        item.deadLetters,
			"drilldown":           analyticsDrilldownItem("trend_bucket_"+item.label, "Investigate "+item.label, "business.analytics.kpi.summary", map[string]any{}),
		})
	}
	return series
}

func analyticsBucketLabel(ts time.Time, bucket string) (string, time.Time) {
	current := ts.UTC()
	switch strings.ToLower(strings.TrimSpace(bucket)) {
	case "month", "monthly":
		start := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start.Format("2006-01"), start
	case "week", "weekly":
		weekday := int(current.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(weekday - 1))
		return start.Format("2006-01-02"), start
	default:
		start := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
		return start.Format("2006-01-02"), start
	}
}
