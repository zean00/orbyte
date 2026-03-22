package httpx

import (
	"net/http"
	"strings"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

func registerUIModelReportingRoutes(mux *http.ServeMux, ident *identity.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, fieldSecurity *securityfields.Service) {
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
		query := model.Query{
			Filters: map[string]string{
				"name":   strings.TrimSpace(r.URL.Query().Get("name")),
				"status": strings.TrimSpace(r.URL.Query().Get("status")),
			},
			SortKey:  strings.TrimSpace(r.URL.Query().Get("sort")),
			Page:     intQuery(r, "page", 1),
			PageSize: intQuery(r, "page_size", 20),
		}
		if err := validateModelQueryAccess(fieldSecurity, ident, p, def, query, "ui"); err != nil {
			respondError(w, err)
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
