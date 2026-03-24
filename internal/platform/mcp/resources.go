package mcp

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type builtInResourceReader func(*Server, ActorContext, *url.URL, string) ([]ResourceContent, error)

type builtInResourceRegistration struct {
	descriptor ResourceDescriptor
	reader     builtInResourceReader
}

func (s *Server) listBuiltInResources(actor ActorContext) []ResourceDescriptor {
	registry := s.mustBuiltInResourceRegistrations()
	items := make([]ResourceDescriptor, 0, len(registry))
	for _, reg := range registry {
		if !scopeMatches(actor.EndpointScope, reg.descriptor.Scope) {
			continue
		}
		if !allowsAll(actor.PermissionChecker, reg.descriptor.Contract.RequiredPermissions) {
			continue
		}
		items = append(items, reg.descriptor)
	}
	return items
}

func (s *Server) readBuiltInResource(actor ActorContext, uri string) ([]ResourceContent, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return nil, true, err
	}
	if parsed.Scheme != "orbyte" {
		return nil, false, nil
	}
	baseURI := builtInResourceBaseURI(parsed)
	reg, ok := s.mustBuiltInResourceIndex()[baseURI]
	if !ok {
		return nil, false, nil
	}
	if !scopeMatches(actor.EndpointScope, reg.descriptor.Scope) {
		return nil, true, fmt.Errorf("resource is not available on this endpoint")
	}
	if !allowsAll(actor.PermissionChecker, reg.descriptor.Contract.RequiredPermissions) {
		return nil, true, fmt.Errorf("resource is not allowed")
	}
	if reg.reader == nil {
		return nil, true, fmt.Errorf("resource reader is unavailable")
	}
	contents, err := reg.reader(s, actor, parsed, uri)
	return contents, true, err
}

func (s *Server) mustBuiltInResourceRegistrations() []builtInResourceRegistration {
	if s == nil {
		return nil
	}
	if len(s.builtInResourceRegistry) == 0 {
		s.mustInitBuiltInResources()
	}
	return append([]builtInResourceRegistration(nil), s.builtInResourceRegistry...)
}

func (s *Server) mustBuiltInResourceIndex() map[string]builtInResourceRegistration {
	if s == nil {
		return nil
	}
	if len(s.builtInResourceIndex) == 0 {
		s.mustInitBuiltInResources()
	}
	index := make(map[string]builtInResourceRegistration, len(s.builtInResourceIndex))
	for key, reg := range s.builtInResourceIndex {
		index[key] = reg
	}
	return index
}

func (s *Server) mustInitBuiltInResources() {
	if s == nil || len(s.builtInResourceRegistry) != 0 {
		return
	}
	registry := s.buildBuiltInResourceRegistrations()
	sort.Slice(registry, func(i, j int) bool {
		return strings.TrimSpace(registry[i].descriptor.URI) < strings.TrimSpace(registry[j].descriptor.URI)
	})
	index := make(map[string]builtInResourceRegistration, len(registry))
	for _, reg := range registry {
		key := strings.TrimSpace(reg.descriptor.URI)
		if key == "" {
			panic("built-in resource registration is missing a uri")
		}
		if _, exists := index[key]; exists {
			panic(fmt.Sprintf("duplicate built-in resource uri %q", key))
		}
		if reg.reader == nil {
			panic(fmt.Sprintf("built-in resource %q is missing a reader", key))
		}
		index[key] = reg
	}
	s.builtInResourceRegistry = registry
	s.builtInResourceIndex = index
}

