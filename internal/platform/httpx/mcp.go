package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/mcp"
	"orbyte/internal/platform/shared"
)

const mcpDelegationGrantHeader = "X-Delegation-Grant-ID"

func registerMCPRoutes(mux *http.ServeMux, ident *identity.Service, auditSvc *audit.Service, server *mcp.Server, analyticsSvc *analytics.Service, stream *mcp.AnalyticsStream, streamPath, scopedStreamPath string) {
	registerMCPJSONRPCRoute(mux, "POST /mcp", ident, auditSvc, server, mcp.EndpointScopeAll)
	registerMCPJSONRPCRoute(mux, "POST /mcp/analytics", ident, auditSvc, server, mcp.EndpointScopeAnalytics)
	if strings.TrimSpace(streamPath) != "" {
		registerMCPStreamRoute(mux, "GET "+streamPath, ident, analyticsSvc, stream, mcp.EndpointScopeAll)
	}
	if strings.TrimSpace(scopedStreamPath) != "" {
		registerMCPStreamRoute(mux, "GET "+scopedStreamPath, ident, analyticsSvc, stream, mcp.EndpointScopeAnalytics)
	}
}

func registerMCPJSONRPCRoute(mux *http.ServeMux, pattern string, ident *identity.Service, auditSvc *audit.Service, server *mcp.Server, endpointScope string) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if server == nil {
			respondError(w, shared.NotFound("mcp server is not configured"))
			return
		}
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		var req mcp.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid mcp request payload"))
			return
		}
		actor, actorErr := mcpActorContext(r, ident, p, endpointScope)
		if actorErr != nil {
			respondError(w, actorErr)
			return
		}
		resp := server.Handle(r.Context(), req, actor)
		recordMCPTrail(auditSvc, server, req, actor, resp)
		respondJSON(w, http.StatusOK, resp)
	})
}

func registerMCPStreamRoute(mux *http.ServeMux, pattern string, ident *identity.Service, analyticsSvc *analytics.Service, stream *mcp.AnalyticsStream, endpointScope string) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if stream == nil || analyticsSvc == nil {
			respondError(w, shared.NotFound("mcp analytics stream is not configured"))
			return
		}
		if err := authError(r); err != nil {
			respondError(w, err)
			return
		}
		p, ok := currentPrincipal(r)
		if !ok {
			respondError(w, shared.Unauthorized("authentication required"))
			return
		}
		if strings.TrimSpace(endpointScope) != "" && endpointScope != mcp.EndpointScopeAnalytics {
			respondError(w, shared.Forbidden("stream is not available on this endpoint"))
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("analytics.read is required"))
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			respondError(w, shared.Conflict("streaming is not supported"))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		if latest, ok := stream.Latest(); ok {
			if err := writeAnalyticsEvent(w, "snapshot", latest); err != nil {
				return
			}
			flusher.Flush()
		} else if snapshot, err := analyticsSvc.CaptureSnapshot(); err == nil {
			if err := writeAnalyticsEvent(w, "snapshot", snapshot); err != nil {
				return
			}
			flusher.Flush()
		}

		events, unsubscribe := stream.Subscribe()
		defer unsubscribe()
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case snapshot, ok := <-events:
				if !ok {
					return
				}
				if err := writeAnalyticsEvent(w, "snapshot", snapshot); err != nil {
					return
				}
				flusher.Flush()
			case tick := <-heartbeat.C:
				if _, err := fmt.Fprintf(w, "event: heartbeat\ndata: {\"time\":%q}\n\n", tick.UTC().Format(time.RFC3339)); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

func mcpActorContext(r *http.Request, ident *identity.Service, p principal, endpointScope string) (mcp.ActorContext, error) {
	actor := mcp.ActorContext{
		ActorID:            principalActorID(p),
		ActorKind:          principalActorKind(p),
		SessionID:          p.sessionID,
		ServicePrincipalID: p.serviceID,
		EffectiveUserID:    principalEffectiveUserID(p),
		OnBehalfOfUserID:   principalOnBehalfOfUserID(p),
		DelegationGrantID:  principalDelegationGrantID(p),
		DeepLinkGrantID:    principalDeepLinkGrantID(p),
		LocationID:         p.currentLocationID,
		EndpointScope:      strings.TrimSpace(endpointScope),
	}
	switch p.kind {
	case userPrincipal:
		actor.PermissionChecker = func(permissionKey string) bool {
			return principalAllowsAll(ident, p, []string{permissionKey})
		}
		return actor, nil
	case servicePrincipal:
		grantID := strings.TrimSpace(r.Header.Get(mcpDelegationGrantHeader))
		if grantID == "" {
			actor.PermissionChecker = func(permissionKey string) bool {
				return ident.DecideServicePrincipal(p.serviceID, permissionKey).Allowed
			}
			return actor, nil
		}
		grant, err := ident.ResolveAgentDelegationGrantForActivation(grantID, p.serviceID, "", time.Now().UTC())
		if err != nil {
			return mcp.ActorContext{}, err
		}
		actor.EffectiveUserID = grant.GrantorUserID
		actor.OnBehalfOfUserID = grant.GrantorUserID
		actor.DelegationGrantID = grant.ID
		actor.LocationID = grant.LocationID
		actor.PermissionChecker = func(permissionKey string) bool {
			return ident.DecideActingServicePrincipal(p.serviceID, grant.GrantorUserID, permissionKey, grant.LocationID, &grant).Allowed
		}
		return actor, nil
	default:
		return mcp.ActorContext{}, shared.Unauthorized("authentication required")
	}
}

