package httpx

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"orbyte/internal/platform/activity"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

func registerUIModelReportingRoutes(mux *http.ServeMux, ident *identity.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, inventorySvc *application.InventoryCoreService, fieldSecurity *securityfields.Service) {
	mux.HandleFunc("GET /ui/data/models", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		modelKey := strings.TrimSpace(r.URL.Query().Get("model"))
		def, found := models.Definition(modelKey)
		if !found {
			respondError(w, shared.NotFound("model definition not found"))
			return
		}
		if !principalAllowsAll(ident, p, []string{def.ListPermissionKey}) {
			respondError(w, shared.Forbidden("model list is not allowed"))
			return
		}
		filters := map[string]string{}
		for key, values := range r.URL.Query() {
			switch key {
			case "model", "sort", "page", "page_size", "desc":
				continue
			}
			if len(values) == 0 {
				continue
			}
			filters[key] = strings.TrimSpace(values[0])
		}
		query := model.Query{
			Filters:  filters,
			SortKey:  strings.TrimSpace(r.URL.Query().Get("sort")),
			Page:     intQuery(r, "page", 1),
			PageSize: intQuery(r, "page_size", 20),
		}
		if err := validateModelQueryAccess(fieldSecurity, ident, p, def, query, "ui"); err != nil {
			respondError(w, err)
			return
		}
		if modelKey == "inventory_batch" {
			items, total, err := listDecoratedInventoryBatchRecords(models, inventorySvc, fieldSecurity, ident, p, def, query)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "definition": def})
			return
		}
		items, total, err := models.List(modelKey, query)
		if err != nil {
			respondError(w, err)
			return
		}
		items = sanitizeModelRecords(fieldSecurity, ident, p, def, items, "ui")
		respondJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "definition": def})
	})

	mux.HandleFunc("GET /ui/data/models/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		modelKey, recordID, ok := modelPath(strings.TrimPrefix(r.URL.Path, "/ui/data"))
		if !ok || recordID == "" {
			respondError(w, shared.NotFound("model not found"))
			return
		}
		def, found := models.Definition(modelKey)
		if !found {
			respondError(w, shared.NotFound("model definition not found"))
			return
		}
		if !principalAllowsAll(ident, p, []string{def.ReadPermissionKey}) {
			respondError(w, shared.Forbidden("model read is not allowed"))
			return
		}
		record, err := models.Get(modelKey, recordID)
		if err != nil {
			respondError(w, err)
			return
		}
		record = sanitizeModelRecord(fieldSecurity, ident, p, def, record, "ui")
		if modelKey == "inventory_batch" && inventorySvc != nil {
			record = inventorySvc.DecorateBatchRecord(record, organizationIDForPrincipal(p), p.currentLocationID, nowUTC())
			record = sanitizeModelRecord(fieldSecurity, ident, p, def, record, "ui")
		}
		payload := map[string]any{"record": record, "definition": def, "timeline": activities.Timeline("model:"+modelKey, recordID), "model_definitions": allModelDefinitions(models)}
		relatedDefs := map[string]model.Definition{}
		for _, relation := range def.Relations {
			items, _, err := models.Related(def.Key, recordID, relation.Key, model.Query{Page: 1, PageSize: 100})
			if err == nil {
				graphItems := make([]map[string]any, 0, len(items))
				for _, item := range items {
					graphItems = append(graphItems, modelGraphNode(models, fieldSecurity, ident, p, relation.TargetModelKey, item, map[string]bool{def.Key: true}, "ui"))
				}
				payload[relation.Key] = graphItems
			}
			if targetDef, ok := models.Definition(relation.TargetModelKey); ok {
				relatedDefs[relation.Key] = targetDef
			}
		}
		payload["related_definitions"] = relatedDefs
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /ui/data/reporting/datasets/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("analytics read is not allowed"))
			return
		}
		datasetKey := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ui/data/reporting/datasets/"))
		if datasetKey == "" {
			respondError(w, shared.NotFound("dataset not found"))
			return
		}
		dimensions := splitCSV(r.URL.Query().Get("dimensions"))
		measures := splitCSV(r.URL.Query().Get("measures"))
		groupBy := splitCSV(r.URL.Query().Get("group_by"))
		if datasetDef, ok := reportingSvc.Definition(datasetKey); ok && datasetDef.SourceKind == "model" {
			modelDef, found := models.Definition(datasetDef.ModelKey)
			if !found {
				respondError(w, shared.NotFound("model definition not found"))
				return
			}
			var err error
			dimensions, measures, groupBy, err = reportingSelectionsForDataset(fieldSecurity, ident, p, datasetDef, modelDef, dimensions, measures, groupBy)
			if err != nil {
				respondError(w, err)
				return
			}
		} else if datasetDef, ok := reportingSvc.Definition(datasetKey); ok && datasetDef.SourceKind == "documents" {
			items := docs.List()
			if len(items) > 0 {
				profile := documentAccessProfile(fieldSecurity, ident, p, items[0], "report")
				filtered := filterDocumentReportingDataset(datasetDef, profile)
				var err error
				dimensions, measures, groupBy, err = validateDocumentReportingSelections(fieldSecurity, ident, p, items[0], dimensions, measures, groupBy)
				if err != nil {
					respondError(w, err)
					return
				}
				if len(dimensions) == 0 {
					for _, item := range filtered.Dimensions {
						dimensions = append(dimensions, item.Key)
					}
				}
				if len(measures) == 0 {
					for _, item := range filtered.Measures {
						measures = append(measures, item.Key)
					}
				}
			}
		}
		payload, err := reportingSvc.ExecuteView(datasetKey, relationQuery(r), reporting.QueryRequest{
			Dimensions: dimensions,
			Measures:   measures,
			GroupBy:    groupBy,
			SortBy:     strings.TrimSpace(r.URL.Query().Get("sort_by")),
			Desc:       strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("desc")), "true"),
			Limit:      intQuery(r, "limit", 0),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /ui/data/reporting/models/", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("analytics read is not allowed"))
			return
		}
		modelKey := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ui/data/reporting/models/"))
		if modelKey == "" {
			respondError(w, shared.NotFound("model not found"))
			return
		}
		def, found := models.Definition(modelKey)
		if !found {
			respondError(w, shared.NotFound("model definition not found"))
			return
		}
		dimensions, measures, groupBy, err := reportingSelectionsForModel(fieldSecurity, ident, p, def, splitCSV(r.URL.Query().Get("dimensions")), splitCSV(r.URL.Query().Get("measures")), splitCSV(r.URL.Query().Get("group_by")))
		if err != nil {
			respondError(w, err)
			return
		}
		payload, err := reportingSvc.ExecuteAdHocModel(relationQuery(r), reporting.QueryRequest{
			ModelKey:   modelKey,
			Dimensions: dimensions,
			Measures:   measures,
			GroupBy:    groupBy,
			SortBy:     strings.TrimSpace(r.URL.Query().Get("sort_by")),
			Desc:       strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("desc")), "true"),
			Limit:      intQuery(r, "limit", 0),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("GET /ui/data/reporting/query", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("analytics read is not allowed"))
			return
		}
		source := strings.TrimSpace(r.URL.Query().Get("source"))
		if source == "" {
			respondError(w, shared.Validation("source is required"))
			return
		}
		dimensions := splitCSV(r.URL.Query().Get("dimensions"))
		measures := splitCSV(r.URL.Query().Get("measures"))
		groupBy := splitCSV(r.URL.Query().Get("group_by"))
		if strings.HasPrefix(source, "models/") {
			modelKey := strings.TrimSpace(strings.TrimPrefix(source, "models/"))
			def, found := models.Definition(modelKey)
			if !found {
				respondError(w, shared.NotFound("model definition not found"))
				return
			}
			var err error
			dimensions, measures, groupBy, err = reportingSelectionsForModel(fieldSecurity, ident, p, def, dimensions, measures, groupBy)
			if err != nil {
				respondError(w, err)
				return
			}
		} else if source == "documents" {
			items := docs.List()
			if len(items) > 0 {
				var err error
				dimensions, measures, groupBy, err = validateDocumentReportingSelections(fieldSecurity, ident, p, items[0], dimensions, measures, groupBy)
				if err != nil {
					respondError(w, err)
					return
				}
			}
		}
		payload, err := reportingSvc.ExecuteAdHocSource(source, relationQuery(r), reporting.QueryRequest{
			ModelKey:   strings.TrimSpace(r.URL.Query().Get("model_key")),
			Dimensions: dimensions,
			Measures:   measures,
			GroupBy:    groupBy,
			SortBy:     strings.TrimSpace(r.URL.Query().Get("sort_by")),
			Desc:       strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("desc")), "true"),
			Limit:      intQuery(r, "limit", 0),
		})
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, payload)
	})
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func listDecoratedInventoryBatchRecords(models *model.Service, inventorySvc *application.InventoryCoreService, fieldSecurity *securityfields.Service, ident *identity.Service, p principal, def model.Definition, query model.Query) ([]model.Record, int, error) {
	baseQuery := query
	baseQuery.Filters = cloneStringMap(query.Filters)
	delete(baseQuery.Filters, "status")
	delete(baseQuery.Filters, "is_issuable")
	baseQuery.Page = 1
	baseQuery.PageSize = model.MaxPageSize

	allItems := make([]model.Record, 0)
	for {
		pageItems, _, err := models.List(def.Key, baseQuery)
		if err != nil {
			return nil, 0, err
		}
		if len(pageItems) == 0 {
			break
		}
		allItems = append(allItems, pageItems...)
		if len(pageItems) < baseQuery.PageSize {
			break
		}
		baseQuery.Page++
	}

	allItems = sanitizeModelRecords(fieldSecurity, ident, p, def, allItems, "ui")
	allItems = inventorySvc.DecorateBatchRecords(allItems, organizationIDForPrincipal(p), p.currentLocationID, nowUTC())
	allItems = sanitizeModelRecords(fieldSecurity, ident, p, def, allItems, "ui")
	allItems = filterInventoryBatchRecords(allItems, query.Filters)

	total := len(allItems)
	pagedItems := paginateModelRecords(allItems, query.Page, query.PageSize)
	return pagedItems, total, nil
}

func paginateModelRecords(items []model.Record, page, pageSize int) []model.Record {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = model.DefaultPageSize
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []model.Record{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func filterInventoryBatchRecords(items []model.Record, filters map[string]string) []model.Record {
	if len(filters) == 0 {
		return items
	}
	filtered := make([]model.Record, 0, len(items))
	for _, item := range items {
		if !inventoryBatchMatchesFilters(item, filters) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func inventoryBatchMatchesFilters(item model.Record, filters map[string]string) bool {
	for key, value := range filters {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		actual := strings.TrimSpace(strings.ToLower(modelFieldString(item.Values[key])))
		switch key {
		case "status", "item_code", "warehouse_code", "batch_code":
			if actual != value {
				return false
			}
		case "is_issuable":
			if actual != value {
				return false
			}
		}
	}
	return true
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func modelFieldString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