func (s *Server) buildBuiltInResourceRegistrations() []builtInResourceRegistration {
	registry := make([]builtInResourceRegistration, 0, 16)
	if s != nil && s.templates != nil {
		registry = append(registry, builtInResourceRegistration{
			descriptor: ResourceDescriptor{
				URI:         templateDesignerResourceURI,
				Name:        "Template Designer",
				Description: "Lightweight MCP app for template draft inspection and preview.",
				Scope:       "template",
				MIMEType:    "text/html",
				Contract:    builtInResourceContract(templateDesignerResourceURI, []string{"template.read"}),
			},
			reader: func(s *Server, actor ActorContext, parsed *url.URL, requestedURI string) ([]ResourceContent, error) {
				htmlText, err := s.renderTemplateDesignerApp(actor, parsed)
				if err != nil {
					return nil, err
				}
				return []ResourceContent{{URI: requestedURI, MIMEType: "text/html", Text: htmlText}}, nil
			},
		})
	}
	if s != nil && s.analytics != nil {
		registry = append(registry, builtInResourceRegistration{
			descriptor: ResourceDescriptor{
				URI:         analyticsStudioResourceURI,
				Name:        "Analytics Studio",
				Description: "Lightweight MCP app for analytics authoring, ad hoc results, and chart previews.",
				Scope:       EndpointScopeAnalytics,
				MIMEType:    "text/html",
				Contract:    builtInResourceContract(analyticsStudioResourceURI, []string{"analytics.read"}),
			},
			reader: func(s *Server, actor ActorContext, parsed *url.URL, requestedURI string) ([]ResourceContent, error) {
				htmlText, err := s.renderAnalyticsStudioApp(actor, parsed)
				if err != nil {
					return nil, err
				}
				return []ResourceContent{{URI: requestedURI, MIMEType: "text/html", Text: htmlText}}, nil
			},
		})
	}
	if s != nil && s.workflows != nil {
		registry = append(registry, builtInResourceRegistration{
			descriptor: ResourceDescriptor{
				URI:         workflowManagerResourceURI,
				Name:        "Workflow Manager",
				Description: "Lightweight MCP app for workflow drafts, routing simulation, and hierarchy inspection.",
				MIMEType:    "text/html",
				Contract:    builtInResourceContract(workflowManagerResourceURI, []string{"configuration.read"}),
			},
			reader: func(s *Server, actor ActorContext, parsed *url.URL, requestedURI string) ([]ResourceContent, error) {
				htmlText, err := s.renderWorkflowManagerApp(actor, parsed)
				if err != nil {
					return nil, err
				}
				return []ResourceContent{{URI: requestedURI, MIMEType: "text/html", Text: htmlText}}, nil
			},
		})
	}
	if s != nil && s.config != nil {
		registry = append(registry,
			s.jsonControlResourceRegistration(configCatalogResourceURI, "Config Catalog", "Configuration definitions, entries, and effective values.", "configuration.read", (*Server).configCatalogResource),
			s.jsonControlResourceRegistration(flagCatalogResourceURI, "Feature Flag Catalog", "Feature flag definitions and stored values.", "configuration.read", (*Server).flagCatalogResource),
			s.jsonControlResourceRegistration(moduleCompatResourceURI, "Module Compatibility", "Installed module and kernel compatibility state.", "configuration.read", (*Server).moduleCompatibilityResource),
			s.jsonControlResourceRegistration(readinessResourceURI, "Implementation Readiness", "Readiness and validation snapshot for control-plane applies.", "configuration.read", (*Server).readinessResource),
			s.jsonControlResourceRegistration(implementationBlueprintsURI, "Implementation Blueprints", "Domain-agnostic implementation blueprint and desired-state guidance.", "configuration.read", (*Server).implementationBlueprintResource),
			s.jsonControlResourceRegistration(mcpCatalogResourceURI, "MCP Catalog", "Protocol versions, capability discovery, tools, resources, and app metadata for external agents.", "configuration.read", (*Server).mcpCatalogResource),
		)
	}
	if s != nil && s.identity != nil {
		registry = append(registry, s.jsonControlResourceRegistration(roleMatrixResourceURI, "Role Matrix", "Roles, permissions, grants, and bindings.", "identity.manage_users", (*Server).roleMatrixResource))
	}
	if s != nil && s.integration != nil {
		registry = append(registry, s.jsonControlResourceRegistration(integrationHealthResourceURI, "Integration Health", "Integration connector health and submission summary.", "configuration.read", (*Server).integrationHealthResource))
	}
	if s != nil && s.search != nil {
		registry = append(registry, s.jsonControlResourceRegistration(searchRuntimeResourceURI, "Search Runtime", "Search index runtime and consistency status.", "search.manage", (*Server).searchRuntimeResource))
	}
	if s != nil && s.offline != nil {
		registry = append(registry, s.jsonControlResourceRegistration(offlineOpsResourceURI, "Offline Sync", "Offline sync batches, outcomes, and conflicts.", "ops.read", (*Server).offlineSyncResource))
	}
	if s != nil && s.policy != nil {
		registry = append(registry, s.jsonControlResourceRegistration(policyRuntimeResourceURI, "Policy Runtime", "Policy hook runtime, compile, and evaluation status.", "configuration.read", (*Server).policyRuntimeResource))
	}
	if s != nil && s.reference != nil {
		registry = append(registry, s.jsonControlResourceRegistration(referenceCatalogResourceURI, "Reference Catalog", "Reference data types and records.", "configuration.read", (*Server).referenceCatalogResource))
	}
	if s != nil && s.health != nil {
		registry = append(registry, s.jsonControlResourceRegistration(runbooksResourceURI, "Runbooks", "Runtime health runbooks and operator hints.", "ops.read", (*Server).runbooksResource))
	}
	if s != nil && s.dataops != nil {
		registry = append(registry,
			s.jsonControlResourceRegistration(dataopsCatalogResourceURI, "DataOps Catalog", "Data class catalog and adapter capability matrix.", "configuration.read", (*Server).dataopsCatalogResource),
			s.jsonControlResourceRegistration(dataopsArtifactsResourceURI, "DataOps Artifacts", "Managed backup, archive, export, and migration artifacts.", "configuration.read", (*Server).dataopsArtifactsResource),
			s.jsonControlResourceRegistration(dataopsCheckpointsResourceURI, "DataOps Checkpoints", "Latest incremental checkpoints by data class and adapter.", "configuration.read", (*Server).dataopsCheckpointsResource),
		)
	}
	return registry
}

func (s *Server) jsonControlResourceRegistration(uri, name, description, permission string, provider func(*Server, ActorContext) (map[string]any, error)) builtInResourceRegistration {
	return builtInResourceRegistration{
		descriptor: ResourceDescriptor{
			URI:         uri,
			Name:        name,
			Description: description,
			MIMEType:    "application/json",
			Contract:    builtInResourceContract(uri, []string{permission}),
		},
		reader: func(s *Server, actor ActorContext, _ *url.URL, requestedURI string) ([]ResourceContent, error) {
			contents, _, err := s.readJSONControlResource(actor, requestedURI, permission, func(actor ActorContext) (map[string]any, error) {
				return provider(s, actor)
			})
			return contents, err
		},
	}
}

func builtInResourceBaseURI(parsed *url.URL) string {
	base := *parsed
	base.RawQuery = ""
	base.Fragment = ""
	return base.String()
}
