package httpx

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/config"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/organization"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func registerAdminOverviewRoutes(mux *http.ServeMux, cfg *config.Service, org *organization.Service, ident *identity.Service, modules *module.Service, workflowSvc *workflow.Service, policySvc *policy.Service, acpSvc *acp.Service, mcpServer *mcp.Server) {
	mux.HandleFunc("GET /admin/api/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read")
		if !ok {
			return
		}
		respondJSON(w, http.StatusOK, buildAdminBootstrapPayload(r, org, ident, modules, p, acpSvc))
	})

	mux.HandleFunc("GET /admin/api/config/validate", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		report := cfg.ValidateAll(strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id")))
		respondJSON(w, http.StatusOK, report)
	})

	mux.HandleFunc("GET /admin/api/modules", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		items := modules.ListForScope(
			strings.TrimSpace(r.URL.Query().Get("organization_id")),
			strings.TrimSpace(r.URL.Query().Get("location_id")),
			strings.TrimSpace(r.URL.Query().Get("operating_unit_id")),
		)
		respondJSON(w, http.StatusOK, map[string]any{
			"items":            items,
			"dependency_graph": buildAdminModuleDependencyGraph(items),
		})
	})

	mux.HandleFunc("GET /admin/api/modules/compatibility", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": modules.CompatibilityReport()})
	})

	mux.HandleFunc("GET /admin/api/modules/dependency-graph", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		items := modules.ListForScope(
			strings.TrimSpace(r.URL.Query().Get("organization_id")),
			strings.TrimSpace(r.URL.Query().Get("location_id")),
			strings.TrimSpace(r.URL.Query().Get("operating_unit_id")),
		)
		respondJSON(w, http.StatusOK, buildAdminModuleDependencyGraph(items))
	})

	mux.HandleFunc("GET /admin/api/security/role-templates", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": modules.RoleTemplates()})
	})

	mux.HandleFunc("GET /admin/api/security/policy-hooks", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "module.read", "", "module.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items": policySvc.Runtimes(strings.TrimSpace(r.URL.Query().Get("organization_id")), strings.TrimSpace(r.URL.Query().Get("location_id"))),
		})
	})

	mux.HandleFunc("GET /admin/api/workflows", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": workflowSvc.ListDefinitions()})
	})

	mux.HandleFunc("GET /admin/api/mcp", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, buildAdminMCPPayload(r, cfg, mcpServer))
	})

	mux.HandleFunc("PUT /admin/api/mcp/tools/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/enabled") {
			respondError(w, shared.NotFound("mcp tool route not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "configuration.manage", "", "configuration.manage")
		if !ok {
			return
		}
		toolKey := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/api/mcp/tools/"), "/enabled"))
		if toolKey == "" {
			respondError(w, shared.NotFound("mcp tool not found"))
			return
		}
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid request body"))
			return
		}
		def, ok := cfg.Definition("platform.mcp")
		if !ok {
			respondError(w, shared.NotFound("mcp configuration definition not found"))
			return
		}
		current, _ := cfg.Resolve("platform.mcp", "", "")
		next := map[string]any{}
		for key, value := range current.Value {
			next[key] = value
		}
		states := parseToolStatesJSON(next["tool_states_json"])
		states[toolKey] = req.Enabled
		body, _ := json.Marshal(states)
		next["tool_states_json"] = string(body)
		entry, err := saveConfigEntry(cfg, modules, nil, def, configUpdateRequest{
			Scope: "deployment",
			Value: next,
		}, principalActorID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"tool_key": toolKey,
			"enabled":  req.Enabled,
			"entry":    entry,
			"runtime":  buildAdminMCPPayload(r, cfg, mcpServer),
		})
	})

	mux.HandleFunc("GET /admin/api/acp", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		def, ok := cfg.Definition("platform.acp")
		if !ok {
			respondError(w, shared.NotFound("acp configuration definition not found"))
			return
		}
		entry, ok := cfg.Resolve("platform.acp", "", "")
		if !ok {
			respondError(w, shared.NotFound("acp configuration not found"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"definition": def,
			"entry":      entry,
			"runtime":    buildACPBootstrap(acpSvc),
		})
	})

}

func buildAdminMCPPayload(r *http.Request, cfg *config.Service, server *mcp.Server) map[string]any {
	address := ":8080"
	if value, ok := cfg.Resolve("platform.http", "", ""); ok {
		if current, ok := value.Value["address"].(string); ok && strings.TrimSpace(current) != "" {
			address = strings.TrimSpace(current)
		}
	}
	baseURL := requestBaseURL(r)
	return map[string]any{
		"runtime": map[string]any{
			"enabled":          server != nil && server.MCPEnabled(),
			"http_address":     address,
			"port":             portFromAddress(address),
			"request_host":     r.Host,
			"base_url":         baseURL,
			"protocol_version": mcp.ProtocolVersion,
			"contract_version": mcp.ContractVersion,
			"paths": []map[string]any{
				{"key": "default", "label": "Default MCP", "path": "/mcp", "url": baseURL + "/mcp"},
				{"key": "analytics", "label": "Analytics MCP", "path": "/mcp/analytics", "url": baseURL + "/mcp/analytics"},
				{"key": "analytics_stream", "label": "Analytics Stream", "path": "/mcp/analytics/events/analytics/snapshot", "url": baseURL + "/mcp/analytics/events/analytics/snapshot"},
			},
		},
		"tools":     toolInventoryPayload(server),
		"resources": resourceInventoryPayload(server),
		"apps":      appInventoryPayload(server),
	}
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "127.0.0.1:8080"
	}
	return scheme + "://" + host
}

func portFromAddress(address string) string {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, ":") {
		return strings.TrimPrefix(trimmed, ":")
	}
	if host, port, err := net.SplitHostPort(trimmed); err == nil {
		_ = host
		return port
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Port() != "" {
		return parsed.Port()
	}
	return ""
}

func parseToolStatesJSON(value any) map[string]bool {
	states := map[string]bool{}
	switch current := value.(type) {
	case string:
		trimmed := strings.TrimSpace(current)
		if trimmed == "" {
			return states
		}
		_ = json.Unmarshal([]byte(trimmed), &states)
	case map[string]any:
		for key, raw := range current {
			flag, ok := raw.(bool)
			if !ok {
				continue
			}
			states[strings.TrimSpace(key)] = flag
		}
	}
	return states
}

func toolInventoryPayload(server *mcp.Server) []any {
	if server == nil {
		return []any{}
	}
	items := server.ToolInventory()
	output := make([]any, 0, len(items))
	for _, item := range items {
		output = append(output, item)
	}
	return output
}

func resourceInventoryPayload(server *mcp.Server) []any {
	if server == nil {
		return []any{}
	}
	items := server.ResourceInventory()
	output := make([]any, 0, len(items))
	for _, item := range items {
		output = append(output, item)
	}
	return output
}

func appInventoryPayload(server *mcp.Server) []any {
	if server == nil {
		return []any{}
	}
	items := server.AppInventory()
	output := make([]any, 0, len(items))
	for _, item := range items {
		output = append(output, item)
	}
	return output
}
