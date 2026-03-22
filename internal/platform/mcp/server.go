package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/dataops"
	"orbyte/internal/platform/engagement"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

const ProtocolVersion = "2024-11-05"

const (
	templateDesignerResourceURI   = "orbyte://apps/template.designer"
	templateDesignerAppKey        = "template.designer"
	analyticsStudioResourceURI    = "orbyte://apps/analytics.studio"
	analyticsStudioAppKey         = "analytics.studio"
	workflowManagerResourceURI    = "orbyte://apps/workflow.manager"
	workflowManagerAppKey         = "workflow.manager"
	configCatalogResourceURI      = "orbyte://control-plane/config.catalog"
	flagCatalogResourceURI        = "orbyte://control-plane/feature-flags.catalog"
	roleMatrixResourceURI         = "orbyte://control-plane/role-matrix"
	moduleCompatResourceURI       = "orbyte://control-plane/module-compatibility"
	integrationHealthResourceURI  = "orbyte://control-plane/integration-health"
	readinessResourceURI          = "orbyte://control-plane/readiness"
	runbooksResourceURI           = "orbyte://control-plane/runbooks"
	searchRuntimeResourceURI      = "orbyte://control-plane/search-runtime"
	offlineOpsResourceURI         = "orbyte://control-plane/offline-sync"
	policyRuntimeResourceURI      = "orbyte://control-plane/policy-runtime"
	referenceCatalogResourceURI   = "orbyte://control-plane/reference-catalog"
	implementationBlueprintsURI   = "orbyte://control-plane/implementation-blueprints"
	dataopsCatalogResourceURI     = "orbyte://control-plane/dataops/catalog"
	dataopsArtifactsResourceURI   = "orbyte://control-plane/dataops/artifacts"
	dataopsCheckpointsResourceURI = "orbyte://control-plane/dataops/checkpoints"
)

const (
	EndpointScopeAll       = ""
	EndpointScopeAnalytics = "analytics"
)

type PermissionChecker func(permissionKey string) bool

type ActorContext struct {
	ActorID            string
	ActorKind          string
	SessionID          string
	ServicePrincipalID string
	EffectiveUserID    string
	OnBehalfOfUserID   string
	DelegationGrantID  string
	OrganizationID     string
	LocationID         string
	EndpointScope      string
	PermissionChecker  PermissionChecker
}

type Server struct {
	modules                   *module.Service
	analytics                 *analytics.Service
	templates                 *templateoutput.Service
	workflows                 *workflow.Service
	identity                  *identity.Service
	config                    *config.Service
	flags                     *featureflags.Service
	integration               *integration.Service
	reference                 *reference.Service
	search                    *search.Service
	policy                    *policy.Service
	eventing                  *eventing.Service
	jobs                      *jobs.Service
	health                    *runtimehealth.Tracker
	audit                     *audit.Service
	observability             *observability.Service
	offline                   *offline.Service
	dataops                   *dataops.Service
	engagement                *engagement.Service
	implementation            *ImplementationService
	analyticsStreamPath       string
	analyticsScopedStreamPath string
}

func NewServer(modules *module.Service, analyticsSvc *analytics.Service, templates *templateoutput.Service, workflows *workflow.Service, identitySvc *identity.Service, configSvc *config.Service, flagsSvc *featureflags.Service, integrationSvc *integration.Service, referenceSvc *reference.Service, searchSvc *search.Service, policySvc *policy.Service, eventingSvc *eventing.Service, jobSvc *jobs.Service, health *runtimehealth.Tracker, auditSvc *audit.Service, obs *observability.Service, offlineSvc *offline.Service, dataopsSvc *dataops.Service, engagementSvc *engagement.Service, analyticsStreamPath, analyticsScopedStreamPath string) *Server {
	return &Server{
		modules:                   modules,
		analytics:                 analyticsSvc,
		templates:                 templates,
		workflows:                 workflows,
		identity:                  identitySvc,
		config:                    configSvc,
		flags:                     flagsSvc,
		integration:               integrationSvc,
		reference:                 referenceSvc,
		search:                    searchSvc,
		policy:                    policySvc,
		eventing:                  eventingSvc,
		jobs:                      jobSvc,
		health:                    health,
		audit:                     auditSvc,
		observability:             obs,
		offline:                   offlineSvc,
		dataops:                   dataopsSvc,
		engagement:                engagementSvc,
		implementation:            NewImplementationService(),
		analyticsStreamPath:       analyticsStreamPath,
		analyticsScopedStreamPath: analyticsScopedStreamPath,
	}
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolDescriptor struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Scope       string         `json:"scope,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

type ResourceDescriptor struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Scope       string `json:"scope,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func (s *Server) Handle(ctx context.Context, req JSONRPCRequest, actor ActorContext) JSONRPCResponse {
	if strings.TrimSpace(req.JSONRPC) == "" {
		req.JSONRPC = "2.0"
	}
	switch req.Method {
	case "initialize":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": ProtocolVersion,
			"serverInfo": map[string]any{
				"name":    "orbyte-mcp",
				"version": "1.0.0",
			},
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
			},
		}}
	case "tools/list":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": s.listTools(actor)}}
	case "resources/list":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"resources": s.listResources(actor)}}
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(req.Params, &params)
		contents, err := s.readResource(actor, params.URI)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"contents": contents}}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &params)
		result, err := s.callTool(ctx, actor, params.Name, params.Arguments)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCError{Code: -32601, Message: "method not found"}}
	}
}

func (s *Server) listTools(actor ActorContext) []ToolDescriptor {
	items := make([]ToolDescriptor, 0)
	if s == nil || s.modules == nil {
		return items
	}
	items = append(items, s.listBuiltInTools(actor)...)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		scope := scopeForModule(detail.Manifest.Key)
		if !scopeMatches(actor.EndpointScope, scope) {
			continue
		}
		for _, def := range detail.Manifest.MCP.Tools {
			if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
				continue
			}
			items = append(items, ToolDescriptor{
				Name:        def.Key,
				Title:       def.Title,
				Description: def.Description,
				Scope:       scope,
				InputSchema: cloneMap(def.InputSchema),
			})
		}
	}
	return items
}

func (s *Server) listResources(actor ActorContext) []ResourceDescriptor {
	items := make([]ResourceDescriptor, 0)
	if s == nil || s.modules == nil {
		return items
	}
	items = append(items, s.listBuiltInResources(actor)...)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		scope := scopeForModule(detail.Manifest.Key)
		if !scopeMatches(actor.EndpointScope, scope) {
			continue
		}
		for _, def := range detail.Manifest.MCP.Resources {
			if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
				continue
			}
			items = append(items, ResourceDescriptor{
				URI:         def.URI,
				Name:        def.Title,
				Description: def.Description,
				Scope:       scope,
				MIMEType:    def.MIMEType,
			})
		}
	}
	return items
}

func (s *Server) readResource(actor ActorContext, uri string) ([]ResourceContent, error) {
	if s == nil || s.modules == nil {
		return nil, fmt.Errorf("mcp resources are unavailable")
	}
	if contents, ok, err := s.readBuiltInResource(actor, uri); ok {
		return contents, err
	}
	def, ok := s.lookupResourceByURI(actor.EndpointScope, uri)
	if !ok {
		if _, found := s.lookupResourceByURI(EndpointScopeAll, uri); found {
			return nil, fmt.Errorf("resource is not available on this endpoint")
		}
		return nil, fmt.Errorf("resource not found")
	}
	if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
		return nil, fmt.Errorf("resource is not allowed")
	}
	switch def.Provider {
	case "analytics.snapshot.current":
		payload, err := s.analyticsSnapshotPayload(actor)
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(payload)
		return []ResourceContent{{URI: def.URI, MIMEType: firstNonEmpty(def.MIMEType, "application/json"), Text: string(body)}}, nil
	case "mcp.app":
		appDef, ok := s.lookupApp(actor.EndpointScope, def.AppKey)
		if !ok {
			return nil, fmt.Errorf("app not found")
		}
		html, err := s.renderApp(actor, appDef)
		if err != nil {
			return nil, err
		}
		return []ResourceContent{{URI: def.URI, MIMEType: firstNonEmpty(def.MIMEType, "text/html"), Text: html}}, nil
	default:
		return nil, fmt.Errorf("unsupported resource provider")
	}
}

func (s *Server) callTool(_ context.Context, actor ActorContext, name string, arguments map[string]any) (map[string]any, error) {
	if s == nil || s.modules == nil {
		return nil, fmt.Errorf("mcp tools are unavailable")
	}
	if result, ok, err := s.callBuiltInTool(actor, strings.TrimSpace(name), arguments); ok {
		return result, err
	}
	def, ok := s.lookupTool(actor.EndpointScope, strings.TrimSpace(name))
	if !ok {
		if _, found := s.lookupTool(EndpointScopeAll, strings.TrimSpace(name)); found {
			return nil, fmt.Errorf("tool is not available on this endpoint")
		}
		return nil, fmt.Errorf("tool not found")
	}
	if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
		return nil, fmt.Errorf("tool is not allowed")
	}
	switch def.Operation {
	case "analytics.snapshot.get":
		payload, err := s.analyticsSnapshotPayload(actor)
		if err != nil {
			return nil, err
		}
		result := map[string]any{
			"content": []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Analytics snapshot generated at %s with %d submitted documents.", payload.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"), payload.Documents.Submitted),
			}},
			"structuredContent": payload,
		}
		if def.AppKey != "" {
			if appDef, ok := s.lookupApp(actor.EndpointScope, def.AppKey); ok {
				if resource, ok := s.lookupResourceByKey(actor.EndpointScope, appDef.ResourceKey); ok {
					result["_meta"] = map[string]any{
						"orbyte/app": map[string]any{
							"key":          appDef.Key,
							"title":        appDef.Title,
							"resource_uri": resource.URI,
							"stream_uri":   s.preferredAnalyticsStreamPath(),
						},
					}
				}
			}
		}
		return result, nil
	default:
		_ = arguments
		return nil, fmt.Errorf("unsupported tool operation")
	}
}

