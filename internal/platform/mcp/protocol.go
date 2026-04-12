package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

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
	Data    any    `json:"data,omitempty"`
}

type ContractDescriptor struct {
	Version              string   `json:"version,omitempty"`
	Stability            string   `json:"stability,omitempty"`
	SideEffectClass      string   `json:"sideEffectClass,omitempty"`
	Idempotency          string   `json:"idempotency,omitempty"`
	AuditAction          string   `json:"auditAction,omitempty"`
	ActionClass          string   `json:"actionClass,omitempty"`
	RiskClass            string   `json:"riskClass,omitempty"`
	DraftOnly            bool     `json:"draftOnly,omitempty"`
	RequiresConfirmation bool     `json:"requiresConfirmation,omitempty"`
	RequiresApproval     bool     `json:"requiresApproval,omitempty"`
	GovernanceTags       []string `json:"governanceTags,omitempty"`
	BusinessDomains      []string `json:"businessDomains,omitempty"`
	RequiredScopes       []string `json:"requiredScopes,omitempty"`
	RequiredPermissions  []string `json:"requiredPermissions,omitempty"`
	Deprecated           bool     `json:"deprecated,omitempty"`
	DeprecationNote      string   `json:"deprecationNote,omitempty"`
}

type ToolDescriptor struct {
	Name                 string             `json:"name"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	ModuleKey            string             `json:"moduleKey,omitempty"`
	SourceType           string             `json:"sourceType,omitempty"`
	CapabilityKeys       []string           `json:"capabilityKeys,omitempty"`
	CapabilityCategories []string           `json:"capabilityCategories,omitempty"`
	CompactEligible      bool               `json:"compactEligible,omitempty"`
	Scope                string             `json:"scope,omitempty"`
	PolicyState          string             `json:"policyState,omitempty"`
	PolicyReason         string             `json:"policyReason,omitempty"`
	EffectiveVisibility  string             `json:"effectiveVisibility,omitempty"`
	InputSchema          map[string]any     `json:"inputSchema,omitempty"`
	Contract             ContractDescriptor `json:"contract,omitempty"`
}

type ResourceDescriptor struct {
	URI         string             `json:"uri"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Scope       string             `json:"scope,omitempty"`
	MIMEType    string             `json:"mimeType,omitempty"`
	Contract    ContractDescriptor `json:"contract,omitempty"`
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
				"version": "1.1.0",
			},
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
				"orbyte": map[string]any{
					"contractVersion":         ContractVersion,
					"endpointScope":           actor.EndpointScope,
					"supportedEndpointScopes": []string{EndpointScopeAll, EndpointScopeAnalytics},
					"capabilityResourceURI":   mcpCatalogResourceURI,
					"errorSemanticsVersion":   "2026-03-23",
					"activeCapabilities":      s.activeCapabilitiesForInit(),
					"supportedMethods": []string{
						"initialize",
						"tools/list",
						"tools/search",
						"tools/describe",
						"tools/call",
						"skills/list",
						"skills/search",
						"skills/describe",
						"playbooks/list",
						"playbooks/search",
						"playbooks/describe",
						"resources/list",
						"resources/read",
					},
				},
			},
		}}
	case "notifications/initialized", "initialized", "ping":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: s.toolsListResult(actor, parseToolCatalogOptions(req.Params))}
	case "tools/search":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.toolsSearchMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	case "tools/describe":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.toolsDescribeMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
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
			Name           string             `json:"name"`
			Arguments      map[string]any     `json:"arguments"`
			CatalogContext ToolCatalogOptions `json:"catalog_context"`
		}
		_ = json.Unmarshal(req.Params, &params)
		result, err := s.callTool(ctx, actor, params.Name, params.Arguments, params.CatalogContext)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	case "playbooks/list":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.playbooksListMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	case "playbooks/search":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.playbooksSearchMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	case "playbooks/describe":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.playbooksDescribeMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	case "skills/list":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.playbooksListMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	case "skills/search":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.playbooksSearchMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	case "skills/describe":
		var params map[string]any
		_ = json.Unmarshal(req.Params, &params)
		result, _, err := s.playbooksDescribeMeta(actor, params)
		if err != nil {
			return errorResponse(req.ID, http.StatusBadRequest, err)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &JSONRPCError{
			Code:    -32601,
			Message: "method not found",
			Data: map[string]any{
				"category":        "method_not_found",
				"http_status":     http.StatusNotFound,
				"retryable":       false,
				"contractVersion": ContractVersion,
			},
		}}
	}
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
	return JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &JSONRPCError{
		Code:    code,
		Message: err.Error(),
		Data: map[string]any{
			"category":        jsonRPCErrorCategory(status),
			"http_status":     status,
			"retryable":       status >= http.StatusInternalServerError,
			"contractVersion": ContractVersion,
		},
	}}
}

func jsonRPCErrorCategory(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		if status >= http.StatusInternalServerError {
			return "internal"
		}
		return "unknown"
	}
}

func scopeMatches(endpointScope, itemScope string) bool {
	if strings.TrimSpace(endpointScope) == "" {
		return true
	}
	return strings.TrimSpace(endpointScope) == strings.TrimSpace(itemScope)
}
