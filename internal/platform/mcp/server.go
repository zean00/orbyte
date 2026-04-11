package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"

	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
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
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/offline"
	"orbyte/internal/platform/otel"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/runtimehealth"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
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
	documentActions           *application.DocumentActions
	models                    *model.Service
	crm                       *application.CRMCoreService
	reference                 *reference.Service
	search                    *search.Service
	fieldSecurity             *securityfields.Service
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
	planning                  *application.PlanningCoreService
	analyticsStreamPath       string
	analyticsScopedStreamPath string
	builtInToolRegistrations  []builtInToolRegistration
	builtInToolIndex          map[string]builtInToolRegistration
	builtInResourceRegistry   []builtInResourceRegistration
	builtInResourceIndex      map[string]builtInResourceRegistration
	otelTracer                trace.Tracer
}

type ServerDeps struct {
	Modules                   *module.Service
	Analytics                 *analytics.Service
	Templates                 *templateoutput.Service
	Workflows                 *workflow.Service
	Identity                  *identity.Service
	Config                    *config.Service
	Flags                     *featureflags.Service
	Integration               *integration.Service
	Documents                 *document.Service
	DocumentActions           *application.DocumentActions
	Models                    *model.Service
	CRM                       *application.CRMCoreService
	Reference                 *reference.Service
	Search                    *search.Service
	FieldSecurity             *securityfields.Service
	Policy                    *policy.Service
	Eventing                  *eventing.Service
	Jobs                      *jobs.Service
	Health                    *runtimehealth.Tracker
	Audit                     *audit.Service
	Observability             *observability.Service
	Offline                   *offline.Service
	Dataops                   *dataops.Service
	Engagement                *engagement.Service
	Planning                  *application.PlanningCoreService
	AnalyticsStreamPath       string
	AnalyticsScopedStreamPath string
	OTel                      *otel.Service
}

func NewServer(deps ServerDeps) *Server {
	server := &Server{
		modules:                   deps.Modules,
		analytics:                 deps.Analytics,
		templates:                 deps.Templates,
		workflows:                 deps.Workflows,
		identity:                  deps.Identity,
		config:                    deps.Config,
		flags:                     deps.Flags,
		integration:               deps.Integration,
		documents:                 deps.Documents,
		documentActions:           deps.DocumentActions,
		models:                    deps.Models,
		crm:                       deps.CRM,
		reference:                 deps.Reference,
		search:                    deps.Search,
		fieldSecurity:             deps.FieldSecurity,
		policy:                    deps.Policy,
		eventing:                  deps.Eventing,
		jobs:                      deps.Jobs,
		health:                    deps.Health,
		audit:                     deps.Audit,
		observability:             deps.Observability,
		offline:                   deps.Offline,
		dataops:                   deps.Dataops,
		engagement:                deps.Engagement,
		implementation:            NewImplementationService(),
		planning:                  deps.Planning,
		analyticsStreamPath:       deps.AnalyticsStreamPath,
		analyticsScopedStreamPath: deps.AnalyticsScopedStreamPath,
	}
	if deps.OTel != nil {
		server.otelTracer = deps.OTel.Tracer()
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
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Loaded analytics dashboard %s.", item.Name)}},
		"structuredContent": map[string]any{
			"dashboard": item,
			"artifact":  s.dashboardBoardArtifactPayload(item, actor),
		},
		"_meta": s.analyticsAppMeta("dashboard", item.ID),
	}, true, nil
}

