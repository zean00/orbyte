package httpx

import (
	"net/http"
	"strings"

	"orbyte/internal/platform/acp"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/shared"
)

func registerUISurfaceRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, docs *document.Service, policySvc *policy.Service, uiPrefs *UIPreferencesService, acpSvc *acp.Service) {
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
