package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

type createDocumentRequest struct {
	Type           string         `json:"type"`
	OrganizationID string         `json:"organization_id"`
	LocationID     string         `json:"location_id"`
	Payload        map[string]any `json:"payload"`
}

type updateDocumentRequest struct {
	ExpectedVersion int            `json:"expected_version,omitempty"`
	ExpectedETag    string         `json:"expected_etag,omitempty"`
	Payload         map[string]any `json:"payload"`
}

type actionRequest struct {
	Action          string `json:"action"`
	ExpectedVersion int    `json:"expected_version,omitempty"`
	ExpectedETag    string `json:"expected_etag,omitempty"`
}

type updateDocumentExtensionRequest struct {
	ExpectedVersion int            `json:"expected_version,omitempty"`
	ExpectedETag    string         `json:"expected_etag,omitempty"`
	Payload         map[string]any `json:"payload"`
}

type createDocumentLinkRequest struct {
	LinkedDocumentID string         `json:"linked_document_id"`
	LinkType         string         `json:"link_type"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type createDocumentAttachmentRequest struct {
	AttachmentType string `json:"attachment_type"`
	FileName       string `json:"file_name"`
	ContentType    string `json:"content_type"`
	StorageKey     string `json:"storage_key"`
	SizeBytes      int64  `json:"size_bytes"`
}

func registerDocumentRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, docs *document.Service, docActions *application.DocumentActions, policySvc *policy.Service, fieldSecurity *securityfields.Service, obs *observability.Service) {
	mux.HandleFunc("GET /documents", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireAuthorization(w, r, ident, "document.list", effectiveLocationID(r), "")
		if !ok {
			return
		}
		locationID := effectiveLocationID(r)
		if locationID == "" && p.kind == userPrincipal {
			locationID = p.currentLocationID
		}
		items := docs.List()
		if locationID != "" {
			filtered := make([]document.Record, 0, len(items))
			for _, item := range items {
				if item.Header.LocationID == locationID && searchVisible(item.Header, p, policySvc) {
					rendered := docs.Render(item, document.ViewNormal, modules.EnabledMap())
					filtered = append(filtered, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
				}
			}
			items = filtered
		} else {
			filtered := make([]document.Record, 0, len(items))
			for i := range items {
				if !searchVisible(items[i].Header, p, policySvc) {
					continue
				}
				rendered := docs.Render(items[i], document.ViewNormal, modules.EnabledMap())
				filtered = append(filtered, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			}
			items = filtered
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("POST /documents", func(w http.ResponseWriter, r *http.Request) {
		var req createDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid document create payload"))
			return
		}
		if req.OrganizationID == "" {
			respondError(w, shared.Validation("organization_id is required"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, "document.create", req.LocationID, "")
		if !ok {
			return
		}
		locationID := req.LocationID
		if locationID == "" && p.kind == userPrincipal {
			locationID = p.currentLocationID
		}
		candidate := document.Record{
			Header: document.Header{
				Type:           req.Type,
				Status:         "draft",
				OrganizationID: req.OrganizationID,
				LocationID:     locationID,
			},
			Body: document.Body{Payload: req.Payload},
		}
		if err := validateDocumentWrite(fieldSecurity, ident, p, candidate, req.Payload, "", "api"); err != nil {
			respondError(w, err)
			return
		}
		record, err := docs.Create(req.Type, req.OrganizationID, locationID, p.userID, req.Payload)
		if err != nil {
			incActionMetric(obs, "create", "error")
			respondError(w, err)
			return
		}
		incActionMetric(obs, "create", "success")
		respondJSON(w, http.StatusCreated, sanitizeDocumentRecord(fieldSecurity, ident, p, record, "api"))
	})

	mux.HandleFunc("GET /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, ok := documentLinkCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.read", record.Header.LocationID, "")
			if !ok {
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"items": sanitizeDocumentRecord(fieldSecurity, ident, p, record, "api").Links})
			return
		}
		if documentID, ok := documentAttachmentCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.read", record.Header.LocationID, "")
			if !ok {
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"items": sanitizeDocumentRecord(fieldSecurity, ident, p, record, "api").Attachments})
			return
		}
		documentID, ok := documentIDFromPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		record, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		p, ok := requireAuthorization(w, r, ident, "document.read", record.Header.LocationID, "")
		if !ok {
			return
		}
		viewMode := documentViewMode(r)
		if viewMode == document.ViewRaw {
			if _, ok := requireAuthorization(w, r, ident, "configuration.read", record.Header.LocationID, "configuration.read"); !ok {
				return
			}
		}
		rendered := docs.Render(record, viewMode, modules.EnabledMap())
		if viewMode == document.ViewExpanded || viewMode == document.ViewRaw {
			rendered = filterDocumentExtensionsForPrincipal(rendered, modules, ident, policySvc, p)
		}
		rendered = sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api")
		respondJSON(w, http.StatusOK, rendered)
	})

	mux.HandleFunc("PUT /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, moduleKey, ok := documentExtensionPath(r.URL.Path); ok {
			current, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			p, ok := requireAuthorization(w, r, ident, "document.update_draft", current.Header.LocationID, "")
			if !ok {
				return
			}
			if !modules.IsEnabled(moduleKey) {
				respondError(w, shared.Conflict("module is disabled"))
				return
			}
			if !extensionWriteAllowed(current, moduleKey, modules, ident, policySvc, p) {
				respondError(w, shared.Forbidden("document extension write is not allowed"))
				return
			}
			var req updateDocumentExtensionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid document extension payload"))
				return
			}
			if err := validateDocumentWrite(fieldSecurity, ident, p, current, req.Payload, "extensions."+moduleKey, "api"); err != nil {
				respondError(w, err)
				return
			}
			record, err := docActions.UpdateExtension(documentID, moduleKey, p.userID, req.Payload, req.ExpectedVersion, req.ExpectedETag)
			if err != nil {
				incActionMetric(obs, "update_extension", "error")
				respondError(w, err)
				return
			}
			incActionMetric(obs, "update_extension", "success")
			rendered := filterDocumentExtensionsForPrincipal(docs.Render(record, document.ViewExpanded, modules.EnabledMap()), modules, ident, policySvc, p)
			respondJSON(w, http.StatusOK, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
			return
		}
		documentID, ok := documentIDFromPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		current, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		p, ok := requireAuthorization(w, r, ident, "document.update_draft", current.Header.LocationID, "")
		if !ok {
			return
		}
		var req updateDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid document update payload"))
			return
		}
		if err := validateDocumentWrite(fieldSecurity, ident, p, current, req.Payload, "", "api"); err != nil {
			respondError(w, err)
			return
		}
		record, err := docActions.UpdateDraft(documentID, p.userID, req.Payload, req.ExpectedVersion, req.ExpectedETag)
		if err != nil {
			incActionMetric(obs, "update", "error")
			respondError(w, err)
			return
		}
		incActionMetric(obs, "update", "success")
		respondJSON(w, http.StatusOK, sanitizeDocumentRecord(fieldSecurity, ident, p, docs.Render(record, document.ViewExpanded, modules.EnabledMap()), "api"))
	})

	mux.HandleFunc("POST /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, ok := documentLinkCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			var req createDocumentLinkRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid document link payload"))
				return
			}
			link, err := docs.AddLink(documentID, strings.TrimSpace(req.LinkedDocumentID), strings.TrimSpace(req.LinkType), req.Metadata)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, link)
			return
		}
		if documentID, ok := documentAttachmentCollectionPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			var req createDocumentAttachmentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid document attachment payload"))
				return
			}
			attachment, err := docs.AddAttachment(documentID, strings.TrimSpace(req.AttachmentType), strings.TrimSpace(req.FileName), strings.TrimSpace(req.ContentType), strings.TrimSpace(req.StorageKey), req.SizeBytes)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusCreated, attachment)
			return
		}
		documentID, ok := documentActionPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		current, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		var req actionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
			respondError(w, shared.Validation("invalid action payload"))
			return
		}
		if req.Action == "" {
			respondError(w, shared.Validation("action is required"))
			return
		}

		permissionByAction := map[string]string{
			"submit":  "document.submit",
			"approve": "document.approve",
			"reject":  "document.reject",
			"reopen":  "document.reopen",
			"cancel":  "document.cancel",
		}
		permissionKey, exists := permissionByAction[req.Action]
		if !exists {
			respondError(w, shared.Validation("unsupported document action"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, permissionKey, current.Header.LocationID, "")
		if !ok {
			return
		}

		var record document.Record
		switch req.Action {
		case "submit":
			record, err = docActions.Submit(documentID, p.userID, req.ExpectedVersion, req.ExpectedETag)
		case "approve":
			record, err = docActions.Approve(documentID, p.userID, req.ExpectedVersion, req.ExpectedETag)
		case "reject":
			record, err = docActions.Reject(documentID, p.userID, req.ExpectedVersion, req.ExpectedETag)
		case "reopen":
			record, err = docActions.Reopen(documentID, p.userID, req.ExpectedVersion, req.ExpectedETag)
		case "cancel":
			record, err = docActions.Cancel(documentID, p.userID, req.ExpectedVersion, req.ExpectedETag)
		}
		if err != nil {
			incActionMetric(obs, req.Action, "error")
			respondError(w, err)
			return
		}
		incActionMetric(obs, req.Action, "success")
		rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
		rendered = filterDocumentExtensionsForPrincipal(rendered, modules, ident, policySvc, p)
		respondJSON(w, http.StatusOK, sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "api"))
	})

	mux.HandleFunc("DELETE /documents/", func(w http.ResponseWriter, r *http.Request) {
		if documentID, linkID, ok := documentLinkItemPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			if err := docs.RemoveLink(documentID, linkID); err != nil {
				respondError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if documentID, attachmentID, ok := documentAttachmentItemPath(r.URL.Path); ok {
			record, err := docs.Get(documentID)
			if err != nil {
				respondError(w, err)
				return
			}
			if _, ok := requireAuthorization(w, r, ident, "document.update_draft", record.Header.LocationID, ""); !ok {
				return
			}
			if err := docs.RemoveAttachment(documentID, attachmentID); err != nil {
				respondError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})
}

func documentLinkCollectionPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "documents" || parts[2] != "links" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func documentAttachmentCollectionPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "documents" || parts[2] != "attachments" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), parts[1] != ""
}

func documentLinkItemPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "documents" || parts[2] != "links" {
		return "", "", false
	}
	return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[3]), parts[1] != "" && parts[3] != ""
}

func documentAttachmentItemPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "documents" || parts[2] != "attachments" {
		return "", "", false
	}
	return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[3]), parts[1] != "" && parts[3] != ""
}

func filterDocumentExtensionsForPrincipal(record document.Record, modules *module.Service, ident *identity.Service, policySvc *policy.Service, p principal) document.Record {
	rawExtensions, ok := record.Body.Payload["extensions"].(map[string]any)
	if !ok {
		return record
	}
	filtered := map[string]any{}
	defs := modules.List()
	defByModule := map[string]module.DocumentExtension{}
	for _, detail := range defs {
		for _, ext := range detail.Manifest.DocumentExtensions {
			defByModule[detail.Manifest.Key+":"+ext.DocumentType] = ext
		}
	}
	for moduleKey, payload := range rawExtensions {
		if !modules.IsEnabled(moduleKey) {
			continue
		}
		extDef, ok := defByModule[moduleKey+":"+record.Header.Type]
		if !ok {
			filtered[moduleKey] = payload
			continue
		}
		if extDef.ReadPermissionKey != "" && !principalAllowsAll(ident, p, []string{extDef.ReadPermissionKey}) {
			continue
		}
		if policySvc != nil {
			decision := policySvc.Evaluate(policy.Request{
				HookKey:        "documents.extension.view",
				ActorID:        principalActorID(p),
				OrganizationID: record.Header.OrganizationID,
				LocationID:     record.Header.LocationID,
				ScopeID:        record.Header.LocationID,
				Inputs: map[string]any{
					"module_key":      moduleKey,
					"document_type":   record.Header.Type,
					"document_id":     record.Header.ID,
					"document_status": record.Header.Status,
					"organization_id": record.Header.OrganizationID,
					"location_id":     record.Header.LocationID,
				},
			})
			if !decision.Allowed {
				continue
			}
		}
		filtered[moduleKey] = payload
	}
	if len(filtered) == 0 {
		delete(record.Body.Payload, "extensions")
		return record
	}
	record.Body.Payload["extensions"] = filtered
	return record
}

func extensionWriteAllowed(record document.Record, moduleKey string, modules *module.Service, ident *identity.Service, policySvc *policy.Service, p principal) bool {
	for _, detail := range modules.List() {
		if detail.Manifest.Key != moduleKey {
			continue
		}
		for _, ext := range detail.Manifest.DocumentExtensions {
			if ext.DocumentType != record.Header.Type {
				continue
			}
			if ext.WritePermissionKey != "" && !principalAllowsAll(ident, p, []string{ext.WritePermissionKey}) {
				return false
			}
			if policySvc == nil {
				return true
			}
			decision := policySvc.Evaluate(policy.Request{
				HookKey:        "documents.extension.write",
				ActorID:        principalActorID(p),
				OrganizationID: record.Header.OrganizationID,
				LocationID:     record.Header.LocationID,
				ScopeID:        record.Header.LocationID,
				Inputs: map[string]any{
					"module_key":      moduleKey,
					"document_type":   record.Header.Type,
					"document_id":     record.Header.ID,
					"document_status": record.Header.Status,
				},
			})
			return decision.Allowed
		}
	}
	return true
}

func searchVisible(header document.Header, p principal, policySvc *policy.Service) bool {
	if policySvc == nil {
		return true
	}
	decision := policySvc.Evaluate(policy.Request{
		HookKey:        "documents.search.visibility",
		ActorID:        principalActorID(p),
		OrganizationID: header.OrganizationID,
		LocationID:     header.LocationID,
		ScopeID:        header.LocationID,
		Inputs: map[string]any{
			"document_id":   header.ID,
			"document_type": header.Type,
			"status":        header.Status,
		},
	})
	return decision.Allowed
}

func incActionMetric(obs *observability.Service, action, outcome string) {
	if obs == nil {
		return
	}
	_ = obs.RecordMetric("document.actions.total", map[string]string{"action": action, "outcome": outcome}, 1)
	obs.Inc("document.actions.total")
	obs.Inc("document.actions." + action + "." + outcome + ".total")
}

func effectiveLocationID(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("location_id"))
}

func documentIDFromPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] != "documents" {
		return "", false
	}
	return parts[1], true
}

func documentActionPath(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "documents" || parts[2] != "actions" {
		return "", false
	}
	return parts[1], true
}

func documentExtensionPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "documents" || parts[2] != "extensions" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func documentViewMode(r *http.Request) document.ViewMode {
	switch strings.TrimSpace(r.URL.Query().Get("view")) {
	case string(document.ViewExpanded):
		return document.ViewExpanded
	case string(document.ViewRaw):
		return document.ViewRaw
	default:
		return document.ViewNormal
	}
}