func (s *Server) analyticsDashboardWidgetCatalog(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	surface := firstNonEmpty(stringArg(arguments, "surface"), string(module.UISurfaceDashboard))
	items := make([]module.DashboardWidgetDefinition, 0)
	for _, item := range s.modules.DashboardWidgetsForSurface(module.UISurface(surface)) {
		if !allowsAll(actor.PermissionChecker, item.RequiredPermissions) {
			continue
		}
		items = append(items, item)
	}
	summary := fmt.Sprintf("Found %d dashboard widgets for %s.", len(items), surface)
	if len(items) > 0 {
		lines := make([]string, 0, len(items))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("%s (%s, %s, %s)", item.Key, firstNonEmpty(item.Title, item.Key), firstNonEmpty(item.RendererKind, "metric"), firstNonEmpty(item.DataPath, "-")))
		}
		summary = summary + " Available widgets: " + strings.Join(lines, "; ")
	}
	return map[string]any{
		"content":           []ContentBlock{{Type: "text", Text: summary}},
		"structuredContent": map[string]any{"surface": surface, "items": items},
	}, true, nil
}

func (s *Server) analyticsDashboardWidgetPreview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	surface := firstNonEmpty(stringArg(arguments, "surface"), string(module.UISurfaceDashboard))
	title := strings.TrimSpace(stringArg(arguments, "title"))
	description := strings.TrimSpace(stringArg(arguments, "description"))
	intent := strings.TrimSpace(stringArg(arguments, "intent"))
	widgetKey := strings.TrimSpace(stringArg(arguments, "widget_key"))
	if widgetKey == "" {
		candidates := s.recommendedDashboardWidgetKeys(actor, module.UISurface(surface), title, description, intent, 1)
		if len(candidates) == 0 {
			return nil, true, shared.Validation("widget_key is required or the title/description must match at least one dashboard widget")
		}
		widgetKey = candidates[0]
	}
	definition, ok := s.modules.DashboardWidgetForSurface(widgetKey, module.UISurface(surface))
	if !ok {
		return nil, true, shared.Validation("dashboard widget not found for the requested surface")
	}
	if !allowsAll(actor.PermissionChecker, definition.RequiredPermissions) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	artifact := s.dashboardWidgetArtifactPayload(definition)
	artifactBlock := dashboardArtifactBlockText(artifact)
	text := fmt.Sprintf("Prepared dashboard widget preview %s using %s. Include this exact dashboard artifact block in your final answer when presenting this preview: %s", firstNonEmpty(title, definition.Title, widgetKey), widgetKey, artifactBlock)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"widget":   definition,
			"artifact": artifact,
		},
	}, true, nil
}

func (s *Server) analyticsDashboardWidgetsPreview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	surface := firstNonEmpty(stringArg(arguments, "surface"), string(module.UISurfaceDashboard))
	title := strings.TrimSpace(stringArg(arguments, "title"))
	description := strings.TrimSpace(stringArg(arguments, "description"))
	intent := strings.TrimSpace(stringArg(arguments, "intent"))
	limit := intArg(arguments, "limit")
	if limit <= 0 {
		limit = 3
	}
	if limit > 3 {
		limit = 3
	}
	widgetKeys := stringSliceArg(arguments, "widget_keys")
	if len(widgetKeys) == 0 {
		widgetKeys = s.recommendedDashboardWidgetKeys(actor, module.UISurface(surface), title, description, intent, limit)
	}
	selected := make([]module.DashboardWidgetDefinition, 0, limit)
	for _, key := range widgetKeys {
		if len(selected) >= limit {
			break
		}
		definition, ok := s.modules.DashboardWidgetForSurface(strings.TrimSpace(key), module.UISurface(surface))
		if !ok || !allowsAll(actor.PermissionChecker, definition.RequiredPermissions) {
			continue
		}
		selected = append(selected, definition)
	}
	if len(selected) == 0 {
		return nil, true, shared.Validation("widget_keys are required or the title/description must match at least one dashboard widget")
	}
	artifacts := make([]map[string]any, 0, len(selected))
	blocks := make([]string, 0, len(selected))
	keys := make([]string, 0, len(selected))
	for _, definition := range selected {
		artifact := s.dashboardWidgetArtifactPayload(definition)
		artifacts = append(artifacts, artifact)
		blocks = append(blocks, dashboardArtifactBlockText(artifact))
		keys = append(keys, definition.Key)
	}
	text := fmt.Sprintf(
		"Prepared %d focused dashboard widget previews using %s. Include each exact dashboard artifact block in your final answer when presenting this preview: %s",
		len(artifacts),
		strings.Join(keys, ", "),
		strings.Join(blocks, " "),
	)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"widgets":   selected,
			"artifacts": artifacts,
		},
	}, true, nil
}

