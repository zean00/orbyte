package httpx

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

func registerUIDocumentRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, docs *document.Service, searchSvc *search.Service, policySvc *policy.Service, fieldSecurity *securityfields.Service) {
	mux.HandleFunc("GET /ui/data/documents", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("document list is not allowed"))
			return
		}
		documentType := strings.TrimSpace(r.URL.Query().Get("type"))
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		sortKey := strings.TrimSpace(r.URL.Query().Get("sort"))
		items := searchSvc.ListDocuments()
		filtered := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if documentType != "" && item.DocumentType != documentType {
				continue
			}
			if statusFilter != "" && item.Status != statusFilter {
				continue
			}
			if p.currentLocationID != "" && item.LocationID != "" && item.LocationID != p.currentLocationID {
				continue
			}
			filtered = append(filtered, map[string]any{
				"header": map[string]any{
					"id":              item.DocumentID,
					"type":            item.DocumentType,
					"status":          item.Status,
					"version":         item.Version,
					"etag":            item.ETag,
					"organization_id": item.OrganizationID,
					"location_id":     item.LocationID,
					"updated_at":      item.UpdatedAt,
				},
			})
		}
		sort.Slice(filtered, func(i, j int) bool {
			left := filtered[i]["header"].(map[string]any)
			right := filtered[j]["header"].(map[string]any)
			switch sortKey {
			case "status":
				return left["status"].(string) < right["status"].(string)
			case "updated_at":
				return left["updated_at"].(time.Time).After(right["updated_at"].(time.Time))
			default:
				return left["id"].(string) < right["id"].(string)
			}
		})
		respondJSON(w, http.StatusOK, map[string]any{"items": filtered})
	})

	mux.HandleFunc("GET /ui/data/documents/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.read"}) {
			respondError(w, shared.Forbidden("document read is not allowed"))
			return
		}
		documentID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ui/data/documents/"))
		if documentID == "" {
			respondError(w, shared.NotFound("document not found"))
			return
		}
		record, err := docs.Get(documentID)
		if err != nil {
			respondError(w, err)
			return
		}
		if p.currentLocationID != "" && record.Header.LocationID != "" && record.Header.LocationID != p.currentLocationID {
			respondError(w, shared.Forbidden("document is not visible"))
			return
		}
		rendered := docs.Render(record, document.ViewExpanded, modules.EnabledMap())
		rendered = filterDocumentExtensionsForPrincipal(rendered, modules, ident, policySvc, p)
		rendered = sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "ui")
		var flowInstance any
		if instance, err := resolveDocumentFlowInstance(modules, docs, "", documentID); err == nil {
			for index := range instance.Items {
				item := instance.Items[index]
				renderedItem := docs.Render(item.Record, document.ViewExpanded, modules.EnabledMap())
				renderedItem = filterDocumentExtensionsForPrincipal(renderedItem, modules, ident, policySvc, p)
				instance.Items[index].Record = sanitizeDocumentRecord(fieldSecurity, ident, p, renderedItem, "ui")
			}
			flowInstance = instance
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"record":        rendered,
			"lines":         record.Lines,
			"links":         record.Links,
			"attachments":   record.Attachments,
			"documentType":  record.Header.Type,
			"flow_instance": flowInstance,
		})
	})

	mux.HandleFunc("GET /ui/data/projections/documents", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"document.list"}) {
			respondError(w, shared.Forbidden("document list is not allowed"))
			return
		}
		items := searchSvc.ListDocuments()
		filtered := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if p.currentLocationID != "" && item.LocationID != "" && item.LocationID != p.currentLocationID {
				continue
			}
			filtered = append(filtered, map[string]any{
				"header": map[string]any{
					"id":              item.DocumentID,
					"type":            item.DocumentType,
					"status":          item.Status,
					"version":         item.Version,
					"etag":            item.ETag,
					"organization_id": item.OrganizationID,
					"location_id":     item.LocationID,
					"updated_at":      item.UpdatedAt,
				},
			})
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": filtered})
	})

}
