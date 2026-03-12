package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"clinic/internal/platform/identity"
	"clinic/internal/platform/jobs"
	"clinic/internal/platform/search"
	"clinic/internal/platform/shared"
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
		respondJSON(w, http.StatusOK, map[string]any{"items": searchSvc.IndexDefinitions()})
	})

	mux.HandleFunc("GET /ops/search/indexes/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rebuild") {
			if _, ok := requireAuthorization(w, r, ident, "search.manage", "", "search.manage"); !ok {
				return
			}
			indexKey := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/ops/search/indexes/"))
			def, found := searchSvc.IndexDefinition(indexKey)
			if !found {
				respondError(w, shared.NotFound("search index not found"))
				return
			}
			respondJSON(w, http.StatusOK, def)
			return
		}
	})

	mux.HandleFunc("POST /ops/search/indexes/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rebuild") {
			respondError(w, shared.NotFound("search operation not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "search.manage", "", "search.manage"); !ok {
			return
		}
		indexKey := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ops/search/indexes/"), "/rebuild"))
		if jobSvc == nil {
			result, err := searchSvc.RebuildIndex(indexKey)
			if err != nil {
				respondError(w, err)
				return
			}
			respondJSON(w, http.StatusOK, result)
			return
		}
		job := jobSvc.Enqueue("search.rebuild."+indexKey, func() (map[string]any, error) {
			return searchSvc.RebuildIndex(indexKey)
		})
		respondJSON(w, http.StatusAccepted, job)
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

func organizationIDForPrincipal(p principal) string {
	if p.currentLocationID != "" {
		return "org_default"
	}
	return "org_default"
}
