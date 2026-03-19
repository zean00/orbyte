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
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/templateoutput"
)

const ProtocolVersion = "2024-11-05"

const (
	templateDesignerResourceURI = "orbyte://apps/template.designer"
	templateDesignerAppKey      = "template.designer"
)

type PermissionChecker func(permissionKey string) bool

type ActorContext struct {
	ActorID           string
	SessionID         string
	OrganizationID    string
	LocationID        string
	PermissionChecker PermissionChecker
}

type Server struct {
	modules             *module.Service
	analytics           *analytics.Service
	templates           *templateoutput.Service
	analyticsStreamPath string
}

func NewServer(modules *module.Service, analyticsSvc *analytics.Service, templates *templateoutput.Service, analyticsStreamPath string) *Server {
	return &Server{modules: modules, analytics: analyticsSvc, templates: templates, analyticsStreamPath: analyticsStreamPath}
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
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

type ResourceDescriptor struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
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
		for _, def := range detail.Manifest.MCP.Tools {
			if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
				continue
			}
			items = append(items, ToolDescriptor{
				Name:        def.Key,
				Title:       def.Title,
				Description: def.Description,
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
		for _, def := range detail.Manifest.MCP.Resources {
			if !allowsAll(actor.PermissionChecker, def.RequiredPermissions) {
				continue
			}
			items = append(items, ResourceDescriptor{
				URI:         def.URI,
				Name:        def.Title,
				Description: def.Description,
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
	def, ok := s.lookupResourceByURI(uri)
	if !ok {
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
		appDef, ok := s.lookupApp(def.AppKey)
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
	def, ok := s.lookupTool(strings.TrimSpace(name))
	if !ok {
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
			if appDef, ok := s.lookupApp(def.AppKey); ok {
				if resource, ok := s.lookupResourceByKey(appDef.ResourceKey); ok {
					result["_meta"] = map[string]any{
						"orbyte/app": map[string]any{
							"key":          appDef.Key,
							"title":        appDef.Title,
							"resource_uri": resource.URI,
							"stream_uri":   s.analyticsStreamPath,
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
	if s == nil || s.templates == nil {
		return nil
	}
	type builtInTool struct {
		name        string
		title       string
		description string
		permission  string
		inputSchema map[string]any
	}
	defs := []builtInTool{
		{
			name:        "template.definition.list",
			title:       "List Template Definitions",
			description: "List available print template definitions.",
			permission:  "template.read",
		},
		{
			name:        "template.definition.get",
			title:       "Get Template Definition",
			description: "Get one template definition and version metadata.",
			permission:  "template.read",
			inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
		},
		{
			name:        "template.draft.get",
			title:       "Get Template Draft",
			description: "Load the latest draft or defaults for a template.",
			permission:  "template.read",
			inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
		},
		{
			name:        "template.draft.save",
			title:       "Save Template Draft",
			description: "Create or update a template draft.",
			permission:  "template.manage",
			inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}, "required": []string{"template_key"}},
		},
		{
			name:        "template.render.preview",
			title:       "Preview Template Render",
			description: "Render a template preview in HTML or the requested output format.",
			permission:  "template.render",
			inputSchema: map[string]any{"type": "object", "properties": map[string]any{"template_key": map[string]any{"type": "string"}}},
		},
	}
	items := make([]ToolDescriptor, 0, len(defs))
	for _, def := range defs {
		if !allowsAll(actor.PermissionChecker, []string{def.permission}) {
			continue
		}
		items = append(items, ToolDescriptor{
			Name:        def.name,
			Title:       def.title,
			Description: def.description,
			InputSchema: cloneMap(def.inputSchema),
		})
	}
	return items
}

func (s *Server) listBuiltInResources(actor ActorContext) []ResourceDescriptor {
	if s == nil || s.templates == nil || !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
		return nil
	}
	return []ResourceDescriptor{{
		URI:         templateDesignerResourceURI,
		Name:        "Template Designer",
		Description: "Lightweight MCP app for template draft inspection and preview.",
		MIMEType:    "text/html",
	}}
}

func (s *Server) readBuiltInResource(actor ActorContext, uri string) ([]ResourceContent, bool, error) {
	if s == nil || s.templates == nil {
		return nil, false, nil
	}
	parsed, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return nil, true, err
	}
	if parsed.Scheme != "orbyte" || parsed.Host != "apps" || parsed.Path != "/template.designer" {
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
}

func (s *Server) callBuiltInTool(actor ActorContext, name string, arguments map[string]any) (map[string]any, bool, error) {
	if s == nil || s.templates == nil {
		return nil, false, nil
	}
	switch name {
	case "template.definition.list":
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDefinitionList(actor), true, nil
	case "template.definition.get":
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDefinitionGet(actor, arguments)
	case "template.draft.get":
		if !allowsAll(actor.PermissionChecker, []string{"template.read"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDraftGet(actor, arguments)
	case "template.draft.save":
		if !allowsAll(actor.PermissionChecker, []string{"template.manage"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateDraftSave(actor, arguments)
	case "template.render.preview":
		if !allowsAll(actor.PermissionChecker, []string{"template.render"}) {
			return nil, true, fmt.Errorf("tool is not allowed")
		}
		return s.templateRenderPreview(actor, arguments)
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
	version, err := s.templates.SaveDraft(templateKey, body, style, actor.ActorID)
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
		streamURI, _ := json.Marshal(s.analyticsStreamPath)
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

func (s *Server) lookupTool(key string) (module.MCPToolDefinition, bool) {
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
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

func (s *Server) lookupResourceByKey(key string) (module.MCPResourceDefinition, bool) {
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
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

func (s *Server) lookupResourceByURI(uri string) (module.MCPResourceDefinition, bool) {
	trimmed := strings.TrimSpace(uri)
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
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

func (s *Server) lookupApp(key string) (module.MCPAppDefinition, bool) {
	for _, detail := range s.modules.List() {
		if !detail.Installed.Enabled {
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
