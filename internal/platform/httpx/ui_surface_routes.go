package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
)

func registerUISurfaceRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, docs *document.Service, leavePolicySvc *application.LeavePolicyCoreService, policySvc *policy.Service, uiPrefs *UIPreferencesService, acpSvc *acp.Service) {
	mux.HandleFunc("GET /ui/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		surface := requestedUISurface(r)
		respondJSON(w, http.StatusOK, buildWorkspaceBootstrapPayload(r, ident, modules, p, surface, uiPrefs, acpSvc))
	})

	mux.HandleFunc("GET /ui/menus", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		menus, _, _, _, _ := visibleUIContracts(ident, modules, p, requestedUISurface(r))
		respondJSON(w, http.StatusOK, map[string]any{"items": menus})
	})

	mux.HandleFunc("GET /ui/actions", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		_, actions, _, _, _ := visibleUIContracts(ident, modules, p, requestedUISurface(r))
		respondJSON(w, http.StatusOK, map[string]any{"items": actions})
	})

	mux.HandleFunc("GET /ui/self-service/apis", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": visibleSelfServiceAPIs(ident, modules, p)})
	})

	registerUILeaveSelfServiceRoutes(mux, ident, leavePolicySvc)

	mux.HandleFunc("GET /ui/actions/render", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		action := strings.TrimSpace(r.URL.Query().Get("action"))
		documentID := strings.TrimSpace(r.URL.Query().Get("document_id"))
		if action == "" || documentID == "" {
			respondError(w, shared.Validation("action and document_id are required"))
			return
		}
		record, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		decision := policy.Decision{Allowed: true, Output: map[string]any{"placement": "secondary"}}
		if policySvc != nil {
			decision = policySvc.Evaluate(policy.Request{
				HookKey:        "documents.action.render",
				ActorID:        principalActorID(p),
				OrganizationID: record.Header.OrganizationID,
				LocationID:     record.Header.LocationID,
				Inputs: map[string]any{
					"document_id":   record.Header.ID,
					"document_type": record.Header.Type,
					"status":        record.Header.Status,
					"action":        action,
				},
			})
		}
		respondJSON(w, http.StatusOK, decision)
	})

	mux.HandleFunc("GET /ui/views/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		viewKey := viewKeyFromPath(r.URL.Path)
		if viewKey == "" {
			respondError(w, shared.NotFound("view not found"))
			return
		}
		_, _, views, _, _ := visibleUIContracts(ident, modules, p, requestedUISurface(r))
		for _, view := range views {
			if view.Key == viewKey {
				respondJSON(w, http.StatusOK, view)
				return
			}
		}
		respondError(w, shared.NotFound("view not found"))
	})

	mux.HandleFunc("GET /ui/routes/resolve", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		if path == "" {
			respondError(w, shared.Validation("path is required"))
			return
		}
		surface := requestedUISurface(r)
		response := resolveUIRoute(ident, modules, p, surface, path)
		respondJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /ui/document-flows/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		flowKey := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ui/document-flows/"))
		if flowKey == "" {
			respondError(w, shared.NotFound("document flow not found"))
			return
		}
		flow, ok := modules.DocumentFlowForSurface(flowKey, requestedUISurface(r))
		if !ok || !principalAllowsAll(ident, p, flow.RequiredPermissions) {
			respondError(w, shared.NotFound("document flow not found"))
			return
		}
		respondJSON(w, http.StatusOK, flow)
	})

	mux.HandleFunc("GET /ui/assets/modules/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireInteractivePrincipal(w, r); !ok {
			return
		}
		bundleKey := bundleKeyFromPath(r.URL.Path)
		if bundleKey == "" {
			respondError(w, shared.NotFound("module bundle not found"))
			return
		}
		for _, detail := range modules.List() {
			if !detail.Installed.Enabled {
				continue
			}
			for _, bundle := range detail.Manifest.Bundles {
				if bundle.Key != bundleKey {
					continue
				}
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
				_, _ = w.Write([]byte(bundle.Script))
				return
			}
		}
		respondError(w, shared.NotFound("module bundle not found"))
	})
}

