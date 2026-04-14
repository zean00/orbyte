package httpx

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/shared"
)

const adminAuditExportLimit = 10000

func registerAdminAuditRoutes(mux *http.ServeMux, ident *identity.Service, auditSvc *audit.Service) {
	mux.HandleFunc("GET /admin/api/audit-events", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "audit.read", "", "audit.read"); !ok {
			return
		}
		filter, err := adminAuditQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, auditSvc.Search(filter))
	})

	mux.HandleFunc("GET /admin/api/audit-events/export", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuthorization(w, r, ident, "audit.read", "", "audit.read"); !ok {
			return
		}
		filter, err := adminAuditQueryFromRequest(r)
		if err != nil {
			respondError(w, err)
			return
		}
		filter.Page = 1
		filter.PageSize = adminAuditExportLimit
		result := auditSvc.Search(filter)
		truncated := result.Total > len(result.Items)
		format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
		if format == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", `attachment; filename="audit_events.json"`)
			_ = json.NewEncoder(w).Encode(map[string]any{"items": result.Items, "total": result.Total, "truncated": truncated, "limit": adminAuditExportLimit})
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="audit_events.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"occurred_at", "action", "actor_id", "actor_kind", "target_type", "target_id", "from_state", "to_state", "organization_id", "location_id", "operating_unit_id", "request_id", "correlation_id", "delegation_grant_id", "on_behalf_of_user_id", "audit_event_id"})
		for _, item := range result.Items {
			_ = writer.Write([]string{
				item.OccurredAt.Format("2006-01-02T15:04:05.000000000Z07:00"),
				item.Action,
				item.ActorID,
				item.ActorKind,
				item.TargetType,
				item.TargetID,
				item.FromState,
				item.ToState,
				item.OrganizationID,
				item.LocationID,
				item.OperatingUnitID,
				item.RequestID,
				item.CorrelationID,
				item.DelegationGrantID,
				item.OnBehalfOfUserID,
				item.ID,
			})
		}
		if truncated {
			_ = writer.Write([]string{"TRUNCATED", "true", "limit", "10000"})
		}
		writer.Flush()
	})
}

func adminAuditQueryFromRequest(r *http.Request) (audit.Query, error) {
	filter, err := auditQueryFromRequest(r)
	if err != nil {
		return audit.Query{}, err
	}
	q := r.URL.Query()
	filter.Text = strings.TrimSpace(q.Get("q"))
	filter.RequestID = strings.TrimSpace(q.Get("request_id"))
	filter.DelegationGrantID = strings.TrimSpace(q.Get("delegation_grant_id"))
	filter.FromState = strings.TrimSpace(q.Get("from_state"))
	filter.ToState = strings.TrimSpace(q.Get("to_state"))
	filter.MetadataKey = strings.TrimSpace(q.Get("metadata_key"))
	filter.MetadataValue = strings.TrimSpace(q.Get("metadata_value"))
	if filter.MetadataKey == "" {
		filter.MetadataValue = ""
	}
	filter.Sort = strings.TrimSpace(q.Get("sort"))
	filter.Direction = strings.ToLower(strings.TrimSpace(q.Get("direction")))
	filter.Page = intQuery(r, "page", 1)
	filter.PageSize = intQuery(r, "page_size", 20)
	if filter.Sort == "" {
		filter.Sort = "occurred_at"
	}
	if !allowedAuditSort(filter.Sort) {
		return audit.Query{}, shared.Validation("invalid audit sort field")
	}
	if filter.Direction == "" {
		filter.Direction = "desc"
	}
	if filter.Direction != "asc" && filter.Direction != "desc" {
		return audit.Query{}, shared.Validation("invalid audit sort direction")
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	return filter, nil
}

func allowedAuditSort(value string) bool {
	switch value {
	case "occurred_at", "action", "target_type", "target_id", "actor_id", "actor_kind", "correlation_id":
		return true
	default:
		return false
	}
}
