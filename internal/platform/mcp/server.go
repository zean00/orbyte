package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/dataops"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/engagement"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/featureflags"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/integration"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/otel"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/templateoutput"
	"orbyte/internal/platform/workflow"
)

const ProtocolVersion = "2024-11-05"
const ContractVersion = "2026-03-23"

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
	mcpCatalogResourceURI         = "orbyte://control-plane/mcp-catalog"
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
	DeepLinkGrantID    string
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
	documents                 *document.Service
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
	builtInToolRegistrations  []builtInToolRegistration
	builtInToolIndex          map[string]builtInToolRegistration
	builtInResourceRegistry   []builtInResourceRegistration
	builtInResourceIndex      map[string]builtInResourceRegistration
	otelTracer                trace.Tracer
}

func NewServer(modules *module.Service, analyticsSvc *analytics.Service, templates *templateoutput.Service, workflows *workflow.Service, identitySvc *identity.Service, configSvc *config.Service, flagsSvc *featureflags.Service, integrationSvc *integration.Service, documentsSvc *document.Service, referenceSvc *reference.Service, searchSvc *search.Service, policySvc *policy.Service, eventingSvc *eventing.Service, jobSvc *jobs.Service, health *runtimehealth.Tracker, auditSvc *audit.Service, obs *observability.Service, offlineSvc *offline.Service, dataopsSvc *dataops.Service, engagementSvc *engagement.Service, analyticsStreamPath, analyticsScopedStreamPath string, otelSvc *otel.Service) *Server {
	server := &Server{
		modules:                   modules,
		analytics:                 analyticsSvc,
		templates:                 templates,
		workflows:                 workflows,
		identity:                  identitySvc,
		config:                    configSvc,
		flags:                     flagsSvc,
		integration:               integrationSvc,
		documents:                 documentsSvc,
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
	if otelSvc != nil {
		server.otelTracer = otelSvc.Tracer()
	}
	server.mustInitBuiltInTools()
	server.mustInitBuiltInResources()
	return server
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

func (s *Server) documentTypeList(actor ActorContext, _ map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"document.list"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	items := s.documents.DocumentTypes()
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d document types.", len(items))}},
		"structuredContent": map[string]any{"items": items},
	}, true, nil
}

func (s *Server) documentTypeGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"document.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	docType := strings.TrimSpace(stringArg(arguments, "document_type"))
	if docType == "" {
		return nil, true, shared.Validation("document_type is required")
	}
	def, err := s.documents.Definition(docType)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded document type %s.", def.Type)}},
		"structuredContent": def,
	}, true, nil
}

func (s *Server) documentList(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"document.list"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	var docType string
	if v := stringArg(arguments, "document_type"); v != "" {
		docType = v
	}
	records := s.documents.List()
	if docType != "" {
		filtered := make([]document.Record, 0, len(records))
		for _, r := range records {
			if r.Header.Type == docType {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Header.CreatedAt.After(records[j].Header.CreatedAt) })
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Found %d documents.", len(records))}},
		"structuredContent": map[string]any{"items": records},
	}, true, nil
}

func (s *Server) documentGet(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"document.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	docID := strings.TrimSpace(stringArg(arguments, "document_id"))
	if docID == "" {
		return nil, true, shared.Validation("document_id is required")
	}
	record, err := s.documents.Get(docID)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded document %s.", record.Header.ID)}},
		"structuredContent": record,
	}, true, nil
}

func (s *Server) documentCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"document.create"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	docType := strings.TrimSpace(stringArg(arguments, "document_type"))
	if docType == "" {
		return nil, true, shared.Validation("document_type is required")
	}
	var payload map[string]any
	_ = decodeOptionalObjectArg(arguments, "payload", &payload)
	if payload == nil {
		payload = make(map[string]any)
	}
	orgID := firstNonEmpty(stringArg(arguments, "organization_id"), actor.OrganizationID)
	locID := firstNonEmpty(stringArg(arguments, "location_id"), actor.LocationID)
	actorID := firstNonEmpty(actor.EffectiveUserID, actor.ActorID)
	record, err := s.documents.Create(docType, orgID, locID, actorID, payload)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created document %s.", record.Header.ID)}},
		"structuredContent": record,
	}, true, nil
}

func (s *Server) documentUpdate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"document.update_draft"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	docID := strings.TrimSpace(stringArg(arguments, "document_id"))
	if docID == "" {
		return nil, true, shared.Validation("document_id is required")
	}
	var record document.Record
	if err := decodeObjectArg(arguments, "document", &record); err != nil {
		return nil, true, err
	}
	if err := s.documents.Save(record); err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Updated document %s.", docID)}},
		"structuredContent": record,
	}, true, nil
}

func (s *Server) documentDelete(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	docID := strings.TrimSpace(stringArg(arguments, "document_id"))
	if docID == "" {
		return nil, true, shared.Validation("document_id is required")
	}
	if err := s.documents.Delete(docID); err != nil {
		return nil, true, err
	}
	return map[string]any{"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Deleted document %s.", docID)}}}, true, nil
}

func (s *Server) searchQuery(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	indexKey := strings.TrimSpace(stringArg(arguments, "index_key"))
	if indexKey == "" {
		return nil, true, shared.Validation("index_key is required")
	}
	var req search.QueryRequest
	if v := stringArg(arguments, "mode"); v != "" {
		req.Mode = v
	}
	if v := stringArg(arguments, "query"); v != "" {
		req.Query = v
	}
	if v := stringArg(arguments, "vector_text"); v != "" {
		req.VectorText = v
	}
	if v, ok := arguments["filters"]; ok {
		if filters, ok := v.(map[string]any); ok {
			req.Filters = make(map[string]string)
			for k, val := range filters {
				if s, ok := val.(string); ok {
					req.Filters[k] = s
				}
			}
		}
	}
	if v, ok := arguments["page"]; ok {
		if n, ok := v.(float64); ok {
			req.Page = int(n)
		}
	}
	if v, ok := arguments["page_size"]; ok {
		if n, ok := v.(float64); ok {
			req.PageSize = int(n)
		}
	}
	orgID := firstNonEmpty(actor.OrganizationID)
	locID := firstNonEmpty(actor.LocationID)
	result, err := s.search.Query(indexKey, orgID, locID, req)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Search returned %d hits.", result.Total)}},
		"structuredContent": result,
	}, true, nil
}

func (s *Server) integrationSubmissionCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if !allowsAll(actor.PermissionChecker, []string{"configuration.manage"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	systemKey := strings.TrimSpace(stringArg(arguments, "system_key"))
	if systemKey == "" {
		return nil, true, shared.Validation("system_key is required")
	}
	operationType := strings.TrimSpace(stringArg(arguments, "operation_type"))
	if operationType == "" {
		return nil, true, shared.Validation("operation_type is required")
	}
	documentID := strings.TrimSpace(stringArg(arguments, "document_id"))
	correlationID := strings.TrimSpace(stringArg(arguments, "correlation_id"))
	var payload map[string]any
	_ = decodeOptionalObjectArg(arguments, "payload", &payload)
	if payload == nil {
		payload = make(map[string]any)
	}
	record, err := s.integration.CreateSubmission(systemKey, operationType, documentID, correlationID, payload)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created integration submission %s.", record.ID)}},
		"structuredContent": record,
	}, true, nil
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