func (s *Server) listBuiltInTools(actor ActorContext) []ToolDescriptor {
	type builtInTool struct {
		name        string
		title       string
		description string
		permission  string
		inputSchema map[string]any
	}
	defs := make([]builtInTool, 0)
	if s != nil && s.templates != nil {
		defs = append(defs,
			builtInTool{
				name:        "template.definition.list",
				title:       "List Template Definitions",
				description: "List available print template definitions.",
				permission:  "template.read",
			},
			builtInTool{
				name:        "template.definition.get",
				title:       "Get Template Definition",
				description: "Get one template definition and version metadata.",
				permission:  "template.read",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
			},
			builtInTool{
				name:        "template.draft.get",
				title:       "Get Template Draft",
				description: "Load the latest draft or defaults for a template.",
				permission:  "template.read",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
			},
			builtInTool{
				name:        "template.draft.save",
				title:       "Save Template Draft",
				description: "Create or update a template draft.",
				permission:  "template.manage",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
			},
			builtInTool{
				name:        "template.render.preview",
				title:       "Preview Template Render",
				description: "Render a template preview in HTML or the requested output format.",
				permission:  "template.render",
				inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}},
			},
		)
	}
	if s != nil && s.analytics != nil {
		defs = append(defs,
			builtInTool{name: "analytics.dashboard.list", title: "List Dashboards", description: "List runtime analytics dashboards.", permission: "analytics.read"},
			builtInTool{name: "analytics.dashboard.get", title: "Get Dashboard", description: "Get one runtime analytics dashboard.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dashboard_id": map[string]any{"type": "string"}}, "required": []string{"dashboard_id"}}},
			builtInTool{name: "analytics.dashboard.save", title: "Save Dashboard", description: "Create or update a runtime analytics dashboard.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dashboard": map[string]any{"type": "object"}}, "required": []string{"dashboard"}}},
			builtInTool{name: "analytics.dashboard.delete", title: "Delete Dashboard", description: "Delete a runtime analytics dashboard.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dashboard_id": map[string]any{"type": "string"}}, "required": []string{"dashboard_id"}}},
			builtInTool{name: "analytics.metric.list", title: "List Saved Metrics", description: "List runtime analytics saved metrics.", permission: "analytics.read"},
			builtInTool{name: "analytics.metric.get", title: "Get Saved Metric", description: "Get one runtime analytics saved metric.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"metric_id": map[string]any{"type": "string"}}, "required": []string{"metric_id"}}},
			builtInTool{name: "analytics.metric.save", title: "Save Saved Metric", description: "Create or update a runtime analytics saved metric.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"metric": map[string]any{"type": "object"}}, "required": []string{"metric"}}},
			builtInTool{name: "analytics.metric.delete", title: "Delete Saved Metric", description: "Delete a runtime analytics saved metric.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"metric_id": map[string]any{"type": "string"}}, "required": []string{"metric_id"}}},
			builtInTool{name: "analytics.query.list", title: "List Saved Queries", description: "List runtime analytics saved queries.", permission: "analytics.read"},
			builtInTool{name: "analytics.query.get", title: "Get Saved Query", description: "Get one runtime analytics saved query.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query_id": map[string]any{"type": "string"}}, "required": []string{"query_id"}}},
			builtInTool{name: "analytics.query.save", title: "Save Saved Query", description: "Create or update a runtime analytics saved query.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "object"}}, "required": []string{"query"}}},
			builtInTool{name: "analytics.query.delete", title: "Delete Saved Query", description: "Delete a runtime analytics saved query.", permission: "analytics.author", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query_id": map[string]any{"type": "string"}}, "required": []string{"query_id"}}},
			builtInTool{name: "analytics.query.execute", title: "Execute Analytics Query", description: "Run an ad hoc analytics query and return table plus chart data.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "object"}}, "required": []string{"query"}}},
			builtInTool{name: "analytics.chart.generate", title: "Generate Analytics Chart", description: "Generate a normalized chart spec from a query or execution result.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "object"}, "result": map[string]any{"type": "object"}}}},
			builtInTool{name: "analytics.report.definition.list", title: "List Report Definitions", description: "List analytics report definitions.", permission: "analytics.read"},
			builtInTool{name: "analytics.report.definition.get", title: "Get Report Definition", description: "Get one analytics report definition.", permission: "analytics.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report_id": map[string]any{"type": "string"}}, "required": []string{"report_id"}}},
			builtInTool{name: "analytics.report.definition.save", title: "Save Report Definition", description: "Create or update an analytics report definition.", permission: "analytics.manage_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report": map[string]any{"type": "object"}}, "required": []string{"report"}}},
			builtInTool{name: "analytics.report.definition.delete", title: "Delete Report Definition", description: "Delete an analytics report definition.", permission: "analytics.manage_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report_id": map[string]any{"type": "string"}}, "required": []string{"report_id"}}},
			builtInTool{name: "analytics.report.run", title: "Run Analytics Report", description: "Run a stored analytics report definition.", permission: "analytics.manage_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"report_id": map[string]any{"type": "string"}}, "required": []string{"report_id"}}},
			builtInTool{name: "analytics.report.deliver", title: "Deliver Analytics Report", description: "Deliver a report artifact or run a report and deliver it.", permission: "analytics.deliver_reports", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "report_id": map[string]any{"type": "string"}, "channel": map[string]any{"type": "string"}, "recipient": map[string]any{"type": "string"}}}},
		)
	}
	if s != nil && s.workflows != nil {
		defs = append(defs,
			builtInTool{name: "workflow.definition.list", title: "List Workflow Definitions", description: "List workflow definitions and published versions.", permission: "configuration.read"},
			builtInTool{name: "workflow.definition.get", title: "Get Workflow Definition", description: "Get one workflow definition plus versions and current draft.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}}, "required": []string{"workflow_key"}}},
			builtInTool{name: "workflow.version.list", title: "List Workflow Versions", description: "List all workflow versions for a definition.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}}, "required": []string{"workflow_key"}}},
			builtInTool{name: "workflow.draft.create", title: "Create Workflow Draft", description: "Create a new workflow draft from the current published version.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}}, "required": []string{"workflow_key"}}},
			builtInTool{name: "workflow.draft.get", title: "Get Workflow Draft", description: "Load the current workflow draft or a draft version.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}}, "required": []string{"workflow_key"}}},
			builtInTool{name: "workflow.draft.save", title: "Save Workflow Draft", description: "Create or update a workflow draft definition.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow": map[string]any{"type": "object"}}, "required": []string{"workflow"}}},
			builtInTool{name: "workflow.draft.validate", title: "Validate Workflow Draft", description: "Validate a workflow draft or draft version.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow": map[string]any{"type": "object"}, "workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}}}},
			builtInTool{name: "workflow.draft.simulate", title: "Simulate Workflow Draft", description: "Simulate a workflow transition and preview routing.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow": map[string]any{"type": "object"}, "workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "input": map[string]any{"type": "object"}}}},
			builtInTool{name: "workflow.draft.publish", title: "Publish Workflow Draft", description: "Publish a workflow draft version. Requires explicit confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "confirm_publish": map[string]any{"type": "boolean"}}, "required": []string{"workflow_key", "version", "confirm_publish"}}},
			builtInTool{name: "workflow.runtime.tasks.list", title: "List Workflow Tasks", description: "List read-only workflow tasks.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "assignee_user_id": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "workflow.runtime.approvals.list", title: "List Workflow Approvals", description: "List read-only workflow approvals.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"workflow_key": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}, "stage_key": map[string]any{"type": "string"}}}},
			builtInTool{name: "workflow.runtime.history.get", title: "Get Workflow History", description: "Get workflow history for one target.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"target_type": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}}, "required": []string{"target_type", "target_id"}}},
			builtInTool{name: "workflow.hierarchy.graph.get", title: "Get Workflow Hierarchy Graph", description: "Get the reporting-line graph for workflow routing.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}}}},
			builtInTool{name: "workflow.hierarchy.chain.get", title: "Get Workflow Hierarchy Chain", description: "Get the manager chain for a user.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"user_id": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}, "required": []string{"user_id"}}},
			builtInTool{name: "workflow.hierarchy.summary.get", title: "Get Workflow Hierarchy Summary", description: "Get hierarchy coverage and exception summary.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}}}},
			builtInTool{name: "workflow.reporting_line.list", title: "List Reporting Lines", description: "List reporting lines used for workflow routing.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"subject_user_id": map[string]any{"type": "string"}, "manager_user_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}}}},
			builtInTool{name: "workflow.reporting_line.save", title: "Save Reporting Line", description: "Create or update a reporting line for workflow routing.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"reporting_line": map[string]any{"type": "object"}}, "required": []string{"reporting_line"}}},
		)
	}
	if s != nil && s.config != nil {
		defs = append(defs,
			builtInTool{name: "config.definition.list", title: "List Config Definitions", description: "List configuration definitions and allowed scopes.", permission: "configuration.read"},
			builtInTool{name: "config.entry.list", title: "List Config Entries", description: "List stored configuration entries.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"config_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "config_scopes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}},
			builtInTool{name: "config.effective.get", title: "Get Effective Config", description: "Get effective configuration for a context.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "config.compare", title: "Compare Config Contexts", description: "Compare effective configuration across two contexts.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"left": map[string]any{"type": "object"}, "right": map[string]any{"type": "object"}}}},
			builtInTool{name: "config.bundle.export", title: "Export Config Bundle", description: "Export config and feature flag values into a promotion bundle.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "config_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "config_scopes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "include_flags": map[string]any{"type": "boolean"}, "flag_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "flag_scopes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}},
			builtInTool{name: "config.bundle.validate", title: "Validate Config Bundle", description: "Validate a configuration bundle without applying it.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"bundle": map[string]any{"type": "object"}}, "required": []string{"bundle"}}},
			builtInTool{name: "config.bundle.apply", title: "Apply Config Bundle", description: "Apply a validated configuration bundle. Requires explicit confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"bundle": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"bundle", "confirm_apply"}}},
		)
	}
	if s != nil && s.flags != nil {
		defs = append(defs,
			builtInTool{name: "feature_flag.definition.list", title: "List Feature Flag Definitions", description: "List feature flag definitions.", permission: "configuration.read"},
			builtInTool{name: "feature_flag.value.list", title: "List Feature Flag Values", description: "List stored feature flag values.", permission: "configuration.read"},
			builtInTool{name: "feature_flag.targeting.get", title: "Get Feature Flag Targeting", description: "Inspect raw overrides and effective resolution for one feature flag.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"flag_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}, "required": []string{"flag_key"}}},
			builtInTool{name: "feature_flag.value.upsert", title: "Upsert Feature Flag Value", description: "Create or update a feature flag override. Requires explicit confirmation when activating changes.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"value", "confirm_apply"}}},
		)
	}
	if s != nil && s.identity != nil {
		defs = append(defs,
			builtInTool{name: "identity.role_permission_matrix.get", title: "Get Role Permission Matrix", description: "Get roles, permissions, grants, and bindings in matrix form.", permission: "identity.manage_users"},
			builtInTool{name: "identity.role_permission.grant", title: "Grant Role Permission", description: "Grant a permission to an existing role. Requires confirmation.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"role_id": map[string]any{"type": "string"}, "permission_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"role_id", "permission_key", "confirm_apply"}}},
			builtInTool{name: "identity.role_permission.revoke", title: "Revoke Role Permission", description: "Revoke a permission from an existing role. Requires confirmation.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"role_id": map[string]any{"type": "string"}, "permission_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"role_id", "permission_key", "confirm_apply"}}},
			builtInTool{name: "identity.role_binding.list", title: "List Role Bindings", description: "List current role bindings.", permission: "identity.manage_users"},
			builtInTool{name: "identity.role_binding.priority.set", title: "Set Role Binding Priority", description: "Set role binding priority. Requires confirmation.", permission: "identity.manage_users", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"binding_id": map[string]any{"type": "string"}, "priority": map[string]any{"type": "integer"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"binding_id", "priority", "confirm_apply"}}},
		)
	}
	if s != nil && s.modules != nil {
		defs = append(defs,
			builtInTool{name: "module.list", title: "List Modules", description: "List installed modules and lifecycle state.", permission: "configuration.read"},
			builtInTool{name: "module.compatibility.list", title: "List Module Compatibility", description: "List module compatibility diagnostics.", permission: "configuration.read"},
			builtInTool{name: "module.enable", title: "Enable Module", description: "Enable one module. Requires confirmation.", permission: "module.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"module_key", "confirm_apply"}}},
			builtInTool{name: "module.disable", title: "Disable Module", description: "Disable one module. Requires confirmation.", permission: "module.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"module_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"module_key", "confirm_apply"}}},
		)
	}
	if s != nil && s.search != nil {
		defs = append(defs,
			builtInTool{name: "search.runtime.list", title: "List Search Runtime", description: "List search indexes and runtime status.", permission: "search.manage"},
			builtInTool{name: "search.runtime.get", title: "Get Search Runtime", description: "Get runtime state for one search index.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}}, "required": []string{"index_key"}}},
			builtInTool{name: "search.consistency.get", title: "Get Search Consistency", description: "Get consistency report for one search index.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}}, "required": []string{"index_key"}}},
			builtInTool{name: "search.rebuild", title: "Rebuild Search Index", description: "Rebuild one search index. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}},
			builtInTool{name: "search.repair", title: "Repair Search Index", description: "Repair one search index. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "mode": map[string]any{"type": "string"}, "target_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}},
			builtInTool{name: "search.reconcile", title: "Reconcile Search Index", description: "Run a consistency scan for one search index. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}},
			builtInTool{name: "search.schema.plan", title: "Plan Search Schema", description: "Plan a candidate schema version for one index.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "string"}}, "required": []string{"index_key", "version"}}},
			builtInTool{name: "search.schema.build", title: "Build Search Schema", description: "Build the candidate search schema. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}},
			builtInTool{name: "search.schema.activate", title: "Activate Search Schema", description: "Activate the candidate search schema. Requires confirmation.", permission: "search.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"index_key": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"index_key", "confirm_apply"}}},
		)
	}
	if s != nil && s.offline != nil {
		defs = append(defs,
			builtInTool{name: "offline.sync.list", title: "List Offline Sync Batches", description: "List offline sync batches and recent outcomes.", permission: "ops.read"},
			builtInTool{name: "offline.sync.get", title: "Get Offline Sync Batch", description: "Get one offline sync batch and its outcomes.", permission: "ops.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"batch_id": map[string]any{"type": "string"}}, "required": []string{"batch_id"}}},
			builtInTool{name: "offline.conflict.list", title: "List Offline Conflicts", description: "List offline sync conflicts.", permission: "ops.read"},
		)
	}
	if s != nil && s.policy != nil {
		defs = append(defs,
			builtInTool{name: "policy.hook.list", title: "List Policy Hook Runtime", description: "List policy hook runtimes.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "policy.hook.get", title: "Get Policy Hook Runtime", description: "Get one policy hook runtime.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"hook_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"hook_key"}}},
			builtInTool{name: "policy.module.upsert", title: "Update Policy Module", description: "Update Rego source for a policy hook. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"hook_key": map[string]any{"type": "string"}, "scope": map[string]any{"type": "string"}, "scope_id": map[string]any{"type": "string"}, "source": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"hook_key", "source", "confirm_apply"}}},
		)
	}
	if s != nil && s.reference != nil {
		defs = append(defs,
			builtInTool{name: "reference.type.list", title: "List Reference Types", description: "List reference data types.", permission: "configuration.read"},
			builtInTool{name: "reference.record.list", title: "List Reference Records", description: "List records for one reference type.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"type_key": map[string]any{"type": "string"}}, "required": []string{"type_key"}}},
			builtInTool{name: "reference.resolve", title: "Resolve Reference Records", description: "Resolve effective records for one reference type.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"type_key": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}}, "required": []string{"type_key"}}},
			builtInTool{name: "reference.record.upsert", title: "Upsert Reference Record", description: "Create or update a reference record. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"record": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"record", "confirm_apply"}}},
		)
	}
	if s != nil && s.integration != nil {
		defs = append(defs,
			builtInTool{name: "integration.adapter.list", title: "List Integration Adapters", description: "List registered integration adapters and config schema.", permission: "configuration.read"},
			builtInTool{name: "integration.system.list", title: "List Integration Systems", description: "List integration systems.", permission: "configuration.read"},
			builtInTool{name: "integration.system.config.get", title: "Get Integration System Config", description: "Inspect one integration system config.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"system_key": map[string]any{"type": "string"}}, "required": []string{"system_key"}}},
			builtInTool{name: "integration.system.config.update", title: "Update Integration System Config", description: "Update integration system config. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"system_key": map[string]any{"type": "string"}, "settings": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"system_key", "settings", "confirm_apply"}}},
			builtInTool{name: "integration.endpoint.list", title: "List Integration Endpoints", description: "List integration endpoints.", permission: "configuration.read"},
			builtInTool{name: "integration.endpoint.config.get", title: "Get Integration Endpoint Config", description: "Inspect one integration endpoint config.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"endpoint_key": map[string]any{"type": "string"}}, "required": []string{"endpoint_key"}}},
			builtInTool{name: "integration.endpoint.config.update", title: "Update Integration Endpoint Config", description: "Update integration endpoint config. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"endpoint_key": map[string]any{"type": "string"}, "settings": map[string]any{"type": "object"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"endpoint_key", "settings", "confirm_apply"}}},
			builtInTool{name: "integration.submission.list", title: "List Integration Submissions", description: "List integration submissions.", permission: "configuration.read"},
			builtInTool{name: "integration.submission.get", title: "Get Integration Submission", description: "Inspect one integration submission.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"submission_id": map[string]any{"type": "string"}}, "required": []string{"submission_id"}}},
			builtInTool{name: "integration.dead_letter.list", title: "List Integration Dead Letters", description: "List integration dead letters.", permission: "configuration.read"},
			builtInTool{name: "integration.dead_letter.replay", title: "Replay Integration Dead Letter", description: "Replay one integration dead letter. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"dead_letter_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"dead_letter_id", "confirm_apply"}}},
		)
	}
	if s != nil && s.dataops != nil {
		defs = append(defs,
			builtInTool{name: "dataops.artifact.list", title: "List Data Operation Artifacts", description: "List backup, archive, export, and migration artifacts.", permission: "configuration.read"},
			builtInTool{name: "dataops.artifact.get", title: "Get Data Operation Artifact", description: "Get one managed data operation artifact.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}}, "required": []string{"artifact_id"}}},
			builtInTool{name: "dataops.operation.get", title: "Get Data Operation Run", description: "Get one data operation execution run.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"operation_id": map[string]any{"type": "string"}}, "required": []string{"operation_id"}}},
			builtInTool{name: "dataops.checkpoint.list", title: "List Data Operation Checkpoints", description: "List incremental checkpoints by adapter and data class.", permission: "configuration.read"},
			builtInTool{name: "dataops.backup.plan", title: "Plan Backup", description: "Plan a class-selective backup, including incremental mode.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "incremental": map[string]any{"type": "boolean"}, "name": map[string]any{"type": "string"}}, "required": []string{"selected_data_classes"}}},
			builtInTool{name: "dataops.backup.run", title: "Run Backup", description: "Run a managed backup artifact creation. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "incremental": map[string]any{"type": "boolean"}, "name": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"selected_data_classes", "confirm_apply"}}},
			builtInTool{name: "dataops.restore.plan", title: "Plan Restore", description: "Build a restore plan from a managed backup or archive artifact.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"artifact_id"}}},
			builtInTool{name: "dataops.restore.validate", title: "Validate Restore", description: "Validate a restore plan without applying it.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"artifact_id"}}},
			builtInTool{name: "dataops.restore.run", title: "Run Restore", description: "Run a class-selective restore. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"artifact_id", "confirm_apply"}}},
			builtInTool{name: "dataops.archive.plan", title: "Plan Archive", description: "Plan a transactional archive operation.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "document_types": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "statuses": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "created_before": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}, "required": []string{"selected_data_classes"}}},
			builtInTool{name: "dataops.archive.run", title: "Run Archive", description: "Archive transactional data and remove it from live storage. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "document_types": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "statuses": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "created_before": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"selected_data_classes", "confirm_apply"}}},
			builtInTool{name: "dataops.export.plan", title: "Plan Export", description: "Plan a structured external export by data class.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "name": map[string]any{"type": "string"}}, "required": []string{"selected_data_classes"}}},
			builtInTool{name: "dataops.export.run", title: "Run Export", description: "Create a structured external export artifact. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "name": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"selected_data_classes", "confirm_apply"}}},
			builtInTool{name: "dataops.migration.register", title: "Register Migration Input", description: "Register structured migration input as a managed artifact.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "segments": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}, "required": []string{"selected_data_classes", "segments"}}},
			builtInTool{name: "dataops.migration.plan", title: "Plan Migration", description: "Build a migration apply plan from a registered migration artifact.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"artifact_id"}}},
			builtInTool{name: "dataops.migration.validate", title: "Validate Migration", description: "Validate a migration artifact without applying it.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"artifact_id"}}},
			builtInTool{name: "dataops.migration.run", title: "Run Migration", description: "Apply a validated migration artifact. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"artifact_id": map[string]any{"type": "string"}, "selected_data_classes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"artifact_id", "confirm_apply"}}},
		)
	}
	if s != nil && s.engagement != nil {
		defs = append(defs,
			builtInTool{name: "engagement.program.list", title: "List Engagement Programs", description: "List engagement programs.", permission: "configuration.read"},
			builtInTool{name: "engagement.program.get", title: "Get Engagement Program", description: "Get one engagement program.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}}, "required": []string{"program_key"}}},
			builtInTool{name: "engagement.program.create", title: "Create Engagement Program", description: "Create one engagement program. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}, "subject_type": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"program_key", "confirm_apply"}}},
			builtInTool{name: "engagement.program.update", title: "Update Engagement Program", description: "Update one engagement program. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}, "subject_type": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"program_key", "confirm_apply"}}},
			builtInTool{name: "engagement.program.version.create", title: "Create Engagement Draft Version", description: "Create a new draft version for an engagement program. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "change_note": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"program_key", "confirm_apply"}}},
			builtInTool{name: "engagement.program.version.save", title: "Save Engagement Draft Version", description: "Save rules onto an engagement draft version. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "rules": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}, "change_note": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"program_key", "version", "rules", "confirm_apply"}}},
			builtInTool{name: "engagement.program.version.validate", title: "Validate Engagement Version", description: "Validate one engagement program version.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}}, "required": []string{"program_key", "version"}}},
			builtInTool{name: "engagement.program.version.publish", title: "Publish Engagement Version", description: "Publish one engagement program version. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"program_key", "version", "confirm_apply"}}},
			builtInTool{name: "engagement.subject.get", title: "Get Engagement Subject", description: "Get balances, qualification, achievements, and recent journal for one subject.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "subject_id": map[string]any{"type": "string"}}, "required": []string{"program_key", "subject_id"}}},
			builtInTool{name: "engagement.account.list", title: "List Engagement Accounts", description: "List accounts for one engagement subject.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "subject_id": map[string]any{"type": "string"}}, "required": []string{"program_key", "subject_id"}}},
			builtInTool{name: "engagement.balance.get", title: "Get Engagement Balance", description: "Get one engagement account balance.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "subject_id": map[string]any{"type": "string"}, "account_key": map[string]any{"type": "string"}}, "required": []string{"program_key", "subject_id"}}},
			builtInTool{name: "engagement.journal.list", title: "List Engagement Journal", description: "List journal entries for one program or subject.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "subject_id": map[string]any{"type": "string"}, "account_key": map[string]any{"type": "string"}}, "required": []string{"program_key"}}},
			builtInTool{name: "engagement.qualification.get", title: "Get Engagement Qualification", description: "Get qualification or tier state for one subject.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "subject_id": map[string]any{"type": "string"}}, "required": []string{"program_key", "subject_id"}}},
			builtInTool{name: "engagement.achievement.list", title: "List Engagement Achievements", description: "List granted achievements for one subject.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "subject_id": map[string]any{"type": "string"}}, "required": []string{"program_key", "subject_id"}}},
			builtInTool{name: "engagement.consumer.list", title: "List Engagement Consumers", description: "List engagement consumer state.", permission: "configuration.read"},
			builtInTool{name: "engagement.consumer.get", title: "Get Engagement Consumer", description: "Get one engagement consumer state.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"consumer_id": map[string]any{"type": "string"}}, "required": []string{"consumer_id"}}},
			builtInTool{name: "engagement.replay.plan", title: "Plan Engagement Replay", description: "Plan a replay for one engagement program version.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}}, "required": []string{"program_key"}}},
			builtInTool{name: "engagement.replay.run", title: "Run Engagement Replay", description: "Queue a replay for one engagement program version. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"program_key", "confirm_apply"}}},
			builtInTool{name: "engagement.replay.get", title: "Get Engagement Replay", description: "Get one engagement replay run.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"replay_run_id": map[string]any{"type": "string"}}, "required": []string{"replay_run_id"}}},
			builtInTool{name: "engagement.simulation.run", title: "Simulate Engagement Rules", description: "Preview engagement rule outcomes for one event.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"program_key": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}, "event": map[string]any{"type": "object"}}, "required": []string{"program_key", "event"}}},
		)
	}
	if s != nil && (s.health != nil || s.audit != nil) {
		defs = append(defs,
			builtInTool{name: "readiness.get", title: "Get Implementation Readiness", description: "Get readiness for applying customer configuration.", permission: "configuration.read"},
			builtInTool{name: "ops.health.get", title: "Get Runtime Health", description: "Get runtime health snapshot and subsystem status.", permission: "ops.read"},
			builtInTool{name: "ops.audit.correlation.get", title: "Get Audit Events By Correlation", description: "Get correlated audit events for one correlation id.", permission: "ops.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"correlation_id": map[string]any{"type": "string"}}, "required": []string{"correlation_id"}}},
			builtInTool{name: "ops.trace.get", title: "Get Operational Trace", description: "Get a stitched trace summary for one correlation id.", permission: "ops.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"correlation_id": map[string]any{"type": "string"}}, "required": []string{"correlation_id"}}},
		)
	}
	if s != nil && s.config != nil {
		defs = append(defs,
			builtInTool{name: "implementation.tenant.inspect", title: "Inspect Tenant Implementation State", description: "Aggregate config, flags, modules, integrations, and readiness for implementation work.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "implementation.config.plan", title: "Plan Implementation Config Changes", description: "Dry-run desired config, flags, and role grants into a normalized plan.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"bundle": map[string]any{"type": "object"}, "role_grants": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}}},
			builtInTool{name: "implementation.config.apply", title: "Apply Implementation Plan", description: "Apply a validated implementation plan. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"bundle": map[string]any{"type": "object"}, "role_grants": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"confirm_apply"}}},
			builtInTool{name: "implementation.readiness.check", title: "Check Implementation Readiness", description: "Return readiness, validation warnings, and operator hints before apply.", permission: "configuration.read"},
			builtInTool{name: "implementation.rollback.inspect", title: "Inspect Recent Implementation Changes", description: "Inspect recent config, feature flag, RBAC, and integration audit trail for rollback planning.", permission: "configuration.read"},
			builtInTool{name: "implementation.session.create", title: "Create Implementation Session", description: "Create an implementation session for staged rollout work.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "implementation.session.list", title: "List Implementation Sessions", description: "List implementation sessions.", permission: "configuration.read"},
			builtInTool{name: "implementation.session.get", title: "Get Implementation Session", description: "Get one implementation session.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.session.close", title: "Close Implementation Session", description: "Close one implementation session. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"session_id", "confirm_apply"}}},
			builtInTool{name: "implementation.plan.build", title: "Build Implementation Plan", description: "Stage a normalized implementation plan onto a session.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "plan": map[string]any{"type": "object"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.plan.validate", title: "Validate Implementation Plan", description: "Validate the currently staged implementation plan for a session.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.stage.diff", title: "Diff Staged Implementation", description: "Compare staged session changes against current runtime state.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.stage.discard", title: "Discard Staged Implementation", description: "Discard the staged plan for a session. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"session_id", "confirm_apply"}}},
			builtInTool{name: "implementation.stage.commit", title: "Commit Staged Implementation", description: "Apply the staged plan and record a change-set. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"session_id", "confirm_apply"}}},
			builtInTool{name: "implementation.verify.state", title: "Verify Implementation State", description: "Verify current runtime state for a session or context.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "implementation.verify.readiness", title: "Verify Implementation Readiness", description: "Verify readiness and runtime validity for a session or context.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "implementation.verify.diff", title: "Verify Implementation Diff", description: "Return the staged diff for verification workflows.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.verify.smoke", title: "Smoke Verify Implementation", description: "Run a high-signal verification sweep for a session or context.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "organization_id": map[string]any{"type": "string"}, "location_id": map[string]any{"type": "string"}, "operating_unit_id": map[string]any{"type": "string"}}}},
			builtInTool{name: "implementation.checkpoint.create", title: "Create Implementation Checkpoint", description: "Create a checkpoint tied to the latest committed change-set.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.checkpoint.list", title: "List Implementation Checkpoints", description: "List checkpoints for a session.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.checkpoint.restore", title: "Restore Implementation Checkpoint", description: "Apply rollback for a checkpoint when reversible. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "checkpoint_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"session_id", "checkpoint_id", "confirm_apply"}}},
			builtInTool{name: "implementation.rollback.plan", title: "Plan Implementation Rollback", description: "Build a rollback plan for a change-set.", permission: "configuration.read", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "change_set_id": map[string]any{"type": "string"}}, "required": []string{"session_id"}}},
			builtInTool{name: "implementation.rollback.apply", title: "Apply Implementation Rollback", description: "Apply a reversible rollback plan. Requires confirmation.", permission: "configuration.manage", inputSchema: map[string]any{"type": "object", "properties": map[string]any{"session_id": map[string]any{"type": "string"}, "change_set_id": map[string]any{"type": "string"}, "confirm_apply": map[string]any{"type": "boolean"}}, "required": []string{"session_id", "confirm_apply"}}},
		)
	}
	items := make([]ToolDescriptor, 0, len(defs))
	for _, def := range defs {
		if !scopeMatches(actor.EndpointScope, builtInToolScope(def.name)) {
			continue
		}
		if !allowsAll(actor.PermissionChecker, []string{def.permission}) {
			continue
		}
		items = append(items, ToolDescriptor{
			Name:        def.name,
			Title:       def.title,
			Description: def.description,
			Scope:       builtInToolScope(def.name),
			InputSchema: cloneMap(def.inputSchema),
		})
	}
	return items
}

func (s *Server) listBuiltInResources(actor ActorContext) []ResourceDescriptor {
	items := make([]ResourceDescriptor, 0, 9)
	if s != nil && s.templates != nil && scopeMatches(actor.EndpointScope, "template") && allowsAll(actor.PermissionChecker, []string{"template.read"}) {
		items = append(items, ResourceDescriptor{
			URI:         templateDesignerResourceURI,
			Name:        "Template Designer",
			Description: "Lightweight MCP app for template draft inspection and preview.",
			Scope:       "template",
			MIMEType:    "text/html",
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
			})
		}
	}
	if s != nil && s.workflows != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{
			URI:         workflowManagerResourceURI,
			Name:        "Workflow Manager",
			Description: "Lightweight MCP app for workflow drafts, routing simulation, and hierarchy inspection.",
			MIMEType:    "text/html",
		})
	}
	if s != nil && s.config != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items,
			ResourceDescriptor{URI: configCatalogResourceURI, Name: "Config Catalog", Description: "Configuration definitions, entries, and effective values.", MIMEType: "application/json"},
			ResourceDescriptor{URI: flagCatalogResourceURI, Name: "Feature Flag Catalog", Description: "Feature flag definitions and stored values.", MIMEType: "application/json"},
			ResourceDescriptor{URI: moduleCompatResourceURI, Name: "Module Compatibility", Description: "Installed module and kernel compatibility state.", MIMEType: "application/json"},
			ResourceDescriptor{URI: readinessResourceURI, Name: "Implementation Readiness", Description: "Readiness and validation snapshot for control-plane applies.", MIMEType: "application/json"},
			ResourceDescriptor{URI: implementationBlueprintsURI, Name: "Implementation Blueprints", Description: "Domain-agnostic implementation blueprint and desired-state guidance.", MIMEType: "application/json"},
		)
	}
	if s != nil && s.identity != nil && allowsAll(actor.PermissionChecker, []string{"identity.manage_users"}) {
		items = append(items, ResourceDescriptor{URI: roleMatrixResourceURI, Name: "Role Matrix", Description: "Roles, permissions, grants, and bindings.", MIMEType: "application/json"})
	}
	if s != nil && s.integration != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{URI: integrationHealthResourceURI, Name: "Integration Health", Description: "Integration connector health and submission summary.", MIMEType: "application/json"})
	}
	if s != nil && s.search != nil && allowsAll(actor.PermissionChecker, []string{"search.manage"}) {
		items = append(items, ResourceDescriptor{URI: searchRuntimeResourceURI, Name: "Search Runtime", Description: "Search index runtime and consistency status.", MIMEType: "application/json"})
	}
	if s != nil && s.offline != nil && allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		items = append(items, ResourceDescriptor{URI: offlineOpsResourceURI, Name: "Offline Sync", Description: "Offline sync batches, outcomes, and conflicts.", MIMEType: "application/json"})
	}
	if s != nil && s.policy != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{URI: policyRuntimeResourceURI, Name: "Policy Runtime", Description: "Policy hook runtime, compile, and evaluation status.", MIMEType: "application/json"})
	}
	if s != nil && s.reference != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items, ResourceDescriptor{URI: referenceCatalogResourceURI, Name: "Reference Catalog", Description: "Reference data types and records.", MIMEType: "application/json"})
	}
	if s != nil && s.health != nil && allowsAll(actor.PermissionChecker, []string{"ops.read"}) {
		items = append(items, ResourceDescriptor{URI: runbooksResourceURI, Name: "Runbooks", Description: "Runtime health runbooks and operator hints.", MIMEType: "application/json"})
	}
	if s != nil && s.dataops != nil && allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		items = append(items,
			ResourceDescriptor{URI: dataopsCatalogResourceURI, Name: "DataOps Catalog", Description: "Data class catalog and adapter capability matrix.", MIMEType: "application/json"},
			ResourceDescriptor{URI: dataopsArtifactsResourceURI, Name: "DataOps Artifacts", Description: "Managed backup, archive, export, and migration artifacts.", MIMEType: "application/json"},
			ResourceDescriptor{URI: dataopsCheckpointsResourceURI, Name: "DataOps Checkpoints", Description: "Latest incremental checkpoints by data class and adapter.", MIMEType: "application/json"},
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

func (s *Server) callBuiltInTool(actor ActorContext, name string, arguments map[string]any) (map[string]any, bool, error) {
	if !scopeMatches(actor.EndpointScope, builtInToolScope(name)) && builtInToolScope(name) != "" {
		return nil, true, fmt.Errorf("tool is not available on this endpoint")
	}
	switch name {
	case "template.definition.list":
		if s == nil || s.templates == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDefinitionList(actor), true, nil
	case "template.definition.get":
		if s == nil || s.templates == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDefinitionGet(actor, arguments)
	case "template.draft.get":
		if s == nil || s.templates == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDraftGet(actor, arguments)
	case "template.draft.save":
		if s == nil || s.templates == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"template.manage"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDraftSave(actor, arguments)
	case "template.render.preview":
		if s == nil || s.templates == nil {
			return nil, false, nil
		}
		if !allowsAll(actor.PermissionChecker, []string{"template.render"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateRenderPreview(actor, arguments)
	case "analytics.dashboard.list":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsDashboardList(actor, arguments)
	case "analytics.dashboard.get":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsDashboardGet(actor, arguments)
	case "analytics.dashboard.save":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsDashboardSave(actor, arguments)
	case "analytics.dashboard.delete":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsDashboardDelete(actor, arguments)
	case "analytics.metric.list":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsMetricList(actor, arguments)
	case "analytics.metric.get":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsMetricGet(actor, arguments)
	case "analytics.metric.save":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsMetricSave(actor, arguments)
	case "analytics.metric.delete":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsMetricDelete(actor, arguments)
	case "analytics.query.list":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsQueryList(actor, arguments)
	case "analytics.query.get":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsQueryGet(actor, arguments)
	case "analytics.query.save":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsQuerySave(actor, arguments)
	case "analytics.query.delete":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsQueryDelete(actor, arguments)
	case "analytics.query.execute":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsQueryExecute(actor, arguments)
	case "analytics.chart.generate":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsChartGenerate(actor, arguments)
	case "analytics.report.definition.list":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsReportDefinitionList(actor, arguments)
	case "analytics.report.definition.get":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsReportDefinitionGet(actor, arguments)
	case "analytics.report.definition.save":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsReportDefinitionSave(actor, arguments)
	case "analytics.report.definition.delete":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsReportDefinitionDelete(actor, arguments)
	case "analytics.report.run":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsReportRun(actor, arguments)
	case "analytics.report.deliver":
		if s == nil || s.analytics == nil {
			return nil, false, nil
		}
		return s.analyticsReportDeliver(actor, arguments)
	case "workflow.definition.list":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDefinitionList(actor, arguments)
	case "workflow.definition.get":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDefinitionGet(actor, arguments)
	case "workflow.version.list":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowVersionList(actor, arguments)
	case "workflow.draft.create":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDraftCreate(actor, arguments)
	case "workflow.draft.get":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDraftGet(actor, arguments)
	case "workflow.draft.save":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDraftSave(actor, arguments)
	case "workflow.draft.validate":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDraftValidate(actor, arguments)
	case "workflow.draft.simulate":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDraftSimulate(actor, arguments)
	case "workflow.draft.publish":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowDraftPublish(actor, arguments)
	case "workflow.runtime.tasks.list":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowRuntimeTasksList(actor, arguments)
	case "workflow.runtime.approvals.list":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowRuntimeApprovalsList(actor, arguments)
	case "workflow.runtime.history.get":
		if s == nil || s.workflows == nil {
			return nil, false, nil
		}
		return s.workflowRuntimeHistoryGet(actor, arguments)
	case "workflow.hierarchy.graph.get":
		if s == nil || s.identity == nil {
			return nil, false, nil
		}
		return s.workflowHierarchyGraphGet(actor, arguments)
	case "workflow.hierarchy.chain.get":
		if s == nil || s.identity == nil {
			return nil, false, nil
		}
		return s.workflowHierarchyChainGet(actor, arguments)
	case "workflow.hierarchy.summary.get":
		if s == nil || s.identity == nil {
			return nil, false, nil
		}
		return s.workflowHierarchySummaryGet(actor, arguments)
	case "workflow.reporting_line.list":
		if s == nil || s.identity == nil {
			return nil, false, nil
		}
		return s.workflowReportingLineList(actor, arguments)
	case "workflow.reporting_line.save":
		if s == nil || s.identity == nil {
			return nil, false, nil
		}
		return s.workflowReportingLineSave(actor, arguments)
	case "config.definition.list":
		return s.configDefinitionList(actor, arguments)
	case "config.entry.list":
		return s.configEntryList(actor, arguments)
	case "config.effective.get":
		return s.configEffectiveGet(actor, arguments)
	case "config.compare":
		return s.configCompare(actor, arguments)
	case "config.bundle.export":
		return s.configBundleExport(actor, arguments)
	case "config.bundle.validate":
		return s.configBundleValidate(actor, arguments)
	case "config.bundle.apply":
		return s.configBundleApply(actor, arguments)
	case "feature_flag.definition.list":
		return s.featureFlagDefinitionList(actor, arguments)
	case "feature_flag.value.list":
		return s.featureFlagValueList(actor, arguments)
	case "feature_flag.targeting.get":
		return s.featureFlagTargetingGet(actor, arguments)
	case "feature_flag.value.upsert":
		return s.featureFlagValueUpsert(actor, arguments)
	case "identity.role_permission_matrix.get":
		return s.identityRolePermissionMatrixGet(actor, arguments)
	case "identity.role_permission.grant":
		return s.identityRolePermissionGrant(actor, arguments)
	case "identity.role_permission.revoke":
		return s.identityRolePermissionRevoke(actor, arguments)
	case "module.list":
		return s.moduleList(actor, arguments)
	case "module.compatibility.list":
		return s.moduleCompatibilityList(actor, arguments)
	case "module.enable":
		return s.moduleEnable(actor, arguments)
	case "module.disable":
		return s.moduleDisable(actor, arguments)
	case "search.runtime.list":
		return s.searchIndexList(actor, arguments)
	case "search.runtime.get":
		return s.searchRuntimeGet(actor, arguments)
	case "search.consistency.get":
		return s.searchConsistencyGet(actor, arguments)
	case "search.rebuild":
		return s.searchRebuild(actor, arguments)
	case "search.repair":
		return s.searchRepair(actor, arguments)
	case "search.reconcile":
		return s.searchReconcile(actor, arguments)
	case "search.schema.plan":
		return s.searchSchemaPlan(actor, arguments)
	case "search.schema.build":
		return s.searchSchemaBuild(actor, arguments)
	case "search.schema.activate":
		return s.searchSchemaActivate(actor, arguments)
	case "offline.sync.list":
		return s.offlineSyncList(actor, arguments)
	case "offline.sync.get":
		return s.offlineSyncGet(actor, arguments)
	case "offline.conflict.list":
		return s.offlineConflictList(actor, arguments)
	case "policy.hook.list":
		return s.policyHookList(actor, arguments)
	case "policy.hook.get":
		return s.policyHookGet(actor, arguments)
	case "policy.module.upsert":
		return s.policyModuleUpsert(actor, arguments)
	case "identity.role_binding.list":
		return s.identityRoleBindingList(actor, arguments)
	case "identity.role_binding.priority.set":
		return s.identityRoleBindingPrioritySet(actor, arguments)
	case "reference.type.list":
		return s.referenceTypeList(actor, arguments)
	case "reference.record.list":
		return s.referenceRecordList(actor, arguments)
	case "reference.resolve":
		return s.referenceResolve(actor, arguments)
	case "reference.record.upsert":
		return s.referenceRecordUpsert(actor, arguments)
	case "integration.adapter.list":
		return s.integrationAdapterList(actor, arguments)
	case "integration.system.list":
		return s.integrationSystemList(actor, arguments)
	case "integration.system.config.get":
		return s.integrationSystemConfigGet(actor, arguments)
	case "integration.system.config.update":
		return s.integrationSystemConfigUpdate(actor, arguments)
	case "integration.endpoint.list":
		return s.integrationEndpointList(actor, arguments)
	case "integration.endpoint.config.get":
		return s.integrationEndpointConfigGet(actor, arguments)
	case "integration.endpoint.config.update":
		return s.integrationEndpointConfigUpdate(actor, arguments)
	case "integration.submission.list":
		return s.integrationSubmissionList(actor, arguments)
	case "integration.submission.get":
		return s.integrationSubmissionGet(actor, arguments)
	case "integration.dead_letter.list":
		return s.integrationDeadLetterList(actor, arguments)
	case "integration.dead_letter.replay":
		return s.integrationDeadLetterReplay(actor, arguments)
	case "dataops.artifact.list":
		return s.dataopsArtifactList(actor, arguments)
	case "dataops.artifact.get":
		return s.dataopsArtifactGet(actor, arguments)
	case "dataops.operation.get":
		return s.dataopsOperationGet(actor, arguments)
	case "dataops.checkpoint.list":
		return s.dataopsCheckpointList(actor, arguments)
	case "dataops.backup.plan":
		return s.dataopsBackupPlan(actor, arguments)
	case "dataops.backup.run":
		return s.dataopsBackupRun(actor, arguments)
	case "dataops.restore.plan":
		return s.dataopsRestorePlan(actor, arguments)
	case "dataops.restore.validate":
		return s.dataopsRestoreValidate(actor, arguments)
	case "dataops.restore.run":
		return s.dataopsRestoreRun(actor, arguments)
	case "dataops.archive.plan":
		return s.dataopsArchivePlan(actor, arguments)
	case "dataops.archive.run":
		return s.dataopsArchiveRun(actor, arguments)
	case "dataops.export.plan":
		return s.dataopsExportPlan(actor, arguments)
	case "dataops.export.run":
		return s.dataopsExportRun(actor, arguments)
	case "dataops.migration.register":
		return s.dataopsMigrationRegister(actor, arguments)
	case "dataops.migration.plan":
		return s.dataopsMigrationPlan(actor, arguments)
	case "dataops.migration.validate":
		return s.dataopsMigrationValidate(actor, arguments)
	case "dataops.migration.run":
		return s.dataopsMigrationRun(actor, arguments)
	case "engagement.program.list":
		return s.engagementProgramList(actor, arguments)
	case "engagement.program.get":
		return s.engagementProgramGet(actor, arguments)
	case "engagement.program.create":
		return s.engagementProgramCreate(actor, arguments)
	case "engagement.program.update":
		return s.engagementProgramUpdate(actor, arguments)
	case "engagement.program.version.create":
		return s.engagementVersionCreate(actor, arguments)
	case "engagement.program.version.save":
		return s.engagementVersionSave(actor, arguments)
	case "engagement.program.version.validate":
		return s.engagementVersionValidate(actor, arguments)
	case "engagement.program.version.publish":
		return s.engagementVersionPublish(actor, arguments)
	case "engagement.subject.get":
		return s.engagementSubjectGet(actor, arguments)
	case "engagement.account.list":
		return s.engagementAccountList(actor, arguments)
	case "engagement.balance.get":
		return s.engagementBalanceGet(actor, arguments)
	case "engagement.journal.list":
		return s.engagementJournalList(actor, arguments)
	case "engagement.qualification.get":
		return s.engagementQualificationGet(actor, arguments)
	case "engagement.achievement.list":
		return s.engagementAchievementList(actor, arguments)
	case "engagement.consumer.list":
		return s.engagementConsumerList(actor, arguments)
	case "engagement.consumer.get":
		return s.engagementConsumerGet(actor, arguments)
	case "engagement.replay.plan":
		return s.engagementReplayPlan(actor, arguments)
	case "engagement.replay.run":
		return s.engagementReplayRun(actor, arguments)
	case "engagement.replay.get":
		return s.engagementReplayGet(actor, arguments)
	case "engagement.simulation.run":
		return s.engagementSimulationRun(actor, arguments)
	case "readiness.get":
		return s.readinessGet(actor, arguments)
	case "ops.health.get":
		return s.opsHealthGet(actor, arguments)
	case "ops.audit.correlation.get":
		return s.opsAuditCorrelationGet(actor, arguments)
	case "ops.trace.get":
		return s.opsTraceGet(actor, arguments)
	case "implementation.tenant.inspect":
		return s.implementationTenantInspect(actor, arguments)
	case "implementation.config.plan":
		return s.implementationConfigPlan(actor, arguments)
	case "implementation.config.apply":
		return s.implementationConfigApply(actor, arguments)
	case "implementation.readiness.check":
		return s.implementationReadinessCheck(actor, arguments)
	case "implementation.rollback.inspect":
		return s.implementationRollbackInspect(actor, arguments)
	case "implementation.session.create":
		return s.implementationSessionCreate(actor, arguments)
	case "implementation.session.list":
		return s.implementationSessionList(actor, arguments)
	case "implementation.session.get":
		return s.implementationSessionGet(actor, arguments)
	case "implementation.session.close":
		return s.implementationSessionClose(actor, arguments)
	case "implementation.plan.build":
		return s.implementationPlanBuild(actor, arguments)
	case "implementation.plan.validate":
		return s.implementationPlanValidate(actor, arguments)
	case "implementation.stage.diff":
		return s.implementationStageDiff(actor, arguments)
	case "implementation.stage.discard":
		return s.implementationStageDiscard(actor, arguments)
	case "implementation.stage.commit":
		return s.implementationStageCommit(actor, arguments)
	case "implementation.verify.state":
		return s.implementationVerifyState(actor, arguments)
	case "implementation.verify.readiness":
		return s.implementationVerifyReadiness(actor, arguments)
	case "implementation.verify.diff":
		return s.implementationVerifyDiff(actor, arguments)
	case "implementation.verify.smoke":
		return s.implementationVerifySmoke(actor, arguments)
	case "implementation.checkpoint.create":
		return s.implementationCheckpointCreate(actor, arguments)
	case "implementation.checkpoint.list":
		return s.implementationCheckpointList(actor, arguments)
	case "implementation.checkpoint.restore":
		return s.implementationCheckpointRestore(actor, arguments)
	case "implementation.rollback.plan":
		return s.implementationRollbackPlan(actor, arguments)
	case "implementation.rollback.apply":
		return s.implementationRollbackApply(actor, arguments)
	default:
		return nil, false, nil
	}
}

func (s *Server) analyticsSnapshotPayload(actor ActorContext) (analytics.Snapshot, error) {
	if s == nil || s.analytics == nil {
		return analytics.Snapshot{}, fmt.Errorf("analytics is unavailable")
	}
	snapshot, ok := s.analytics.LatestSnapshot(analytics.Query{Window: "current_state"})
	if !ok {
		var err error
		snapshot, err = s.analytics.CaptureSnapshot()
		if err != nil {
			return analytics.Snapshot{}, err
		}
	}
	_ = actor
	return snapshot, nil
}

func (s *Server) templateDefinitionList(actor ActorContext) map[string]any {
	items := s.visibleTemplateDefinitions(actor)
	content := []ContentBlock{{
		Type: "text",
		Text: fmt.Sprintf("Found %d template definitions.", len(items)),
	}}
	return map[string]any{
		"content":           content,
		"structuredContent": map[string]any{"items": items},
	}
}

func (s *Server) templateDefinitionGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	templateKey := strings.TrimSpace(stringArg(arguments, "template_key"))
	if templateKey == "" {
		return nil, true, shared.Validation("template_key is required")
	}
	def, versions, draft, err := s.templateState(actor, templateKey)
	if err != nil {
		return nil, true, err
	}
	result := map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Loaded template definition %s.", def.Key),
		}},
		"structuredContent": map[string]any{
			"definition":      def,
			"versions":        versions,
			"current_draft":   draft,
			"visual_template": parseVisualTemplate(draft),
		},
		"_meta": s.templateAppMeta(def.Key),
	}
	return result, true, nil
}

func (s *Server) templateDraftGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	templateKey := strings.TrimSpace(stringArg(arguments, "template_key"))
	if templateKey == "" {
		return nil, true, shared.Validation("template_key is required")
	}
	def, versions, draft, err := s.templateState(actor, templateKey)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Loaded draft context for template %s.", def.Key),
		}},
		"structuredContent": map[string]any{
			"definition":      def,
			"versions":        versions,
			"draft":           draft,
			"visual_template": parseVisualTemplate(draft),
		},
		"_meta": s.templateAppMeta(def.Key),
	}, true, nil
}

func (s *Server) templateDraftSave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	templateKey := strings.TrimSpace(stringArg(arguments, "template_key"))
	if templateKey == "" {
		return nil, true, shared.Validation("template_key is required")
	}
	def, ok := s.templates.Definition(templateKey)
	if !ok {
		return nil, true, shared.NotFound("template definition not found")
	}
	if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	body := strings.TrimSpace(stringArg(arguments, "body"))
	if visual, ok := arguments["visual_template"]; ok {
		raw, err := json.Marshal(visual)
		if err != nil {
			return nil, true, shared.Validation("visual_template must be valid json")
		}
		body = string(raw)
	}
	style := stringArg(arguments, "style")
	updatedBy := strings.TrimSpace(actor.EffectiveUserID)
	if updatedBy == "" {
		updatedBy = strings.TrimSpace(actor.ActorID)
	}
	version, err := s.templates.SaveDraft(templateKey, body, style, updatedBy)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Saved draft v%d for template %s.", version.Version, templateKey),
		}},
		"structuredContent": map[string]any{
			"definition":      def,
			"draft":           version,
			"visual_template": parseVisualTemplate(version),
		},
		"_meta": s.templateAppMeta(templateKey),
	}, true, nil
}

func (s *Server) templateRenderPreview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	req, def, err := s.templateRenderRequest(actor, arguments)
	if err != nil {
		return nil, true, err
	}
	rendered, err := s.templates.Render(req)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Rendered preview for template %s.", rendered.TemplateKey),
		}},
		"structuredContent": map[string]any{
			"definition": def,
			"request":    req,
			"output": map[string]any{
				"template_key": rendered.TemplateKey,
				"version":      rendered.Version,
				"format":       rendered.Format,
				"content_type": rendered.ContentType,
				"file_name":    rendered.FileName,
				"generated_at": rendered.GeneratedAt,
				"official":     rendered.Official,
				"html":         rendered.HTML,
				"bytes_base64": encodeBytes(rendered.Bytes),
			},
		},
		"_meta": s.templateAppMeta(def.Key),
	}, true, nil
}

func (s *Server) visibleTemplateDefinitions(actor ActorContext) []templateoutput.Definition {
	items := make([]templateoutput.Definition, 0)
	for _, item := range s.templates.Definitions() {
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			continue
		}
		if !allowsAll(actor.PermissionChecker, item.RequiredPermissions) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func (s *Server) templateState(actor ActorContext, templateKey string) (templateoutput.Definition, []templateoutput.Version, templateoutput.Version, error) {
	def, ok := s.templates.Definition(templateKey)
	if !ok {
		return templateoutput.Definition{}, nil, templateoutput.Version{}, shared.NotFound("template definition not found")
	}
	if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
		return templateoutput.Definition{}, nil, templateoutput.Version{}, fmt.Errorf("tool is not allowed")
	}
	versions := s.templates.Versions(templateKey)
	draft := defaultDraftVersion(def)
	for _, item := range versions {
		if item.Status == "draft" {
			draft = item
			break
		}
	}
	return def, versions, draft, nil
}

func defaultDraftVersion(def templateoutput.Definition) templateoutput.Version {
	return templateoutput.Version{
		TemplateKey:  def.Key,
		Version:      1,
		Status:       "draft",
		RendererKind: def.RendererKind,
		Body:         def.DefaultBody,
		Style:        def.DefaultStyle,
	}
}

func parseVisualTemplate(version templateoutput.Version) any {
	if strings.TrimSpace(version.RendererKind) != "visual" || strings.TrimSpace(version.Body) == "" {
		return nil
	}
	var visual templateoutput.VisualTemplate
	if err := json.Unmarshal([]byte(version.Body), &visual); err != nil {
		return nil
	}
	return visual
}

func (s *Server) templateRenderRequest(actor ActorContext, arguments map[string]any) (templateoutput.RenderRequest, templateoutput.Definition, error) {
	templateKey := strings.TrimSpace(stringArg(arguments, "template_key"))
	if templateKey == "" {
		return templateoutput.RenderRequest{}, templateoutput.Definition{}, shared.Validation("template_key is required")
	}
	def, ok := s.templates.Definition(templateKey)
	if !ok {
		return templateoutput.RenderRequest{}, templateoutput.Definition{}, shared.NotFound("template definition not found")
	}
	if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
		return templateoutput.RenderRequest{}, templateoutput.Definition{}, fmt.Errorf("tool is not allowed")
	}
	req := templateoutput.RenderRequest{
		TemplateKey:    templateKey,
		RendererKind:   stringArg(arguments, "renderer_kind"),
		Body:           stringArg(arguments, "body"),
		Style:          stringArg(arguments, "style"),
		TargetKind:     firstNonEmpty(stringArg(arguments, "target_kind"), def.TargetKind),
		TargetKey:      firstNonEmpty(stringArg(arguments, "target_key"), def.TargetKey),
		TargetID:       stringArg(arguments, "target_id"),
		Sample:         boolArg(arguments, "sample"),
		OrganizationID: firstNonEmpty(stringArg(arguments, "organization_id"), actor.OrganizationID),
		LocationID:     firstNonEmpty(stringArg(arguments, "location_id"), actor.LocationID),
		ScopeType:      stringArg(arguments, "scope_type"),
		ScopeID:        stringArg(arguments, "scope_id"),
		Purpose:        firstNonEmpty(stringArg(arguments, "purpose"), def.Purpose),
		Channel:        firstNonEmpty(stringArg(arguments, "channel"), def.Channel),
		Format:         stringArg(arguments, "format"),
		Draft:          true,
	}
	if visual, ok := arguments["visual_template"]; ok {
		raw, err := json.Marshal(visual)
		if err != nil {
			return templateoutput.RenderRequest{}, templateoutput.Definition{}, shared.Validation("visual_template must be valid json")
		}
		req.Body = string(raw)
	}
	if !req.Sample && strings.TrimSpace(req.TargetID) == "" && req.TargetKind == "document" {
		req.Sample = true
	}
	return req, def, nil
}

func (s *Server) templateAppMeta(templateKey string) map[string]any {
	return map[string]any{
		"orbyte/app": map[string]any{
			"key":          templateDesignerAppKey,
			"title":        "Template Designer",
			"resource_uri": templateResourceURI(templateKey),
		},
	}
}

func templateResourceURI(templateKey string) string {
	if strings.TrimSpace(templateKey) == "" {
		return templateDesignerResourceURI
	}
	return templateDesignerResourceURI + "?template_key=" + url.QueryEscape(strings.TrimSpace(templateKey))
}

func (s *Server) renderTemplateDesignerApp(actor ActorContext, parsed *url.URL) (string, error) {
	if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
		return "", fmt.Errorf("resource is not allowed")
	}
	templateKey := strings.TrimSpace(parsed.Query().Get("template_key"))
	body := `<p>Select a template definition through MCP tools to inspect its draft and preview.</p>`
	title := "Template Designer"
	if templateKey != "" {
		def, versions, draft, err := s.templateState(actor, templateKey)
		if err != nil {
			return "", err
		}
		req := templateoutput.RenderRequest{
			TemplateKey: templateKey,
			TargetKind:  def.TargetKind,
			TargetKey:   def.TargetKey,
			Purpose:     def.Purpose,
			Channel:     def.Channel,
			Sample:      true,
			Draft:       true,
		}
		rendered, err := s.templates.Render(req)
		if err != nil {
			return "", err
		}
		rawDraft, _ := json.MarshalIndent(draft, "", "  ")
		rawVisual, _ := json.MarshalIndent(parseVisualTemplate(draft), "", "  ")
		body = `<section><h3>Definition</h3><p><strong>` + escapeHTML(def.Title) + `</strong> <span>` + escapeHTML(def.Key) + `</span></p></section>` +
			`<section><h3>Versions</h3><p>` + escapeHTML(fmt.Sprintf("%d version(s) recorded", len(versions))) + `</p></section>` +
			`<section><h3>Draft</h3><pre>` + escapeHTML(string(rawDraft)) + `</pre></section>` +
			`<section><h3>Visual Layout</h3><pre>` + escapeHTML(string(rawVisual)) + `</pre></section>` +
			`<section><h3>Preview</h3><div class="preview">` + rendered.HTML + `</div></section>`
		title = "Template Designer: " + def.Title
	}
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` +
		escapeHTML(title) +
		`</title><style>body{font-family:Georgia,serif;background:#f7f1e6;color:#16221b;margin:0;padding:24px}main{display:grid;gap:16px}.panel{background:#fffdf8;border:1px solid #d8d0c2;border-radius:14px;padding:16px}pre{white-space:pre-wrap;background:#f6f1e7;border:1px solid #d8d0c2;border-radius:12px;padding:12px;overflow:auto}.preview{background:#fff;border:1px solid #d8d0c2;border-radius:12px;padding:16px}h1,h3{margin:0 0 12px}p{margin:0 0 12px}</style></head><body><main class="panel"><h1>` +
		escapeHTML(title) +
		`</h1>` + body + `</main></body></html>`, nil
}

func (s *Server) analyticsAppMeta(kind, id string) map[string]any {
	resourceURI := analyticsStudioResourceURI
	values := url.Values{}
	if strings.TrimSpace(kind) != "" {
		values.Set("kind", strings.TrimSpace(kind))
	}
	if strings.TrimSpace(id) != "" {
		values.Set("id", strings.TrimSpace(id))
	}
	if encoded := values.Encode(); encoded != "" {
		resourceURI += "?" + encoded
	}
	return map[string]any{
		"orbyte/app": map[string]any{
			"key":          analyticsStudioAppKey,
			"title":        "Analytics Studio",
			"resource_uri": resourceURI,
			"stream_uri":   s.preferredAnalyticsStreamPath(),
		},
	}
}

func (s *Server) renderAnalyticsStudioApp(actor ActorContext, parsed *url.URL) (string, error) {
	if s == nil || s.analytics == nil {
		return "", fmt.Errorf("analytics is unavailable")
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return "", fmt.Errorf("resource is not allowed")
	}
	kind := strings.TrimSpace(parsed.Query().Get("kind"))
	id := strings.TrimSpace(parsed.Query().Get("id"))
	body := `<section><h3>Runtime Objects</h3><p>Select a dashboard, metric, query, or report through MCP tools to inspect it here.</p></section>`
	title := "Analytics Studio"
	if kind != "" && id != "" {
		title = "Analytics Studio: " + kind
		switch kind {
		case "dashboard":
			item, ok := s.analytics.Dashboard(id)
			if !ok {
				return "", shared.NotFound("dashboard not found")
			}
			raw, _ := json.MarshalIndent(item, "", "  ")
			body = `<section><h3>Dashboard</h3><pre>` + escapeHTML(string(raw)) + `</pre></section>`
		case "metric":
			item, ok := s.analytics.SavedMetric(id)
			if !ok {
				return "", shared.NotFound("saved metric not found")
			}
			raw, _ := json.MarshalIndent(item, "", "  ")
			body = `<section><h3>Saved Metric</h3><pre>` + escapeHTML(string(raw)) + `</pre></section>`
		case "query":
			item, ok := s.analytics.SavedQuery(id)
			if !ok {
				return "", shared.NotFound("saved query not found")
			}
			exec, err := s.analytics.ExecuteQuerySpec(item.Spec)
			if err != nil {
				return "", err
			}
			rawQuery, _ := json.MarshalIndent(item, "", "  ")
			rawExec, _ := json.MarshalIndent(exec, "", "  ")
			body = `<section><h3>Saved Query</h3><pre>` + escapeHTML(string(rawQuery)) + `</pre></section>` +
				`<section><h3>Latest Preview</h3><pre>` + escapeHTML(string(rawExec)) + `</pre></section>`
		case "report":
			item, ok := s.analytics.ReportDefinition(id)
			if !ok {
				return "", shared.NotFound("report definition not found")
			}
			raw, _ := json.MarshalIndent(item, "", "  ")
			body = `<section><h3>Report Definition</h3><pre>` + escapeHTML(string(raw)) + `</pre></section>`
		case "execution":
			exec, ok, err := s.analyticsExecutionFromEncoded(parsed.Query().Get("payload"))
			if err != nil {
				return "", err
			}
			if ok {
				raw, _ := json.MarshalIndent(exec, "", "  ")
				body = `<section><h3>Ad Hoc Result</h3><pre>` + escapeHTML(string(raw)) + `</pre></section>`
			}
		}
	}
	dashboards, metrics, queries, reports := s.analytics.Dashboards(), s.analytics.SavedMetrics(), s.analytics.SavedQueries(), s.analytics.ListReportDefinitions()
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` +
		escapeHTML(title) +
		`</title><style>body{font-family:Georgia,serif;background:#f3efe7;color:#1d2520;margin:0;padding:24px}main{display:grid;gap:16px}.panel{background:#fffdf8;border:1px solid #d8d0c2;border-radius:14px;padding:16px}pre{white-space:pre-wrap;background:#f7f3ea;border:1px solid #d8d0c2;border-radius:12px;padding:12px;overflow:auto}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}.card{background:#faf6ef;border:1px solid #d8d0c2;border-radius:12px;padding:12px}.meta{font-size:12px;color:#5f6c62}h1,h3{margin:0 0 12px}p{margin:0 0 12px}</style></head><body><main><section class="panel"><h1>` +
		escapeHTML(title) +
		`</h1><div class="grid"><article class="card"><div class="meta">Dashboards</div><strong>` + escapeHTML(fmt.Sprintf("%d", len(dashboards))) + `</strong></article><article class="card"><div class="meta">Saved Metrics</div><strong>` + escapeHTML(fmt.Sprintf("%d", len(metrics))) + `</strong></article><article class="card"><div class="meta">Saved Queries</div><strong>` + escapeHTML(fmt.Sprintf("%d", len(queries))) + `</strong></article><article class="card"><div class="meta">Reports</div><strong>` + escapeHTML(fmt.Sprintf("%d", len(reports))) + `</strong></article></div></section><section class="panel">` +
		body + `</section></main></body></html>`, nil
}

func (s *Server) analyticsDashboardList(actor ActorContext, _ map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.analytics.Dashboards()
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d analytics dashboards.", len(items))}},
		"structuredContent": map[string]any{"items": items},
		"_meta":             s.analyticsAppMeta("dashboard", ""),
	}, true, nil
}

func (s *Server) analyticsDashboardGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "dashboard_id"))
	if id == "" {
		return nil, true, shared.Validation("dashboard_id is required")
	}
	item, ok := s.analytics.Dashboard(id)
	if !ok {
		return nil, true, shared.NotFound("dashboard not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded analytics dashboard %s.", item.Name)}},
		"structuredContent": item,
		"_meta":             s.analyticsAppMeta("dashboard", item.ID),
	}, true, nil
}

func (s *Server) analyticsDashboardSave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.analytics == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.author"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var item analytics.Dashboard
	if err := decodeObjectArg(arguments, "dashboard", &item); err != nil {
		return nil, true, err
	}
	item.OwnerUserID = firstNonEmpty(item.OwnerUserID, actor.EffectiveUserID, actor.ActorID)
	item.OrganizationID = firstNonEmpty(item.OrganizationID, actor.OrganizationID)
	item.LocationID = firstNonEmpty(item.LocationID, actor.LocationID)
	item.UpdatedBy = firstNonEmpty(actor.EffectiveUserID, actor.ActorID)
	saved, err := s.analytics.SaveDashboard(item)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved analytics dashboard %s.", saved.Name)}},
		"structuredContent": saved,
		"_meta":             s.analyticsAppMeta("dashboard", saved.ID),
	}, true, nil
}

func (s *Server) analyticsDashboardDelete(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.author"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "dashboard_id"))
	if id == "" {
		return nil, true, shared.Validation("dashboard_id is required")
	}
	if err := s.analytics.DeleteDashboard(id); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Deleted analytics dashboard %s.", id)}}}, true, nil
}

func (s *Server) analyticsMetricList(actor ActorContext, _ map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.analytics.SavedMetrics()
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d saved metrics.", len(items))}},
		"structuredContent": map[string]any{"items": items},
		"_meta":             s.analyticsAppMeta("metric", ""),
	}, true, nil
}

func (s *Server) analyticsMetricGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "metric_id"))
	if id == "" {
		return nil, true, shared.Validation("metric_id is required")
	}
	item, ok := s.analytics.SavedMetric(id)
	if !ok {
		return nil, true, shared.NotFound("saved metric not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded saved metric %s.", item.Name)}},
		"structuredContent": item,
		"_meta":             s.analyticsAppMeta("metric", item.ID),
	}, true, nil
}

func (s *Server) analyticsMetricSave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.author"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var item analytics.SavedMetric
	if err := decodeObjectArg(arguments, "metric", &item); err != nil {
		return nil, true, err
	}
	item.OwnerUserID = firstNonEmpty(item.OwnerUserID, actor.EffectiveUserID, actor.ActorID)
	item.OrganizationID = firstNonEmpty(item.OrganizationID, actor.OrganizationID)
	item.LocationID = firstNonEmpty(item.LocationID, actor.LocationID)
	item.UpdatedBy = firstNonEmpty(actor.EffectiveUserID, actor.ActorID)
	saved, err := s.analytics.SaveSavedMetric(item)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved metric %s.", saved.Name)}},
		"structuredContent": saved,
		"_meta":             s.analyticsAppMeta("metric", saved.ID),
	}, true, nil
}

func (s *Server) analyticsMetricDelete(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.author"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "metric_id"))
	if id == "" {
		return nil, true, shared.Validation("metric_id is required")
	}
	if err := s.analytics.DeleteSavedMetric(id); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Deleted saved metric %s.", id)}}}, true, nil
}

func (s *Server) analyticsQueryList(actor ActorContext, _ map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.analytics.SavedQueries()
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d saved analytics queries.", len(items))}},
		"structuredContent": map[string]any{"items": items},
		"_meta":             s.analyticsAppMeta("query", ""),
	}, true, nil
}

func (s *Server) analyticsQueryGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "query_id"))
	if id == "" {
		return nil, true, shared.Validation("query_id is required")
	}
	item, ok := s.analytics.SavedQuery(id)
	if !ok {
		return nil, true, shared.NotFound("saved query not found")
	}
	exec, err := s.analytics.ExecuteQuerySpec(item.Spec)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded saved analytics query %s.", item.Name)}},
		"structuredContent": map[string]any{
			"query":   item,
			"preview": exec,
			"chart":   exec.Chart,
			"result":  exec,
		},
		"_meta": s.analyticsAppMeta("query", item.ID),
	}, true, nil
}

func (s *Server) analyticsQuerySave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.author"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var item analytics.SavedQuery
	if err := decodeObjectArg(arguments, "query", &item); err != nil {
		return nil, true, err
	}
	item.OwnerUserID = firstNonEmpty(item.OwnerUserID, actor.EffectiveUserID, actor.ActorID)
	item.OrganizationID = firstNonEmpty(item.OrganizationID, actor.OrganizationID)
	item.LocationID = firstNonEmpty(item.LocationID, actor.LocationID)
	item.UpdatedBy = firstNonEmpty(actor.EffectiveUserID, actor.ActorID)
	saved, err := s.analytics.SaveSavedQuery(item)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved analytics query %s.", saved.Name)}},
		"structuredContent": saved,
		"_meta":             s.analyticsAppMeta("query", saved.ID),
	}, true, nil
}

