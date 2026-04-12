package mcp

import (
	"encoding/json"
	"sort"
	"strings"
)

type ToolCatalogOptions struct {
	CatalogMode         string   `json:"catalog_mode,omitempty"`
	ExposureMode        string   `json:"exposure_mode,omitempty"`
	Capabilities        []string `json:"capabilities,omitempty"`
	Domains             []string `json:"domains,omitempty"`
	ModuleKeys          []string `json:"module_keys,omitempty"`
	SourceTypes         []string `json:"source_types,omitempty"`
	ActionClasses       []string `json:"action_classes,omitempty"`
	IncludeSummary      bool     `json:"include_summary,omitempty"`
	MaxTools            int      `json:"max_tools,omitempty"`
	IncludeHiddenCounts bool     `json:"include_hidden_counts,omitempty"`
}

type ToolCapabilityDescriptor struct {
	Key                string `json:"key"`
	Title              string `json:"title"`
	Description        string `json:"description,omitempty"`
	Category           string `json:"category,omitempty"`
	DefaultForAgent    bool   `json:"default_for_agent,omitempty"`
	RecommendedOrder   int    `json:"recommended_order,omitempty"`
	EstimatedToolCount int    `json:"estimated_tool_count,omitempty"`
}

type toolCapabilityRuntimeDescriptor struct {
	ToolCapabilityDescriptor
	Matcher func(ToolDescriptor) bool
}