func (s *Server) analyticsDashboardBoardPreview(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.read"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	board, err := s.dashboardBoardFromArguments(actor, arguments, false)
	if err != nil {
		return nil, true, err
	}
	artifact := s.dashboardBoardArtifactPayload(board, actor)
	artifactBlock := dashboardArtifactBlockText(artifact)
	insightSummary := s.dashboardBoardInsightSummary(actor, board)
	text := fmt.Sprintf("Prepared dashboard board preview %s. Widget keys: %s.", board.Name, dashboardWidgetKeySummary(board.Widgets))
	if strings.TrimSpace(insightSummary) != "" {
		text += " " + insightSummary
	}
	text += fmt.Sprintf(" Include this exact dashboard artifact block in your final answer when presenting this preview: %s", artifactBlock)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: text}},
		"structuredContent": map[string]any{
			"dashboard": board,
			"artifact":  artifact,
		},
	}, true, nil
}

func (s *Server) analyticsDashboardBoardCreate(actor ActorContext, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.analytics == nil || s.modules == nil {
		return nil, false, nil
	}
	if !allowsAll(actor.PermissionChecker, []string{"analytics.author"}) {
		return nil, true, fmt.Errorf("tool is not allowed")
	}
	board, err := s.dashboardBoardFromArguments(actor, arguments, true)
	if err != nil {
		return nil, true, err
	}
	saved, err := s.analytics.SaveDashboard(board)
	if err != nil {
		return nil, true, err
	}
	artifact := s.dashboardBoardArtifactPayload(saved, actor)
	artifactBlock := dashboardArtifactBlockText(artifact)
	return map[string]any{
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Created dashboard board %s. Widget keys: %s. Include this exact dashboard artifact block in your final answer so the workspace can render it live: %s", saved.Name, dashboardWidgetKeySummary(saved.Widgets), artifactBlock)}},
		"structuredContent": map[string]any{
			"dashboard": saved,
			"artifact":  artifact,
		},
		"_meta": s.analyticsAppMeta("dashboard", saved.ID),
	}, true, nil
}

func dashboardWidgetKeySummary(widgets []analytics.DashboardWidget) string {
	if len(widgets) == 0 {
		return "none"
	}
	lines := make([]string, 0, len(widgets))
	for _, widget := range widgets {
		lines = append(lines, firstNonEmpty(widget.WidgetKey, firstNonEmpty(widget.Title, "widget")))
	}
	return strings.Join(lines, ", ")
}

func (s *Server) dashboardBoardInsightSummary(actor ActorContext, board analytics.Dashboard) string {
	if len(board.Widgets) == 0 {
		return ""
	}
	hasSalesDemo := false
	hasPlanningReplenishment := false
	for _, widget := range board.Widgets {
		key := strings.TrimSpace(widget.WidgetKey)
		if strings.HasPrefix(key, "analytics.demo.sales.") {
			hasSalesDemo = true
		}
		if strings.HasPrefix(key, "planning.replenishment.") {
			hasPlanningReplenishment = true
		}
	}
	if hasSalesDemo {
		return s.dashboardDemoSalesSummary()
	}
	if hasPlanningReplenishment && s != nil && s.planning != nil {
		return s.dashboardPlanningReplenishmentSummary(actor, board)
	}
	return ""
}

