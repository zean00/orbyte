package mcp

import (
	"fmt"
	"net/url"
	"strings"
)

func (s *Server) listBuiltInResources(actor ActorContext) []ResourceDescriptor {
	items := make([]ResourceDescriptor, 0, 9)
	if s != nil && s.templates != nil && scopeMatches(actor.EndpointScope, "template") && allowsAll(actor.PermissionChecker, []string{"template.read"}) {
		items = append(items, ResourceDescriptor{
			URI:         templateDesignerResourceURI,
			Name:        "Template Designer",
			Description: "Lightweight MCP app for template draft inspection and preview.",
			Scope:       "template",
			MIMEType:    "text/html",
			Contract:    builtInResourceContract(templateDesignerResourceURI, []string{"template.read"}),
		})
	}
	if s != nil && s.analytics != nil && allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		if scopeMatches(actor.EndpointScope, EndpointScopeAnalytics) {
			items = append(items, ResourceDescriptor{
				URI:         analyticsStudioResourceURI,
				Name:        "Analytics Studio",
				Description: "Lightweight MCP app for analytics authoring, ad hoc results, and chart previews.",
				Scope:       EndpointScopeAnalytics,
				MIMEType:    "text/html",
				Contract:    builtInResourceContract(analyticsStudioResourceURI, []string{"analytics.read"}),
			})
		}
	}
	if s != nil && s.workflows != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{
			URI:         workflowManagerResourceURI,
			Name:        "Workflow Manager",
			Description: "Lightweight MCP app for workflow drafts, routing simulation, and hierarchy inspection.",
			MIMEType:    "text/html",
			Contract:    builtInResourceContract(workflowManagerResourceURI, []string{"configuration.read"}),
		})
	}
	if s != nil && s.config != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items,
			ResourceDescriptor{URI: configCatalogResourceURI, Name: "Config Catalog", Description: "Configuration definitions, entries, and effective values.", MIMEType: "application/json", Contract: builtInResourceContract(configCatalogResourceURI, []string{"configuration.read"})},
			ResourceDescriptor{URI: flagCatalogResourceURI, Name: "Feature Flag Catalog", Description: "Feature flag definitions and stored values.", MIMEType: "application/json", Contract: builtInResourceContract(flagCatalogResourceURI, []string{"configuration.read"})},
			ResourceDescriptor{URI: moduleCompatResourceURI, Name: "Module Compatibility", Description: "Installed module and kernel compatibility state.", MIMEType: "application/json", Contract: builtInResourceContract(moduleCompatResourceURI, []string{"configuration.read"})},
			ResourceDescriptor{URI: readinessResourceURI, Name: "Implementation Readiness", Description: "Readiness and validation snapshot for control-plane applies.", MIMEType: "application/json", Contract: builtInResourceContract(readinessResourceURI, []string{"configuration.read"})},
			ResourceDescriptor{URI: implementationBlueprintsURI, Name: "Implementation Blueprints", Description: "Domain-agnostic implementation blueprint and desired-state guidance.", MIMEType: "application/json", Contract: builtInResourceContract(implementationBlueprintsURI, []string{"configuration.read"})},
			ResourceDescriptor{URI: mcpCatalogResourceURI, Name: "MCP Catalog", Description: "Protocol versions, capability discovery, tools, resources, and app metadata for external agents.", MIMEType: "application/json", Contract: builtInResourceContract(mcpCatalogResourceURI, []string{"configuration.read"})},
		)
	}
	if s != nil && s.identity != nil && allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		items = append(items, ResourceDescriptor{URI: roleMatrixResourceURI, Name: "Role Matrix", Description: "Roles, permissions, grants, and bindings.", MIMEType: "application/json", Contract: builtInResourceContract(roleMatrixResourceURI, []string{"identity.manage_users"})})
	}
	if s != nil && s.integration != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{URI: integrationHealthResourceURI, Name: "Integration Health", Description: "Integration connector health and submission summary.", MIMEType: "application/json", Contract: builtInResourceContract(integrationHealthResourceURI, []string{"configuration.read"})})
	}
	if s != nil && s.search != nil && allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		items = append(items, ResourceDescriptor{URI: searchRuntimeResourceURI, Name: "Search Runtime", Description: "Search index runtime and consistency status.", MIMEType: "application/json", Contract: builtInResourceContract(searchRuntimeResourceURI, []string{"search.manage"})})
	}
	if s != nil && s.offline != nil && allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		items = append(items, ResourceDescriptor{URI: offlineOpsResourceURI, Name: "Offline Sync", Description: "Offline sync batches, outcomes, and conflicts.", MIMEType: "application/json", Contract: builtInResourceContract(offlineOpsResourceURI, []string{"ops.read"})})
	}
	if s != nil && s.policy != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{URI: policyRuntimeResourceURI, Name: "Policy Runtime", Description: "Policy hook runtime, compile, and evaluation status.", MIMEType: "application/json", Contract: builtInResourceContract(policyRuntimeResourceURI, []string{"configuration.read"})})
	}
	if s != nil && s.reference != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{URI: referenceCatalogResourceURI, Name: "Reference Catalog", Description: "Reference data types and records.", MIMEType: "application/json", Contract: builtInResourceContract(referenceCatalogResourceURI, []string{"configuration.read"})})
	}
	if s != nil && s.health != nil && allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		items = append(items, ResourceDescriptor{URI: runbooksResourceURI, Name: "Runbooks", Description: "Runtime health runbooks and operator hints.", MIMEType: "application/json", Contract: builtInResourceContract(runbooksResourceURI, []string{"ops.read"})})
	}
	if s != nil && s.dataops != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items,
			ResourceDescriptor{URI: dataopsCatalogResourceURI, Name: "DataOps Catalog", Description: "Data class catalog and adapter capability matrix.", MIMEType: "application/json", Contract: builtInResourceContract(dataopsCatalogResourceURI, []string{"configuration.read"})},
			ResourceDescriptor{URI: dataopsArtifactsResourceURI, Name: "DataOps Artifacts", Description: "Managed backup, archive, export, and migration artifacts.", MIMEType: "application/json", Contract: builtInResourceContract(dataopsArtifactsResourceURI, []string{"configuration.read"})},
			ResourceDescriptor{URI: dataopsCheckpointsResourceURI, Name: "DataOps Checkpoints", Description: "Latest incremental checkpoints by data class and adapter.", MIMEType: "application/json", Contract: builtInResourceContract(dataopsCheckpointsResourceURI, []string{"configuration.read"})},
		)
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
	if parsed.Host == "control-plane" {
		switch parsed.Path {
		case "/config.catalog":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.configCatalogResource)
		case "/feature-flags.catalog":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.flagCatalogResource)
		case "/role-matrix":
			return s.readJSONControlResource(actor, uri, "identity.manage_users", s.roleMatrixResource)
		case "/module-compatibility":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.moduleCompatibilityResource)
		case "/integration-health":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.integrationHealthResource)
		case "/readiness":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.readinessResource)
		case "/search-runtime":
			return s.readJSONControlResource(actor, uri, "search.manage", s.searchRuntimeResource)
		case "/offline-sync":
			return s.readJSONControlResource(actor, uri, "ops.read", s.offlineSyncResource)
		case "/policy-runtime":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.policyRuntimeResource)
		case "/reference-catalog":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.referenceCatalogResource)
		case "/implementation-blueprints":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.implementationBlueprintResource)
		case "/mcp-catalog":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.mcpCatalogResource)
		case "/runbooks":
			return s.readJSONControlResource(actor, uri, "ops.read", s.runbooksResource)
		case "/dataops/catalog":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.dataopsCatalogResource)
		case "/dataops/artifacts":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.dataopsArtifactsResource)
		case "/dataops/checkpoints":
			return s.readJSONControlResource(actor, uri, "configuration.read", s.dataopsCheckpointsResource)
		default:
			return nil, false, nil
		}
	}
	if parsed.Host != "apps" {
		return nil, false, nil
	}
	switch parsed.Path {
	case "/template.designer":
		if !scopeMatches(actor.EndpointScope, "template") {
			return nil, true, fmt.Errorf("resource is not available on this endpoint")
		}
		if s == nil || s.templates == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			return nil, true, fmt.Errorf("resource is not allowed")
		}
		htmlText, err := s.renderTemplateDesignerApp(actor, parsed)
		if err != nil {
			return nil, true, err
		}
		return []ResourceContent{{URI: uri, MIMEType: "text/html", Text: htmlText}}, true, nil
	case "/analytics.studio":
		if !scopeMatches(actor.EndpointScope, EndpointScopeAnalytics) {
			return nil, true, fmt.Errorf("resource is not available on this endpoint")
		}
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
			return nil, true, fmt.Errorf("resource is not allowed")
		}
		htmlText, err := s.renderAnalyticsStudioApp(actor, parsed)
		if err != nil {
			return nil, true, err
		}
		return []ResourceContent{{URI: uri, MIMEType: "text/html", Text: htmlText}}, true, nil
	case "/workflow.manager":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
			return nil, true, fmt.Errorf("resource is not allowed")
		}
		htmlText, err := s.renderWorkflowManagerApp(actor, parsed)
		if err != nil {
			return nil, true, err
		}
		return []ResourceContent{{URI: uri, MIMEType: "text/html", Text: htmlText}}, true, nil
	default:
		return nil, false, nil
	}
}