func recordMCPTrail(auditSvc *audit.Service, server *mcp.Server, req mcp.JSONRPCRequest, actor mcp.ActorContext, resp mcp.JSONRPCResponse) {
	if auditSvc == nil {
		return
	}
	action := "mcp." + strings.TrimSpace(req.Method)
	targetType := "mcp_method"
	targetID := strings.TrimSpace(req.Method)
	metadata := map[string]any{}
	switch req.Method {
	case "tools/call":
		var params struct {
			Name           string                 `json:"name"`
			Arguments      map[string]any         `json:"arguments"`
			CatalogContext mcp.ToolCatalogOptions `json:"catalog_context"`
		}
		_ = json.Unmarshal(req.Params, &params)
		targetType = "mcp_tool"
		targetID = strings.TrimSpace(params.Name)
		metadata["tool_name"] = targetID
		if strings.TrimSpace(params.CatalogContext.CatalogMode) != "" {
			metadata["catalog_mode"] = params.CatalogContext.CatalogMode
			metadata["catalog_capabilities"] = append([]string(nil), params.CatalogContext.Capabilities...)
		}
		if server != nil {
			if descriptor, ok := server.ToolDescriptorForArguments(targetID, actor, params.Arguments); ok {
				metadata["contract_version"] = descriptor.Contract.Version
				metadata["side_effect_class"] = descriptor.Contract.SideEffectClass
				metadata["idempotency"] = descriptor.Contract.Idempotency
				metadata["required_scopes"] = descriptor.Contract.RequiredScopes
				metadata["required_permissions"] = descriptor.Contract.RequiredPermissions
				metadata["audit_action"] = descriptor.Contract.AuditAction
				metadata["stability"] = descriptor.Contract.Stability
				metadata["action_class"] = descriptor.Contract.ActionClass
				metadata["risk_class"] = descriptor.Contract.RiskClass
				metadata["policy_state"] = descriptor.PolicyState
				metadata["policy_reason"] = descriptor.PolicyReason
			}
		}
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(req.Params, &params)
		targetType = "mcp_resource"
		targetID = strings.TrimSpace(params.URI)
		metadata["resource_uri"] = targetID
		if server != nil {
			if descriptor, ok := server.ResourceDescriptor(targetID, actor); ok {
				metadata["contract_version"] = descriptor.Contract.Version
				metadata["side_effect_class"] = descriptor.Contract.SideEffectClass
				metadata["idempotency"] = descriptor.Contract.Idempotency
				metadata["required_scopes"] = descriptor.Contract.RequiredScopes
				metadata["required_permissions"] = descriptor.Contract.RequiredPermissions
				metadata["audit_action"] = descriptor.Contract.AuditAction
				metadata["stability"] = descriptor.Contract.Stability
			}
		}
	}
	metadata["jsonrpc_method"] = req.Method
	metadata["endpoint_scope"] = actor.EndpointScope
	metadata["effective_user_id"] = actor.EffectiveUserID
	metadata["deep_link_grant_id"] = actor.DeepLinkGrantID
	metadata["contract_version"] = defaultString(metadata["contract_version"], mcp.ContractVersion)
	metadata["status"] = "ok"
	if resp.Error != nil {
		metadata["status"] = "error"
		metadata["error"] = resp.Error.Message
	}
	recordAudit(auditSvc, audit.Event{
		ID:                "audit:mcp:" + req.Method + ":" + targetID + ":" + time.Now().UTC().Format("20060102150405.000000000"),
		Action:            action,
		TargetType:        targetType,
		TargetID:          targetID,
		ActorID:           actor.ActorID,
		ActorKind:         actor.ActorKind,
		OnBehalfOfUserID:  actor.OnBehalfOfUserID,
		DelegationGrantID: actor.DelegationGrantID,
		LocationID:        actor.LocationID,
		OccurredAt:        time.Now().UTC(),
		Metadata:          metadata,
	})
}

func defaultString(value any, fallback string) string {
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		return text
	}
	return fallback
}

func writeAnalyticsEvent(w http.ResponseWriter, event string, snapshot analytics.Snapshot) error {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
	return err
}