func (s *Server) dashboardDemoSalesSummary() string {
	if s == nil || s.analytics == nil {
		return ""
	}
	points := mcpDashboardDemoSalesRows(s.analytics.Snapshot())
	if len(points) == 0 {
		return ""
	}
	best := points[0]
	laggards := make([]mcpDashboardDemoSalesRow, 0, 2)
	for _, point := range points[1:] {
		laggards = append(laggards, point)
	}
	sort.SliceStable(laggards, func(i, j int) bool {
		return laggards[i].NetSales < laggards[j].NetSales
	})
	parts := []string{
		fmt.Sprintf("Demo sales highlights: %s leads on net sales at %s.", best.Label, formatWholeNumber(best.NetSales)),
	}
	if len(laggards) > 0 {
		names := make([]string, 0, len(laggards))
		for _, point := range laggards {
			names = append(names, point.Label)
			if len(names) >= 2 {
				break
			}
		}
		if len(names) > 0 {
			parts = append(parts, fmt.Sprintf("%s are trailing the benchmark in this demo dataset.", strings.Join(names, " and ")))
		}
	}
	return strings.Join(parts, " ")
}

type mcpDashboardDemoSalesRow struct {
	LocationID  string
	Label       string
	NetSales    float64
	TargetSales float64
}

func mcpDashboardDemoSalesRows(snapshot analytics.Snapshot) []mcpDashboardDemoSalesRow {
	keys := make([]string, 0, len(snapshot.Segments.ByLocation))
	for key := range snapshot.Segments.ByLocation {
		if !strings.HasPrefix(strings.TrimSpace(key), "loc_demo_") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]mcpDashboardDemoSalesRow, 0, len(keys))
	for _, key := range keys {
		kpi := snapshot.Segments.ByLocation[key]
		netSales, targetSales := mcpSyntheticSalesForLocation(key, kpi)
		rows = append(rows, mcpDashboardDemoSalesRow{
			LocationID:  key,
			Label:       mcpDashboardLocationLabel(key),
			NetSales:    netSales,
			TargetSales: targetSales,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].NetSales > rows[j].NetSales
	})
	return rows
}

func mcpSyntheticSalesForLocation(locationID string, kpi analytics.DocumentKPI) (float64, float64) {
	switch strings.TrimSpace(locationID) {
	case "loc_demo_central":
		return 7900000, 10800000
	case "loc_demo_west":
		return 9800000, 11800000
	case "loc_demo_east":
		return 15800000, 14900000
	default:
		baseSales := float64(kpi.Submitted*1250000 + kpi.Approved*1850000 + 3500000)
		targetSales := baseSales * 1.1
		return baseSales, targetSales
	}
}

func mcpDashboardLocationLabel(locationID string) string {
	switch strings.TrimSpace(locationID) {
	case "":
		return "Unscoped"
	case "loc_hq":
		return "Head Office"
	case "loc_demo_central":
		return "Loc Demo Central"
	case "loc_demo_east":
		return "Loc Demo East"
	case "loc_demo_west":
		return "Loc Demo West"
	default:
		return locationID
	}
}

func formatWholeNumber(value float64) string {
	return strconv.FormatFloat(math.Round(value), 'f', 0, 64)
}