func (s *Server) analyticsQueryDelete(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.author"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "query_id"))
	if id == "" {
		return nil, true, shared.Validation("query_id is required")
	}
	if err := s.analytics.DeleteSavedQuery(id); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Deleted analytics query %s.", id)}}}, true, nil
}

func (s *Server) analyticsQueryExecute(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	spec, err := analyticsQuerySpec(arguments)
	if err != nil {
		return nil, true, err
	}
	exec, err := s.analytics.ExecuteQuerySpec(spec)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Executed ad hoc analytics query with %d result row(s).", len(exec.Rows))}},
		"structuredContent": map[string]any{
			"result": exec,
			"chart":  exec.Chart,
		},
		"_meta": s.analyticsExecutionMeta(exec),
	}, true, nil
}

func (s *Server) analyticsChartGenerate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var exec analytics.QueryExecution
	ok := false
	if err := decodeOptionalObjectArg(arguments, "result", &exec); err != nil {
		return nil, true, err
	} else if exec.Generated.IsZero() == false || len(exec.Rows) > 0 {
		ok = true
	}
	if !ok {
		spec, err := analyticsQuerySpec(arguments)
		if err != nil {
			return nil, true, err
		}
		exec, err = s.analytics.ExecuteQuerySpec(spec)
		if err != nil {
			return nil, true, err
		}
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Generated %s chart spec.", exec.Chart.Type)}},
		"structuredContent": map[string]any{"chart": exec.Chart, "result": exec},
		"_meta":             s.analyticsExecutionMeta(exec),
	}, true, nil
}