func normalizeToolDescriptorProtocolShape(descriptor ToolDescriptor) ToolDescriptor {
	if descriptor.InputSchema == nil {
		descriptor.InputSchema = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return descriptor
}

func (s *Server) decorateToolDescriptorWithGovernance(descriptor ToolDescriptor, arguments map[string]any) ToolDescriptor {
	descriptor = normalizeToolDescriptorProtocolShape(descriptor)
	descriptor = s.decorateToolDescriptorWithCapabilities(descriptor)
	evaluation := s.evaluateToolGovernance(descriptor, arguments)
	descriptor.PolicyState = evaluation.PolicyState
	descriptor.PolicyReason = evaluation.PolicyReason
	descriptor.EffectiveVisibility = evaluation.EffectiveVisibility
	return descriptor
}

func (s *Server) decorateToolDescriptorWithCapabilities(descriptor ToolDescriptor) ToolDescriptor {
	capabilities := s.toolCapabilitiesForDescriptor(descriptor)
	keys := make([]string, 0, len(capabilities))
	categories := make([]string, 0, len(capabilities))
	for _, item := range capabilities {
		keys = append(keys, item.Key)
		if strings.TrimSpace(item.Category) != "" {
			categories = append(categories, item.Category)
		}
	}
	descriptor.CapabilityKeys = mergeUniqueStrings(nil, keys)
	descriptor.CapabilityCategories = mergeUniqueStrings(nil, categories)
	descriptor.CompactEligible = len(descriptor.CapabilityKeys) > 0 && !containsString(descriptor.CapabilityKeys, "platform_admin")
	return descriptor
}

func (s *Server) toolCapabilities() []toolCapabilityRuntimeDescriptor {
	return []toolCapabilityRuntimeDescriptor{
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "discovery",
				Title:            "Discovery",
				Description:      "Core module, capability, dataset, and business entry-point discovery.",
				Category:         "core",
				DefaultForAgent:  true,
				RecommendedOrder: 10,
			},
			Matcher: func(item ToolDescriptor) bool {
				switch strings.TrimSpace(item.Name) {
				case "business.module.list", "business.module.get", "business.module.search", "business.capability.search", "business.document.type.list", "business.dataset.list", "business.reference.type.list", "tools.list", "tools.search", "tools.describe", "tools.call", "skills.list", "skills.search", "skills.describe", "playbooks.list", "playbooks.search", "playbooks.describe":
					return true
				default:
					return false
				}
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "business_overview",
				Title:            "Business Overview",
				Description:      "High-signal overview of health, KPIs, and cross-domain operating state.",
				Category:         "core",
				DefaultForAgent:  true,
				RecommendedOrder: 20,
			},
			Matcher: func(item ToolDescriptor) bool {
				switch strings.TrimSpace(item.Name) {
				case "business.health.summary", "business.analytics.overview", "business.analytics.kpi.summary", "business.exception.search":
					return true
				default:
					return false
				}
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "cross_domain_analytics",
				Title:            "Cross-Domain Analytics",
				Description:      "Analytical summaries, trends, anomalies, drilldowns, and exception clusters.",
				Category:         "core",
				DefaultForAgent:  true,
				RecommendedOrder: 30,
			},
			Matcher: func(item ToolDescriptor) bool {
				return strings.HasPrefix(strings.TrimSpace(item.Name), "business.analytics.")
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "relationships_timeline",
				Title:            "Relationships and Timeline",
				Description:      "Relationship traversal, record drilldown, and activity/timeline inspection.",
				Category:         "core",
				DefaultForAgent:  true,
				RecommendedOrder: 40,
			},
			Matcher: func(item ToolDescriptor) bool {
				switch strings.TrimSpace(item.Name) {
				case "business.timeline.get", "business.relationships.get", "business.record.related", "business.record.get", "business.record.search", "business.document.search", "business.document.get", "business.analytics.drilldown":
					return true
				default:
					return false
				}
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "governed_drafts",
				Title:            "Governed Drafts",
				Description:      "Draft-oriented business actions with confirmation and governance.",
				Category:         "core",
				DefaultForAgent:  true,
				RecommendedOrder: 50,
			},
			Matcher: func(item ToolDescriptor) bool {
				return strings.Contains(strings.TrimSpace(item.Name), ".business.document.draft.") || strings.HasPrefix(strings.TrimSpace(item.Name), "business.document.draft.")
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "pricing_promotion",
				Title:            "Pricing and Promotion",
				Description:      "Pricing, promotions, and related advisor review tools.",
				Category:         "domain",
				RecommendedOrder: 60,
			},
			Matcher: func(item ToolDescriptor) bool {
				return toolMatchesBusinessDomain(item, "pricing") || strings.HasPrefix(strings.TrimSpace(item.Name), "pricing.")
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "tax_structure",
				Title:            "Tax Structure",
				Description:      "Tax-oriented discovery, review, and setup analysis tools.",
				Category:         "domain",
				RecommendedOrder: 70,
			},
			Matcher: func(item ToolDescriptor) bool {
				return toolMatchesBusinessDomain(item, "tax") || strings.HasPrefix(strings.TrimSpace(item.Name), "tax.")
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "treasury_reconciliation",
				Title:            "Treasury and Reconciliation",
				Description:      "Treasury, reconciliation, and cash exception investigation tools.",
				Category:         "domain",
				RecommendedOrder: 80,
			},
			Matcher: func(item ToolDescriptor) bool {
				return toolMatchesBusinessDomain(item, "treasury") || strings.HasPrefix(strings.TrimSpace(item.Name), "treasury.")
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "inventory_health",
				Title:            "Inventory Health",
				Description:      "Inventory and operations exception and health investigation tools.",
				Category:         "domain",
				RecommendedOrder: 90,
			},
			Matcher: func(item ToolDescriptor) bool {
				return toolMatchesBusinessDomain(item, "inventory") || strings.HasPrefix(strings.TrimSpace(item.Name), "inventory.")
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "party_master",
				Title:            "Party Master",
				Description:      "Customer, vendor, contact, and party master investigation tools.",
				Category:         "domain",
				RecommendedOrder: 100,
			},
			Matcher: func(item ToolDescriptor) bool {
				return toolMatchesBusinessDomain(item, "party") || toolMatchesBusinessDomain(item, "masterdata") || strings.HasPrefix(strings.TrimSpace(item.Name), "party.")
			},
		},
		{
			ToolCapabilityDescriptor: ToolCapabilityDescriptor{
				Key:              "platform_admin",
				Title:            "Platform Administration",
				Description:      "Administrative and control-plane tools not intended for default agent catalogs.",
				Category:         "admin",
				RecommendedOrder: 200,
			},
			Matcher: func(item ToolDescriptor) bool {
				name := strings.TrimSpace(item.Name)
				return strings.HasPrefix(name, "config.") ||
					strings.HasPrefix(name, "feature_flag.") ||
					strings.HasPrefix(name, "module.") ||
					strings.HasPrefix(name, "identity.") ||
					strings.HasPrefix(name, "workflow.") ||
					strings.HasPrefix(name, "template.") ||
					strings.HasPrefix(name, "policy.") ||
					strings.HasPrefix(name, "integration.")
			},
		},
	}
}

func toolMatchesBusinessDomain(item ToolDescriptor, domain string) bool {
	target := strings.TrimSpace(domain)
	if target == "" {
		return false
	}
	for _, current := range item.Contract.BusinessDomains {
		if strings.TrimSpace(current) == target {
			return true
		}
	}
	return false
}