func (s *Server) dashboardPlanningReplenishmentSummary(actor ActorContext, board analytics.Dashboard) string {
	if s == nil || s.planning == nil {
		return ""
	}
	warehouseCode := extractWarehouseCode(board.Description + " " + board.Name)
	summary := s.planning.ReplenishmentSummaryScoped(
		actor.OrganizationID,
		actor.LocationID,
		warehouseCode,
		"",
		"",
		"",
		false,
		false,
		false,
		time.Now().UTC(),
	)
	if len(summary.Items) == 0 {
		return ""
	}
	atRisk := make([]string, 0, 2)
	healthy := make([]string, 0, 2)
	for _, row := range summary.Items {
		name := firstNonEmpty(textValue(row["item_name"]), textValue(row["item_code"]))
		if name == "" {
			continue
		}
		if roundedQuantity(numberValue(row["suggested_request_quantity"])) > 0 {
			atRisk = append(atRisk, fmt.Sprintf("%s (%s suggested)", name, formatQuantityLabel(numberValue(row["suggested_request_quantity"]))))
		} else {
			healthy = append(healthy, name)
		}
		if len(atRisk) >= 2 && len(healthy) >= 2 {
			break
		}
	}
	parts := []string{
		fmt.Sprintf("Replenishment highlights: %d shortage candidates and %s suggested units", summary.ShortageItemCount, formatQuantityLabel(summary.TotalSuggestedRequestQuantity)),
	}
	if warehouseCode != "" {
		parts = append(parts, fmt.Sprintf("for warehouse %s", warehouseCode))
	}
	if len(atRisk) > 0 {
		parts = append(parts, "At-risk items: "+strings.Join(atRisk, "; "))
	}
	if len(healthy) > 0 {
		parts = append(parts, "Healthy items to skip: "+strings.Join(healthy, "; "))
	}
	return strings.Join(parts, ". ") + "."
}

func extractWarehouseCode(text string) string {
	for _, token := range strings.Fields(strings.NewReplacer(",", " ", ".", " ", ";", " ", "\"", " ").Replace(text)) {
		token = strings.TrimSpace(token)
		if strings.HasPrefix(token, "WH-") {
			return token
		}
	}
	return ""
}