func (s *Server) analyticsReportDefinitionList(actor ActorContext, _ map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.analytics.ListReportDefinitions()
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d analytics report definitions.", len(items))}},
		"structuredContent": map[string]any{"items": items},
		"_meta":             s.analyticsAppMeta("report", ""),
	}, true, nil
}

func (s *Server) analyticsReportDefinitionGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "report_id"))
	if id == "" {
		return nil, true, shared.Validation("report_id is required")
	}
	item, ok := s.analytics.ReportDefinition(id)
	if !ok {
		return nil, true, shared.NotFound("report definition not found")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded report definition %s.", item.Name)}},
		"structuredContent": item,
		"_meta":             s.analyticsAppMeta("report", item.ID),
	}, true, nil
}

func (s *Server) analyticsReportDefinitionSave(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.manage_reports"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var item analytics.ReportDefinition
	if err := decodeObjectArg(arguments, "report", &item); err != nil {
		return nil, true, err
	}
	saved, err := s.analytics.SaveOrUpdateReportDefinition(item)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved report definition %s.", saved.Name)}},
		"structuredContent": saved,
		"_meta":             s.analyticsAppMeta("report", saved.ID),
	}, true, nil
}

func (s *Server) analyticsReportDefinitionDelete(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.manage_reports"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "report_id"))
	if id == "" {
		return nil, true, shared.Validation("report_id is required")
	}
	if err := s.analytics.DeleteReportDefinition(id); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Deleted report definition %s.", id)}}}, true, nil
}