func (s *Server) toolCapabilitiesForDescriptor(descriptor ToolDescriptor) []ToolCapabilityDescriptor {
	items := make([]ToolCapabilityDescriptor, 0)
	for _, capability := range s.toolCapabilities() {
		if capability.Matcher != nil && capability.Matcher(descriptor) {
			items = append(items, capability.ToolCapabilityDescriptor)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].RecommendedOrder == items[j].RecommendedOrder {
			return items[i].Key < items[j].Key
		}
		return items[i].RecommendedOrder < items[j].RecommendedOrder
	})
	return items
}

func (s *Server) CapabilityInventory(items []ToolDescriptor) []ToolCapabilityDescriptor {
	capabilities := s.toolCapabilities()
	counts := map[string]int{}
	for _, item := range items {
		for _, key := range item.CapabilityKeys {
			counts[key]++
		}
	}
	output := make([]ToolCapabilityDescriptor, 0, len(capabilities))
	for _, item := range capabilities {
		copyItem := item.ToolCapabilityDescriptor
		copyItem.EstimatedToolCount = counts[copyItem.Key]
		output = append(output, copyItem)
	}
	return output
}

func (s *Server) defaultCompactCapabilities() []string {
	cfg := s.mcpRuntimeConfig()
	if len(cfg.DefaultCapabilities) > 0 {
		return mergeUniqueStrings(nil, cfg.DefaultCapabilities)
	}
	return []string{
		"discovery",
		"business_overview",
		"cross_domain_analytics",
		"relationships_timeline",
		"governed_drafts",
	}
}

func (s *Server) activeCapabilitiesForInit() []string {
	mode := s.effectiveExposureMode(ToolCatalogOptions{})
	switch mode {
	case MCPExposureModeMinimal:
		return []string{}
	case MCPExposureModeCompact:
		return s.defaultCompactCapabilities()
	default:
		return []string{}
	}
}

func (s *Server) catalogOptionsFromExposureMode() ToolCatalogOptions {
	mode := s.effectiveExposureMode(ToolCatalogOptions{})
	if mode == MCPExposureModeCompact {
		return ToolCatalogOptions{CatalogMode: MCPExposureModeCompact}
	}
	return ToolCatalogOptions{}
}

func (s *Server) normalizeCompactCapabilities(options ToolCatalogOptions) []string {
	activeCapabilities := append([]string(nil), options.Capabilities...)
	if strings.TrimSpace(options.CatalogMode) == "compact" && len(activeCapabilities) == 0 && !hasExplicitCompactFilters(options) {
		activeCapabilities = s.defaultCompactCapabilities()
	}
	return activeCapabilities
}

func parseToolCatalogOptions(raw json.RawMessage) ToolCatalogOptions {
	options := ToolCatalogOptions{}
	if len(raw) == 0 {
		return options
	}
	_ = json.Unmarshal(raw, &options)
	options.CatalogMode = strings.TrimSpace(options.CatalogMode)
	options.ExposureMode = strings.TrimSpace(strings.ToLower(options.ExposureMode))
	options.Capabilities = mergeUniqueStrings(nil, options.Capabilities)
	options.Domains = mergeUniqueStrings(nil, options.Domains)
	options.ModuleKeys = mergeUniqueStrings(nil, options.ModuleKeys)
	options.SourceTypes = mergeUniqueStrings(nil, options.SourceTypes)
	options.ActionClasses = mergeUniqueStrings(nil, options.ActionClasses)
	if options.MaxTools < 0 {
		options.MaxTools = 0
	}
	return options
}

func (s *Server) toolsListResult(actor ActorContext, options ToolCatalogOptions) map[string]any {
	exposureMode := s.effectiveExposureMode(options)
	discoverable := s.nonMetaToolDescriptors(actor)
	full := s.listTools(actor)
	if exposureMode == MCPExposureModeMinimal {
		full = s.minimalExposedTools(actor)
	} else {
		full = s.nonMetaToolDescriptors(actor)
	}
	tools, catalog, groups, suggested := s.filterToolCatalog(full, options)
	if exposureMode == MCPExposureModeMinimal {
		discoverableFiltered, activeCapabilities := s.filterToolCatalogScope(discoverable, options)
		catalog["capabilities"] = activeCapabilities
		catalog["total_matching_tools"] = len(discoverableFiltered)
		catalog["hidden_tools"] = len(discoverableFiltered)
		groups = s.groupToolCatalog(discoverableFiltered)
		suggested = s.suggestedCapabilityExpansions(activeCapabilities)
	}
	result := map[string]any{
		"tools": tools,
	}
	catalog["exposure_mode"] = exposureMode
	if options.IncludeSummary || strings.TrimSpace(options.CatalogMode) == "compact" {
		result["catalog"] = catalog
		result["groups"] = groups
		result["suggested_expansions"] = suggested
	}
	return result
}