func formatQuantityLabel(value float64) string {
	rounded := roundedQuantity(value)
	if math.Abs(rounded-math.Round(rounded)) < 0.00001 {
		return strconv.FormatInt(int64(math.Round(rounded)), 10)
	}
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

func roundedQuantity(value float64) float64 {
	return math.Round(value*100) / 100
}

func dashboardArtifactBlockText(artifact map[string]any) string {
	body, err := json.Marshal(artifact)
	if err != nil {
		return "<orbyte-dashboard-artifact>{}</orbyte-dashboard-artifact>"
	}
	return "<orbyte-dashboard-artifact>" + string(body) + "</orbyte-dashboard-artifact>"
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
		"content": []ContentBlock{{Type: "text", Text: fmt.Sprintf("Saved analytics dashboard %s.", saved.Name)}},
		"structuredContent": map[string]any{
			"dashboard": saved,
			"artifact":  s.dashboardBoardArtifactPayload(saved, actor),
		},
		"_meta": s.analyticsAppMeta("dashboard", saved.ID),
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

func (s *Server) dashboardBoardFromArguments(actor ActorContext, arguments map[string]any, requireTitle bool) (analytics.Dashboard, error) {
	surface := firstNonEmpty(stringArg(arguments, "surface"), string(module.UISurfaceDashboard))
	widgetKeys := stringArrayArg(arguments, "widget_keys")
	title := strings.TrimSpace(stringArg(arguments, "title"))
	if requireTitle && title == "" {
		return analytics.Dashboard{}, shared.Validation("title is required")
	}
	if title == "" {
		title = "Agent Dashboard Preview"
	}
	description := strings.TrimSpace(stringArg(arguments, "description"))
	if len(widgetKeys) == 0 {
		widgetKeys = s.recommendedDashboardWidgetKeys(actor, module.UISurface(surface), title, description, "", 6)
	}
	if len(widgetKeys) == 0 {
		return analytics.Dashboard{}, shared.Validation("widget_keys is required when Orbyte cannot infer matching widgets from the title and description")
	}
	widgets := make([]analytics.DashboardWidget, 0, len(widgetKeys))
	for index, key := range widgetKeys {
		def, ok := s.modules.DashboardWidgetForSurface(key, module.UISurface(surface))
		if !ok {
			return analytics.Dashboard{}, shared.NotFound("dashboard widget not found")
		}
		if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
			return analytics.Dashboard{}, shared.Forbidden("dashboard widget is not allowed")
		}
		widgets = append(widgets, analytics.DashboardWidget{
			Title:     firstNonEmpty(def.Title, key),
			Kind:      firstNonEmpty(def.RendererKind, "metric"),
			WidgetKey: def.Key,
			Width:     widgetSizeValue(def.DefaultWidth, 3),
			Height:    widgetSizeValue(def.DefaultHeight, 1),
			Order:     index + 1,
		})
	}
	return analytics.Dashboard{
		Name:        title,
		Description: description,
		Surface:     surface,
		Visibility:  "private",
		IsDefault:   false,
		Status:      "active",
		Widgets:     widgets,
		RuntimeScope: analytics.RuntimeScope{
			ScopeType:      "user",
			OwnerUserID:    firstNonEmpty(actor.EffectiveUserID, actor.ActorID),
			OrganizationID: actor.OrganizationID,
			LocationID:     actor.LocationID,
		},
		UpdatedBy: firstNonEmpty(actor.EffectiveUserID, actor.ActorID),
	}, nil
}

func (s *Server) recommendedDashboardWidgetKeys(actor ActorContext, surface module.UISurface, title, description, explicitIntent string, limit int) []string {
	if s == nil || s.modules == nil {
		return nil
	}
	items := s.modules.DashboardWidgetsForSurface(surface)
	if len(items) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	needles := strings.ToLower(strings.TrimSpace(title + " " + description))
	if needles == "" {
		return nil
	}
	intent := inferDashboardIntent(firstNonEmpty(explicitIntent, needles))
	selected := make([]string, 0, limit)
	seen := make(map[string]struct{})
	add := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		selected = append(selected, key)
	}
	// Demo-only sales widgets are intentionally never inferred for generic
	// sales prompts. They should only be selected explicitly through the
	// catalog or via scenario-specific prompts that pass widget keys.
	if strings.Contains(needles, "replenishment") ||
		strings.Contains(needles, "shortage") ||
		(strings.Contains(needles, "warehouse") &&
			(strings.Contains(needles, "suggested") ||
				strings.Contains(needles, "risk") ||
				strings.Contains(needles, "stock"))) {
		for _, key := range []string{
			"planning.replenishment.shortages",
			"planning.replenishment.items",
		} {
			if def, ok := s.modules.DashboardWidgetForSurface(key, surface); ok && allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
				add(key)
			}
		}
		if len(selected) > 0 {
			return selected
		}
	}
	type scoredWidget struct {
		def          module.DashboardWidgetDefinition
		score        int
		rendererBias int
	}
	scored := make([]scoredWidget, 0, len(items))
	available := make([]module.DashboardWidgetDefinition, 0, len(items))
	for _, item := range items {
		if !allowsAll(actor.PermissionChecker, item.RequiredPermissions) {
			continue
		}
		available = append(available, item)
		text := strings.ToLower(strings.Join([]string{item.Key, item.Title, item.RendererKind, item.DataPath}, " "))
		score := 0
		for _, token := range strings.Fields(needles) {
			if len(token) < 3 {
				continue
			}
			if strings.Contains(text, token) {
				score++
			}
		}
		if dashboardNeedlesSuggestTargetAttainment(needles) && (strings.Contains(text, "attainment") || strings.Contains(text, "target")) {
			score += 3
		}
		if dashboardNeedlesSuggestComparison(needles) && (strings.Contains(text, "branch") || strings.Contains(text, "mix") || strings.Contains(text, "compare")) {
			score += 3
		}
		if dashboardNeedlesSuggestTrend(needles) && strings.Contains(text, "trend") {
			score += 3
		}
		if score == 0 {
			continue
		}
		scored = append(scored, scoredWidget{
			def:          item,
			score:        score,
			rendererBias: dashboardIntentRendererBias(intent, item.RendererKind),
		})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		left := scored[i].score + scored[i].rendererBias
		right := scored[j].score + scored[j].rendererBias
		if left == right {
			return scored[i].def.Key < scored[j].def.Key
		}
		return left > right
	})
	preferredRenderers := dashboardIntentRendererPreference(intent)
	if len(scored) == 0 {
		for _, renderer := range preferredRenderers {
			for _, item := range available {
				if len(selected) >= limit {
					break
				}
				if item.RendererKind != renderer || !dashboardRendererAllowedForIntent(intent, renderer, needles) {
					continue
				}
				add(item.Key)
				break
			}
		}
		if len(selected) > 0 {
			return selected
		}
	}
	usedRenderers := make(map[string]struct{})
	for _, renderer := range preferredRenderers {
		for _, item := range scored {
			if len(selected) >= limit {
				break
			}
			if item.def.RendererKind != renderer {
				continue
			}
			if dashboardRendererAllowedForIntent(intent, renderer, needles) {
				add(item.def.Key)
				usedRenderers[renderer] = struct{}{}
				break
			}
		}
	}
	for _, item := range scored {
		if len(selected) >= limit {
			break
		}
		if !dashboardRendererAllowedForIntent(intent, item.def.RendererKind, needles) {
			continue
		}
		if _, ok := usedRenderers[item.def.RendererKind]; ok && len(selected) < len(preferredRenderers) {
			continue
		}
		add(item.def.Key)
		usedRenderers[item.def.RendererKind] = struct{}{}
	}
	if len(selected) < limit {
		for _, renderer := range preferredRenderers {
			if len(selected) >= limit {
				break
			}
			if _, ok := usedRenderers[renderer]; ok {
				continue
			}
			for _, item := range available {
				if item.RendererKind != renderer || !dashboardRendererAllowedForIntent(intent, renderer, needles) {
					continue
				}
				add(item.Key)
				if len(selected) > 0 {
					usedRenderers[renderer] = struct{}{}
				}
				break
			}
		}
	}
	return selected
}