func (s *Server) analyticsReportRun(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.manage_reports"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	id := strings.TrimSpace(stringArg(arguments, "report_id"))
	if id == "" {
		return nil, true, shared.Validation("report_id is required")
	}
	def, ok := s.analytics.ReportDefinition(id)
	if !ok {
		return nil, true, shared.NotFound("report definition not found")
	}
	run, content, err := s.analytics.RunReport(def)
	if err != nil {
		return nil, true, err
	}
	artifact, _ := s.analytics.GetReportArtifact(run.ArtifactID)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Ran report %s and generated artifact %s.", def.Name, run.ArtifactID)}},
		"structuredContent": map[string]any{
			"report":       def,
			"run":          run,
			"artifact":     artifact,
			"bytes_base64": encodeBytes(content),
		},
		"_meta": s.analyticsAppMeta("report", def.ID),
	}, true, nil
}

func (s *Server) analyticsReportDeliver(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"analytics.deliver_reports"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	artifactID := strings.TrimSpace(stringArg(arguments, "artifact_id"))
	if artifactID == "" {
		reportID := strings.TrimSpace(stringArg(arguments, "report_id"))
		if reportID == "" {
			return nil, true, shared.Validation("artifact_id or report_id is required")
		}
		def, ok := s.analytics.ReportDefinition(reportID)
		if !ok {
			return nil, true, shared.NotFound("report definition not found")
		}
		run, _, err := s.analytics.RunReport(def)
		if err != nil {
			return nil, true, err
		}
		artifactID = run.ArtifactID
	}
	channel := firstNonEmpty(stringArg(arguments, "channel"), "download")
	delivery, err := s.analytics.DeliverArtifact(artifactID, channel, stringArg(arguments, "recipient"))
	if err != nil && delivery.ID == "" {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Processed report delivery %s with status %s.", delivery.ID, delivery.Status)}},
		"structuredContent": delivery,
	}, true, nil
}

