package mcp

import (
	"fmt"
	"sort"
	"strings"

	"orbyte/internal/platform/module"
)

func contractDescriptorFromModule(meta module.MCPContractMetadata, requiredPermissions []string, defaultSideEffectClass, defaultIdempotency, defaultAuditAction string) ContractDescriptor {
	defaultAction := defaultToolActionClass(defaultAuditAction, "")
	descriptor := ContractDescriptor{
		Version:              ContractVersion,
		Stability:            "stable",
		SideEffectClass:      strings.TrimSpace(defaultSideEffectClass),
		Idempotency:          strings.TrimSpace(defaultIdempotency),
		AuditAction:          strings.TrimSpace(defaultAuditAction),
		ActionClass:          defaultAction,
		RiskClass:            defaultToolRiskClass(defaultAction, defaultAuditAction, ""),
		DraftOnly:            defaultAction == "draft",
		RequiresConfirmation: defaultToolRequiresConfirmation(defaultAction, defaultAuditAction, ""),
		RequiresApproval:     defaultToolRequiresApproval(defaultAction, defaultAuditAction, ""),
		GovernanceTags:       defaultToolGovernanceTags(defaultAction, defaultAuditAction, ""),
		BusinessDomains:      defaultToolBusinessDomains(defaultAuditAction),
		RequiredScopes:       append([]string(nil), meta.RequiredScopes...),
		RequiredPermissions:  append([]string(nil), requiredPermissions...),
		Deprecated:           meta.Deprecated,
		DeprecationNote:      strings.TrimSpace(meta.DeprecationNote),
	}
	if version := strings.TrimSpace(meta.Version); version != "" {
		descriptor.Version = version
	}
	if stability := strings.TrimSpace(meta.Stability); stability != "" {
		descriptor.Stability = stability
	}
	if sideEffectClass := strings.TrimSpace(meta.SideEffectClass); sideEffectClass != "" {
		descriptor.SideEffectClass = sideEffectClass
	}
	if idempotency := strings.TrimSpace(meta.Idempotency); idempotency != "" {
		descriptor.Idempotency = idempotency
	}
	if auditAction := strings.TrimSpace(meta.AuditAction); auditAction != "" {
		descriptor.AuditAction = auditAction
	}
	if actionClass := strings.TrimSpace(meta.ActionClass); actionClass != "" {
		descriptor.ActionClass = actionClass
	}
	if riskClass := strings.TrimSpace(meta.RiskClass); riskClass != "" {
		descriptor.RiskClass = riskClass
	}
	if meta.DraftOnly {
		descriptor.DraftOnly = true
	}
	if meta.RequiresConfirmation {
		descriptor.RequiresConfirmation = true
	}
	if meta.RequiresApproval {
		descriptor.RequiresApproval = true
	}
	descriptor.GovernanceTags = mergeUniqueStrings(descriptor.GovernanceTags, meta.GovernanceTags)
	descriptor.BusinessDomains = mergeUniqueStrings(descriptor.BusinessDomains, meta.BusinessDomains)
	if descriptor.RiskClass == "" {
		descriptor.RiskClass = defaultToolRiskClass(descriptor.ActionClass, descriptor.AuditAction, "")
	}
	return descriptor
}

func defaultToolSideEffectClass(name, operation string) string {
	key := strings.ToLower(strings.TrimSpace(name + " " + operation))
	switch {
	case strings.Contains(key, ".list"), strings.Contains(key, ".get"), strings.Contains(key, ".resolve"), strings.Contains(key, ".preview"), strings.Contains(key, ".validate"), strings.Contains(key, ".simulate"), strings.Contains(key, ".plan"), strings.Contains(key, ".execute"):
		return "read"
	case strings.Contains(key, ".save"), strings.Contains(key, ".create"), strings.Contains(key, ".update"), strings.Contains(key, ".upsert"), strings.Contains(key, ".delete"), strings.Contains(key, ".publish"), strings.Contains(key, ".deliver"), strings.Contains(key, ".run"), strings.Contains(key, ".replay"):
		return "mutation"
	default:
		return "operation"
	}
}

func defaultToolActionClass(name, operation string) string {
	key := strings.ToLower(strings.TrimSpace(name + " " + operation))
	switch {
	case strings.Contains(key, ".submit"), strings.Contains(key, ".publish"), strings.Contains(key, ".approve"):
		return "submit"
	case strings.Contains(key, ".draft."), strings.Contains(key, ".draft_create"), strings.Contains(key, ".draft_update"):
		return "draft"
	case strings.Contains(key, ".create"), strings.Contains(key, ".update"), strings.Contains(key, ".save"), strings.Contains(key, ".delete"), strings.Contains(key, ".enable"), strings.Contains(key, ".disable"), strings.Contains(key, ".replay"), strings.Contains(key, ".restore"), strings.Contains(key, ".apply"), strings.Contains(key, ".commit"):
		return "controlled_mutation"
	case strings.Contains(key, ".summary"), strings.Contains(key, ".map"), strings.Contains(key, ".timeline"), strings.Contains(key, ".analytics"), strings.Contains(key, ".health"), strings.Contains(key, ".relationships"), strings.Contains(key, ".review"), strings.Contains(key, ".advisor"), strings.Contains(key, ".search"), strings.Contains(key, ".query"):
		return "analyze"
	default:
		return "read"
	}
}