func inferDashboardIntent(raw string) string {
	text := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(text, "map") || strings.Contains(text, "location") || strings.Contains(text, "region") || strings.Contains(text, "where"):
		return "geography"
	case strings.Contains(text, "table") || strings.Contains(text, "list") || strings.Contains(text, "breakdown") || strings.Contains(text, "rows"):
		return "detail"
	case strings.Contains(text, "monitor") || strings.Contains(text, "status") || strings.Contains(text, "risk") || strings.Contains(text, "alert") || strings.Contains(text, "watch"):
		return "monitoring"
	case strings.Contains(text, "trend") || strings.Contains(text, "over time") || strings.Contains(text, "daily") || strings.Contains(text, "weekly") || strings.Contains(text, "monthly"):
		return "trend"
	case strings.Contains(text, "compare") || strings.Contains(text, "benchmark") || strings.Contains(text, "strongest") || strings.Contains(text, "weakest") || strings.Contains(text, "underperform"):
		return "comparison"
	default:
		return "insight"
	}
}

func dashboardIntentRendererPreference(intent string) []string {
	switch intent {
	case "geography":
		return []string{"map", "metric", "chart_bar"}
	case "detail":
		return []string{"table", "chart_bar", "metric"}
	case "monitoring":
		return []string{"gauge", "metric", "table"}
	case "trend":
		return []string{"chart_line", "metric", "chart_bar"}
	case "comparison":
		return []string{"chart_bar", "gauge", "chart_line"}
	default:
		return []string{"gauge", "chart_bar", "chart_line"}
	}
}

func dashboardIntentRendererBias(intent, renderer string) int {
	for index, candidate := range dashboardIntentRendererPreference(intent) {
		if candidate == renderer {
			return (len(dashboardIntentRendererPreference(intent)) - index) * 2
		}
	}
	return 0
}