func analyticsQuerySpec(arguments map[string]any) (analytics.QuerySpec, error) {
	var spec analytics.QuerySpec
	if err := decodeObjectArg(arguments, "query", &spec); err == nil {
		return spec, nil
	}
	if err := decodeOptionalObjectArg(arguments, "spec", &spec); err == nil {
		if strings.TrimSpace(spec.SourceKind) != "" || strings.TrimSpace(spec.Window) != "" || len(spec.Measures) > 0 || len(spec.Filters) > 0 || !spec.From.IsZero() || !spec.To.IsZero() {
			return spec, nil
		}
	}
	return analytics.QuerySpec{}, shared.Validation("query is required")
}

func decodeObjectArg(arguments map[string]any, key string, target any) error {
	value, ok := arguments[key]
	if !ok || value == nil {
		return shared.Validation(key + " is required")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return shared.Validation(key + " must be valid json")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return shared.Validation(key + " has invalid shape")
	}
	return nil
}

func decodeOptionalObjectArg(arguments map[string]any, key string, target any) error {
	value, ok := arguments[key]
	if !ok || value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return shared.Validation(key + " must be valid json")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return shared.Validation(key + " has invalid shape")
	}
	return nil
}

func (s *Server) analyticsExecutionMeta(exec analytics.QueryExecution) map[string]any {
	raw, _ := json.Marshal(exec)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return map[string]any{
		"orbyte/app": map[string]any{
			"key":          analyticsStudioAppKey,
			"title":        "Analytics Studio",
			"resource_uri": analyticsStudioResourceURI + "?kind=execution&payload=" + url.QueryEscape(encoded),
		},
	}
}

