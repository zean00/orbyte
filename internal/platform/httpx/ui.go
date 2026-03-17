package httpx

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/activity"
	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/i18n"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/monitoring"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/reporting"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
)

func registerUIRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, searchSvc *search.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, policySvc *policy.Service, fieldSecurity *securityfields.Service) {
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(uiShellHTML))
	})

	mux.HandleFunc("GET /ui/assets/platform.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(platformCSS)
	})

	mux.HandleFunc("GET /ui/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"name":             "Orbyte Platform UI",
			"short_name":       "Orbyte UI",
			"start_url":        "/ui",
			"display":          "standalone",
			"background_color": "#f3efe7",
			"theme_color":      "#1f6f5f",
			"description":      "Manifest-driven offline-capable platform shell.",
		})
	})

	mux.HandleFunc("GET /ui/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(uiServiceWorkerJS))
	})

	mux.HandleFunc("GET /ui/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		menus, actions, views, _ := visibleUIContracts(ident, modules, p, module.UISurfaceUser)
		adminMenus, adminActions, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceAdmin)
		defaultPath := ""
		if len(menus) > 0 {
			for _, action := range actions {
				if action.Key == menus[0].ActionKey {
					defaultPath = action.RoutePath
					break
				}
			}
		}
		adminPath := "/admin"
		if len(adminMenus) > 0 {
			for _, action := range adminActions {
				if action.Key == adminMenus[0].ActionKey {
					adminPath = "/admin#" + action.RoutePath
					break
				}
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"menus":             menus,
			"actions":           actions,
			"views":             views,
			"default_path":      defaultPath,
			"admin_access":      len(adminMenus) > 0,
			"admin_path":        adminPath,
			"locale":            localeFromRequest(r, ident),
			"supported_locales": i18n.SupportedLocales(),
		})
	})

	mux.HandleFunc("GET /ui/menus", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		menus, _, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceUser)
		respondJSON(w, http.StatusOK, map[string]any{"items": menus})
	})

	mux.HandleFunc("GET /ui/actions", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		_, actions, _, _ := visibleUIContracts(ident, modules, p, module.UISurfaceUser)
		respondJSON(w, http.StatusOK, map[string]any{"items": actions})
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
		_, _, views, _ := visibleUIContracts(ident, modules, p, module.UISurfaceUser)
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
		resolution, ok := modules.ResolveRouteForSurface(path, module.UISurfaceUser)
		if !ok {
			respondError(w, shared.NotFound("route not found"))
			return
		}
		if !principalAllowsAll(ident, p, resolution.Action.RequiredPermissions) {
			respondError(w, shared.Forbidden("route is not allowed"))
			return
		}
		if resolution.View != nil && !principalAllowsAll(ident, p, resolution.View.RequiredPermissions) {
			respondError(w, shared.Forbidden("view is not allowed"))
			return
		}
		if resolution.CustomEntry != nil && !principalAllowsAll(ident, p, resolution.CustomEntry.RequiredPermissions) {
			respondError(w, shared.Forbidden("route is not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, resolution)
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
		respondJSON(w, http.StatusOK, map[string]any{
			"record":       rendered,
			"lines":        record.Lines,
			"links":        record.Links,
			"attachments":  record.Attachments,
			"documentType": record.Header.Type,
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

	mux.HandleFunc("GET /ui/data/analytics/snapshot", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("analytics read is not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, analyticsSvc.Snapshot())
	})

	mux.HandleFunc("GET /ui/data/monitoring/summary", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"monitoring.read"}) {
			respondError(w, shared.Forbidden("monitoring read is not allowed"))
			return
		}
		respondJSON(w, http.StatusOK, monitoringSvc.Summary())
	})

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

func relatedModelItems(models *model.Service, def model.Definition, recordID, relationKey string) []model.Record {
	for _, relation := range def.Relations {
		if relation.Key != relationKey {
			continue
		}
		items, _, err := models.Related(def.Key, recordID, relationKey, model.Query{Page: 1, PageSize: 100})
		if err != nil {
			return nil
		}
		return items
	}
	return nil
}

func splitCSV(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func requireInteractivePrincipal(w http.ResponseWriter, r *http.Request) (principal, bool) {
	if err := authError(r); err != nil {
		respondError(w, err)
		return principal{}, false
	}
	p, ok := currentPrincipal(r)
	if !ok {
		respondError(w, shared.Unauthorized("authentication required"))
		return principal{}, false
	}
	if p.kind != userPrincipal {
		respondError(w, shared.Forbidden("interactive user session is required"))
		return principal{}, false
	}
	return p, true
}

func principalAllowsAll(ident *identity.Service, p principal, permissions []string) bool {
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == "" {
			continue
		}
		if decision := ident.DecideSession(p.sessionID, permission, p.currentLocationID); !decision.Allowed {
			return false
		}
	}
	return true
}

func visibleUIContracts(ident *identity.Service, modules *module.Service, p principal, surface module.UISurface) ([]module.MenuDefinition, []module.ActionDefinition, []module.ViewDefinition, []module.CustomEntryDefinition) {
	allowedMenus := make([]module.MenuDefinition, 0)
	allowedActions := make([]module.ActionDefinition, 0)
	allowedViews := make([]module.ViewDefinition, 0)
	allowedEntries := make([]module.CustomEntryDefinition, 0)
	actionKeys := map[string]bool{}
	viewKeys := map[string]bool{}
	entryKeys := map[string]bool{}

	for _, detail := range modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		for _, action := range detail.Manifest.Frontend.Actions {
			if !surfaceMatches(action.Surface, surface) {
				continue
			}
			if !principalAllowsAll(ident, p, action.RequiredPermissions) {
				continue
			}
			switch action.RenderMode {
			case module.RenderModeGeneric:
				if action.ViewKey != "" {
					view, ok := modules.ViewForSurface(action.ViewKey, surface)
					if !ok || !principalAllowsAll(ident, p, view.RequiredPermissions) {
						continue
					}
					if !viewKeys[view.Key] {
						allowedViews = append(allowedViews, view)
						viewKeys[view.Key] = true
					}
				}
			case module.RenderModeCustom:
				entryAllowed := false
				for _, entry := range detail.Manifest.Frontend.CustomEntries {
					if entry.Key == action.CustomEntryKey && surfaceMatches(entry.Surface, surface) && principalAllowsAll(ident, p, entry.RequiredPermissions) {
						entryAllowed = true
						if !entryKeys[entry.Key] {
							allowedEntries = append(allowedEntries, entry)
							entryKeys[entry.Key] = true
						}
						break
					}
				}
				if !entryAllowed {
					continue
				}
			}
			if !actionKeys[action.Key] {
				allowedActions = append(allowedActions, action)
				actionKeys[action.Key] = true
			}
		}
	}

	for _, detail := range modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		for _, menuDef := range detail.Manifest.Frontend.Menus {
			if !surfaceMatches(menuDef.Surface, surface) {
				continue
			}
			if !principalAllowsAll(ident, p, menuDef.RequiredPermissions) || !actionKeys[menuDef.ActionKey] {
				continue
			}
			allowedMenus = append(allowedMenus, menuDef)
		}
	}

	sort.Slice(allowedMenus, func(i, j int) bool {
		if allowedMenus[i].Order == allowedMenus[j].Order {
			return allowedMenus[i].Key < allowedMenus[j].Key
		}
		return allowedMenus[i].Order < allowedMenus[j].Order
	})
	sort.Slice(allowedActions, func(i, j int) bool { return allowedActions[i].Key < allowedActions[j].Key })
	sort.Slice(allowedViews, func(i, j int) bool { return allowedViews[i].Key < allowedViews[j].Key })
	sort.Slice(allowedEntries, func(i, j int) bool { return allowedEntries[i].Key < allowedEntries[j].Key })

	return allowedMenus, allowedActions, allowedViews, allowedEntries
}

func surfaceMatches(itemSurface, requested module.UISurface) bool {
	effective := itemSurface
	if effective == "" {
		effective = module.UISurfaceUser
	}
	if requested == "" || requested == module.UISurfaceBoth {
		return true
	}
	return effective == module.UISurfaceBoth || effective == requested
}

func viewKeyFromPath(path string) string {
	const prefix = "/ui/views/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(path, prefix))
}

func bundleKeyFromPath(path string) string {
	const prefix = "/ui/assets/modules/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(path, prefix)), ".js")
}

const uiShellHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="theme-color" content="#1f6f5f">
  <link rel="manifest" href="/ui/manifest.webmanifest">
  <title>Orbyte Platform UI</title>
  <link rel="stylesheet" href="/ui/assets/platform.css?v=` + platformAssetVersion + `">
</head>
<body>
  <div class="shell" id="shell-root">
    <aside class="sidebar" id="shell-sidebar">
      <div class="toolbar">
        <div>
          <h1 class="brand" id="shell-brand">Orbyte Platform UI</h1>
          <p class="subtitle" id="shell-subtitle">Manifest-driven shell with generic pages and module custom entries.</p>
        </div>
        <div class="actions">
          <label class="locale-switch">
            <span id="locale-label">Language</span>
            <select id="locale-switcher"></select>
          </label>
          <a id="admin-link-button" class="button secondary" href="/admin" hidden>Admin</a>
          <button type="button" id="logout-button" class="secondary" hidden>Log out</button>
        </div>
      </div>
      <nav id="menu" class="menu-list"></nav>
    </aside>
    <main class="content">
      <div class="route-panel" id="route-panel">
        <div class="route-copy">
          <div id="route-title"><h2>Loading…</h2></div>
          <p class="status" id="route-status">Resolving module UI registry.</p>
        </div>
        <div class="status-bar">
          <span class="badge"><strong id="network-status">online</strong></span>
          <span class="badge">sync <strong id="sync-status">0 pending</strong></span>
          <span class="badge">cache <strong id="cache-status">cold</strong></span>
        </div>
      </div>
      <div id="view-root"></div>
    </main>
  </div>
  <script>
    const offlineDBName = 'orbyte_ui_offline_v1';
    const offlineDBVersion = 1;
    const defaultSupportedLocales = ['en', 'id'];
    const uiMessages = {
      en: {
        shell_brand: 'Orbyte Platform UI',
        shell_subtitle: 'Manifest-driven shell with generic pages and module custom entries.',
        locale_label: 'Language',
        admin_link: 'Admin',
        logout: 'Log out',
        loading: 'Loading…',
        online: 'online',
        offline: 'offline',
        cache_cold: 'cold',
        cache_warm: 'warm',
        route_resolving: 'Resolving module UI registry.',
        using_cached_data: 'Using cached data for',
        login_title: 'Platform Access',
        login_subtitle: 'Sign in to continue.',
        google_button: 'Continue with Google',
        username: 'Username',
        password: 'Password',
        sign_in: 'Sign in',
        sign_in_unavailable: 'No interactive sign-in method is enabled for this deployment.',
        or: 'or',
        view_unavailable: 'View unavailable',
        custom_loading: 'Loading custom module page…',
        search: 'Search',
        sort: 'Sort',
        sort_document: 'Document',
        sort_updated: 'Updated',
        sort_status: 'Status',
        sort_name: 'Name',
        all: 'All',
        new: 'New',
        open: 'Open',
        previous: 'Previous',
        next: 'Next',
        page: 'Page',
        standard_list: 'Standard list page rendered from the module manifest.',
        no_records: 'No records yet.',
        select_record: 'Select a record from the list to inspect its canonical record.',
        queue_sync: 'Queue Sync',
        save: 'Save',
        create: 'Create',
        save_local: 'Save Local',
        save_draft: 'Save Draft',
        record_updated: 'Record updated.',
        record_created: 'Record created.',
        draft_saved_local: 'Draft saved locally.',
        draft_queued: 'Draft queued for sync.',
        draft_updated: 'Draft updated through manifest-driven form.',
        ui_bootstrap_failed: 'UI bootstrap failed',
        ui_bootstrap_failed_status: 'Failed to bootstrap module UI.',
        no_routes: 'No permitted routes are available for this principal.',
        resolved_from_module: 'Resolved from module',
        using_rendering: 'using',
        sync_pending: 'pending',
        sync_conflict: 'conflict',
        value_active: 'Active',
        value_inactive: 'Inactive',
        value_blocked: 'Blocked',
        value_draft: 'Draft',
        value_registered: 'Registered',
        value_completed: 'Completed',
        value_submitted: 'Submitted',
        value_approved: 'Approved',
        value_rejected: 'Rejected',
        value_cancelled: 'Cancelled',
        value_failed: 'Failed',
        value_conflict: 'Conflict',
        value_queued: 'Queued',
        value_pending: 'Pending',
        value_enabled: 'Enabled',
        value_disabled: 'Disabled',
        value_true: 'Yes',
        value_false: 'No',
        action_submit: 'Submit',
        action_approve: 'Approve',
        action_reject: 'Reject',
        action_reopen: 'Reopen',
        action_cancel: 'Cancel',
        add: 'Add',
        add_row: 'Add Row',
        remove: 'Remove',
        no_related_items: 'No related items.',
        no_related_items_yet: 'No related items yet.',
        related_record_created: 'Related record created.'
      },
      id: {
        shell_brand: 'UI Platform Orbyte',
        shell_subtitle: 'Shell berbasis manifest dengan halaman generik dan entri modul kustom.',
        locale_label: 'Bahasa',
        admin_link: 'Admin',
        logout: 'Keluar',
        loading: 'Memuat…',
        online: 'online',
        offline: 'offline',
        cache_cold: 'dingin',
        cache_warm: 'hangat',
        route_resolving: 'Menyelesaikan registri UI modul.',
        using_cached_data: 'Menggunakan data cache untuk',
        login_title: 'Akses Platform',
        login_subtitle: 'Masuk untuk melanjutkan.',
        google_button: 'Lanjut dengan Google',
        username: 'Nama pengguna',
        password: 'Kata sandi',
        sign_in: 'Masuk',
        sign_in_unavailable: 'Tidak ada metode masuk interaktif yang aktif untuk deployment ini.',
        or: 'atau',
        view_unavailable: 'Tampilan tidak tersedia',
        custom_loading: 'Memuat halaman modul kustom…',
        search: 'Cari',
        sort: 'Urutkan',
        sort_document: 'Dokumen',
        sort_updated: 'Diperbarui',
        sort_status: 'Status',
        sort_name: 'Nama',
        all: 'Semua',
        new: 'Baru',
        open: 'Buka',
        previous: 'Sebelumnya',
        next: 'Berikutnya',
        page: 'Halaman',
        standard_list: 'Halaman daftar standar yang dirender dari manifest modul.',
        no_records: 'Belum ada data.',
        select_record: 'Pilih data dari daftar untuk melihat catatan kanonisnya.',
        queue_sync: 'Antrikan Sinkronisasi',
        save: 'Simpan',
        create: 'Buat',
        save_local: 'Simpan Lokal',
        save_draft: 'Simpan Draf',
        record_updated: 'Data diperbarui.',
        record_created: 'Data dibuat.',
        draft_saved_local: 'Draf disimpan secara lokal.',
        draft_queued: 'Draf diantrikan untuk sinkronisasi.',
        draft_updated: 'Draf diperbarui melalui formulir berbasis manifest.',
        ui_bootstrap_failed: 'Bootstrap UI gagal',
        ui_bootstrap_failed_status: 'Gagal melakukan bootstrap UI modul.',
        no_routes: 'Tidak ada rute yang diizinkan untuk principal ini.',
        resolved_from_module: 'Diselesaikan dari modul',
        using_rendering: 'menggunakan',
        sync_pending: 'tertunda',
        sync_conflict: 'konflik',
        value_active: 'Aktif',
        value_inactive: 'Tidak Aktif',
        value_blocked: 'Diblokir',
        value_draft: 'Draf',
        value_registered: 'Terdaftar',
        value_completed: 'Selesai',
        value_submitted: 'Diajukan',
        value_approved: 'Disetujui',
        value_rejected: 'Ditolak',
        value_cancelled: 'Dibatalkan',
        value_failed: 'Gagal',
        value_conflict: 'Konflik',
        value_queued: 'Diantrikan',
        value_pending: 'Tertunda',
        value_enabled: 'Aktif',
        value_disabled: 'Nonaktif',
        value_true: 'Ya',
        value_false: 'Tidak',
        action_submit: 'Ajukan',
        action_approve: 'Setujui',
        action_reject: 'Tolak',
        action_reopen: 'Buka Kembali',
        action_cancel: 'Batalkan',
        add: 'Tambah',
        add_row: 'Tambah Baris',
        remove: 'Hapus',
        no_related_items: 'Belum ada item terkait.',
        no_related_items_yet: 'Belum ada item terkait.',
        related_record_created: 'Data terkait berhasil dibuat.'
      }
    };
    const state = {
      bootstrap: null,
      route: null,
      bundles: {},
      authOptions: null,
      offlineBootstrap: null,
      syncStats: {pending: 0, conflict: 0, failed: 0},
      cacheWarm: false,
      locale: 'en',
      supportedLocales: defaultSupportedLocales
    };

    function normalizeLocale(locale) {
      const value = String(locale || '').trim().toLowerCase().replace(/_/g, '-');
      if (!value) return 'en';
      if (value === 'id' || value.indexOf('id-') === 0) return 'id';
      return 'en';
    }

    function detectPreferredLocale() {
      if (navigator.languages && navigator.languages.length) return normalizeLocale(navigator.languages[0]);
      return normalizeLocale(navigator.language || 'en');
    }

    function t(key) {
      const locale = state.locale || 'en';
      return (uiMessages[locale] && uiMessages[locale][key]) || (uiMessages.en && uiMessages.en[key]) || key;
    }

    function pickText(item, baseField) {
      if (!item) return '';
      const localized = item[baseField + '_i18n'];
      if (localized && typeof localized === 'object') {
        const current = localized[state.locale];
        if (current) return current;
        if (localized.en) return localized.en;
        if (localized.id) return localized.id;
      }
      return item[baseField] || '';
    }

    function humanizeToken(value) {
      const raw = String(value == null ? '' : value).trim();
      if (!raw) return '';
      if (/[A-Z]/.test(raw) || raw.indexOf(' ') >= 0 || !/^[a-z0-9_-]+$/.test(raw)) return raw;
      return raw.split(/[_-]+/).filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
    }

    function translateToken(prefix, value) {
      const raw = String(value == null ? '' : value).trim();
      if (!raw) return '';
      const key = prefix + '_' + raw.toLowerCase().replace(/[^a-z0-9]+/g, '_');
      const translated = t(key);
      if (translated !== key) return translated;
      return humanizeToken(raw);
    }

    function displayValue(value) {
      if (value == null) return '';
      if (typeof value === 'boolean') return t(value ? 'value_true' : 'value_false');
      if (typeof value === 'number') return String(value);
      if (typeof value === 'string') return translateToken('value', value);
      return String(value);
    }

    async function persistLocale(locale) {
      try {
        const response = await fetch('/locale?locale=' + encodeURIComponent(locale), {credentials: 'same-origin'});
        if (!response.ok) throw new Error('locale update failed');
        const payload = await response.json();
        state.locale = normalizeLocale(payload.locale || locale);
        state.supportedLocales = payload.supported_locales || state.supportedLocales || defaultSupportedLocales;
      } catch (_) {
        state.locale = normalizeLocale(locale);
      }
    }

    function escapeHTML(value) {
      return String(value == null ? '' : value).replace(/[&<>"]/g, (char) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[char]));
    }

    function applyLocale() {
      document.documentElement.lang = state.locale;
      document.title = t('shell_brand');
      const brand = document.getElementById('shell-brand');
      const subtitle = document.getElementById('shell-subtitle');
      const localeLabel = document.getElementById('locale-label');
      const adminLinkButton = document.getElementById('admin-link-button');
      const logoutButton = document.getElementById('logout-button');
      const routeTitle = document.getElementById('route-title');
      const routeStatus = document.getElementById('route-status');
      if (brand) brand.textContent = t('shell_brand');
      if (subtitle) subtitle.textContent = t('shell_subtitle');
      if (localeLabel) localeLabel.textContent = t('locale_label');
      if (adminLinkButton) adminLinkButton.textContent = t('admin_link');
      if (logoutButton) logoutButton.textContent = t('logout');
      if (routeTitle && !state.route) routeTitle.innerHTML = '<h2>' + escapeHTML(t('loading')) + '</h2>';
      if (routeStatus && !state.route) routeStatus.textContent = t('route_resolving');
      refreshOfflineStatus();
    }

    function applyShellLayout(authenticated) {
      const shell = document.getElementById('shell-root');
      const sidebar = document.getElementById('shell-sidebar');
      const routePanel = document.getElementById('route-panel');
      const content = document.querySelector('.content');
      if (authenticated) {
        if (shell) {
          shell.classList.remove('login-mode');
          shell.classList.add('workspace-mode');
        }
        if (sidebar) sidebar.hidden = false;
        if (routePanel) routePanel.hidden = false;
        if (content) {
          content.classList.remove('login-mode');
          content.classList.add('workspace-mode');
        }
        return;
      }
      if (shell) {
        shell.classList.remove('workspace-mode');
        shell.classList.add('login-mode');
      }
      if (sidebar) sidebar.hidden = true;
      if (routePanel) routePanel.hidden = true;
      if (content) {
        content.classList.remove('workspace-mode');
        content.classList.add('login-mode');
      }
    }

    function renderLocaleSwitcher() {
      const select = document.getElementById('locale-switcher');
      if (!select) return;
      select.innerHTML = (state.supportedLocales || defaultSupportedLocales).map((locale) => {
        const name = locale === 'id' ? 'Bahasa Indonesia' : 'English';
        return '<option value="' + locale + '">' + name + '</option>';
      }).join('');
      select.value = state.locale;
      select.onchange = async () => {
        await persistLocale(select.value);
        applyLocale();
        if (state.bootstrap) {
          await renderRoute();
        } else {
          renderLogin(authErrorFromQuery());
        }
      };
    }

    async function registerServiceWorker() {
      if (!('serviceWorker' in navigator)) return;
      try {
        await navigator.serviceWorker.register('/ui/sw.js', {scope: '/'});
      } catch (_) {}
    }

    function openOfflineDB() {
      return new Promise((resolve, reject) => {
        const request = indexedDB.open(offlineDBName, offlineDBVersion);
        request.onupgradeneeded = () => {
          const db = request.result;
          ['app_meta', 'contracts', 'reference_packages', 'projection_packages', 'drafts', 'sync_queue', 'sync_results', 'records'].forEach((storeName) => {
            if (!db.objectStoreNames.contains(storeName)) db.createObjectStore(storeName);
          });
        };
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error || new Error('indexeddb unavailable'));
      });
    }

    async function idbPut(storeName, key, value) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readwrite');
        tx.objectStore(storeName).put(value, key);
        tx.oncomplete = () => { db.close(); resolve(); };
        tx.onerror = () => { db.close(); reject(tx.error || new Error('indexeddb write failed')); };
      });
    }

    async function idbGet(storeName, key) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readonly');
        const req = tx.objectStore(storeName).get(key);
        req.onsuccess = () => { db.close(); resolve(req.result || null); };
        req.onerror = () => { db.close(); reject(req.error || new Error('indexeddb read failed')); };
      });
    }

    async function idbDelete(storeName, key) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readwrite');
        tx.objectStore(storeName).delete(key);
        tx.oncomplete = () => { db.close(); resolve(); };
        tx.onerror = () => { db.close(); reject(tx.error || new Error('indexeddb delete failed')); };
      });
    }

    async function idbEntries(storeName) {
      const db = await openOfflineDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(storeName, 'readonly');
        const req = tx.objectStore(storeName).getAll();
        req.onsuccess = () => { db.close(); resolve(req.result || []); };
        req.onerror = () => { db.close(); reject(req.error || new Error('indexeddb list failed')); };
      });
    }

    function cachedResponseKey(path) {
      return 'response:' + path;
    }

    function shouldCacheResponse(path, options) {
      const method = ((options && options.method) || 'GET').toUpperCase();
      if (method !== 'GET') return false;
      return path === '/auth/options' ||
        path === '/ui/bootstrap' ||
        path.indexOf('/ui/routes/resolve') === 0 ||
        path.indexOf('/ui/views/') === 0 ||
        path.indexOf('/ui/data/') === 0;
    }

    function isNetworkError(err) {
      const message = String((err && err.message) || err || '').toLowerCase();
      return !message || message.indexOf('failed to fetch') >= 0 || message.indexOf('network') >= 0 || message.indexOf('load failed') >= 0;
    }

    async function rememberResponse(path, payload, kind) {
      await idbPut('contracts', cachedResponseKey(path), {
        payload,
        kind,
        updated_at: new Date().toISOString()
      });
      state.cacheWarm = true;
      refreshOfflineStatus();
    }

    async function loadCachedResponse(path) {
      const cached = await idbGet('contracts', cachedResponseKey(path));
      return cached ? cached.payload : null;
    }

    async function api(path, options) {
      const requestOptions = Object.assign({credentials: 'same-origin'}, options || {});
      try {
        const response = await fetch(path, requestOptions);
        if (!response.ok) {
          let message = response.statusText;
          try {
            const payload = await response.json();
            message = payload.error && payload.error.message ? payload.error.message : message;
          } catch (_) {}
          throw new Error(message);
        }
        const contentType = response.headers.get('content-type') || '';
        const payload = contentType.includes('application/json') ? await response.json() : await response.text();
        if (shouldCacheResponse(path, requestOptions)) {
          await rememberResponse(path, payload, contentType.includes('application/json') ? 'json' : 'text');
        }
        return payload;
      } catch (err) {
        if (shouldCacheResponse(path, requestOptions) && (isNetworkError(err) || !navigator.onLine)) {
          const cached = await loadCachedResponse(path);
          if (cached != null) {
            setStatus(t('using_cached_data') + ' ' + path + '.');
            return cached;
          }
        }
        throw err;
      }
    }

    function currentPath() {
      const raw = window.location.hash.replace(/^#/, '');
      if (!raw) return '';
      const qIndex = raw.indexOf('?');
      return qIndex >= 0 ? raw.slice(0, qIndex) : raw;
    }

    function currentParams() {
      const raw = window.location.hash.replace(/^#/, '');
      const qIndex = raw.indexOf('?');
      return new URLSearchParams(qIndex >= 0 ? raw.slice(qIndex + 1) : '');
    }

    function setStatus(text) {
      document.getElementById('route-status').textContent = text;
    }

    function refreshOfflineStatus() {
      const networkNode = document.getElementById('network-status');
      const syncNode = document.getElementById('sync-status');
      const cacheNode = document.getElementById('cache-status');
      if (networkNode) networkNode.textContent = navigator.onLine ? t('online') : t('offline');
      if (syncNode) syncNode.textContent = state.syncStats.pending + ' ' + t('sync_pending') + ' / ' + state.syncStats.conflict + ' ' + t('sync_conflict');
      if (cacheNode) cacheNode.textContent = state.cacheWarm ? t('cache_warm') : t('cache_cold');
    }

    function authErrorFromQuery() {
      const params = new URLSearchParams(window.location.search);
      return params.get('auth_error') || '';
    }

	function loginTitle() {
		if (state.authOptions && state.authOptions['login_title_' + state.locale]) return state.authOptions['login_title_' + state.locale];
		if (state.authOptions && state.authOptions.login_title && !(state.locale !== 'en' && state.authOptions.login_title === 'Platform Access')) return state.authOptions.login_title;
		return t('login_title');
	}

	function loginSubtitle() {
		if (state.authOptions && state.authOptions['login_subtitle_' + state.locale]) return state.authOptions['login_subtitle_' + state.locale];
		if (state.authOptions && state.authOptions.login_subtitle && !(state.locale !== 'en' && state.authOptions.login_subtitle === 'Sign in to continue.')) return state.authOptions.login_subtitle;
		return t('login_subtitle');
	}

	function googleButtonLabel() {
		if (state.authOptions && state.authOptions['google_button_label_' + state.locale]) return state.authOptions['google_button_label_' + state.locale];
		if (state.authOptions && state.authOptions.google_button_label && !(state.locale !== 'en' && state.authOptions.google_button_label === 'Continue with Google')) return state.authOptions.google_button_label;
		return t('google_button');
	}

    function requestedUIRoute() {
      return currentPath();
    }

    function requestedUIHref() {
      const path = requestedUIRoute();
      if (!path) return '/ui';
      const params = currentParams().toString();
      return '/ui#' + path + (params ? '?' + params : '');
    }

    function offlineDocumentCapability(documentType) {
      return (state.offlineBootstrap && state.offlineBootstrap.documents || []).find((item) => item.type === documentType) || null;
    }

    function offlineModelCapability(modelKey) {
      return (state.offlineBootstrap && state.offlineBootstrap.models || []).find((item) => item.model_key === modelKey) || null;
    }

    function offlineProjectionCapabilityForView(view) {
      if (!state.offlineBootstrap || !state.offlineBootstrap.projections) return null;
      if (view.projection_key) {
        return state.offlineBootstrap.projections.find((item) => item.index_key.indexOf('documents.') === 0) || null;
      }
      if (view.model_key) {
        return state.offlineBootstrap.projections.find((item) => item.index_key.indexOf(view.model_key) >= 0 || item.title.toLowerCase().indexOf(view.model_key) >= 0) || null;
      }
      return null;
    }

    function draftKey(kind, targetKey, targetID) {
      return [kind, targetKey, targetID || 'new'].join(':');
    }

    async function loadDraft(kind, targetKey, targetID) {
      return idbGet('drafts', draftKey(kind, targetKey, targetID));
    }

    async function saveDraft(kind, targetKey, targetID, draft) {
      const key = draftKey(kind, targetKey, targetID);
      await idbPut('drafts', key, Object.assign({draft_key: key, updated_at: new Date().toISOString()}, draft));
      return key;
    }

    function queueKey(idempotencyKey) {
      return 'queue:' + idempotencyKey;
    }

    async function queueSyncItem(item) {
      const idempotencyKey = item.idempotency_key || ('sync-' + Date.now() + '-' + Math.random().toString(36).slice(2));
      item.idempotency_key = idempotencyKey;
      await idbPut('sync_queue', queueKey(idempotencyKey), Object.assign({queued_at: new Date().toISOString()}, item));
      await refreshSyncStats();
      return item;
    }

    async function refreshSyncStats() {
      const queued = await idbEntries('sync_queue');
      const drafts = await idbEntries('drafts');
      state.syncStats.pending = queued.length;
      state.syncStats.conflict = drafts.filter((item) => item && item.status === 'conflict').length;
      state.syncStats.failed = drafts.filter((item) => item && item.status === 'failed').length;
      refreshOfflineStatus();
    }

    async function rememberProjectionPackage(pkg) {
      await idbPut('projection_packages', pkg.package_key, pkg);
    }

    async function rememberReferencePackage(pkg) {
      await idbPut('reference_packages', pkg.package_key, pkg);
    }

    async function prefetchOfflinePackages() {
      if (!state.offlineBootstrap) return;
      for (const item of (state.offlineBootstrap.references || [])) {
        try {
          const pkg = await api('/offline/packages/references', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({type_key: item.type_key})
          });
          await rememberReferencePackage(pkg);
        } catch (_) {}
      }
      for (const item of (state.offlineBootstrap.projections || [])) {
        try {
          const pkg = await api('/offline/packages/projections', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({index_key: item.index_key, query: {page: 1, page_size: 100, include_fields: item.default_include_fields || []}})
          });
          await rememberProjectionPackage(pkg);
        } catch (_) {}
      }
    }

    async function loadOfflineBootstrap() {
      try {
        state.offlineBootstrap = await api('/offline/bootstrap');
        await idbPut('app_meta', 'offline_bootstrap', state.offlineBootstrap);
        await prefetchOfflinePackages();
      } catch (err) {
        state.offlineBootstrap = await idbGet('app_meta', 'offline_bootstrap');
      }
      return state.offlineBootstrap;
    }

    async function projectionFallback(view) {
      const capability = offlineProjectionCapabilityForView(view);
      if (!capability) return null;
      const pkg = await idbGet('projection_packages', 'projection:' + capability.index_key);
      if (!pkg || !pkg.result || !Array.isArray(pkg.result.hits)) return null;
      if (view.model_key) {
        return {
          items: pkg.result.hits.map((hit) => ({
            id: hit.source_id,
            model_key: view.model_key,
            values: Object.assign({}, hit.fields || {})
          })),
          total: pkg.result.total || pkg.result.hits.length
        };
      }
      return {
        items: pkg.result.hits.map((hit) => ({
          header: {
            id: hit.fields && (hit.fields.document_id || hit.source_id) || hit.source_id,
            type: hit.fields && hit.fields.document_type || view.document_type || '',
            status: hit.fields && hit.fields.status || '',
            updated_at: hit.fields && hit.fields.updated_at || '',
            etag: hit.fields && hit.fields.etag || '',
            version: hit.fields && hit.fields.version || 0
          },
          body: {payload: hit.fields || {}}
        })),
        total: pkg.result.total || pkg.result.hits.length
      };
    }

    async function processSyncQueue() {
      if (!navigator.onLine) {
        await refreshSyncStats();
        return;
      }
      const queued = await idbEntries('sync_queue');
      if (!queued.length) {
        await refreshSyncStats();
        return;
      }
      try {
        const payload = await api('/offline/sync', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({items: queued})
        });
        const results = payload.items || [];
        for (const result of results) {
          const key = queueKey(result.idempotency_key);
          const queueItem = await idbGet('sync_queue', key);
          if (!queueItem) continue;
          const targetKey = queueItem.kind === 'model' ? queueItem.model_key : queueItem.document_type;
          const draft = await loadDraft(queueItem.kind, targetKey, queueItem.target_id);
          if (result.status === 'accepted') {
            if (draft) {
              draft.status = 'accepted';
              draft.target_id = result.target_id || draft.target_id || '';
              draft.version = result.version || draft.version || 0;
              draft.etag = result.etag || draft.etag || '';
              await saveDraft(queueItem.kind, targetKey, draft.target_id || queueItem.target_id, draft);
            }
            await idbDelete('sync_queue', key);
          } else if (result.status === 'conflict') {
            if (draft) {
              draft.status = 'conflict';
              draft.conflict = result.conflict || {};
              await saveDraft(queueItem.kind, targetKey, queueItem.target_id, draft);
            }
            await idbDelete('sync_queue', key);
          } else {
            if (draft) {
              draft.status = 'failed';
              draft.last_error = result.error || 'sync failed';
              await saveDraft(queueItem.kind, targetKey, queueItem.target_id, draft);
            }
          }
          await idbPut('sync_results', result.idempotency_key, result);
        }
      } catch (_) {}
      await refreshSyncStats();
    }

    async function performLogout() {
      const csrf = readCookie('orbyte_csrf');
      try {
        await fetch('/auth/logout', {
          method: 'POST',
          credentials: 'same-origin',
          headers: csrf ? {'X-CSRF-Token': csrf} : {}
        });
      } catch (_) {}
      state.bootstrap = null;
      state.route = null;
      state.offlineBootstrap = null;
      window.location.hash = '';
      applyShellLayout(false);
      renderLogin(loginSubtitle());
    }

    function renderLogin(message) {
      const root = document.getElementById('view-root');
      const passwordEnabled = !state.authOptions || !!state.authOptions.password_enabled;
      const googleEnabled = !!(state.authOptions && state.authOptions.google_enabled);
      const statusMessage = message || (authErrorFromQuery() === 'google_login_failed' ? 'Google sign-in failed. Try again or use a local account.' : loginSubtitle());
      applyShellLayout(false);
      document.getElementById('route-title').innerHTML = '<h2>' + escapeHTML(loginTitle()) + '</h2>';
      setStatus(statusMessage);
      document.getElementById('menu').innerHTML = '';
      document.getElementById('admin-link-button').hidden = true;
      document.getElementById('logout-button').hidden = true;
      const passwordForm = passwordEnabled
        ? '<form id="login-form"><label class="field"><span class="meta">' + escapeHTML(t('username')) + '</span><input id="login-username" name="username" autocomplete="username"></label><label class="field"><span class="meta">' + escapeHTML(t('password')) + '</span><input id="login-password" name="password" type="password" autocomplete="current-password"></label><div class="actions"><button type="submit">' + escapeHTML(t('sign_in')) + '</button></div></form>'
        : '';
      const emptyState = !passwordEnabled && !googleEnabled
        ? '<p class="status">' + escapeHTML(t('sign_in_unavailable')) + '</p>'
        : '';
      const divider = passwordEnabled && googleEnabled ? '<div class="divider">' + escapeHTML(t('or')) + '</div>' : '';
      const googleAction = googleEnabled ? '<div class="actions"><button type="button" id="google-login" class="google">' + escapeHTML(googleButtonLabel()) + '</button></div>' : '';
      root.innerHTML = '<section class="login-shell"><div class="panel"><h3>' + escapeHTML(loginTitle()) + '</h3><p class="status">' + escapeHTML(statusMessage) + '</p>' + passwordForm + divider + googleAction + emptyState + '</div></section>';
      const form = document.getElementById('login-form');
      if (form) {
        form.addEventListener('submit', async (event) => {
          event.preventDefault();
          const username = document.getElementById('login-username').value.trim();
          const password = document.getElementById('login-password').value;
          try {
            await api('/auth/login', {
              method: 'POST',
              headers: {'Content-Type': 'application/json'},
              body: JSON.stringify({username, password})
            });
            state.bootstrap = await api('/ui/bootstrap');
            state.supportedLocales = state.bootstrap.supported_locales || defaultSupportedLocales;
            if (state.bootstrap.locale) {
              state.locale = normalizeLocale(state.bootstrap.locale);
            }
            renderLocaleSwitcher();
            applyLocale();
            await loadOfflineBootstrap();
            await processSyncQueue();
            if (!window.location.hash && state.bootstrap.default_path) {
              window.location.hash = '#' + state.bootstrap.default_path;
            }
            await renderRoute();
          } catch (err) {
            renderLogin(err.message);
          }
        });
      }
      const googleButton = document.getElementById('google-login');
      if (googleButton) {
        googleButton.addEventListener('click', () => {
          const target = requestedUIHref();
          window.location.assign('/auth/google/start?next=' + encodeURIComponent(target));
        });
      }
    }

    function renderMenus() {
      const container = document.getElementById('menu');
      container.innerHTML = '';
      if (!state.bootstrap) {
        document.getElementById('admin-link-button').hidden = true;
        document.getElementById('logout-button').hidden = true;
        return;
      }
      applyShellLayout(true);
      document.getElementById('admin-link-button').hidden = !state.bootstrap || !state.bootstrap.admin_access;
      document.getElementById('admin-link-button').href = state.bootstrap && state.bootstrap.admin_path ? state.bootstrap.admin_path : '/admin';
      document.getElementById('logout-button').hidden = !state.bootstrap;
      for (const menu of state.bootstrap.menus) {
        const action = state.bootstrap.actions.find((item) => item.key === menu.action_key);
        if (!action) continue;
        const link = document.createElement('a');
        link.className = 'menu-link' + (currentPath() === action.route_path ? ' active' : '');
        link.href = '#' + action.route_path;
        link.textContent = pickText(menu, 'label');
        container.appendChild(link);
      }
    }

    function findActionByView(predicate) {
      return (state.bootstrap.actions || []).find((action) => {
        if (!action.view_key) return false;
        const view = (state.bootstrap.views || []).find((item) => item.key === action.view_key);
        return !!view && predicate(view);
      }) || null;
    }

    function routeForModel(modelKey, kind) {
      const action = findActionByView((view) => view.model_key === modelKey && view.kind === kind);
      return action ? action.route_path : '';
    }

    function routeForDocument(documentType, kind) {
      const action = findActionByView((view) => view.document_type === documentType && view.kind === kind);
      return action ? action.route_path : '';
    }

    async function loadBundle(bundleKey) {
      if (state.bundles[bundleKey]) return state.bundles[bundleKey];
      const script = document.createElement('script');
      script.src = '/ui/assets/modules/' + encodeURIComponent(bundleKey) + '.js';
      script.async = true;
      const loaded = await new Promise((resolve, reject) => {
        script.onload = resolve;
        script.onerror = () => reject(new Error('failed to load module bundle'));
        document.head.appendChild(script);
      });
      void loaded;
      const bundles = window.ClinicModuleBundles || {};
      if (!bundles[bundleKey]) throw new Error('module bundle not registered');
      state.bundles[bundleKey] = bundles[bundleKey];
      return bundles[bundleKey];
    }

    function renderJSONCard(title, payload) {
      const root = document.getElementById('view-root');
      root.innerHTML = '<section class="panel"><h3>' + escapeHTML(title) + '</h3><pre></pre></section>';
      root.querySelector('pre').textContent = JSON.stringify(payload, null, 2);
    }

    async function renderGeneric(route) {
      const root = document.getElementById('view-root');
      const view = route.view;
      if (!view) {
        renderJSONCard(t('view_unavailable'), route);
        return;
      }
      if (view.kind === 'list') {
        const params = currentParams();
        const query = new URLSearchParams();
        if (view.document_type) query.set('type', view.document_type);
        if (view.model_key) query.set('model', view.model_key);
        if (params.get('status')) query.set('status', params.get('status'));
        if (params.get('name')) query.set('name', params.get('name'));
        if (params.get('sort')) query.set('sort', params.get('sort'));
        const pageSize = parseInt(params.get('page_size') || view.default_page_size || '10', 10);
        const page = parseInt(params.get('page') || '1', 10);
        query.set('page', String(page));
        query.set('page_size', String(pageSize));
        const listPath = view.model_key ? '/ui/data/models?' : (view.projection_key ? '/ui/data/projections/documents?' : '/ui/data/documents?');
        let payload;
        try {
          payload = await api(listPath + query.toString());
        } catch (err) {
          payload = await projectionFallback(view);
          if (!payload) throw err;
        }
        const pagedItems = payload.items || [];
        const newRoute = view.model_key ? routeForModel(view.model_key, 'form') : routeForDocument(view.document_type, 'form');
        const filterBar = '<div class="toolbar-row">' +
          ((view.filters || []).map((filter) => {
            if (filter.type !== 'enum') return '';
            const options = ['<option value="">' + t('all') + ' ' + escapeHTML(pickText(filter, 'label')) + '</option>'].concat((filter.options || []).map((option) => '<option value="' + option + '"' + (params.get(filter.key) === option ? ' selected' : '') + '>' + escapeHTML(displayValue(option)) + '</option>'));
            return '<label class="control-tile"><span class="meta">' + escapeHTML(pickText(filter, 'label')) + '</span><select data-filter="' + filter.key + '">' + options.join('') + '</select></label>';
          }).join('')) +
          (view.model_key ? '<label class="control-tile grow"><span class="meta">' + t('search') + '</span><input data-filter="name" value="' + escapeHTML(params.get('name') || '') + '" placeholder="' + escapeHTML(t('search')) + '"></label>' : '') +
          '<label class="control-tile"><span class="meta">' + t('sort') + '</span><select data-filter="sort"><option value="">' + t('sort_document') + '</option><option value="updated_at"' + (params.get('sort') === 'updated_at' ? ' selected' : '') + '>' + t('sort_updated') + '</option><option value="status"' + (params.get('sort') === 'status' ? ' selected' : '') + '>' + t('sort_status') + '</option><option value="name"' + (params.get('sort') === 'name' ? ' selected' : '') + '>' + t('sort_name') + '</option></select></label>' +
          (newRoute ? '<button type="button" data-new="1">' + t('new') + '</button>' : '') +
          '</div>';
        const columnDefs = view.columns || [];
        const rows = pagedItems.map((item) => {
          const openID = item.id || (item.header && item.header.id) || '';
          const cells = columnDefs.map((column, index) => {
            const value = escapeHTML(displayValue(resolvePath(item, column.path)));
            if (index === 0) {
              return '<td><div class="row-primary">' + value + '</div><div class="row-secondary">' + escapeHTML(openID) + '</div></td>';
            }
            return '<td>' + value + '</td>';
          }).join('');
          return '<tr>' + (cells || ('<td><div class="row-primary">' + escapeHTML(openID) + '</div></td>')) + '<td><button class="secondary" data-open="' + openID + '">' + t('open') + '</button></td></tr>';
        }).join('');
        const total = payload.total || pagedItems.length;
        const tableHeader = columnDefs.map((column) => '<th>' + escapeHTML(pickText(column, 'label')) + '</th>').join('') + '<th></th>';
        const tableMarkup = rows
          ? '<div class="table-shell"><table class="data-table"><thead><tr>' + tableHeader + '</tr></thead><tbody>' + rows + '</tbody></table></div>'
          : '<div class="table-shell"><div class="page-body"><p class="status">' + t('no_records') + '</p></div></div>';
        const pagination = '<div class="pagination-bar"><span class="status">' + t('page') + ' ' + page + ' / ' + Math.max(1, Math.ceil(total / pageSize)) + '</span><div class="actions"><button class="secondary" data-page="' + Math.max(1, page - 1) + '"' + (page <= 1 ? ' disabled' : '') + '>' + t('previous') + '</button><button class="secondary" data-page="' + (page + 1) + '"' + (page * pageSize >= total ? ' disabled' : '') + '>' + t('next') + '</button></div></div>';
        root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(pickText(view, 'empty_state') || t('standard_list')) + '</p></div></div><div class="page-body">' + filterBar + tableMarkup + pagination + '</div></section>';
        root.querySelectorAll('[data-filter]').forEach((input) => {
          input.addEventListener('change', () => {
            const next = currentParams();
            if (input.value) next.set(input.dataset.filter, input.value); else next.delete(input.dataset.filter);
            next.set('page', '1');
            window.location.hash = '#' + currentPath() + (next.toString() ? '?' + next.toString() : '');
          });
        });
        root.querySelectorAll('[data-page]').forEach((button) => {
          button.addEventListener('click', () => {
            const next = currentParams();
            next.set('page', button.dataset.page);
            next.set('page_size', String(pageSize));
            window.location.hash = '#' + currentPath() + '?' + next.toString();
          });
        });
        root.querySelectorAll('[data-open]').forEach((button) => {
          button.addEventListener('click', () => {
            const targetPath = view.model_key ? routeForModel(view.model_key, 'detail') : routeForDocument(view.document_type, 'detail');
            if (!targetPath) return;
            window.location.hash = '#' + targetPath + '?id=' + encodeURIComponent(button.dataset.open);
          });
        });
        root.querySelectorAll('[data-new]').forEach((button) => {
          button.addEventListener('click', () => {
            if (!newRoute) return;
            window.location.hash = '#' + newRoute;
          });
        });
        return;
      }
      if (view.kind === 'detail') {
        const documentID = currentParams().get('id');
        if (!documentID) {
          root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(t('select_record')) + '</p></div></div></section>';
          return;
        }
        if (view.model_key) {
          const payload = await api('/ui/data/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(documentID));
          const record = payload.record;
          const tabMarkup = (view.tabs || []).map((tab) => {
            const sections = (tab.sections || []).map((section) => renderModelSection(section, record)).join('');
            return '<section class="panel"><h3>' + escapeHTML(pickText(tab, 'title')) + '</h3>' + sections + '</section>';
          }).join('');
          const sectionMarkup = (view.sections || []).map((section) => renderModelSection(section, record)).join('');
          const relatedViews = (view.related_views || []).map((item) => renderRelatedView(item, payload, view)).join('');
          root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(record.id + ' · v' + record.version) + '</p></div></div><div class="page-body"><div class="section-stack">' + (tabMarkup || sectionMarkup) + '</div></div></section>' + relatedViews;
          root.querySelectorAll('[data-related-save]').forEach((button) => {
            button.addEventListener('click', async () => {
              const sourceKey = button.dataset.relatedSave;
              const section = button.closest('section');
              const values = {};
              section.querySelectorAll('[data-path]').forEach((input) => assignPath(values, input.dataset.path.replace(/^values\\./, ''), readFieldValue(input)));
              const csrf = readCookie('orbyte_csrf');
              try {
                await api('/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(record.id) + '/relations/' + encodeURIComponent(sourceKey), {
                  method: 'POST',
                  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                  body: JSON.stringify({values})
                });
                const statusNode = section.querySelector('[data-related-status="' + sourceKey + '"]');
                if (statusNode) statusNode.textContent = t('related_record_created');
                await renderRoute();
              } catch (err) {
                const statusNode = section.querySelector('[data-related-status="' + sourceKey + '"]');
                if (statusNode) statusNode.textContent = err.message;
                setStatus(err.message);
              }
            });
          });
          return;
        }
        const payload = await api('/ui/data/documents/' + encodeURIComponent(documentID));
        const record = payload.record;
        const tabMarkup = (view.tabs || []).map((tab) => {
          const sections = (tab.sections || []).map((section) => renderSection(section, record)).join('');
          return '<section class="panel"><h3>' + escapeHTML(pickText(tab, 'title')) + '</h3>' + sections + '</section>';
        }).join('');
        const sectionMarkup = (view.sections || []).map((section) => renderSection(section, record)).join('');
        const relatedViews = (view.related_views || []).map((item) => renderRelatedView(item, payload, view)).join('');
        const actionZones = renderActionZones(view);
        root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(record.header.id + ' · v' + record.header.version + ' · ' + displayValue(record.header.status)) + '</p></div></div><div class="page-body"><div class="section-stack">' + (tabMarkup || sectionMarkup || ('<pre>' + escapeHTML(JSON.stringify(record.body.payload, null, 2)) + '</pre>')) + '</div></div><div class="page-actions">' + actionZones + '</div></section>' + relatedViews;
        for (const actionKey of view.allowed_actions || []) {
          const placement = await api('/ui/actions/render?action=' + encodeURIComponent(actionKey) + '&document_id=' + encodeURIComponent(record.header.id));
          if (!placement.allowed) {
            continue;
          }
          const button = document.createElement('button');
          button.textContent = translateToken('action', actionKey);
          const zone = resolveActionPlacement(view, actionKey, placement);
          if (zone === 'primary') {
            button.className = '';
          } else if (actionKey === 'reject' || actionKey === 'cancel') {
            button.className = 'warn';
          } else {
            button.className = 'secondary';
          }
          button.addEventListener('click', async () => {
            try {
              await invokeDocumentAction(record.header.id, actionKey, record.header.version, record.header.etag);
              await renderRoute();
            } catch (err) {
              setStatus(err.message);
            }
          });
          (root.querySelector('[data-zone="' + zone + '"]') || root.querySelector('[data-zone="secondary"]')).appendChild(button);
        }
        return;
      }
      if (view.kind === 'form') {
        const documentID = currentParams().get('id');
        if (view.model_key) {
          let payload = {record: {id: '', version: 0, values: {}}, definition: {relations: []}, related_definitions: {}};
          let record = {id: '', version: 0, values: {}};
          const localDraft = await loadDraft('model', view.model_key, documentID);
          if (localDraft && localDraft.values) {
            record = {id: documentID || '', version: localDraft.version || 0, values: localDraft.values};
          }
          try {
            if (documentID) {
              payload = await api('/ui/data/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(documentID));
              record = payload.record;
            } else {
              payload = await api('/ui/data/models?model=' + encodeURIComponent(view.model_key) + '&page_size=1');
            }
          } catch (_) {
            if (!documentID) {
              payload = {record, definition: {relations: []}, related_definitions: {}, model_definitions: {}};
            }
          }
          const formSections = (view.sections || []).length > 0
            ? (view.sections || []).map((section) => renderModelFormSection(section, record)).join('')
            : '<div class="form-grid">' + (view.fields || []).map((field) => renderEditableModelField(field, record)).join('') + '</div>';
          const relationViews = (view.related_views && view.related_views.length) ? view.related_views : deriveRelatedViews(payload.definition);
          const relationEditors = relationViews.map((item) => renderRelationEditor(item, payload)).join('');
          const offlineCapable = !!offlineModelCapability(view.model_key);
          root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(documentID ? record.id + ' · v' + record.version : t('record_created')) + '</p></div></div><div class="page-body"><div class="section-stack">' + formSections + relationEditors + '</div><p class="status" id="form-status"></p></div><div class="page-actions"><button id="save-form">' + ((offlineCapable && !navigator.onLine) ? t('queue_sync') : (documentID ? t('save') : t('create'))) + '</button><button id="save-local" class="secondary"' + (offlineCapable ? '' : ' disabled') + '>' + t('save_local') + '</button></div></section>';
          bindRelationRemove(root);
          root.querySelectorAll('[data-relation-add]').forEach((button) => {
            button.addEventListener('click', () => appendRelationRow(button.dataset.relationAdd, payload));
          });
          const saveLocalButton = root.querySelector('#save-local');
          if (saveLocalButton) {
            saveLocalButton.addEventListener('click', async () => {
              const values = {};
              root.querySelectorAll('[data-path]').forEach((input) => {
                if (input.closest('[data-relation-editor]')) return;
                assignPath(values, input.dataset.path.replace(/^values\\./, ''), readFieldValue(input));
              });
              const relations = collectRelationMutations(root);
              await saveDraft('model', view.model_key, documentID, {
                kind: 'model',
                model_key: view.model_key,
                target_id: documentID || '',
                version: record.version || 0,
                values,
                relations,
                status: 'local_only'
              });
              await refreshSyncStats();
              document.getElementById('form-status').textContent = t('draft_saved_local');
              setStatus(t('draft_saved_local'));
            });
          }
          const button = root.querySelector('#save-form');
          if (button) {
            button.addEventListener('click', async () => {
              const values = {};
              root.querySelectorAll('[data-path]').forEach((input) => {
                if (input.closest('[data-relation-editor]')) return;
                assignPath(values, input.dataset.path.replace(/^values\\./, ''), readFieldValue(input));
              });
              const relations = collectRelationMutations(root);
              const csrf = readCookie('orbyte_csrf');
              try {
                if (!navigator.onLine && offlineCapable) {
                  const queued = await queueSyncItem({
                    kind: 'model',
                    operation: documentID ? 'update' : 'create',
                    model_key: view.model_key,
                    target_id: documentID || '',
                    expected_version: record.version || 0,
                    values,
                    relations
                  });
                  await saveDraft('model', view.model_key, documentID, {
                    kind: 'model',
                    model_key: view.model_key,
                    target_id: documentID || '',
                    version: record.version || 0,
                    values,
                    relations,
                    status: 'queued',
                    idempotency_key: queued.idempotency_key
                  });
                  document.getElementById('form-status').textContent = t('draft_queued');
                  setStatus(t('draft_queued'));
                } else {
                  const created = await api('/models/' + encodeURIComponent(view.model_key) + (documentID ? '/' + encodeURIComponent(documentID) : ''), {
                    method: documentID ? 'PUT' : 'POST',
                    headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                    body: JSON.stringify(documentID ? {values, expected_version: record.version, relations} : {values, relations})
                  });
                  document.getElementById('form-status').textContent = documentID ? t('record_updated') : t('record_created');
                  setStatus(documentID ? t('record_updated') : t('record_created'));
                  if (!documentID) {
                    const detailRoute = routeForModel(view.model_key, 'detail');
                    const createdRecord = created && (created.record || created);
                    if (detailRoute && createdRecord && createdRecord.id) {
                      window.location.hash = '#' + detailRoute + '?id=' + encodeURIComponent(createdRecord.id);
                    }
                  }
                }
              } catch (err) {
                document.getElementById('form-status').textContent = err.message;
                setStatus(err.message);
              }
            });
          }
          return;
        }
        const offlineCapable = !!offlineDocumentCapability(view.document_type);
        let record = {header: {id: documentID || '', version: 0, etag: '', type: view.document_type || '', status: 'draft'}, body: {payload: {}}};
        const localDraft = await loadDraft('document', view.document_type, documentID);
        if (localDraft && localDraft.payload) {
          record.body.payload = localDraft.payload;
          record.header.version = localDraft.version || 0;
          record.header.etag = localDraft.etag || '';
        }
        if (documentID) {
          try {
            record = await api('/documents/' + encodeURIComponent(documentID) + '?view=expanded');
          } catch (_) {}
        }
        const formSections = (view.sections || []).length > 0
          ? (view.sections || []).map((section) => renderFormSection(section, record)).join('')
          : '<div class="form-grid">' + (view.fields || []).map((field) => renderEditableField(field, record)).join('') + '</div>';
        root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3><p class="status">' + escapeHTML(displayValue(record.header.status || 'draft')) + '</p></div></div><div class="page-body"><div class="section-stack">' + formSections + '</div><p class="status" id="form-status"></p></div><div class="page-actions"><button id="save-form">' + ((offlineCapable && !navigator.onLine) ? t('queue_sync') : t('save_draft')) + '</button><button id="save-local" class="secondary"' + (offlineCapable ? '' : ' disabled') + '>' + t('save_local') + '</button></div></section>';
        const saveLocalButton = root.querySelector('#save-local');
        if (saveLocalButton) {
          saveLocalButton.addEventListener('click', async () => {
            const payload = {};
            root.querySelectorAll('[data-path]').forEach((input) => assignPath(payload, input.dataset.path.replace(/^body\\.payload\\./, ''), readFieldValue(input)));
            await saveDraft('document', view.document_type, documentID, {
              kind: 'document',
              document_type: view.document_type,
              target_id: documentID || '',
              version: record.header.version || 0,
              etag: record.header.etag || '',
              payload,
              status: 'local_only'
            });
            await refreshSyncStats();
            document.getElementById('form-status').textContent = t('draft_saved_local');
            setStatus(t('draft_saved_local'));
          });
        }
        const button = root.querySelector('#save-form');
        if (button) {
          button.addEventListener('click', async () => {
            const payload = {};
            root.querySelectorAll('[data-path]').forEach((input) => assignPath(payload, input.dataset.path.replace(/^body\\.payload\\./, ''), readFieldValue(input)));
            const csrf = readCookie('orbyte_csrf');
            try {
              if (offlineCapable && (!navigator.onLine || !documentID)) {
                const queued = await queueSyncItem({
                  kind: 'document',
                  operation: documentID ? 'update' : 'create',
                  document_type: view.document_type,
                  target_id: documentID || '',
                  expected_version: record.header.version || 0,
                  expected_etag: record.header.etag || '',
                  organization_id: (record.header && record.header.organization_id) || 'org_default',
                  location_id: (record.header && record.header.location_id) || '',
                  payload
                });
                await saveDraft('document', view.document_type, documentID, {
                  kind: 'document',
                  document_type: view.document_type,
                  target_id: documentID || '',
                  version: record.header.version || 0,
                  etag: record.header.etag || '',
                  payload,
                  status: 'queued',
                  idempotency_key: queued.idempotency_key
                });
                if (navigator.onLine) await processSyncQueue();
                document.getElementById('form-status').textContent = t('draft_queued');
                setStatus(t('draft_queued'));
              } else {
                await api('/documents/' + encodeURIComponent(documentID), {
                  method: 'PUT',
                  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                  body: JSON.stringify({payload})
                });
                document.getElementById('form-status').textContent = t('draft_updated');
                setStatus(t('draft_updated'));
              }
            } catch (err) {
              document.getElementById('form-status').textContent = err.message;
              setStatus(err.message);
            }
          });
        }
        return;
      }
      if (view.kind === 'dashboard') {
        const source = view.dataset_key
          ? await api('/ui/data/reporting/datasets/' + encodeURIComponent(view.dataset_key))
          : (view.projection_key === 'monitoring.summary'
          ? await api('/ui/data/monitoring/summary')
          : await api('/ui/data/analytics/snapshot'));
        const summary = (view.cards || []).map((card) => ({card, value: resolvePath(source, card.path)}));
        root.innerHTML = '<section class="page-panel"><div class="page-header"><div><h3>' + escapeHTML(pickText(view, 'title')) + '</h3></div></div><div class="page-body"><div class="metric-grid">' + summary.map((item) => {
          if (item.card.widget === 'json') {
            return '<article class="metric-card"><span class="meta">' + escapeHTML(pickText(item.card, 'label')) + '</span><pre>' + escapeHTML(JSON.stringify(item.value, null, 2)) + '</pre></article>';
          }
          if (item.card.widget === 'table' && Array.isArray(item.value)) {
            return '<article class="metric-card" data-action="' + (item.card.action_key || '') + '"><span class="meta">' + escapeHTML(pickText(item.card, 'label')) + '</span><pre>' + escapeHTML(JSON.stringify(item.value, null, 2)) + '</pre></article>';
          }
          return '<article class="metric-card" data-action="' + (item.card.action_key || '') + '"><span class="meta">' + escapeHTML(pickText(item.card, 'label')) + '</span><strong>' + escapeHTML(displayValue(item.value)) + '</strong></article>';
        }).join('') + '</div></div></section>';
        root.querySelectorAll('[data-action]').forEach((card) => {
          if (!card.dataset.action) return;
          card.addEventListener('click', () => {
            const action = state.bootstrap.actions.find((item) => item.key === card.dataset.action);
            if (action) window.location.hash = '#' + action.route_path;
          });
        });
        return;
      }
      renderJSONCard(pickText(view, 'title'), route);
    }

    async function invokeDocumentAction(documentID, action, expectedVersion, expectedETag) {
      const csrf = readCookie('orbyte_csrf');
      return api('/documents/' + encodeURIComponent(documentID) + '/actions', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
        body: JSON.stringify({action, expected_version: expectedVersion, expected_etag: expectedETag})
      });
    }

    async function renderCustom(route) {
      const root = document.getElementById('view-root');
      root.innerHTML = '<section class="panel"><h3>' + escapeHTML(t('custom_loading')) + '</h3></section>';
      const entry = route.custom_entry;
      const bundle = await loadBundle(entry.bundle_key);
      const renderFn = bundle[entry.component_export];
      if (typeof renderFn !== 'function') throw new Error('module component export not found');
      const mount = document.getElementById('view-root');
      mount.innerHTML = '';
      await renderFn({
        mount,
        route,
        api,
        params: Object.fromEntries(currentParams().entries()),
        t,
        locale: state.locale
      });
    }

    async function renderRoute() {
      if (!state.bootstrap) {
        applyShellLayout(false);
        return;
      }
      renderMenus();
      const path = currentPath() || state.bootstrap.default_path;
      if (!path) {
        setStatus(t('no_routes'));
        document.getElementById('view-root').innerHTML = '';
        return;
      }
      const route = await api('/ui/routes/resolve?path=' + encodeURIComponent(path));
      state.route = route;
      document.getElementById('route-title').innerHTML = '<h2>' + escapeHTML(pickText(route.action, 'label') || route.path) + '</h2>';
      setStatus(t('resolved_from_module') + ' ' + route.module_key + ' ' + t('using_rendering') + ' ' + route.render_mode + ' rendering.');
      if (route.render_mode === 'custom') {
        await renderCustom(route);
        return;
      }
      await renderGeneric(route);
      renderMenus();
    }

    function readCookie(name) {
      return document.cookie.split(';').map((item) => item.trim()).find((item) => item.startsWith(name + '='))?.slice(name.length + 1) || '';
    }

    function resolvePath(payload, path) {
      if (!path) return '';
      return path.split('.').reduce((current, key) => current && current[key] != null ? current[key] : '', payload);
    }

    function assignPath(target, path, value) {
      const parts = path.split('.');
      let current = target;
      while (parts.length > 1) {
        const key = parts.shift();
        current[key] = current[key] || {};
        current = current[key];
      }
      current[parts[0]] = value;
    }

    function readFieldValue(input) {
      if (input.type === 'checkbox') return !!input.checked;
      return input.value;
    }

    function renderFieldInput(field, value) {
      const readonly = field.read_only ? ' readonly disabled' : '';
      const current = value == null ? '' : value;
      if (field.widget === 'textarea') {
        return '<textarea data-path="' + field.path + '"' + readonly + ' placeholder="' + escapeHTML(pickText(field, 'placeholder')) + '">' + escapeHTML(String(current)) + '</textarea>';
      }
      if (field.widget === 'select' || (field.options || []).length > 0) {
        const options = (field.options || []).map((option) => '<option value="' + option + '"' + (String(current) === option ? ' selected' : '') + '>' + escapeHTML(displayValue(option)) + '</option>').join('');
        return '<select data-path="' + field.path + '"' + readonly + '>' + options + '</select>';
      }
      if (field.type === 'bool') {
        return '<input type="checkbox" data-path="' + field.path + '"' + (current ? ' checked' : '') + readonly + '>';
      }
      if (field.type === 'int' || field.type === 'number') {
        return '<input type="number" data-path="' + field.path + '" value="' + escapeHTML(String(current)) + '"' + readonly + ' placeholder="' + escapeHTML(pickText(field, 'placeholder')) + '">';
      }
      return '<input data-path="' + field.path + '" value="' + escapeHTML(String(current)) + '"' + readonly + ' placeholder="' + escapeHTML(pickText(field, 'placeholder')) + '">';
    }

    function renderRelatedView(def, payload, view) {
      const items = payload[def.source] || [];
      const relatedDef = payload.related_definitions ? payload.related_definitions[def.source] : null;
      const relation = (payload.definition && payload.definition.relations || []).find((item) => item.key === def.source);
      const createForm = relatedDef && relation ? renderRelatedCreateForm(def.source, relatedDef, relation) : '';
      const content = items.length ? '<div class="list">' + items.map((item) => {
        if (typeof item !== 'object' || item == null) {
          return '<article class="detail-item"><strong>' + escapeHTML(String(item)) + '</strong></article>';
        }
        const values = (item.record && item.record.values) || item.values || item;
        const entries = Object.keys(values).sort().slice(0, 6).map((key) => '<div><span class="meta">' + key + '</span><strong>' + escapeHTML(displayValue(values[key])) + '</strong></div>').join('');
        return '<article class="detail-item"><div class="kv">' + entries + '</div></article>';
      }).join('') + '</div>' : '<p class="status">' + escapeHTML(pickText(def, 'empty_state') || t('no_related_items')) + '</p>';
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(pickText(def, 'title')) + '</h3></div><div class="section-body">' + content + createForm + '</div></section>';
    }

    function renderSection(section, record) {
      const fields = (section.fields || []).map((field) => {
        return '<article class="detail-item"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span><strong>' + escapeHTML(displayValue(resolvePath(record, field.path))) + '</strong></article>';
      }).join('');
      const extensionModule = section.extension_slot_key || '';
      let extensionFields = '';
      if (extensionModule && record.body && record.body.payload && record.body.payload.extensions && record.body.payload.extensions[extensionModule]) {
        const ext = record.body.payload.extensions[extensionModule];
        extensionFields = Object.keys(ext).sort().map((key) => {
          return '<article class="detail-item"><span class="meta">' + extensionModule + '.' + key + '</span><strong>' + escapeHTML(displayValue(ext[key])) + '</strong></article>';
        }).join('');
      }
      return '<section class="section-block"><div class="section-head"><h4>' + escapeHTML(pickText(section, 'title')) + '</h4></div><div class="section-body"><div class="detail-grid">' + fields + extensionFields + '</div></div></section>';
    }

    function renderModelSection(section, record) {
      const fields = (section.fields || []).map((field) => {
        return '<article class="detail-item"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span><strong>' + escapeHTML(displayValue(resolvePath(record, field.path))) + '</strong></article>';
      }).join('');
      return '<section class="section-block"><div class="section-head"><h4>' + escapeHTML(pickText(section, 'title')) + '</h4></div><div class="section-body"><div class="detail-grid">' + fields + '</div></div></section>';
    }

    function renderEditableField(field, record) {
      const value = resolvePath(record, field.path);
      const helpText = pickText(field, 'help_text');
      return '<label class="form-field' + (((field.widget === 'textarea') || (field.type === 'json') || (field.type === 'text')) ? ' wide' : '') + '"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput(field, value) + (helpText ? '<span class="status">' + escapeHTML(helpText) + '</span>' : '') + '</label>';
    }

    function renderEditableModelField(field, record) {
      const value = resolvePath(record, field.path);
      const helpText = pickText(field, 'help_text');
      return '<label class="form-field' + (((field.widget === 'textarea') || (field.type === 'json') || (field.type === 'text')) ? ' wide' : '') + '"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput(field, value) + (helpText ? '<span class="status">' + escapeHTML(helpText) + '</span>' : '') + '</label>';
    }

    function renderFormSection(section, record) {
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(pickText(section, 'title')) + '</h3></div><div class="section-body"><div class="form-grid">' + (section.fields || []).map((field) => renderEditableField(field, record)).join('') + '</div></div></section>';
    }

    function renderModelFormSection(section, record) {
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(pickText(section, 'title')) + '</h3></div><div class="section-body"><div class="form-grid">' + (section.fields || []).map((field) => renderEditableModelField(field, record)).join('') + '</div></div></section>';
    }

    function renderRelationEditor(def, payload) {
      const relatedDef = payload.related_definitions ? payload.related_definitions[def.source] : null;
      const relation = (payload.definition && payload.definition.relations || []).find((item) => item.key === def.source);
      if (!relatedDef || !relation) return '';
      const rows = (payload[def.source] || []).map((item) => renderRelationRow(def.source, relatedDef, relation, item, payload.model_definitions || {})).join('');
      return '<section class="section-block" data-relation-editor="' + def.source + '" data-parent-model-key="' + escapeHTML(payload.definition.key || '') + '" data-target-model-key="' + escapeHTML(relatedDef.key || '') + '"><div class="section-head"><h3>' + escapeHTML(pickText(def, 'title')) + '</h3></div><div class="section-body"><div class="list" data-relation-list="' + def.source + '">' + (rows || '<p class="status">' + t('no_related_items_yet') + '</p>') + '</div><div class="actions"><button type="button" class="secondary" data-relation-add="' + def.source + '">' + t('add_row') + '</button></div></div></section>';
    }

    function deriveRelatedViews(definition) {
      const relations = definition && definition.relations ? definition.relations : [];
      return relations.map((relation) => ({
        key: relation.key,
        title: relation.key.replace(/_/g, ' '),
        source: relation.key,
        empty_state: t('no_related_items_yet')
      }));
    }

    function renderRelationRow(relationKey, relatedDef, relation, item, modelDefinitions) {
      const graphNode = item && item.record ? item : null;
      const record = graphNode ? graphNode.record : (item || {id: '', version: 0, values: {}});
      const values = record.values || {};
      const fields = (relatedDef.fields || []).filter((field) => field.key !== relation.foreign_key && !field.read_only).map((field) => {
        const enriched = {path: 'values.' + field.key, type: field.type, widget: field.widget, options: field.options || [], placeholder: pickText(field, 'placeholder') || '', help_text: pickText(field, 'help_text') || ''};
        return '<label class="form-field"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput(enriched, values[field.key]) + '</label>';
      }).join('');
      const nested = renderNestedRelationEditors(graphNode, relatedDef, modelDefinitions);
      return '<article class="detail-item" data-relation-row="' + relationKey + '" data-record-id="' + escapeHTML(record.id || '') + '" data-record-version="' + escapeHTML(String(record.version || 0)) + '" data-record-op="upsert"><div class="form-grid">' + fields + '</div>' + nested + '<div class="actions"><button type="button" class="secondary" data-relation-remove="' + relationKey + '">' + t('remove') + '</button></div></article>';
    }

    function renderNestedRelationEditors(graphNode, relatedDef, modelDefinitions) {
      const nestedRelations = relatedDef.relations || [];
      if (!nestedRelations.length) return '';
      const relatedMap = graphNode && graphNode.related ? graphNode.related : {};
      return nestedRelations.map((relation) => {
        const targetDef = modelDefinitions[relation.target_model_key];
        if (!targetDef) return '';
        const rows = (relatedMap[relation.key] || []).map((item) => renderRelationRow(relation.key, targetDef, relation, item, modelDefinitions)).join('');
        return '<section class="section-block" data-relation-editor="' + relation.key + '" data-parent-model-key="' + escapeHTML(relatedDef.key || '') + '" data-target-model-key="' + escapeHTML(targetDef.key || '') + '"><div class="section-head"><h4>' + relation.key.replace(/_/g, ' ') + '</h4></div><div class="section-body"><div class="list" data-relation-list="' + relation.key + '">' + (rows || '<p class="status">' + t('no_related_items_yet') + '</p>') + '</div><div class="actions"><button type="button" class="secondary" data-relation-add="' + relation.key + '">' + t('add_row') + '</button></div></div></section>';
      }).join('');
    }

    function appendRelationRow(relationKey, payload) {
      const editor = document.querySelector('[data-relation-editor="' + relationKey + '"]');
      const list = editor && editor.querySelector('[data-relation-list="' + relationKey + '"]');
      if (!editor || !list) return;
      const modelDefinitions = payload.model_definitions || {};
      const relatedDef = resolveRelatedDefinition(editor, relationKey, payload);
      const relation = resolveRelationDefinition(editor, relationKey, payload);
      if (!relatedDef || !relation) return;
      if (list.querySelector('.status')) list.innerHTML = '';
      list.insertAdjacentHTML('beforeend', renderRelationRow(relationKey, relatedDef, relation, null, modelDefinitions));
      bindRelationRemove(editor);
    }

    function resolveRelatedDefinition(editor, relationKey, payload) {
      const modelDefinitions = payload.model_definitions || {};
      const targetModelKey = editor && editor.dataset ? editor.dataset.targetModelKey : '';
      if (targetModelKey && modelDefinitions[targetModelKey]) return modelDefinitions[targetModelKey];
      if (payload.related_definitions && payload.related_definitions[relationKey]) return payload.related_definitions[relationKey];
      return modelDefinitions[relationKey] || null;
    }

    function resolveRelationDefinition(editor, relationKey, payload) {
      const modelDefinitions = payload.model_definitions || {};
      const parentModelKey = editor && editor.dataset ? editor.dataset.parentModelKey : '';
      const parentDefinition = (parentModelKey && modelDefinitions[parentModelKey]) || payload.definition || null;
      return findRelationDefinition(parentDefinition, relationKey, modelDefinitions);
    }

    function findRelationDefinition(definition, relationKey, modelDefinitions) {
      if (!definition) return null;
      const direct = (definition.relations || []).find((item) => item.key === relationKey);
      if (direct) return direct;
      for (const relation of (definition.relations || [])) {
        const nestedDef = modelDefinitions[relation.target_model_key];
        const nested = findRelationDefinition(nestedDef, relationKey, modelDefinitions);
        if (nested) return nested;
      }
      return null;
    }

    function directRelationEditors(root) {
      return Array.from(root.children || []).filter((child) => child.matches && child.matches('[data-relation-editor]'));
    }

    function bindRelationRemove(root) {
      root.querySelectorAll('[data-relation-remove]').forEach((button) => {
        button.onclick = () => {
          const row = button.closest('[data-relation-row]');
          if (row && row.dataset.recordId) {
            row.dataset.recordOp = 'delete';
            row.style.display = 'none';
          } else if (row) {
            row.remove();
          }
          const editor = button.closest('[data-relation-editor]');
          const relationKey = editor ? editor.dataset.relationEditor : '';
          const list = editor && editor.querySelector('[data-relation-list="' + relationKey + '"]');
          if (list && !Array.from(list.querySelectorAll('[data-relation-row]')).some((item) => item.dataset.recordOp !== 'delete')) {
            list.innerHTML = '<p class="status">' + t('no_related_items_yet') + '</p>';
          }
        };
      });
    }

    function collectRelationMutations(root) {
      const relations = {};
      directRelationEditors(root).forEach((editor) => {
        const relationKey = editor.dataset.relationEditor;
        const rows = [];
        Array.from(editor.querySelectorAll(':scope > [data-relation-list] > [data-relation-row]')).forEach((row) => {
          const op = row.dataset.recordOp || 'upsert';
          const values = {};
          row.querySelectorAll(':scope > label [data-path], :scope > .card > label [data-path]').forEach((input) => assignPath(values, input.dataset.path.replace(/^values\\./, ''), readFieldValue(input)));
          const nested = collectRelationMutations(row);
          rows.push({
            operation: op,
            id: row.dataset.recordId || '',
            expected_version: parseInt(row.dataset.recordVersion || '0', 10) || 0,
            values: values,
            relations: nested
          });
        });
        if (rows.length > 0 || editor.querySelector('[data-relation-list]')) {
          relations[relationKey] = rows;
        }
      });
      return relations;
    }

    function renderRelatedCreateForm(sourceKey, relatedDef, relation) {
      const editableFields = (relatedDef.fields || []).filter((field) => field.key !== relation.foreign_key && !field.read_only);
      if (!editableFields.length) return '';
      return '<section class="section-block"><div class="section-head"><h3>' + escapeHTML(t('add')) + ' ' + escapeHTML(pickText(relatedDef, 'display_name') || relatedDef.key) + '</h3></div><div class="section-body"><div class="form-grid">' +
        editableFields.map((field) => '<label class="form-field"><span class="meta">' + escapeHTML(pickText(field, 'label')) + '</span>' + renderFieldInput({path: 'values.' + field.key, type: field.type, widget: field.widget, options: field.options || [], placeholder: pickText(field, 'placeholder') || ''}, '') + '</label>').join('') +
        '</div><p class="status" data-related-status="' + sourceKey + '"></p><div class="actions"><button type="button" data-related-save="' + sourceKey + '">' + t('add') + '</button></div></div></section>';
    }

    function renderActionZones(view) {
      const placements = view.action_placements || [];
      const zones = {};
      placements.forEach((placement) => { zones[placement.zone] = true; });
      if (!zones.primary) zones.primary = true;
      if (!zones.secondary) zones.secondary = true;
      return Object.keys(zones).map((zone) => '<div class="actions" data-zone="' + zone + '"></div>').join('');
    }

    function resolveActionPlacement(view, actionKey, policyDecision) {
      const fromPolicy = policyDecision && policyDecision.output && policyDecision.output.placement;
      if (fromPolicy) return fromPolicy;
      const placement = (view.action_placements || []).find((item) => item.action_key === actionKey);
      return placement && placement.zone ? placement.zone : 'secondary';
    }

    async function bootstrap() {
      state.locale = detectPreferredLocale();
      try {
        const localePayload = await api('/locale');
        state.supportedLocales = localePayload.supported_locales || defaultSupportedLocales;
        state.locale = normalizeLocale(localePayload.locale || state.locale);
      } catch (_) {}
      renderLocaleSwitcher();
      applyLocale();
      refreshOfflineStatus();
      await registerServiceWorker();
      await refreshSyncStats();
      try {
        state.authOptions = await api('/auth/options');
        state.bootstrap = await api('/ui/bootstrap');
        state.supportedLocales = state.bootstrap.supported_locales || defaultSupportedLocales;
        if (state.bootstrap.locale) {
          state.locale = normalizeLocale(state.bootstrap.locale);
        }
        renderLocaleSwitcher();
        applyLocale();
        await loadOfflineBootstrap();
        await processSyncQueue();
        if (!window.location.hash && state.bootstrap.default_path) {
          window.location.hash = '#' + state.bootstrap.default_path;
        }
        await renderRoute();
      } catch (err) {
        await loadOfflineBootstrap();
        if (err.message === 'authentication required' || err.message === 'session not found' || err.message === 'session not active' || err.message === 'session revoked' || err.message === 'session expired') {
          renderLocaleSwitcher();
          applyLocale();
          renderLogin('');
          return;
        }
        document.getElementById('view-root').innerHTML = '<section class="panel"><h3>' + escapeHTML(t('ui_bootstrap_failed')) + '</h3><p class="status">' + escapeHTML(err.message) + '</p></section>';
        setStatus(t('ui_bootstrap_failed_status'));
      }
    }

    window.addEventListener('online', () => { refreshOfflineStatus(); void processSyncQueue(); });
    window.addEventListener('offline', () => { refreshOfflineStatus(); });
    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) void processSyncQueue();
    });
    window.addEventListener('hashchange', () => { void renderRoute(); });
    document.getElementById('logout-button').addEventListener('click', () => { void performLogout(); });
    void bootstrap();
  </script>
