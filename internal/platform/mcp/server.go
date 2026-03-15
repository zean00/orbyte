package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/module"
)

const ProtocolVersion = "2024-11-05"

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
	analyticsStreamPath string
}

func NewServer(modules *module.Service, analyticsSvc *analytics.Service, analyticsStreamPath string) *Server {
	return &Server{modules: modules, analytics: analyticsSvc, analyticsStreamPath: analyticsStreamPath}
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