func (s *Server) analyticsExecutionFromEncoded(value string) (analytics.QueryExecution, bool, error) {
	if strings.TrimSpace(value) == "" {
		return analytics.QueryExecution{}, false, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return analytics.QueryExecution{}, false, err
	}
	var exec analytics.QueryExecution
	if err := json.Unmarshal(raw, &exec); err != nil {
		return analytics.QueryExecution{}, false, err
	}
	return exec, true, nil
}

func (s *Server) preferredAnalyticsStreamPath() string {
	if strings.TrimSpace(s.analyticsScopedStreamPath) != "" {
		return strings.TrimSpace(s.analyticsScopedStreamPath)
	}
	return strings.TrimSpace(s.analyticsStreamPath)
}

func stringArg(arguments map[string]any, key string) string {
	value, _ := arguments[key]
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func boolArg(arguments map[string]any, key string) bool {
	value, _ := arguments[key]
	typed, _ := value.(bool)
	return typed
}

func encodeBytes(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(payload)
}

func (s *Server) renderApp(actor ActorContext, appDef module.MCPAppDefinition) (string, error) {
	if strings.TrimSpace(appDef.CustomEntryKey) != "" {
		payload, err := s.analyticsSnapshotPayload(actor)
		if err != nil {
			return "", err
		}
		body, _ := json.Marshal(payload)
		streamURI, _ := json.Marshal(s.preferredAnalyticsStreamPath())
		return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` +
			escapeHTML(appDef.Title) +
			`</title><style>body{font-family:Georgia,serif;background:#f7f1e6;color:#16221b;margin:0;padding:24px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px}.card{border:1px solid #d8d0c2;border-radius:14px;background:#fffdf8;padding:14px}.meta{color:#5f6c62;font-size:12px}strong{display:block;font-size:24px;margin-top:6px}pre{white-space:pre-wrap;background:#f6f1e7;border:1px solid #d8d0c2;border-radius:12px;padding:12px}</style></head><body><h2>` +
			escapeHTML(appDef.Title) +
			`</h2><div class="grid" id="cards"></div><pre id="raw"></pre><script>const streamURI=` + string(streamURI) + `;let payload=` + string(body) + `;function render(){const cards=[["Documents",(payload.documents.created||0)+(payload.documents.draft||0)+(payload.documents.submitted||0)+(payload.documents.approved||0)+(payload.documents.rejected||0)+(payload.documents.cancelled||0)],["Draft",payload.documents.draft||0],["Submitted",payload.documents.submitted||0],["Approvals Pending",payload.workflow.pending_approvals||0]];document.getElementById("cards").innerHTML=cards.map(function(item){return '<article class="card"><span class="meta">'+item[0]+'</span><strong>'+item[1]+'</strong></article>'}).join('');document.getElementById("raw").textContent=JSON.stringify(payload,null,2);}render();if(streamURI&&typeof EventSource!=="undefined"){const stream=new EventSource(streamURI);stream.addEventListener("snapshot",function(event){try{payload=JSON.parse(event.data);render();}catch(_err){}});}</script></body></html>`, nil
	}
	if strings.TrimSpace(appDef.ViewKey) != "" {
		view, ok := s.modules.View(appDef.ViewKey)
		if !ok {
			return "", fmt.Errorf("view not found")
		}
		body, _ := json.Marshal(view)
		return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` +
			escapeHTML(appDef.Title) +
			`</title><style>body{font-family:Georgia,serif;background:#f7f1e6;color:#16221b;margin:0;padding:24px}pre{white-space:pre-wrap;background:#f6f1e7;border:1px solid #d8d0c2;border-radius:12px;padding:12px}</style></head><body><h2>` +
			escapeHTML(appDef.Title) +
			`</h2><p>Generic MCP app bound to shared view definition.</p><pre>` + escapeHTML(string(body)) + `</pre></body></html>`, nil
	}
	return "", fmt.Errorf("app renderer is not configured")
}

func (s *Server) lookupTool(endpointScope, key string) (module.MCPToolDefinition, bool) {
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		if !scopeMatches(endpointScope, scopeForModule(detail.Manifest.Key)) {
			continue
		}
		for _, item := range detail.Manifest.MCP.Tools {
			if item.Key == key {
				return item, true
			}
		}
	}
	return module.MCPToolDefinition{}, false
}

func (s *Server) lookupResourceByKey(endpointScope, key string) (module.MCPResourceDefinition, bool) {
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		if !scopeMatches(endpointScope, scopeForModule(detail.Manifest.Key)) {
			continue
		}
		for _, item := range detail.Manifest.MCP.Resources {
			if item.Key == key {
				return item, true
			}
		}
	}
	return module.MCPResourceDefinition{}, false
}

func (s *Server) lookupResourceByURI(endpointScope, uri string) (module.MCPResourceDefinition, bool) {
	trimmed := strings.TrimSpace(uri)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		if !scopeMatches(endpointScope, scopeForModule(detail.Manifest.Key)) {
			continue
		}
		for _, item := range detail.Manifest.MCP.Resources {
			if item.URI == trimmed {
				return item, true
			}
		}
	}
	return module.MCPResourceDefinition{}, false
}