func defaultToolRiskClass(actionClass, name, operation string) string {
	switch actionClass {
	case "draft":
		return "medium"
	case "submit", "controlled_mutation":
		return "high"
	case "analyze":
		return "low"
	default:
		if defaultToolSideEffectClass(name, operation) == "mutation" {
			return "high"
		}
		return "low"
	}
}

func defaultToolRequiresConfirmation(actionClass, name, operation string) bool {
	if actionClass == "draft" || actionClass == "submit" || actionClass == "controlled_mutation" {
		return true
	}
	key := strings.ToLower(strings.TrimSpace(name + " " + operation))
	return strings.Contains(key, "confirm_apply") || strings.Contains(key, ".publish") || strings.Contains(key, ".approve")
}

func defaultToolRequiresApproval(actionClass, name, operation string) bool {
	if actionClass == "submit" {
		return true
	}
	key := strings.ToLower(strings.TrimSpace(name + " " + operation))
	return strings.Contains(key, ".approve")
}

func defaultToolGovernanceTags(actionClass, name, operation string) []string {
	tags := []string{}
	if actionClass == "draft" {
		tags = append(tags, "draft-first")
	}
	if defaultToolRequiresConfirmation(actionClass, name, operation) {
		tags = append(tags, "confirm-required")
	}
	if defaultToolRequiresApproval(actionClass, name, operation) {
		tags = append(tags, "approval-aware")
	}
	if actionClass == "analyze" {
		tags = append(tags, "business-comprehension")
	}
	return tags
}

func defaultToolBusinessDomains(name string) []string {
	key := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(key, "business."):
		return []string{"cross-domain"}
	case strings.HasPrefix(key, "pricing."):
		return []string{"pricing", "commercial"}
	case strings.HasPrefix(key, "tax."):
		return []string{"tax", "finance"}
	case strings.HasPrefix(key, "treasury."):
		return []string{"treasury", "finance"}
	case strings.HasPrefix(key, "inventory."):
		return []string{"inventory", "operations"}
	case strings.HasPrefix(key, "party."):
		return []string{"masterdata", "party"}
	default:
		return []string{"platform"}
	}
}

func defaultToolIdempotency(name, operation string) string {
	sideEffectClass := defaultToolSideEffectClass(name, operation)
	if sideEffectClass == "read" {
		return "read-only"
	}
	key := strings.ToLower(strings.TrimSpace(name + " " + operation))
	switch {
	case strings.Contains(key, ".upsert"), strings.Contains(key, ".update"), strings.Contains(key, ".save"):
		return "best-effort"
	case strings.Contains(key, ".create"), strings.Contains(key, ".publish"), strings.Contains(key, ".deliver"), strings.Contains(key, ".run"), strings.Contains(key, ".replay"), strings.Contains(key, ".delete"):
		return "non-idempotent"
	default:
		return "best-effort"
	}
}

func builtInToolContract(name, permission string, contract ContractDescriptor) ContractDescriptor {
	descriptor := ContractDescriptor{
		Version:              ContractVersion,
		Stability:            "stable",
		SideEffectClass:      defaultToolSideEffectClass(name, ""),
		Idempotency:          defaultToolIdempotency(name, ""),
		AuditAction:          "mcp.tool." + strings.TrimSpace(name),
		ActionClass:          defaultToolActionClass(name, ""),
		RiskClass:            defaultToolRiskClass(defaultToolActionClass(name, ""), name, ""),
		DraftOnly:            defaultToolActionClass(name, "") == "draft",
		RequiresConfirmation: defaultToolRequiresConfirmation(defaultToolActionClass(name, ""), name, ""),
		RequiresApproval:     defaultToolRequiresApproval(defaultToolActionClass(name, ""), name, ""),
		GovernanceTags:       defaultToolGovernanceTags(defaultToolActionClass(name, ""), name, ""),
		BusinessDomains:      defaultToolBusinessDomains(name),
		RequiredPermissions:  []string{strings.TrimSpace(permission)},
	}
	if strings.TrimSpace(contract.Version) != "" {
		descriptor.Version = contract.Version
	}
	if strings.TrimSpace(contract.Stability) != "" {
		descriptor.Stability = contract.Stability
	}
	if strings.TrimSpace(contract.SideEffectClass) != "" {
		descriptor.SideEffectClass = contract.SideEffectClass
	}
	if strings.TrimSpace(contract.Idempotency) != "" {
		descriptor.Idempotency = contract.Idempotency
	}
	if strings.TrimSpace(contract.AuditAction) != "" {
		descriptor.AuditAction = contract.AuditAction
	}
	if strings.TrimSpace(contract.ActionClass) != "" {
		descriptor.ActionClass = contract.ActionClass
	}
	if strings.TrimSpace(contract.RiskClass) != "" {
		descriptor.RiskClass = contract.RiskClass
	}
	if contract.DraftOnly {
		descriptor.DraftOnly = true
	}
	if contract.RequiresConfirmation {
		descriptor.RequiresConfirmation = true
	}
	if contract.RequiresApproval {
		descriptor.RequiresApproval = true
	}
	descriptor.GovernanceTags = mergeUniqueStrings(descriptor.GovernanceTags, contract.GovernanceTags)
	descriptor.BusinessDomains = mergeUniqueStrings(descriptor.BusinessDomains, contract.BusinessDomains)
	if len(contract.RequiredScopes) > 0 {
		descriptor.RequiredScopes = append([]string(nil), contract.RequiredScopes...)
	}
	if len(contract.RequiredPermissions) > 0 {
		descriptor.RequiredPermissions = append([]string(nil), contract.RequiredPermissions...)
	}
	descriptor.Deprecated = contract.Deprecated
	if strings.TrimSpace(contract.DeprecationNote) != "" {
		descriptor.DeprecationNote = contract.DeprecationNote
	}
	return descriptor
}

