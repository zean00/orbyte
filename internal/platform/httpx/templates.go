package httpx

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/templateoutput"
)

type templateDraftRequest struct {
	Body  string `json:"body"`
	Style string `json:"style"`
}

func registerTemplateRoutes(mux *http.ServeMux, ident *identity.Service, templates *templateoutput.Service) {
	mux.HandleFunc("GET /outputs/templates/resolve", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.render", "", "template.render"); !ok {
			return
		}
		req := templateoutput.RenderRequest{
			TargetKind:     strings.TrimSpace(r.URL.Query().Get("target_kind")),
			TargetKey:      strings.TrimSpace(r.URL.Query().Get("target_key")),
			OrganizationID: strings.TrimSpace(r.URL.Query().Get("organization_id")),
			LocationID:     strings.TrimSpace(r.URL.Query().Get("location_id")),
			Purpose:        strings.TrimSpace(r.URL.Query().Get("purpose")),
			Channel:        strings.TrimSpace(r.URL.Query().Get("channel")),
			ScopeType:      strings.TrimSpace(r.URL.Query().Get("scope_type")),
			ScopeID:        strings.TrimSpace(r.URL.Query().Get("scope_id")),
		}
		def, version, err := templates.Resolve(req)
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]any{"resolved": false})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"resolved": true, "definition": def, "version": version})
	})
	mux.HandleFunc("GET /admin/api/templates/definitions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.read", "", "template.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": templates.Definitions()})
	})
	mux.HandleFunc("GET /admin/api/templates/versions", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.read", "", "template.read"); !ok {
			return
		}
		key := strings.TrimSpace(r.URL.Query().Get("template_key"))
		if key == "" {
			respondError(w, shared.Validation("template_key is required"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": templates.Versions(key)})
	})
	mux.HandleFunc("GET /admin/api/template-bindings", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.read", "", "template.read"); !ok {
			return
		}
		items := templates.Bindings()
		templateKey := strings.TrimSpace(r.URL.Query().Get("template_key"))
		if templateKey != "" {
			filtered := make([]templateoutput.Binding, 0, len(items))
			for _, item := range items {
				if item.TemplateKey == templateKey {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	mux.HandleFunc("PUT /admin/api/template-bindings", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "template.bind", "", "template.bind")
		if !ok {
			return
		}
		var binding templateoutput.Binding
		if err := json.NewDecoder(r.Body).Decode(&binding); err != nil {
			respondError(w, shared.Validation("invalid template binding payload"))
			return
		}
		binding.UpdatedBy = p.userID
		saved, err := templates.SaveBinding(binding)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"binding": saved})
	})
	mux.HandleFunc("PUT /admin/api/templates/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "template.manage", "", "template.manage")
		if !ok {
			return
		}
		key, version, action, ok := templateVersionPath(r.URL.Path)
		if !ok || action != "draft" {
			return
		}
		if version != "" {
			_ = version
		}
		var req templateDraftRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid template draft payload"))
			return
		}
		saved, err := templates.SaveDraft(key, req.Body, req.Style, p.userID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"version": saved})
	})
	mux.HandleFunc("POST /admin/api/templates/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "template.publish", "", "template.publish")
		if !ok {
			return
		}
		key, version, action, ok := templateVersionPath(r.URL.Path)
		if !ok || action != "publish" {
			return
		}
		versionNo, err := strconv.Atoi(version)
		if err != nil || versionNo <= 0 {
			respondError(w, shared.Validation("template version is invalid"))
			return
		}
		saved, err := templates.Publish(key, versionNo, p.userID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"version": saved})
	})
	mux.HandleFunc("POST /outputs/render", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.render", "", "template.render"); !ok {
			return
		}
		var req templateoutput.RenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid template render payload"))
			return
		}
		rendered, err := templates.Render(req)
		if err != nil {
			respondError(w, err)
			return
		}
		if strings.EqualFold(strings.TrimSpace(req.Format), "html") || strings.TrimSpace(req.Format) == "" {
			respondJSON(w, http.StatusOK, map[string]any{"output": rendered})
			return
		}
		w.Header().Set("Content-Type", rendered.ContentType)
		if len(rendered.Bytes) > 0 {
			w.Header().Set("Content-Disposition", `inline; filename="`+rendered.FileName+`"`)
			_, _ = w.Write(rendered.Bytes)
			return
		}
		_, _ = w.Write([]byte(rendered.HTML))
	})
}

func templateVersionPath(path string) (key string, version string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 6 || parts[0] != "admin" || parts[1] != "api" || parts[2] != "templates" {
		return "", "", "", false
	}
	key = parts[3]
	if len(parts) == 6 && parts[4] == "actions" {
		return key, "", parts[5], true
	}
	if len(parts) == 7 && parts[4] == "versions" {
		return key, parts[5], parts[6], true
	}
	return "", "", "", false
}