func (s *Server) lookupApp(endpointScope, key string) (module.MCPAppDefinition, bool) {
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		if !scopeMatches(endpointScope, scopeForModule(detail.Manifest.Key)) {
			continue
		}
		for _, item := range detail.Manifest.MCP.Apps {
			if item.Key == key {
				return item, true
			}
		}
	}
	return module.MCPAppDefinition{}, false
}

func errorResponse(id any, status int, err error) JSONRPCResponse {
	code := -32000
	switch status {
	case http.StatusBadRequest:
		code = -32602
	case http.StatusUnauthorized:
		code = -32001
	case http.StatusForbidden:
		code = -32003
	case http.StatusNotFound:
		code = -32004
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &JSONRPCError{Code: code, Message: err.Error()}}
}

func scopeMatches(endpointScope, itemScope string) bool {
	if strings.TrimSpace(endpointScope) == "" {
		return true
	}
	return strings.TrimSpace(endpointScope) == strings.TrimSpace(itemScope)
}

func scopeForModule(moduleKey string) string {
	switch strings.TrimSpace(moduleKey) {
	case "analytics":
		return EndpointScopeAnalytics
	default:
		return strings.TrimSpace(moduleKey)
	}
}

func builtInToolScope(name string) string {
	switch {
	case strings.HasPrefix(strings.TrimSpace(name), "analytics."):
		return EndpointScopeAnalytics
	case strings.HasPrefix(strings.TrimSpace(name), "template."):
		return "template"
	default:
		return ""
	}
}

func allowsAll(check PermissionChecker, permissions []string) bool {
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == "" {
			continue
		}
		if check == nil || !check(permission) {
			return false
		}
	}
	return true
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func escapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