func (s *Server) FilterToolCatalogForPreview(options ToolCatalogOptions) ([]ToolDescriptor, map[string]any, []map[string]any, []ToolCapabilityDescriptor) {
	actor := ActorContext{
		PermissionChecker: func(string) bool { return true },
	}
	discoverable := s.nonMetaToolDescriptors(actor)
	items := s.listTools(actor)
	if s.effectiveExposureMode(options) == MCPExposureModeMinimal {
		items = s.minimalExposedTools(actor)
	} else {
		items = s.nonMetaToolDescriptors(actor)
	}
	tools, catalog, groups, suggested := s.filterToolCatalog(items, options)
	if s.effectiveExposureMode(options) == MCPExposureModeMinimal {
		discoverableFiltered, activeCapabilities := s.filterToolCatalogScope(discoverable, options)
		catalog["capabilities"] = activeCapabilities
		catalog["total_matching_tools"] = len(discoverableFiltered)
		catalog["hidden_tools"] = len(discoverableFiltered)
		groups = s.groupToolCatalog(discoverableFiltered)
		suggested = s.suggestedCapabilityExpansions(activeCapabilities)
	}
	return tools, catalog, groups, suggested
}

func (s *Server) effectiveExposureMode(options ToolCatalogOptions) string {
	if mode := normalizeExposureMode(options.ExposureMode); mode != "" {
		return mode
	}
	cfg := s.mcpRuntimeConfig()
	if mode := normalizeExposureMode(cfg.ExposureMode); mode != "" {
		return mode
	}
	return MCPExposureModeFull
}