</body>
</html>`

const uiServiceWorkerJS = `const CACHE_NAME = 'orbyte-ui-shell-v1';
const PRECACHE_URLS = ['/ui', '/ui/manifest.webmanifest', '/ui/assets/platform.css?v=` + platformAssetVersion + `'];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE_URLS)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (event) => {
  event.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key)))).then(() => self.clients.claim()));
});

function shouldCache(requestURL) {
  return requestURL.pathname === '/ui' ||
    requestURL.pathname === '/ui/assets/platform.css' ||
    requestURL.pathname === '/auth/options' ||
    requestURL.pathname === '/ui/bootstrap' ||
    requestURL.pathname.indexOf('/ui/routes/resolve') === 0 ||
    requestURL.pathname.indexOf('/ui/views/') === 0 ||
    requestURL.pathname.indexOf('/ui/assets/modules/') === 0;
}

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  const requestURL = new URL(event.request.url);
  if (!shouldCache(requestURL)) return;
  event.respondWith(
    fetch(event.request)
      .then((response) => {
        if (response && response.ok) {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
        }
        return response;
      })
      .catch(() => caches.match(event.request).then((cached) => cached || caches.match('/ui')))
  );
});`

func AnalyticsCockpitBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["analytics-cockpit"] = {
    render: async function(ctx) {
      const payload = await ctx.api('/ui/data/analytics/snapshot');
      const text = function(en, id) { return ctx.locale === "id" ? id : en; };
      const escapeHTML = function(value) {
        return String(value == null ? '' : value).replace(/[&<>"]/g, function(char) {
          return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[char];
        });
      };
      const formatInt = function(value) {
        return new Intl.NumberFormat(ctx.locale === "id" ? "id-ID" : "en-US").format(Number(value || 0));
      };
      const formatPercent = function(value) {
        return (Number(value || 0) * 100).toFixed(1) + '%';
      };
      const totalDocuments = (payload.documents.created || 0) + (payload.documents.draft || 0) + (payload.documents.submitted || 0) + (payload.documents.approved || 0) + (payload.documents.rejected || 0) + (payload.documents.cancelled || 0);
      const renderRows = function(entries, emptyLabel) {
        if (!entries.length) {
          return '<tr><td colspan="3" class="status">' + escapeHTML(emptyLabel) + '</td></tr>';
        }
        return entries.map(function(entry) {
          return '<tr><td><div class="row-primary">' + escapeHTML(entry.label) + '</div></td><td>' + escapeHTML(entry.primary) + '</td><td>' + escapeHTML(entry.secondary) + '</td></tr>';
        }).join('');
      };
      const typeRows = Object.keys((payload.segments && payload.segments.by_document_type) || {}).sort().map(function(key) {
        const item = payload.segments.by_document_type[key] || {};
        return {
          label: key,
          primary: formatInt((item.submitted || 0) + (item.approved || 0)),
          secondary: formatInt(item.draft || 0)
        };
      });
      const locationRows = Object.keys((payload.segments && payload.segments.by_location) || {}).sort().map(function(key) {
        const item = payload.segments.by_location[key] || {};
        return {
          label: key || text('Unassigned', 'Tanpa Lokasi'),
          primary: formatInt((item.approved || 0) + (item.submitted || 0)),
          secondary: formatInt(item.rejected || 0)
        };
      });
      const metrics = Object.keys(payload.metrics || {}).sort(function(a, b) {
        return (payload.metrics[b] || 0) - (payload.metrics[a] || 0);
      }).slice(0, 8).map(function(key) {
        return {label: key, value: payload.metrics[key]};
      });
      const metricsRows = metrics.length ? metrics.map(function(item) {
        return '<tr><td><div class="row-primary">' + escapeHTML(item.label) + '</div></td><td>' + escapeHTML(formatInt(item.value)) + '</td><td></td></tr>';
      }).join('') : '<tr><td colspan="3" class="status">' + text('No metrics captured yet.', 'Belum ada metrik yang terekam.') + '</td></tr>';
      ctx.mount.innerHTML = ''
        + '<section class="page-panel">'
        +   '<div class="page-header">'
        +     '<div>'
        +       '<h3>' + text('Analytics Cockpit', 'Kokpit Analitik') + '</h3>'
        +       '<p class="status">' + text('Operational analytics overview for documents, workflow, and reliability.', 'Ringkasan analitik operasional untuk dokumen, workflow, dan reliabilitas.') + '</p>'
        +     '</div>'
        +     '<div class="actions">'
        +       '<button type="button" class="secondary" data-nav="#/documents">' + text('Open Requests', 'Buka Permintaan') + '</button>'
        +       '<button type="button" class="secondary" data-nav="#/monitoring">' + text('Open Monitoring', 'Buka Monitoring') + '</button>'
        +     '</div>'
        +   '</div>'
        +   '<div class="page-body">'
        +     '<div class="metric-grid">'
        +       '<article class="metric-card"><span class="meta">' + text('Documents', 'Dokumen') + '</span><strong>' + formatInt(totalDocuments) + '</strong></article>'
        +       '<article class="metric-card"><span class="meta">' + text('Pending Approvals', 'Persetujuan Tertunda') + '</span><strong>' + formatInt(payload.workflow.pending_approvals) + '</strong></article>'
        +       '<article class="metric-card"><span class="meta">' + text('Approval Rate', 'Tingkat Persetujuan') + '</span><strong>' + formatPercent(payload.workflow.approval_rate) + '</strong></article>'
        +       '<article class="metric-card"><span class="meta">' + text('Dead Letter Rate', 'Tingkat Dead Letter') + '</span><strong>' + formatPercent(payload.reliability.dead_letter_rate) + '</strong></article>'
        +     '</div>'
        +     '<div class="admin-shell-grid">'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Document Flow', 'Arus Dokumen') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Draft', 'Draf') + '</span><strong>' + formatInt(payload.documents.draft) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Submitted', 'Diajukan') + '</span><strong>' + formatInt(payload.documents.submitted) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Approved', 'Disetujui') + '</span><strong>' + formatInt(payload.documents.approved) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Rejected', 'Ditolak') + '</span><strong>' + formatInt(payload.documents.rejected) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Cancelled', 'Dibatalkan') + '</span><strong>' + formatInt(payload.documents.cancelled) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Created', 'Dibuat') + '</span><strong>' + formatInt(payload.documents.created) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Workflow Health', 'Kesehatan Workflow') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Open Tasks', 'Tugas Terbuka') + '</span><strong>' + formatInt(payload.workflow.open_tasks) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Completed Tasks', 'Tugas Selesai') + '</span><strong>' + formatInt(payload.workflow.completed_tasks) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Rejection Rate', 'Tingkat Penolakan') + '</span><strong>' + formatPercent(payload.workflow.rejection_rate) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +     '</div>'
        +     '<div class="admin-shell-grid">'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('By Document Type', 'Per Jenis Dokumen') + '</h3></div>'
        +         '<div class="section-body"><div class="table-shell"><table class="data-table"><thead><tr><th>' + text('Document Type', 'Jenis Dokumen') + '</th><th>' + text('Active Flow', 'Arus Aktif') + '</th><th>' + text('Draft', 'Draf') + '</th></tr></thead><tbody>' + renderRows(typeRows, text('No document activity yet.', 'Belum ada aktivitas dokumen.')) + '</tbody></table></div></div>'
        +       '</section>'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('By Location', 'Per Lokasi') + '</h3></div>'
        +         '<div class="section-body"><div class="table-shell"><table class="data-table"><thead><tr><th>' + text('Location', 'Lokasi') + '</th><th>' + text('Active Flow', 'Arus Aktif') + '</th><th>' + text('Rejected', 'Ditolak') + '</th></tr></thead><tbody>' + renderRows(locationRows, text('No location activity yet.', 'Belum ada aktivitas lokasi.')) + '</tbody></table></div></div>'
        +       '</section>'
        +     '</div>'
        +     '<div class="admin-shell-grid">'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Reliability', 'Reliabilitas') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Outbox Pending', 'Outbox Tertunda') + '</span><strong>' + formatInt(payload.reliability.outbox_pending) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Dead Letters', 'Dead Letter') + '</span><strong>' + formatInt(payload.reliability.outbox_dead_letters) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Dispatch Success', 'Dispatch Berhasil') + '</span><strong>' + formatInt(payload.reliability.dispatch_success) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Dispatch Retries', 'Dispatch Retry') + '</span><strong>' + formatInt(payload.reliability.dispatch_retries) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +       '<section class="stack-card">'
        +         '<div class="section-head"><h3>' + text('Coverage', 'Cakupan') + '</h3></div>'
        +         '<div class="section-body"><div class="detail-grid">'
        +           '<article class="detail-item"><span class="meta">' + text('Document Summaries', 'Ringkasan Dokumen') + '</span><strong>' + formatInt(payload.coverage.document_summaries) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Projection Coverage', 'Cakupan Proyeksi') + '</span><strong>' + formatPercent(payload.coverage.projection_coverage) + '</strong></article>'
        +           '<article class="detail-item"><span class="meta">' + text('Audit Events', 'Event Audit') + '</span><strong>' + formatInt(payload.coverage.audit_events) + '</strong></article>'
        +         '</div></div>'
        +       '</section>'
        +     '</div>'
        +     '<section class="stack-card">'
        +       '<div class="section-head"><h3>' + text('Top Metrics', 'Metrik Utama') + '</h3></div>'
        +       '<div class="section-body"><div class="table-shell"><table class="data-table"><thead><tr><th>' + text('Metric', 'Metrik') + '</th><th>' + text('Value', 'Nilai') + '</th><th></th></tr></thead><tbody>' + metricsRows + '</tbody></table></div></div>'
        +     '</section>'
        +     '<details class="stack-card"><summary class="row-primary">' + text('Raw Snapshot', 'Snapshot Mentah') + '</summary><div class="section-body"><pre></pre></div></details>'
        +   '</div>'
        + '</section>';
      ctx.mount.querySelectorAll('[data-nav]').forEach(function(node) {
        node.addEventListener('click', function() {
          window.location.hash = node.getAttribute('data-nav');
        });
      });
      ctx.mount.querySelector('pre').textContent = JSON.stringify(payload, null, 2);
    }
  };
})();`
}
