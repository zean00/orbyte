package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/shared"
)

func registerSearchRoutes(mux *http.ServeMux, ident *identity.Service, searchSvc *search.Service, jobSvc *jobs.Service) {
	mux.HandleFunc("GET /search/indexes", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		items := make([]search.IndexDefinition, 0)
		for _, def := range searchSvc.IndexDefinitions() {
			if principalAllowsAll(ident, p, def.RequiredPermissions) {
				items = append(items, def)
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	mux.HandleFunc("GET /search/indexes/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/query") && !strings.HasSuffix(r.URL.Path, "/vector") && !strings.HasSuffix(r.URL.Path, "/hybrid") {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			indexKey := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/search/indexes/"))
			def, found := searchSvc.IndexDefinition(indexKey)
			if !found {
				respondError(w, shared.NotFound("search index not found"))
				return
			}
			if !principalAllowsAll(ident, p, def.RequiredPermissions) {
				respondError(w, shared.Forbidden("search index is not allowed"))
				return
			}
			respondJSON(w, http.StatusOK, def)
			return
		}
	})

	for _, route := range []struct {
		pattern string
		mode    string
	}{
		{pattern: "POST /search/indexes/{key}/query", mode: "keyword"},
		{pattern: "POST /search/indexes/{key}/query/vector", mode: "vector"},
		{pattern: "POST /search/indexes/{key}/query/hybrid", mode: "hybrid"},
	} {
		mode := route.mode
		mux.HandleFunc(route.pattern, func(w http.ResponseWriter, r *http.Request) {
			p, ok := requireInteractivePrincipal(w, r)
			if !ok {
				return
			}
			indexKey := searchIndexKeyFromPath(r.URL.Path)
			def, found := searchSvc.IndexDefinition(indexKey)
			if !found {
				respondError(w, shared.NotFound("search index not found"))
				return
			}
			if !principalAllowsAll(ident, p, def.RequiredPermissions) {
				respondError(w, shared.Forbidden("search index is not allowed"))
				return
			}
			var req search.QueryRequest
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&req)
			}
			req.Mode = mode
			result, err := searchSvc.Query(indexKey, organizationIDForPrincipal(p), p.currentLocationID, req)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, result)
		})
	}

	mux.HandleFunc("GET /ops/search/indexes", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "search.manage", "", "search.manage"); !ok {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": searchSvc.IndexDefinitions(), "runtime_items": searchSvc.IndexRuntimes()})
	})

	mux.HandleFunc("GET /ops/search/indexes/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "search.manage", "", "search.manage"); !ok {
			return
		}
		indexKey, action := opsSearchIndexPath(r.URL.Path)
		if indexKey == "" {
			respondError(w, shared.NotFound("search index not found"))
			return
		}
		switch {
		case action == "":
			def, found := searchSvc.IndexDefinition(indexKey)
			if !found {
				respondError(w, shared.NotFound("search index not found"))
				return
			}
			respondJSON(w, http.StatusOK, def)
		case action == "runtime":
			runtime, found := searchSvc.IndexRuntime(indexKey)
			if !found {
				respondError(w, shared.NotFound("search index not found"))
				return
			}
			respondJSON(w, http.StatusOK, runtime)
		case action == "consistency":
			report, err := searchSvc.ConsistencyReport(indexKey)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, report)
		default:
			respondError(w, shared.NotFound("search operation not found"))
		}
	})

	mux.HandleFunc("POST /ops/search/indexes/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rebuild") &&
			!strings.HasSuffix(r.URL.Path, "/repair") &&
			!strings.HasSuffix(r.URL.Path, "/reconcile") &&
			!strings.HasSuffix(r.URL.Path, "/schema/plan") &&
			!strings.HasSuffix(r.URL.Path, "/schema/build") &&
			!strings.HasSuffix(r.URL.Path, "/schema/activate") {
			respondError(w, shared.NotFound("search operation not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "search.manage", "", "search.manage"); !ok {
			return
		}
		indexKey, action := opsSearchIndexPath(r.URL.Path)
		switch action {
		case "rebuild":
			if jobSvc == nil {
				result, err := searchSvc.RebuildIndex(indexKey)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, result)
				return
			}
			job, err := jobSvc.Enqueue(search.JobRebuildIndex, map[string]any{"index_key": indexKey})
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusAccepted, job)
		case "repair":
			mode := strings.TrimSpace(r.URL.Query().Get("mode"))
			targetID := strings.TrimSpace(r.URL.Query().Get("target_id"))
			if jobSvc == nil {
				result, err := searchSvc.RepairIndex(indexKey, mode, targetID)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, result)
				return
			}
			job, err := jobSvc.Enqueue(search.JobRepairIndex, map[string]any{"index_key": indexKey, "mode": mode, "target_id": targetID})
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusAccepted, job)
		case "reconcile":
			if jobSvc == nil {
				report, err := searchSvc.ConsistencyReport(indexKey)
				if err != nil {
					respondError(w, err)
					return
				}
				respondJSON(w, http.StatusOK, map[string]any{"index_key": indexKey, "report": report})
				return
			}
			job, err := jobSvc.Enqueue(search.JobScanIndex, map[string]any{"index_key": indexKey})
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusAccepted, job)
		case "schema/plan":
			var req struct {
				Version string `json:"version"`
			}
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&req)
			}
			runtime, err := searchSvc.PlanIndexSchemaVersion(indexKey, req.Version)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, runtime)
		case "schema/build":
			runtime, err := searchSvc.BuildCandidateIndex(indexKey)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, runtime)
		case "schema/activate":
			runtime, err := searchSvc.ActivateCandidateIndex(indexKey)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, runtime)
		default:
			respondError(w, shared.NotFound("search operation not found"))
		}
	})
}

func searchIndexKeyFromPath(path string) string {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/search/indexes/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func opsSearchIndexPath(path string) (string, string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/ops/search/indexes/"), "/")
	switch {
	case strings.HasSuffix(trimmed, "/schema/plan"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/schema/plan")), "schema/plan"
	case strings.HasSuffix(trimmed, "/schema/build"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/schema/build")), "schema/build"
	case strings.HasSuffix(trimmed, "/schema/activate"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/schema/activate")), "schema/activate"
	case strings.HasSuffix(trimmed, "/rebuild"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/rebuild")), "rebuild"
	case strings.HasSuffix(trimmed, "/runtime"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/runtime")), "runtime"
	case strings.HasSuffix(trimmed, "/consistency"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/consistency")), "consistency"
	case strings.HasSuffix(trimmed, "/repair"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/repair")), "repair"
	case strings.HasSuffix(trimmed, "/reconcile"):
		return strings.TrimSpace(strings.TrimSuffix(trimmed, "/reconcile")), "reconcile"
	default:
		return strings.TrimSpace(trimmed), ""
	}
}

func organizationIDForPrincipal(p principal) string {
	if p.currentLocationID != "" {
		return "org_default"
	}
	return "org_default"
}