func registerUILeaveSelfServiceRoutes(mux *http.ServeMux, ident *identity.Service, leavePolicySvc *application.LeavePolicyCoreService) {
	mux.HandleFunc("GET /ui/self-service/leave/balances", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"leave.self_service.read"}) {
			respondError(w, shared.Forbidden("leave self-service read is not allowed"))
			return
		}
		if leavePolicySvc == nil {
			respondError(w, shared.NotFound("leave self-service is not available"))
			return
		}
		items, err := leavePolicySvc.BalanceSummaryForUser(principalEffectiveUserID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("GET /ui/self-service/leave/requests", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"leave.self_service.read"}) {
			respondError(w, shared.Forbidden("leave self-service read is not allowed"))
			return
		}
		if leavePolicySvc == nil {
			respondError(w, shared.NotFound("leave self-service is not available"))
			return
		}
		items, err := leavePolicySvc.RequestSummariesForUser(principalEffectiveUserID(p), map[string]string{
			"approval_status": strings.TrimSpace(r.URL.Query().Get("approval_status")),
			"status":          strings.TrimSpace(r.URL.Query().Get("status")),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /ui/self-service/leave/requests", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"leave.self_service.write"}) {
			respondError(w, shared.Forbidden("leave self-service write is not allowed"))
			return
		}
		if leavePolicySvc == nil {
			respondError(w, shared.NotFound("leave self-service is not available"))
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("request body is invalid"))
			return
		}
		record, err := leavePolicySvc.CreateSelfServiceLeaveRequest(principalEffectiveUserID(p), req, principalEffectiveUserID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		payload, err := leavePolicySvc.RequestSummaryForUser(principalEffectiveUserID(p), record.ID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": payload})
	})

	mux.HandleFunc("GET /ui/self-service/leave/requests/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"leave.self_service.read"}) {
			respondError(w, shared.Forbidden("leave self-service read is not allowed"))
			return
		}
		if leavePolicySvc == nil {
			respondError(w, shared.NotFound("leave self-service is not available"))
			return
		}
		requestID, action := selfServiceLeaveRequestPath(r.URL.Path)
		if requestID == "" || action != "" {
			respondError(w, shared.NotFound("leave request not found"))
			return
		}
		payload, err := leavePolicySvc.RequestSummaryForUser(principalEffectiveUserID(p), requestID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": payload})
	})

	mux.HandleFunc("PUT /ui/self-service/leave/requests/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"leave.self_service.write"}) {
			respondError(w, shared.Forbidden("leave self-service write is not allowed"))
			return
		}
		if leavePolicySvc == nil {
			respondError(w, shared.NotFound("leave self-service is not available"))
			return
		}
		requestID, action := selfServiceLeaveRequestPath(r.URL.Path)
		if requestID == "" || action != "" {
			respondError(w, shared.NotFound("leave request not found"))
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("request body is invalid"))
			return
		}
		record, err := leavePolicySvc.UpdateSelfServiceLeaveRequest(principalEffectiveUserID(p), requestID, req, principalEffectiveUserID(p))
		if err != nil {
			respondError(w, err)
			return
		}
		payload, err := leavePolicySvc.RequestSummaryForUser(principalEffectiveUserID(p), record.ID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": payload})
	})

	mux.HandleFunc("POST /ui/self-service/leave/requests/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"leave.self_service.write"}) {
			respondError(w, shared.Forbidden("leave self-service write is not allowed"))
			return
		}
		if leavePolicySvc == nil {
			respondError(w, shared.NotFound("leave self-service is not available"))
			return
		}
		requestID, action := selfServiceLeaveRequestPath(r.URL.Path)
		if requestID == "" {
			respondError(w, shared.NotFound("leave request not found"))
			return
		}
		var (
			record model.Record
			err    error
		)
		switch action {
		case "submit":
			record, err = leavePolicySvc.SubmitSelfServiceLeaveRequest(principalEffectiveUserID(p), requestID, principalEffectiveUserID(p))
		case "cancel":
			record, err = leavePolicySvc.CancelSelfServiceLeaveRequest(principalEffectiveUserID(p), requestID, principalEffectiveUserID(p))
		default:
			respondError(w, shared.NotFound("leave request action not found"))
			return
		}
		if err != nil {
			respondError(w, err)
			return
		}
		payload, err := leavePolicySvc.RequestSummaryForUser(principalEffectiveUserID(p), record.ID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": payload})
	})
}

func selfServiceLeaveRequestPath(path string) (string, string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(path, "/ui/self-service/leave/requests/"))
	if trimmed == "" {
		return "", ""
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 {
		return "", ""
	}
	requestID := strings.TrimSpace(parts[0])
	action := ""
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	return requestID, action
}