func mergeUniqueStrings(base []string, more []string) []string {
	if len(base) == 0 && len(more) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	items := make([]string, 0, len(base)+len(more))
	for _, current := range append(append([]string(nil), base...), more...) {
		trimmed := strings.TrimSpace(current)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	sort.Strings(items)
	return items
}

func builtInResourceContract(uri string, requiredPermissions []string) ContractDescriptor {
	return ContractDescriptor{
		Version:             ContractVersion,
		Stability:           "stable",
		SideEffectClass:     "read",
		Idempotency:         "read-only",
		AuditAction:         "mcp.resource." + strings.TrimSpace(uri),
		RequiredPermissions: append([]string(nil), requiredPermissions...),
	}
}

func (s *Server) toolDescriptorByName(actor ActorContext, name string) (ToolDescriptor, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ToolDescriptor{}, false
	}
	for _, item := range s.listTools(actor) {
		if strings.TrimSpace(item.Name) == name {
			return item, true
		}
	}
	return ToolDescriptor{}, false
}

func (s *Server) ToolDescriptor(name string, actor ActorContext) (ToolDescriptor, bool) {
	return s.toolDescriptorByName(actor, name)
}

func (s *Server) ToolDescriptorForArguments(name string, actor ActorContext, arguments map[string]any) (ToolDescriptor, bool) {
	descriptor, ok := s.toolDescriptorByName(actor, name)
	if !ok {
		return ToolDescriptor{}, false
	}
	return s.decorateToolDescriptorWithGovernance(descriptor, arguments), true
}

func (s *Server) resourceDescriptorByURI(actor ActorContext, uri string) (ResourceDescriptor, bool) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ResourceDescriptor{}, false
	}
	for _, item := range s.listResources(actor) {
		if strings.TrimSpace(item.URI) == uri {
			return item, true
		}
	}
	return ResourceDescriptor{}, false
}

func (s *Server) ResourceDescriptor(uri string, actor ActorContext) (ResourceDescriptor, bool) {
	return s.resourceDescriptorByURI(actor, uri)
}

func (s *Server) mcpCatalogResource(actor ActorContext) (map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("mcp server is unavailable")
	}
	tools := s.listTools(actor)
	resources := s.listResources(actor)
	apps := s.listMCPApps(actor)
	return map[string]any{
		"protocol_version": ProtocolVersion,
		"contract_version": ContractVersion,
		"endpoint_scope":   actor.EndpointScope,
		"endpoint_scopes":  []string{EndpointScopeAll, EndpointScopeAnalytics},
		"tools":            tools,
		"resources":        resources,
		"apps":             apps,
	}, nil
}

func (s *Server) Catalog(actor ActorContext) (map[string]any, error) {
	return s.mcpCatalogResource(actor)
}

func (s *Server) listMCPApps(actor ActorContext) []map[string]any {
	items := make([]map[string]any, 0)
	if s == nil || s.modules == nil {
		return items
	}
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		scope := scopeForModule(detail.Manifest.Key)
		if !scopeMatches(actor.EndpointScope, scope) {
			continue
		}
		for _, app := range detail.Manifest.MCP.Apps {
			if !allowsAll(actor.PermissionChecker, app.RequiredPermissions) {
				continue
			}
			items = append(items, map[string]any{
				"key":                  app.Key,
				"title":                app.Title,
				"description":          app.Description,
				"resource_key":         app.ResourceKey,
				"view_key":             app.ViewKey,
				"custom_entry_key":     app.CustomEntryKey,
				"required_permissions": append([]string(nil), app.RequiredPermissions...),
				"contract": contractDescriptorFromModule(
					app.Contract,
					app.RequiredPermissions,
					"read",
					"read-only",
					"mcp.app."+strings.TrimSpace(app.Key),
				),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.TrimSpace(anyString(items[i]["key"])) < strings.TrimSpace(anyString(items[j]["key"]))
	})
	return items
}

func anyString(value any) string {
	text, _ := value.(string)
	return text
}
