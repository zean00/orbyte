package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"clinic/internal/platform/activity"
	application "clinic/internal/platform/application"
	"clinic/internal/platform/identity"
	"clinic/internal/platform/model"
	"clinic/internal/platform/shared"
)

func registerModelRoutes(mux *http.ServeMux, ident *identity.Service, models *model.Service, activities *activity.Service, modelActions *application.ModelActions) {
	mux.HandleFunc("POST /models/", func(w http.ResponseWriter, r *http.Request) {
		if modelKey, recordID, relationKey, ok := modelRelationPath(r.URL.Path); ok && recordID != "" && relationKey != "" {
			def, found := models.Definition(modelKey)
			if !found {
				respondError(w, shared.NotFound("model definition not found"))
				return
			}
			targetPermission := def.UpdatePermissionKey
			for _, relation := range def.Relations {
				if relation.Key != relationKey {
					continue
				}
				if targetDef, ok := models.Definition(relation.TargetModelKey); ok && strings.TrimSpace(targetDef.CreatePermissionKey) != "" {
					targetPermission = targetDef.CreatePermissionKey
				}
				break
			}
			p, ok := requireAuthorization(w, r, ident, targetPermission, "", "")
			if !ok {
				return
			}
			var req struct {
				Values map[string]any `json:"values"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondError(w, shared.Validation("invalid model relation payload"))
				return
			}
			var (
				record model.Record
				err    error
			)
			if modelActions != nil {
				_, related, actionErr := modelActions.PatchRelation(modelKey, recordID, relationKey, p.userID, []model.ChildMutation{{Operation: "upsert", Values: req.Values}})
				err = actionErr
				if err == nil && len(related[relationKey]) > 0 {
					record = related[relationKey][0]
				}
			} else {
				record, err = models.CreateRelated(modelKey, recordID, relationKey, p.userID, req.Values)
			}
			if err != nil {
				respondError(w, err)
				return
			}
			if modelActions == nil {
				_, _ = activities.AddMessage("model:"+modelKey, recordID, p.userID, "Related record created", map[string]any{"model_key": modelKey, "relation_key": relationKey, "related_record_id": record.ID})
			}
			respondJSON(w, http.StatusCreated, record)
			return
		}

		modelKey, recordID, ok := modelPath(r.URL.Path)
		if !ok || recordID != "" {
			respondError(w, shared.NotFound("model route not found"))
			return
		}
		def, found := models.Definition(modelKey)
		if !found {
			respondError(w, shared.NotFound("model definition not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, def.CreatePermissionKey, "", "")
		if !ok {
			return
		}
		var req model.CompositeMutation
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid model payload"))
			return
		}
		var (
			record  model.Record
			related map[string][]model.Record
			err     error
		)
		if modelActions != nil {
			record, related, err = modelActions.CreateComposite(modelKey, p.userID, req)
		} else {
			record, related, err = models.CreateComposite(modelKey, p.userID, req)
		}
		if err != nil {
			respondError(w, err)
			return
		}
		if modelActions == nil {
			_, _ = activities.AddMessage("model:"+modelKey, record.ID, p.userID, "Record created", map[string]any{"model_key": modelKey})
		}
		respondJSON(w, http.StatusCreated, map[string]any{"record": record, "related": related})
	})

	mux.HandleFunc("GET /models/", func(w http.ResponseWriter, r *http.Request) {
		if modelKey, recordID, relationKey, ok := modelRelationPath(r.URL.Path); ok && recordID != "" && relationKey != "" {
			def, found := models.Definition(modelKey)
			if !found {
				respondError(w, shared.NotFound("model definition not found"))
				return
			}
			if _, ok := requireAuthorization(w, r, ident, def.ReadPermissionKey, "", ""); !ok {
				return
			}
			items, total, err := models.Related(modelKey, recordID, relationKey, relationQuery(r))
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "relation_key": relationKey})
			return
		}
		modelKey, recordID, ok := modelPath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("model route not found"))
			return
		}
		def, found := models.Definition(modelKey)
		if !found {
			respondError(w, shared.NotFound("model definition not found"))
			return
		}
		if recordID == "" {
			if _, ok := requireAuthorization(w, r, ident, def.ListPermissionKey, "", ""); !ok {
				return
			}
			items, total, err := models.List(modelKey, relationQuery(r))
			if err != nil {
				respondError(w, err)
				return
			}
			_, relatedDefs := relatedModelPayload(models, def, "")
			respondJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "definition": def, "related_definitions": relatedDefs})
			return
		}
		if _, ok := requireAuthorization(w, r, ident, def.ReadPermissionKey, "", ""); !ok {
			return
		}
		record, err := models.Get(modelKey, recordID)
		if err != nil {
			respondError(w, err)
			return
		}
		related, relatedDefs := relatedModelPayload(models, def, record.ID)
		respondJSON(w, http.StatusOK, map[string]any{
			"record":              record,
			"definition":          def,
			"timeline":            activities.Timeline("model:"+modelKey, record.ID),
			"related":             related,
			"related_definitions": relatedDefs,
			"model_definitions":   allModelDefinitions(models),
		})
	})

	mux.HandleFunc("PUT /models/", func(w http.ResponseWriter, r *http.Request) {
		modelKey, recordID, ok := modelPath(r.URL.Path)
		if !ok || recordID == "" {
			respondError(w, shared.NotFound("model route not found"))
			return
		}
		def, found := models.Definition(modelKey)
		if !found {
			respondError(w, shared.NotFound("model definition not found"))
			return
		}
		p, ok := requireAuthorization(w, r, ident, def.UpdatePermissionKey, "", "")
		if !ok {
			return
		}
		var req model.CompositeMutation
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid model payload"))
			return
		}
		var (
			record  model.Record
			related map[string][]model.Record
			err     error
		)
		if modelActions != nil {
			record, related, err = modelActions.UpdateComposite(modelKey, recordID, p.userID, req)
		} else {
			record, related, err = models.UpdateComposite(modelKey, recordID, p.userID, req)
		}
		if err != nil {
			respondError(w, err)
			return
		}
		if modelActions == nil {
			_, _ = activities.AddMessage("model:"+modelKey, record.ID, p.userID, "Record updated", map[string]any{"model_key": modelKey})
		}
		respondJSON(w, http.StatusOK, map[string]any{"record": record, "related": related})
	})

	mux.HandleFunc("GET /activity/timeline", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "configuration.read", "", "configuration.read"); !ok {
			return
		}
		targetType := strings.TrimSpace(r.URL.Query().Get("target_type"))
		targetID := strings.TrimSpace(r.URL.Query().Get("target_id"))
		if targetType == "" || targetID == "" {
			respondError(w, shared.Validation("target_type and target_id are required"))
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": activities.Timeline(targetType, targetID)})
	})
}

func relatedModelPayload(models *model.Service, def model.Definition, recordID string) (map[string]any, map[string]model.Definition) {
	items := map[string]any{}
	defs := map[string]model.Definition{}
	for _, relation := range def.Relations {
		if targetDef, ok := models.Definition(relation.TargetModelKey); ok {
			defs[relation.Key] = targetDef
		}
		if strings.TrimSpace(recordID) == "" {
			items[relation.Key] = []map[string]any{}
			continue
		}
		records, _, err := models.Related(def.Key, recordID, relation.Key, model.Query{Page: 1, PageSize: 100})
		if err != nil {
			continue
		}
		graphItems := make([]map[string]any, 0, len(records))
		for _, item := range records {
			graphItems = append(graphItems, modelGraphNode(models, relation.TargetModelKey, item, map[string]bool{def.Key: true}))
		}
		items[relation.Key] = graphItems
	}
	return items, defs
}

func modelGraphNode(models *model.Service, modelKey string, record model.Record, visited map[string]bool) map[string]any {
	node := map[string]any{"record": record}
	if visited[modelKey] {
		return node
	}
	nextVisited := map[string]bool{}
	for key, value := range visited {
		nextVisited[key] = value
	}
	nextVisited[modelKey] = true
	def, ok := models.Definition(modelKey)
	if !ok {
		return node
	}
	related := map[string]any{}
	for _, relation := range def.Relations {
		items, _, err := models.Related(modelKey, record.ID, relation.Key, model.Query{Page: 1, PageSize: 100})
		if err != nil {
			continue
		}
		children := make([]map[string]any, 0, len(items))
		for _, item := range items {
			children = append(children, modelGraphNode(models, relation.TargetModelKey, item, nextVisited))
		}
		related[relation.Key] = children
	}
	node["related"] = related
	return node
}

func allModelDefinitions(models *model.Service) map[string]model.Definition {
	items := map[string]model.Definition{}
	for _, def := range models.Definitions() {
		items[def.Key] = def
	}
	return items
}

func modelPath(path string) (string, string, bool) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[0] != "models" {
		return "", "", false
	}
	if len(parts) == 2 {
		return parts[1], "", true
	}
	return parts[1], parts[2], true
}

func modelRelationPath(path string) (string, string, string, bool) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 || parts[0] != "models" || parts[3] != "relations" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[4], true
}

func relationQuery(r *http.Request) model.Query {
	filters := map[string]string{
		"name":   strings.TrimSpace(r.URL.Query().Get("name")),
		"status": strings.TrimSpace(r.URL.Query().Get("status")),
	}
	reserved := map[string]struct{}{
		"name":       {},
		"status":     {},
		"sort":       {},
		"page":       {},
		"page_size":  {},
		"source":     {},
		"model_key":  {},
		"dimensions": {},
		"measures":   {},
		"group_by":   {},
		"sort_by":    {},
		"desc":       {},
		"limit":      {},
	}
	for key, values := range r.URL.Query() {
		if len(values) == 0 {
			continue
		}
		if _, skip := reserved[key]; skip {
			continue
		}
		filters[key] = strings.TrimSpace(values[0])
	}
	return model.Query{
		Filters:  filters,
		SortKey:  strings.TrimSpace(r.URL.Query().Get("sort")),
		Page:     intQuery(r, "page", 1),
		PageSize: intQuery(r, "page_size", 20),
	}
}

func intQuery(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	var out int
	for _, char := range raw {
		if char < '0' || char > '9' {
			return fallback
		}
		out = out*10 + int(char-'0')
	}
	if out <= 0 {
		return fallback
	}
	return out
}
