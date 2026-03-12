package httpx

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"clinic/internal/platform/activity"
	"clinic/internal/platform/analytics"
	"clinic/internal/platform/document"
	"clinic/internal/platform/identity"
	"clinic/internal/platform/model"
	"clinic/internal/platform/module"
	"clinic/internal/platform/monitoring"
	"clinic/internal/platform/policy"
	"clinic/internal/platform/reporting"
	"clinic/internal/platform/search"
	"clinic/internal/platform/shared"
)

func registerUIRoutes(mux *http.ServeMux, ident *identity.Service, modules *module.Service, models *model.Service, activities *activity.Service, reportingSvc *reporting.Service, docs *document.Service, searchSvc *search.Service, analyticsSvc *analytics.Service, monitoringSvc *monitoring.Service, policySvc *policy.Service) {
	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireInteractivePrincipal(w, r); !ok {
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(uiShellHTML))
	})

	mux.HandleFunc("GET /ui/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		menus, actions, views := visibleUIContracts(ident, modules, p)
		defaultPath := ""
		if len(menus) > 0 {
			for _, action := range actions {
				if action.Key == menus[0].ActionKey {
					defaultPath = action.RoutePath
					break
				}
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"menus":        menus,
			"actions":      actions,
			"views":        views,
			"default_path": defaultPath,
		})
	})

	mux.HandleFunc("GET /ui/menus", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		menus, _, _ := visibleUIContracts(ident, modules, p)
		respondJSON(w, http.StatusOK, map[string]any{"items": menus})
	})

	mux.HandleFunc("GET /ui/actions", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		_, actions, _ := visibleUIContracts(ident, modules, p)
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
		_, _, views := visibleUIContracts(ident, modules, p)
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
		resolution, ok := modules.ResolveRoute(path)
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
		respondJSON(w, http.StatusOK, map[string]any{
			"record":       docs.Render(record, document.ViewExpanded, modules.EnabledMap()),
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
		items, total, err := models.List(modelKey, model.Query{
			Filters: map[string]string{
				"name":   strings.TrimSpace(r.URL.Query().Get("name")),
				"status": strings.TrimSpace(r.URL.Query().Get("status")),
			},
			SortKey:  strings.TrimSpace(r.URL.Query().Get("sort")),
			Page:     intQuery(r, "page", 1),
			PageSize: intQuery(r, "page_size", 20),
		})
		if err != nil {
			respondError(w, err)
			return
		}
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
		payload := map[string]any{"record": record, "definition": def, "timeline": activities.Timeline("model:"+modelKey, recordID), "model_definitions": allModelDefinitions(models)}
		relatedDefs := map[string]model.Definition{}
		for _, relation := range def.Relations {
			items, _, err := models.Related(def.Key, recordID, relation.Key, model.Query{Page: 1, PageSize: 100})
			if err == nil {
				graphItems := make([]map[string]any, 0, len(items))
				for _, item := range items {
					graphItems = append(graphItems, modelGraphNode(models, relation.TargetModelKey, item, map[string]bool{def.Key: true}))
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
		payload, err := reportingSvc.ExecuteView(datasetKey, relationQuery(r), reporting.QueryRequest{
			Dimensions: splitCSV(r.URL.Query().Get("dimensions")),
			Measures:   splitCSV(r.URL.Query().Get("measures")),
			GroupBy:    splitCSV(r.URL.Query().Get("group_by")),
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
		payload, err := reportingSvc.ExecuteAdHocModel(relationQuery(r), reporting.QueryRequest{
			ModelKey:   modelKey,
			Dimensions: splitCSV(r.URL.Query().Get("dimensions")),
			Measures:   splitCSV(r.URL.Query().Get("measures")),
			GroupBy:    splitCSV(r.URL.Query().Get("group_by")),
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
		payload, err := reportingSvc.ExecuteAdHocSource(source, relationQuery(r), reporting.QueryRequest{
			ModelKey:   strings.TrimSpace(r.URL.Query().Get("model_key")),
			Dimensions: splitCSV(r.URL.Query().Get("dimensions")),
			Measures:   splitCSV(r.URL.Query().Get("measures")),
			GroupBy:    splitCSV(r.URL.Query().Get("group_by")),
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

func visibleUIContracts(ident *identity.Service, modules *module.Service, p principal) ([]module.MenuDefinition, []module.ActionDefinition, []module.ViewDefinition) {
	allowedMenus := make([]module.MenuDefinition, 0)
	allowedActions := make([]module.ActionDefinition, 0)
	allowedViews := make([]module.ViewDefinition, 0)
	actionKeys := map[string]bool{}
	viewKeys := map[string]bool{}

	for _, detail := range modules.List() {
		if !detail.Installed.Enabled {
			continue
		}
		for _, action := range detail.Manifest.Frontend.Actions {
			if !principalAllowsAll(ident, p, action.RequiredPermissions) {
				continue
			}
			switch action.RenderMode {
			case module.RenderModeGeneric:
				if action.ViewKey != "" {
					view, ok := modules.View(action.ViewKey)
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
					if entry.Key == action.CustomEntryKey && principalAllowsAll(ident, p, entry.RequiredPermissions) {
						entryAllowed = true
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

	return allowedMenus, allowedActions, allowedViews
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
  <title>Clinic UI</title>
  <style>
    :root { --bg:#f3efe7; --panel:#fffdf8; --ink:#16221b; --muted:#5f6c62; --line:#d8d0c2; --accent:#1f6f5f; --accent-soft:#d7ece5; --warn:#8f2d1f; }
    * { box-sizing:border-box; }
    body { margin:0; font-family: "Iowan Old Style", Georgia, serif; background: radial-gradient(circle at top, #fff9ee, #efe8da 55%, #e6dfd2); color:var(--ink); }
    .shell { min-height:100vh; display:grid; grid-template-columns:280px 1fr; }
    .sidebar { border-right:1px solid var(--line); padding:24px 18px; background:rgba(255,253,248,.86); backdrop-filter: blur(8px); }
    .brand { font-size:26px; margin:0 0 6px; }
    .subtitle { color:var(--muted); margin:0 0 22px; font-size:14px; }
    .menu-list { display:grid; gap:10px; }
    .menu-link { display:block; padding:12px 14px; border:1px solid var(--line); border-radius:14px; color:inherit; text-decoration:none; background:#fffdf8; }
    .menu-link.active { border-color:var(--accent); background:var(--accent-soft); }
    .content { padding:28px; display:grid; gap:18px; }
    .panel { background:var(--panel); border:1px solid var(--line); border-radius:18px; padding:20px; box-shadow:0 12px 32px rgba(31,42,33,.06); }
    .panel h2, .panel h3 { margin-top:0; }
    .status { color:var(--muted); font-size:14px; }
    .list { display:grid; gap:12px; }
    .card { border:1px solid var(--line); border-radius:14px; padding:14px; background:#fffefa; }
    .meta { color:var(--muted); font-size:13px; }
    button { border:0; border-radius:999px; background:var(--accent); color:#fff; padding:10px 16px; cursor:pointer; font:inherit; }
    button.secondary { background:#e5ece8; color:var(--ink); }
    button.warn { background:var(--warn); }
    .actions { display:flex; gap:10px; flex-wrap:wrap; margin-top:12px; }
    pre { margin:0; white-space:pre-wrap; word-break:break-word; background:#f6f1e7; padding:14px; border-radius:12px; border:1px solid var(--line); }
    .kv { display:grid; grid-template-columns:repeat(auto-fit, minmax(180px, 1fr)); gap:12px; }
    .kv .card strong { display:block; font-size:24px; margin-top:4px; }
    @media (max-width: 900px) {
      .shell { grid-template-columns:1fr; }
      .sidebar { border-right:0; border-bottom:1px solid var(--line); }
    }
  </style>
</head>
<body>
  <div class="shell">
    <aside class="sidebar">
      <h1 class="brand">Clinic UI</h1>
      <p class="subtitle">Manifest-driven shell with generic pages and module custom entries.</p>
      <nav id="menu" class="menu-list"></nav>
    </aside>
    <main class="content">
      <div class="panel">
        <div id="route-title"><h2>Loading…</h2></div>
        <p class="status" id="route-status">Resolving module UI registry.</p>
      </div>
      <div id="view-root"></div>
    </main>
  </div>
  <script>
    const state = { bootstrap: null, route: null, bundles: {} };

    async function api(path, options) {
      const response = await fetch(path, Object.assign({credentials: 'same-origin'}, options || {}));
      if (!response.ok) {
        let message = response.statusText;
        try {
          const payload = await response.json();
          message = payload.error && payload.error.message ? payload.error.message : message;
        } catch (_) {}
        throw new Error(message);
      }
      const contentType = response.headers.get('content-type') || '';
      if (contentType.includes('application/json')) return response.json();
      return response.text();
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

    function renderMenus() {
      const container = document.getElementById('menu');
      container.innerHTML = '';
      for (const menu of state.bootstrap.menus) {
        const action = state.bootstrap.actions.find((item) => item.key === menu.action_key);
        if (!action) continue;
        const link = document.createElement('a');
        link.className = 'menu-link' + (currentPath() === action.route_path ? ' active' : '');
        link.href = '#' + action.route_path;
        link.textContent = menu.label;
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
      root.innerHTML = '<section class="panel"><h3>' + title + '</h3><pre></pre></section>';
      root.querySelector('pre').textContent = JSON.stringify(payload, null, 2);
    }

    async function renderGeneric(route) {
      const root = document.getElementById('view-root');
      const view = route.view;
      if (!view) {
        renderJSONCard('View unavailable', route);
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
        const payload = await api(listPath + query.toString());
        const pagedItems = payload.items || [];
        const newRoute = view.model_key ? routeForModel(view.model_key, 'form') : routeForDocument(view.document_type, 'form');
        const filterBar = '<div class="actions">' +
          ((view.filters || []).map((filter) => {
            if (filter.type !== 'enum') return '';
            const options = ['<option value="">All ' + filter.label + '</option>'].concat((filter.options || []).map((option) => '<option value="' + option + '"' + (params.get(filter.key) === option ? ' selected' : '') + '>' + option + '</option>'));
            return '<label class="card"><span class="meta">' + filter.label + '</span><select data-filter="' + filter.key + '">' + options.join('') + '</select></label>';
          }).join('')) +
          (view.model_key ? '<label class="card"><span class="meta">Search</span><input data-filter="name" value="' + escapeHTML(params.get('name') || '') + '" placeholder="Search"></label>' : '') +
          '<label class="card"><span class="meta">Sort</span><select data-filter="sort"><option value="">Document</option><option value="updated_at"' + (params.get('sort') === 'updated_at' ? ' selected' : '') + '>Updated</option><option value="status"' + (params.get('sort') === 'status' ? ' selected' : '') + '>Status</option><option value="name"' + (params.get('sort') === 'name' ? ' selected' : '') + '>Name</option></select></label>' +
          (newRoute ? '<button type="button" data-new="1">New</button>' : '') +
          '</div>';
        const rows = pagedItems.map((item) => {
          const cells = (view.columns || []).map((column) => {
            return '<div><span class="meta">' + column.label + '</span><strong>' + resolvePath(item, column.path) + '</strong></div>';
          }).join('');
          const openID = item.id || (item.header && item.header.id) || '';
          return '<article class="card"><div class="kv">' + (cells || ('<div><span class="meta">Record</span><strong>' + openID + '</strong></div>')) + '</div><div class="actions"><button data-open="' + openID + '">Open</button></div></article>';
        }).join('');
        const total = payload.total || pagedItems.length;
        const pagination = '<div class="actions"><button class="secondary" data-page="' + Math.max(1, page - 1) + '"' + (page <= 1 ? ' disabled' : '') + '>Previous</button><span class="status">Page ' + page + ' / ' + Math.max(1, Math.ceil(total / pageSize)) + '</span><button class="secondary" data-page="' + (page + 1) + '"' + (page * pageSize >= total ? ' disabled' : '') + '>Next</button></div>';
        root.innerHTML = '<section class="panel"><h3>' + view.title + '</h3><p class="status">' + (view.empty_state || 'Standard list page rendered from the module manifest.') + '</p>' + filterBar + '<div class="list">' + (rows || '<p class="status">No records yet.</p>') + '</div>' + pagination + '</section>';
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
          root.innerHTML = '<section class="panel"><h3>' + view.title + '</h3><p class="status">Select a record from the list to inspect its canonical document.</p></section>';
          return;
        }
        if (view.model_key) {
          const payload = await api('/ui/data/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(documentID));
          const record = payload.record;
          const tabMarkup = (view.tabs || []).map((tab) => {
            const sections = (tab.sections || []).map((section) => renderModelSection(section, record)).join('');
            return '<section class="panel"><h3>' + tab.title + '</h3>' + sections + '</section>';
          }).join('');
          const sectionMarkup = (view.sections || []).map((section) => renderModelSection(section, record)).join('');
          const relatedViews = (view.related_views || []).map((item) => renderRelatedView(item, payload, view)).join('');
          root.innerHTML = '<section class="panel"><h3>' + view.title + '</h3><p class="meta">' + record.id + ' · v' + record.version + '</p>' + (tabMarkup || sectionMarkup) + '</section>' + relatedViews;
          root.querySelectorAll('[data-related-save]').forEach((button) => {
            button.addEventListener('click', async () => {
              const sourceKey = button.dataset.relatedSave;
              const section = button.closest('section');
              const values = {};
              section.querySelectorAll('[data-path]').forEach((input) => assignPath(values, input.dataset.path.replace(/^values\\./, ''), readFieldValue(input)));
              const csrf = readCookie('clinic_csrf');
              try {
                await api('/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(record.id) + '/relations/' + encodeURIComponent(sourceKey), {
                  method: 'POST',
                  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                  body: JSON.stringify({values})
                });
                const statusNode = section.querySelector('[data-related-status="' + sourceKey + '"]');
                if (statusNode) statusNode.textContent = 'Related record created.';
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
          return '<section class="panel"><h3>' + tab.title + '</h3>' + sections + '</section>';
        }).join('');
        const sectionMarkup = (view.sections || []).map((section) => renderSection(section, record)).join('');
        const relatedViews = (view.related_views || []).map((item) => renderRelatedView(item, payload, view)).join('');
        const actionZones = renderActionZones(view);
        root.innerHTML = '<section class="panel"><h3>' + view.title + '</h3><p class="meta">' + record.header.id + ' · v' + record.header.version + ' · ' + record.header.status + '</p>' + (tabMarkup || sectionMarkup || ('<pre>' + escapeHTML(JSON.stringify(record.body.payload, null, 2)) + '</pre>')) + actionZones + '</section>' + relatedViews;
        for (const actionKey of view.allowed_actions || []) {
          const placement = await api('/ui/actions/render?action=' + encodeURIComponent(actionKey) + '&document_id=' + encodeURIComponent(record.header.id));
          if (!placement.allowed) {
            continue;
          }
          const button = document.createElement('button');
          button.textContent = actionKey;
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
          if (documentID) {
            payload = await api('/ui/data/models/' + encodeURIComponent(view.model_key) + '/' + encodeURIComponent(documentID));
            record = payload.record;
          } else {
            payload = await api('/ui/data/models?model=' + encodeURIComponent(view.model_key) + '&page_size=1');
          }
          const formSections = (view.sections || []).length > 0
            ? (view.sections || []).map((section) => renderModelFormSection(section, record)).join('')
            : '<div class="list">' + (view.fields || []).map((field) => renderEditableModelField(field, record)).join('') + '</div>';
          const relationViews = (view.related_views && view.related_views.length) ? view.related_views : deriveRelatedViews(payload.definition);
          const relationEditors = relationViews.map((item) => renderRelationEditor(item, payload)).join('');
          root.innerHTML = '<section class="panel"><h3>' + view.title + '</h3>' + formSections + relationEditors + '<p class="status" id="form-status"></p><div class="actions"><button id="save-form">' + (documentID ? 'Save' : 'Create') + '</button></div></section>';
          bindRelationRemove(root);
          root.querySelectorAll('[data-relation-add]').forEach((button) => {
            button.addEventListener('click', () => appendRelationRow(button.dataset.relationAdd, payload));
          });
          const button = root.querySelector('#save-form');
          if (button) {
            button.addEventListener('click', async () => {
              const values = {};
              root.querySelectorAll('[data-path]').forEach((input) => {
                if (input.closest('[data-relation-editor]')) return;
                assignPath(values, input.dataset.path.replace(/^values\\./, ''), readFieldValue(input));
              });
              const relations = collectRelationMutations(root);
              const csrf = readCookie('clinic_csrf');
              try {
                const created = await api('/models/' + encodeURIComponent(view.model_key) + (documentID ? '/' + encodeURIComponent(documentID) : ''), {
                  method: documentID ? 'PUT' : 'POST',
                  headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                  body: JSON.stringify(documentID ? {values, expected_version: record.version, relations} : {values, relations})
                });
                document.getElementById('form-status').textContent = documentID ? 'Record updated.' : 'Record created.';
                setStatus(documentID ? 'Record updated.' : 'Record created.');
                if (!documentID) {
                  const detailRoute = routeForModel(view.model_key, 'detail');
                  const createdRecord = created && (created.record || created);
                  if (detailRoute && createdRecord && createdRecord.id) {
                    window.location.hash = '#' + detailRoute + '?id=' + encodeURIComponent(createdRecord.id);
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
        const record = await api('/documents/' + encodeURIComponent(documentID) + '?view=expanded');
        const formSections = (view.sections || []).length > 0
          ? (view.sections || []).map((section) => renderFormSection(section, record)).join('')
          : '<div class="list">' + (view.fields || []).map((field) => renderEditableField(field, record)).join('') + '</div>';
        root.innerHTML = '<section class="panel"><h3>' + view.title + '</h3>' + formSections + '<p class="status" id="form-status"></p><div class="actions"><button id="save-form">Save Draft</button></div></section>';
        const button = root.querySelector('#save-form');
        if (button) {
          button.addEventListener('click', async () => {
            const payload = {};
            root.querySelectorAll('[data-path]').forEach((input) => assignPath(payload, input.dataset.path.replace(/^body\\.payload\\./, ''), readFieldValue(input)));
            const csrf = readCookie('clinic_csrf');
            try {
              await api('/documents/' + encodeURIComponent(documentID), {
                method: 'PUT',
                headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
                body: JSON.stringify({payload})
              });
              document.getElementById('form-status').textContent = 'Draft updated through manifest-driven form.';
              setStatus('Draft updated through manifest-driven form.');
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
        root.innerHTML = '<section class="panel"><h3>' + view.title + '</h3><div class="kv">' + summary.map((item) => {
          if (item.card.widget === 'json') {
            return '<article class="card"><span class="meta">' + item.card.label + '</span><pre>' + escapeHTML(JSON.stringify(item.value, null, 2)) + '</pre></article>';
          }
          if (item.card.widget === 'table' && Array.isArray(item.value)) {
            return '<article class="card" data-action="' + (item.card.action_key || '') + '"><span class="meta">' + item.card.label + '</span><pre>' + escapeHTML(JSON.stringify(item.value, null, 2)) + '</pre></article>';
          }
          return '<article class="card" data-action="' + (item.card.action_key || '') + '"><span class="meta">' + item.card.label + '</span><strong>' + escapeHTML(String(item.value == null ? '' : item.value)) + '</strong></article>';
        }).join('') + '</div></section>';
        root.querySelectorAll('[data-action]').forEach((card) => {
          if (!card.dataset.action) return;
          card.addEventListener('click', () => {
            const action = state.bootstrap.actions.find((item) => item.key === card.dataset.action);
            if (action) window.location.hash = '#' + action.route_path;
          });
        });
        return;
      }
      renderJSONCard(view.title, route);
    }

    async function invokeDocumentAction(documentID, action, expectedVersion, expectedETag) {
      const csrf = readCookie('clinic_csrf');
      return api('/documents/' + encodeURIComponent(documentID) + '/actions', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': csrf},
        body: JSON.stringify({action, expected_version: expectedVersion, expected_etag: expectedETag})
      });
    }

    async function renderCustom(route) {
      const root = document.getElementById('view-root');
      root.innerHTML = '<section class="panel"><h3>Loading custom module page…</h3></section>';
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
        params: Object.fromEntries(currentParams().entries())
      });
    }

    async function renderRoute() {
      renderMenus();
      const path = currentPath() || state.bootstrap.default_path;
      if (!path) {
        setStatus('No permitted routes are available for this principal.');
        document.getElementById('view-root').innerHTML = '';
        return;
      }
      const route = await api('/ui/routes/resolve?path=' + encodeURIComponent(path));
      state.route = route;
      document.getElementById('route-title').innerHTML = '<h2>' + (route.action.label || route.path) + '</h2>';
      setStatus('Resolved from module ' + route.module_key + ' using ' + route.render_mode + ' rendering.');
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

    function escapeHTML(value) {
      return value.replace(/[&<>"]/g, (char) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[char]));
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
        return '<textarea data-path="' + field.path + '"' + readonly + ' placeholder="' + escapeHTML(field.placeholder || '') + '">' + escapeHTML(String(current)) + '</textarea>';
      }
      if (field.widget === 'select' || (field.options || []).length > 0) {
        const options = (field.options || []).map((option) => '<option value="' + option + '"' + (String(current) === option ? ' selected' : '') + '>' + option + '</option>').join('');
        return '<select data-path="' + field.path + '"' + readonly + '>' + options + '</select>';
      }
      if (field.type === 'bool') {
        return '<input type="checkbox" data-path="' + field.path + '"' + (current ? ' checked' : '') + readonly + '>';
      }
      if (field.type === 'int' || field.type === 'number') {
        return '<input type="number" data-path="' + field.path + '" value="' + escapeHTML(String(current)) + '"' + readonly + ' placeholder="' + escapeHTML(field.placeholder || '') + '">';
      }
      return '<input data-path="' + field.path + '" value="' + escapeHTML(String(current)) + '"' + readonly + ' placeholder="' + escapeHTML(field.placeholder || '') + '">';
    }

    function renderRelatedView(def, payload, view) {
      const items = payload[def.source] || [];
      const relatedDef = payload.related_definitions ? payload.related_definitions[def.source] : null;
      const relation = (payload.definition && payload.definition.relations || []).find((item) => item.key === def.source);
      const createForm = relatedDef && relation ? renderRelatedCreateForm(def.source, relatedDef, relation) : '';
      const content = items.length ? '<div class="list">' + items.map((item) => {
        if (typeof item !== 'object' || item == null) {
          return '<article class="card"><strong>' + escapeHTML(String(item)) + '</strong></article>';
        }
        const values = (item.record && item.record.values) || item.values || item;
        const entries = Object.keys(values).sort().slice(0, 6).map((key) => '<div><span class="meta">' + key + '</span><strong>' + escapeHTML(String(values[key])) + '</strong></div>').join('');
        return '<article class="card"><div class="kv">' + entries + '</div></article>';
      }).join('') + '</div>' : '<p class="status">' + (def.empty_state || 'No related items.') + '</p>';
      return '<section class="panel"><h3>' + def.title + '</h3>' + content + createForm + '</section>';
    }

    function renderSection(section, record) {
      const fields = (section.fields || []).map((field) => {
        return '<article class="card"><span class="meta">' + field.label + '</span><strong>' + escapeHTML(String(resolvePath(record, field.path))) + '</strong></article>';
      }).join('');
      const extensionModule = section.extension_slot_key || '';
      let extensionFields = '';
      if (extensionModule && record.body && record.body.payload && record.body.payload.extensions && record.body.payload.extensions[extensionModule]) {
        const ext = record.body.payload.extensions[extensionModule];
        extensionFields = Object.keys(ext).sort().map((key) => {
          return '<article class="card"><span class="meta">' + extensionModule + '.' + key + '</span><strong>' + escapeHTML(String(ext[key])) + '</strong></article>';
        }).join('');
      }
      return '<section><h4>' + section.title + '</h4><div class="kv">' + fields + extensionFields + '</div></section>';
    }

    function renderModelSection(section, record) {
      const fields = (section.fields || []).map((field) => {
        return '<article class="card"><span class="meta">' + field.label + '</span><strong>' + escapeHTML(String(resolvePath(record, field.path))) + '</strong></article>';
      }).join('');
      return '<section><h4>' + section.title + '</h4><div class="kv">' + fields + '</div></section>';
    }

    function renderEditableField(field, record) {
      const value = resolvePath(record, field.path);
      return '<label class="card"><span class="meta">' + field.label + '</span>' + renderFieldInput(field, value) + (field.help_text ? '<span class="meta">' + field.help_text + '</span>' : '') + '</label>';
    }

    function renderEditableModelField(field, record) {
      const value = resolvePath(record, field.path);
      return '<label class="card"><span class="meta">' + field.label + '</span>' + renderFieldInput(field, value) + (field.help_text ? '<span class="meta">' + field.help_text + '</span>' : '') + '</label>';
    }

    function renderFormSection(section, record) {
      return '<section class="panel"><h3>' + section.title + '</h3><div class="list">' + (section.fields || []).map((field) => renderEditableField(field, record)).join('') + '</div></section>';
    }

    function renderModelFormSection(section, record) {
      return '<section class="panel"><h3>' + section.title + '</h3><div class="list">' + (section.fields || []).map((field) => renderEditableModelField(field, record)).join('') + '</div></section>';
    }

    function renderRelationEditor(def, payload) {
      const relatedDef = payload.related_definitions ? payload.related_definitions[def.source] : null;
      const relation = (payload.definition && payload.definition.relations || []).find((item) => item.key === def.source);
      if (!relatedDef || !relation) return '';
      const rows = (payload[def.source] || []).map((item) => renderRelationRow(def.source, relatedDef, relation, item, payload.model_definitions || {})).join('');
      return '<section class="panel" data-relation-editor="' + def.source + '" data-parent-model-key="' + escapeHTML(payload.definition.key || '') + '" data-target-model-key="' + escapeHTML(relatedDef.key || '') + '"><h3>' + def.title + '</h3><div class="list" data-relation-list="' + def.source + '">' + (rows || '<p class="status">No related items yet.</p>') + '</div><div class="actions"><button type="button" class="secondary" data-relation-add="' + def.source + '">Add Row</button></div></section>';
    }

    function deriveRelatedViews(definition) {
      const relations = definition && definition.relations ? definition.relations : [];
      return relations.map((relation) => ({
        key: relation.key,
        title: relation.key.replace(/_/g, ' '),
        source: relation.key,
        empty_state: 'No related items yet.'
      }));
    }

    function renderRelationRow(relationKey, relatedDef, relation, item, modelDefinitions) {
      const graphNode = item && item.record ? item : null;
      const record = graphNode ? graphNode.record : (item || {id: '', version: 0, values: {}});
      const values = record.values || {};
      const fields = (relatedDef.fields || []).filter((field) => field.key !== relation.foreign_key && !field.read_only).map((field) => {
        const enriched = {path: 'values.' + field.key, type: field.type, widget: field.widget, options: field.options || [], placeholder: field.placeholder || '', help_text: field.help_text || ''};
        return '<label class="card"><span class="meta">' + field.label + '</span>' + renderFieldInput(enriched, values[field.key]) + '</label>';
      }).join('');
      const nested = renderNestedRelationEditors(graphNode, relatedDef, modelDefinitions);
      return '<article class="card" data-relation-row="' + relationKey + '" data-record-id="' + escapeHTML(record.id || '') + '" data-record-version="' + escapeHTML(String(record.version || 0)) + '" data-record-op="upsert">' + fields + nested + '<div class="actions"><button type="button" class="secondary" data-relation-remove="' + relationKey + '">Remove</button></div></article>';
    }

    function renderNestedRelationEditors(graphNode, relatedDef, modelDefinitions) {
      const nestedRelations = relatedDef.relations || [];
      if (!nestedRelations.length) return '';
      const relatedMap = graphNode && graphNode.related ? graphNode.related : {};
      return nestedRelations.map((relation) => {
        const targetDef = modelDefinitions[relation.target_model_key];
        if (!targetDef) return '';
        const rows = (relatedMap[relation.key] || []).map((item) => renderRelationRow(relation.key, targetDef, relation, item, modelDefinitions)).join('');
        return '<section class="panel" data-relation-editor="' + relation.key + '" data-parent-model-key="' + escapeHTML(relatedDef.key || '') + '" data-target-model-key="' + escapeHTML(targetDef.key || '') + '"><h4>' + relation.key.replace(/_/g, ' ') + '</h4><div class="list" data-relation-list="' + relation.key + '">' + (rows || '<p class="status">No related items yet.</p>') + '</div><div class="actions"><button type="button" class="secondary" data-relation-add="' + relation.key + '">Add Row</button></div></section>';
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
            list.innerHTML = '<p class="status">No related items yet.</p>';
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
      return '<section class="panel"><h3>Add ' + escapeHTML(relatedDef.display_name || relatedDef.key) + '</h3><div class="list">' +
        editableFields.map((field) => '<label class="card"><span class="meta">' + field.label + '</span>' + renderFieldInput({path: 'values.' + field.key, type: field.type, widget: field.widget, options: field.options || [], placeholder: field.placeholder || ''}, '') + '</label>').join('') +
        '</div><p class="status" data-related-status="' + sourceKey + '"></p><div class="actions"><button type="button" data-related-save="' + sourceKey + '">Add</button></div></section>';
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
      try {
        state.bootstrap = await api('/ui/bootstrap');
        if (!window.location.hash && state.bootstrap.default_path) {
          window.location.hash = '#' + state.bootstrap.default_path;
        }
        await renderRoute();
      } catch (err) {
        document.getElementById('view-root').innerHTML = '<section class="panel"><h3>UI bootstrap failed</h3><p class="status">' + err.message + '</p></section>';
        setStatus('Failed to bootstrap module UI.');
      }
    }

    window.addEventListener('hashchange', () => { void renderRoute(); });
    void bootstrap();
  </script>
</body>
</html>`

func AnalyticsCockpitBundle() string {
	return `(function() {
  window.ClinicModuleBundles = window.ClinicModuleBundles || {};
  window.ClinicModuleBundles["analytics-cockpit"] = {
    render: async function(ctx) {
      const payload = await ctx.api('/ui/data/analytics/snapshot');
      ctx.mount.innerHTML = '<section class="panel"><h3>Analytics Cockpit</h3><p class="status">Custom module page loaded from the analytics bundle.</p><div class="kv"></div><section class="panel"><h3>Raw Snapshot</h3><pre></pre></section></section>';
      const grid = ctx.mount.querySelector('.kv');
      const cards = [
        ['Documents', (payload.documents.created || 0) + (payload.documents.draft || 0) + (payload.documents.submitted || 0) + (payload.documents.approved || 0) + (payload.documents.rejected || 0) + (payload.documents.cancelled || 0)],
        ['Draft', payload.documents.draft],
        ['Submitted', payload.documents.submitted],
        ['Approvals Pending', payload.workflow.pending_approvals]
      ];
      grid.innerHTML = cards.map(function(item) {
        return '<article class="card"><span class="meta">' + item[0] + '</span><strong>' + item[1] + '</strong></article>';
      }).join('');
      ctx.mount.querySelector('pre').textContent = JSON.stringify(payload, null, 2);
    }
  };
})();`
}