func dashboardRendererAllowedForIntent(intent, renderer, needles string) bool {
	switch renderer {
	case "map":
		return intent == "geography" || strings.Contains(needles, "map") || strings.Contains(needles, "location")
	case "table":
		return intent == "detail" || intent == "monitoring" || strings.Contains(needles, "table") || strings.Contains(needles, "breakdown")
	default:
		return true
	}
}

func dashboardNeedlesSuggestTargetAttainment(needles string) bool {
	return strings.Contains(needles, "target") ||
		strings.Contains(needles, "attainment") ||
		strings.Contains(needles, "underperform") ||
		strings.Contains(needles, "benchmark")
}

func dashboardNeedlesSuggestComparison(needles string) bool {
	return strings.Contains(needles, "compare") ||
		strings.Contains(needles, "versus") ||
		strings.Contains(needles, "against") ||
		strings.Contains(needles, "benchmark") ||
		strings.Contains(needles, "strongest") ||
		strings.Contains(needles, "weakest") ||
		strings.Contains(needles, "branch")
}

func dashboardNeedlesSuggestTrend(needles string) bool {
	return strings.Contains(needles, "trend") ||
		strings.Contains(needles, "over time") ||
		strings.Contains(needles, "daily") ||
		strings.Contains(needles, "weekly") ||
		strings.Contains(needles, "monthly")
}

func (s *Server) dashboardBoardArtifactPayload(item analytics.Dashboard, actor ActorContext) map[string]any {
	surface := firstNonEmpty(strings.TrimSpace(item.Surface), string(module.UISurfaceDashboard))
	widgets := make([]map[string]any, 0, len(item.Widgets))
	for _, widget := range item.Widgets {
		def, ok := s.modules.DashboardWidgetForSurface(widget.WidgetKey, module.UISurface(surface))
		if !ok {
			continue
		}
		if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
			continue
		}
		widgets = append(widgets, map[string]any{
			"id":               widget.ID,
			"title":            firstNonEmpty(widget.Title, def.Title),
			"kind":             firstNonEmpty(widget.Kind, def.RendererKind),
			"width":            widgetSizeValue(widget.Width, widgetSizeValue(def.DefaultWidth, 3)),
			"height":           widgetSizeValue(widget.Height, widgetSizeValue(def.DefaultHeight, 1)),
			"refresh_override": widget.RefreshOverride,
			"definition":       def,
		})
	}
	return map[string]any{
		"id":      firstNonEmpty(item.ID, shared.NewID("artifact")),
		"kind":    "dashboard_board",
		"title":   firstNonEmpty(item.Name, "Dashboard board"),
		"content": "",
		"metadata": map[string]any{
			"kind":      "dashboard_board",
			"title":     firstNonEmpty(item.Name, "Dashboard board"),
			"surface":   surface,
			"board_id":  item.ID,
			"open_path": "/ui/dashboard",
			"widgets":   widgets,
		},
	}
}

func (s *Server) dashboardWidgetArtifactPayload(def module.DashboardWidgetDefinition) map[string]any {
	title := firstNonEmpty(def.Title, def.Key, "Dashboard widget")
	return map[string]any{
		"id":      shared.NewID("artifact"),
		"kind":    "dashboard_widget",
		"title":   title,
		"content": "",
		"metadata": map[string]any{
			"kind":  "dashboard_widget",
			"title": title,
			"widget": map[string]any{
				"id":         shared.NewID("widget"),
				"title":      title,
				"kind":       firstNonEmpty(def.RendererKind, "metric"),
				"width":      widgetSizeValue(def.DefaultWidth, 4),
				"height":     widgetSizeValue(def.DefaultHeight, 1),
				"definition": def,
			},
		},
	}
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

func stringArrayArg(arguments map[string]any, key string) []string {
	value, _ := arguments[key]
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		text = strings.TrimSpace(text)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func widgetSizeValue(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
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
			if !s.ToolEnabled(item.Key) {
				continue
			}
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
