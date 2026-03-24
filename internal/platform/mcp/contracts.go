package mcp

import (
	"fmt"
	"sort"
	"strings"

	"orbyte/internal/platform/module"
)

func contractDescriptorFromModule(meta module.MCPContractMetadata, requiredPermissions []string, defaultSideEffectClass, defaultIdempotency, defaultAuditAction string) ContractDescriptor {
	descriptor := ContractDescriptor{
		Version:             ContractVersion,
		Stability:           "stable",
		SideEffectClass:     strings.TrimSpace(defaultSideEffectClass),
		Idempotency:         strings.TrimSpace(defaultIdempotency),
		AuditAction:         strings.TrimSpace(defaultAuditAction),
		RequiredScopes:      append([]string(nil), meta.RequiredScopes...),
		RequiredPermissions: append([]string(nil), requiredPermissions...),
		Deprecated:          meta.Deprecated,
		DeprecationNote:     strings.TrimSpace(meta.DeprecationNote),
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
	if strings.TrimSpace(contract.Version) != "" ||
		strings.TrimSpace(contract.Stability) != "" ||
		strings.TrimSpace(contract.SideEffectClass) != "" ||
		strings.TrimSpace(contract.Idempotency) != "" ||
		strings.TrimSpace(contract.AuditAction) != "" ||
		len(contract.RequiredPermissions) > 0 ||
		contract.Deprecated {
		return contract
	}
	return ContractDescriptor{
		Version:             ContractVersion,
		Stability:           "stable",
		SideEffectClass:     defaultToolSideEffectClass(name, ""),
		Idempotency:         defaultToolIdempotency(name, ""),
		AuditAction:         "mcp.tool." + strings.TrimSpace(name),
		RequiredPermissions: []string{strings.TrimSpace(permission)},
	}
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