func (s *Server) nonMetaToolDescriptors(actor ActorContext) []ToolDescriptor {
	all := s.listTools(actor)
	items := make([]ToolDescriptor, 0, len(all))
	for _, item := range all {
		if isMetaToolName(item.Name) {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (s *Server) nonPlaybookToolDescriptors(actor ActorContext, tools []ToolDescriptor) []ToolDescriptor {
	items := make([]ToolDescriptor, 0, len(tools))
	for _, item := range tools {
		if strings.HasPrefix(item.Name, "playbooks.") || strings.HasPrefix(item.Name, "skills.") {
			continue
		}
		items = append(items, item)
	}
	return items
}

func hasExplicitCompactFilters(options ToolCatalogOptions) bool {
	return len(options.Capabilities) > 0 ||
		len(options.Domains) > 0 ||
		len(options.ModuleKeys) > 0 ||
		len(options.SourceTypes) > 0 ||
		len(options.ActionClasses) > 0
}

func (s *Server) filterToolCatalogScope(items []ToolDescriptor, options ToolCatalogOptions) ([]ToolDescriptor, []string) {
	activeCapabilities := s.normalizeCompactCapabilities(options)
	minimalExposure := s.effectiveExposureMode(options) == MCPExposureModeMinimal
	filtered := make([]ToolDescriptor, 0, len(items))
	for _, item := range items {
		if minimalExposure && isMetaToolName(item.Name) {
			filtered = append(filtered, item)
			continue
		}
		if len(activeCapabilities) > 0 && !intersectsStrings(item.CapabilityKeys, activeCapabilities) {
			continue
		}
		if len(options.Domains) > 0 && !intersectsStrings(item.Contract.BusinessDomains, options.Domains) {
			continue
		}
		if len(options.ModuleKeys) > 0 && !containsString(options.ModuleKeys, item.ModuleKey) {
			continue
		}
		if len(options.SourceTypes) > 0 && !containsString(options.SourceTypes, item.SourceType) {
			continue
		}
		if len(options.ActionClasses) > 0 && !containsString(options.ActionClasses, item.Contract.ActionClass) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, activeCapabilities
}

func (s *Server) filterToolCatalog(items []ToolDescriptor, options ToolCatalogOptions) ([]ToolDescriptor, map[string]any, []map[string]any, []ToolCapabilityDescriptor) {
	mode := strings.TrimSpace(options.CatalogMode)
	if mode == "" {
		return items, map[string]any{
			"mode":                 "full",
			"total_matching_tools": len(items),
			"returned_tools":       len(items),
			"hidden_tools":         0,
		}, s.groupToolCatalog(items), []ToolCapabilityDescriptor{}
	}

	filtered, activeCapabilities := s.filterToolCatalogScope(items, options)
	sort.SliceStable(filtered, func(i, j int) bool {
		left := toolCatalogRank(filtered[i], activeCapabilities)
		right := toolCatalogRank(filtered[j], activeCapabilities)
		if left == right {
			return filtered[i].Name < filtered[j].Name
		}
		return left < right
	})
	totalMatching := len(filtered)
	maxTools := options.MaxTools
	if mode == "compact" && maxTools <= 0 {
		maxTools = 16
	}
	if maxTools > 0 && len(filtered) > maxTools {
		filtered = filtered[:maxTools]
	}
	catalog := map[string]any{
		"mode":                 firstNonEmpty(mode, "full"),
		"capabilities":         activeCapabilities,
		"domains":              options.Domains,
		"module_keys":          options.ModuleKeys,
		"source_types":         options.SourceTypes,
		"action_classes":       options.ActionClasses,
		"max_tools":            maxTools,
		"total_matching_tools": totalMatching,
		"returned_tools":       len(filtered),
	}
	if options.IncludeHiddenCounts || mode == "compact" {
		catalog["hidden_tools"] = maxInt(totalMatching-len(filtered), 0)
	}
	return filtered, catalog, s.groupToolCatalog(filtered), s.suggestedCapabilityExpansions(activeCapabilities)
}

func (s *Server) groupToolCatalog(items []ToolDescriptor) []map[string]any {
	type groupKey struct {
		Group string
		Key   string
	}
	counts := map[groupKey]int{}
	for _, item := range items {
		for _, capability := range item.CapabilityKeys {
			counts[groupKey{Group: "capability", Key: capability}]++
		}
		for _, domain := range item.Contract.BusinessDomains {
			counts[groupKey{Group: "domain", Key: domain}]++
		}
		if strings.TrimSpace(item.SourceType) != "" {
			counts[groupKey{Group: "source_type", Key: item.SourceType}]++
		}
		if strings.TrimSpace(item.Contract.ActionClass) != "" {
			counts[groupKey{Group: "action_class", Key: item.Contract.ActionClass}]++
		}
	}
	rows := make([]map[string]any, 0, len(counts))
	for key, count := range counts {
		rows = append(rows, map[string]any{
			"group": key.Group,
			"key":   key.Key,
			"count": count,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		leftGroup := anyString(rows[i]["group"])
		rightGroup := anyString(rows[j]["group"])
		if leftGroup == rightGroup {
			return anyString(rows[i]["key"]) < anyString(rows[j]["key"])
		}
		return leftGroup < rightGroup
	})
	return rows
}

func (s *Server) suggestedCapabilityExpansions(active []string) []ToolCapabilityDescriptor {
	items := make([]ToolCapabilityDescriptor, 0)
	for _, capability := range s.toolCapabilities() {
		if containsString(active, capability.Key) || capability.Category == "admin" {
			continue
		}
		items = append(items, capability.ToolCapabilityDescriptor)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].RecommendedOrder == items[j].RecommendedOrder {
			return items[i].Key < items[j].Key
		}
		return items[i].RecommendedOrder < items[j].RecommendedOrder
	})
	if len(items) > 6 {
		items = items[:6]
	}
	return items
}

func toolCatalogRank(item ToolDescriptor, activeCapabilities []string) int {
	bestCapability := 1000
	for _, current := range item.CapabilityKeys {
		for idx, active := range activeCapabilities {
			if current == active && idx < bestCapability {
				bestCapability = idx
			}
		}
	}
	actionRank := 40
	switch item.Contract.ActionClass {
	case "read":
		actionRank = 0
	case "analyze":
		actionRank = 5
	case "recommend":
		actionRank = 10
	case "draft":
		actionRank = 20
	case "submit":
		actionRank = 30
	case "controlled_mutation":
		actionRank = 35
	}
	sourceRank := 20
	switch item.SourceType {
	case "built_in":
		sourceRank = 0
	case "synthetic":
		sourceRank = 5
	case "module":
		sourceRank = 10
	}
	return bestCapability*100 + actionRank*10 + sourceRank
}

func intersectsStrings(left, right []string) bool {
	for _, current := range left {
		if containsString(right, current) {
			return true
		}
	}
	return false
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
