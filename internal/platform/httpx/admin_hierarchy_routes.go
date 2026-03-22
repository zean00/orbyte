package httpx

import (
	"encoding/json"
	"net/http"
	"strings"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

func registerAdminHierarchyRoutes(mux *http.ServeMux, ident *identity.Service) {
	mux.HandleFunc("GET /admin/api/reporting-lines", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		items := ident.ReportingLines()
		subjectUserID := strings.TrimSpace(r.URL.Query().Get("subject_user_id"))
		managerUserID := strings.TrimSpace(r.URL.Query().Get("manager_user_id"))
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		filtered := make([]identity.ReportingLine, 0, len(items))
		for _, item := range items {
			if subjectUserID != "" && item.SubjectUserID != subjectUserID {
				continue
			}
			if managerUserID != "" && item.ManagerUserID != managerUserID {
				continue
			}
			if status != "" && item.Status != status {
				continue
			}
			filtered = append(filtered, item)
		}
		respondJSON(w, http.StatusOK, map[string]any{"items": filtered})
	})

	mux.HandleFunc("GET /admin/api/hierarchy/graph", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		users, edges := hierarchyGraphData(
			ident,
			strings.TrimSpace(r.URL.Query().Get("organization_id")),
			strings.TrimSpace(r.URL.Query().Get("location_id")),
			strings.TrimSpace(r.URL.Query().Get("operating_unit_id")),
			strings.TrimSpace(r.URL.Query().Get("status")),
		)
		respondJSON(w, http.StatusOK, map[string]any{
			"nodes":   users,
			"edges":   edges,
			"summary": hierarchySummary(users, edges),
		})
	})

	mux.HandleFunc("GET /admin/api/hierarchy/summary", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		users, edges := hierarchyGraphData(
			ident,
			strings.TrimSpace(r.URL.Query().Get("organization_id")),
			strings.TrimSpace(r.URL.Query().Get("location_id")),
			strings.TrimSpace(r.URL.Query().Get("operating_unit_id")),
			strings.TrimSpace(r.URL.Query().Get("status")),
		)
		respondJSON(w, http.StatusOK, hierarchySummary(users, edges))
	})

	mux.HandleFunc("GET /admin/api/hierarchy/chain", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			respondError(w, shared.Validation("user_id is required"))
			return
		}
		chain := hierarchyChain(
			ident,
			userID,
			strings.TrimSpace(r.URL.Query().Get("organization_id")),
			strings.TrimSpace(r.URL.Query().Get("location_id")),
			strings.TrimSpace(r.URL.Query().Get("operating_unit_id")),
		)
		respondJSON(w, http.StatusOK, map[string]any{"items": chain})
	})

	mux.HandleFunc("POST /admin/api/reporting-lines", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		var req reportingLineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid reporting line request"))
			return
		}
		item, err := ident.UpsertReportingLine(reportingLineFromRequest("", req))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, item)
	})

	mux.HandleFunc("PUT /admin/api/reporting-lines/", func(w http.ResponseWriter, r *http.Request) {
		id, ok := adminReportingLinePath(r.URL.Path)
		if !ok {
			respondError(w, shared.NotFound("reporting line not found"))
			return
		}
		if _, ok := requireAuthorization(w, r, ident, "identity.manage_users", "", "identity.manage_users"); !ok {
			return
		}
		var req reportingLineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.Validation("invalid reporting line request"))
			return
		}
		item, err := ident.UpsertReportingLine(reportingLineFromRequest(id, req))
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, item)
	})

}
