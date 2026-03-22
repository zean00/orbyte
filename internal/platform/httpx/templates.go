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
	Body       string `json:"body"`
	Style      string `json:"style"`
	ChangeNote string `json:"change_note"`
}

type templateDuplicateRequest struct {
	FromVersion int `json:"from_version"`
}

type templateFixtureRequest struct {
	FixtureKey  string         `json:"fixture_key"`
	Name        string         `json:"name"`
	TargetKind  string         `json:"target_kind"`
	TemplateKey string         `json:"template_key"`
	SourceType  string         `json:"source_type"`
	Payload     map[string]any `json:"payload"`
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
	mux.HandleFunc("GET /admin/api/templates/compare", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.read", "", "template.read"); !ok {
			return
		}
		key := strings.TrimSpace(r.URL.Query().Get("template_key"))
		leftVersion, leftErr := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("left")))
		rightVersion, rightErr := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("right")))
		if key == "" || leftErr != nil || rightErr != nil {
			respondError(w, shared.Validation("template_key, left, and right are required"))
			return
		}
		item, err := templates.CompareVersions(key, leftVersion, rightVersion)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"comparison": item})
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
	mux.HandleFunc("GET /admin/api/template-fixtures", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.read", "", "template.read"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"items": templates.Fixtures(strings.TrimSpace(r.URL.Query().Get("template_key")), strings.TrimSpace(r.URL.Query().Get("target_kind"))),
		})
	})
	mux.HandleFunc("PUT /admin/api/template-fixtures", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "template.manage", "", "template.manage")
		if !ok {
			return
		}
		var req templateFixtureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid template fixture payload"))
			return
		}
		saved, err := templates.SaveFixture(templateoutput.TemplateFixture{
			FixtureKey:  req.FixtureKey,
			Name:        req.Name,
			TargetKind:  req.TargetKind,
			TemplateKey: req.TemplateKey,
			SourceType:  req.SourceType,
			Payload:     req.Payload,
			UpdatedBy:   p.userID,
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"fixture": saved})
	})
	mux.HandleFunc("GET /admin/api/templates/binding-debug", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.read", "", "template.read"); !ok {
			return
		}
		debug, err := templates.ResolveBindingDebug(templateoutput.RenderRequest{
			TemplateKey:    strings.TrimSpace(r.URL.Query().Get("template_key")),
			TargetKind:     strings.TrimSpace(r.URL.Query().Get("target_kind")),
			TargetKey:      strings.TrimSpace(r.URL.Query().Get("target_key")),
			TargetID:       strings.TrimSpace(r.URL.Query().Get("target_id")),
			OrganizationID: strings.TrimSpace(r.URL.Query().Get("organization_id")),
			LocationID:     strings.TrimSpace(r.URL.Query().Get("location_id")),
			ScopeType:      strings.TrimSpace(r.URL.Query().Get("scope_type")),
			ScopeID:        strings.TrimSpace(r.URL.Query().Get("scope_id")),
			Purpose:        strings.TrimSpace(r.URL.Query().Get("purpose")),
			Channel:        strings.TrimSpace(r.URL.Query().Get("channel")),
			Draft:          strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("mode")), "draft"),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"binding_resolution": debug})
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
		saved, err := templates.SaveDraftWithOptions(key, req.Body, req.Style, p.userID, req.ChangeNote, 0)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"version": saved})
	})
	mux.HandleFunc("POST /admin/api/templates/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "template.manage", "", "template.manage")
		if !ok {
			return
		}
		key, version, action, ok := templateVersionPath(r.URL.Path)
		if !ok {
			return
		}
		if action == "duplicate-draft" {
			var req templateDuplicateRequest
			if r.ContentLength > 0 {
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					respondError(w, shared.Validation("invalid template duplicate payload"))
					return
				}
			}
			saved, err := templates.DuplicateDraft(key, req.FromVersion, p.userID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"version": saved})
			return
		}
		if action == "reset-draft" {
			saved, err := templates.ResetDraftToPublished(key, p.userID)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"version": saved})
			return
		}
		if action != "publish" {
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "template.publish", "", "template.publish"); !ok {
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
	mux.HandleFunc("POST /admin/api/templates/validate", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.manage", "", "template.manage"); !ok {
			return
		}
		var req templateoutput.RenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid template validate payload"))
			return
		}
		issues := templates.Validate(req)
		respondJSON(w, http.StatusOK, map[string]any{"valid": len(filterValidationIssues(issues, "error")) == 0, "issues": issues})
	})
	mux.HandleFunc("POST /admin/api/templates/preview", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "template.render", "", "template.render"); !ok {
			return
		}
		var req templateoutput.RenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid template preview payload"))
			return
		}
		preview, err := templates.Preview(req)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"preview": preview})
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

func filterValidationIssues(issues []templateoutput.ValidationIssue, severity string) []templateoutput.ValidationIssue {
	items := make([]templateoutput.ValidationIssue, 0, len(issues))
	for _, item := range issues {
		if item.Severity == severity {
			items = append(items, item)
		}
	}
	return items
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
