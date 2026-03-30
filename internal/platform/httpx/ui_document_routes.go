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
		receivableState := strings.TrimSpace(r.URL.Query().Get("receivable_state"))
		payableState := strings.TrimSpace(r.URL.Query().Get("payable_state"))
		includePayload := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_payload")), "1") || strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_payload")), "true")
		sortKey := strings.TrimSpace(r.URL.Query().Get("sort"))
		today := time.Now().UTC().Format("2006-01-02")
		items := searchSvc.ListDocuments()
		filtered := make([]map[string]any, 0, len(items))
		if !includePayload && receivableState == "" && payableState == "" {
			for _, item := range items {
				if documentType != "" && item.DocumentType != documentType {
					continue
				}
				if statusFilter != "" && item.Status != statusFilter {
					continue
				}
				if organizationID := organizationIDForPrincipal(p); organizationID != "" && item.OrganizationID != "" && item.OrganizationID != organizationID {
					continue
				}
				if p.currentLocationID != "" && item.LocationID != "" && item.LocationID != p.currentLocationID {
					continue
				}
				record, err := docs.Get(item.DocumentID)
				if err != nil {
					continue
				}
				if manualJournalReadBlocked(ident, p, record) {
					continue
				}
				filtered = append(filtered, documentListProjectionItem(item))
			}
			sortDocumentProjectionItems(filtered, sortKey)
			respondJSON(w, http.StatusOK, map[string]any{"items": filtered})
			return
		}
		for _, item := range items {
			if documentType != "" && item.DocumentType != documentType {
				continue
			}
			if statusFilter != "" && item.Status != statusFilter {
				continue
			}
			if organizationID := organizationIDForPrincipal(p); organizationID != "" && item.OrganizationID != "" && item.OrganizationID != organizationID {
				continue
			}
			if p.currentLocationID != "" && item.LocationID != "" && item.LocationID != p.currentLocationID {
				continue
			}
			record, err := docs.Get(item.DocumentID)
			if err != nil {
				continue
			}
			if manualJournalReadBlocked(ident, p, record) {
				continue
			}
			rendered := docs.Render(record, document.ViewNormal, modules.EnabledMap())
			rendered = sanitizeDocumentRecord(fieldSecurity, ident, p, rendered, "ui")
			if !matchesReceivableStateFilter(receivableState, rendered, today) {
				continue
			}
			if !matchesPayableStateFilter(payableState, rendered, today) {
				continue
			}
			filtered = append(filtered, map[string]any{
				"header": map[string]any{
					"id":              rendered.Header.ID,
					"type":            rendered.Header.Type,
					"status":          rendered.Header.Status,
					"version":         rendered.Header.Version,
					"etag":            rendered.Header.ETag,
					"organization_id": rendered.Header.OrganizationID,
					"location_id":     rendered.Header.LocationID,
					"number":          rendered.Header.Number,
					"created_at":      rendered.Header.CreatedAt,
					"updated_at":      rendered.Header.UpdatedAt,
				},
				"body": map[string]any{
					"schema_version": rendered.Body.SchemaVersion,
					"payload":        rendered.Body.Payload,
					"content_hash":   rendered.Body.ContentHash,
				},
			})
		}
		sortDocumentProjectionItems(filtered, sortKey)
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
		if manualJournalReadBlocked(ident, p, record) {
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
			record, err := docs.Get(item.DocumentID)
			if err != nil {
				continue
			}
			if manualJournalReadBlocked(ident, p, record) {
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

func documentListProjectionItem(item search.DocumentSummary) map[string]any {
	return map[string]any{
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
	}
}

func sortDocumentProjectionItems(filtered []map[string]any, sortKey string) {
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
}

func matchesReceivableStateFilter(filter string, record document.Record, today string) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	if record.Header.Type != "invoice" {
		return false
	}
	payload := record.Body.Payload
	balance := recordNumberValue(payload["balance_due_amount"])
	dueDate := strings.TrimSpace(recordStringValue(payload["due_date"]))
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "open":
		return balance > 0 && (record.Header.Status == "issued" || record.Header.Status == "partially_paid")
	case "due_today":
		return balance > 0 && dueDate == today
	case "overdue":
		return balance > 0 && dueDate != "" && dueDate < today
	case "current":
		return balance > 0 && (dueDate == "" || dueDate > today)
	case "paid":
		return record.Header.Status == "paid" || balance <= 0
	default:
		return true
	}
}

func matchesPayableStateFilter(filter string, record document.Record, today string) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	if record.Header.Type != "vendor_bill" {
		return false
	}
	payload := record.Body.Payload
	balance := recordNumberValue(payload["balance_due_amount"])
	dueDate := strings.TrimSpace(recordStringValue(payload["due_date"]))
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "open":
		return balance > 0 && (record.Header.Status == "issued" || record.Header.Status == "partially_paid")
	case "due_today":
		return balance > 0 && dueDate == today
	case "overdue":
		return balance > 0 && dueDate != "" && dueDate < today
	case "current":
		return balance > 0 && (dueDate == "" || dueDate > today)
	case "paid":
		return record.Header.Status == "paid" || balance <= 0
	default:
		return true
	}
}

func recordStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func recordNumberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	default:
		return 0
	}
}
