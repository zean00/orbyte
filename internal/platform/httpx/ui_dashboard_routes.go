package httpx

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/analytics"
	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/shared"
)

func registerUIDashboardRoutes(mux *http.ServeMux, ident *identity.Service, analyticsSvc *analytics.Service, modules *module.Service) {
	if analyticsSvc == nil {
		return
	}

	mux.HandleFunc("GET /ui/data/dashboard/boards/effective", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("dashboard is not allowed"))
			return
		}
		surface := strings.TrimSpace(r.URL.Query().Get("surface"))
		if surface == "" {
			surface = string(module.UISurfaceDashboard)
		}
		roleIDs := ident.ActiveRoleIDsForUser(
			principalEffectiveUserID(p),
			organizationIDForPrincipal(p),
			p.currentLocationID,
			"",
			time.Now().UTC(),
		)
		board, found := analyticsSvc.EffectiveDashboard(surface, organizationIDForPrincipal(p), p.currentLocationID, roleIDs)
		if !found {
			respondJSON(w, http.StatusOK, map[string]any{
				"surface": surface,
				"board":   nil,
				"widgets": []map[string]any{},
			})
			return
		}
		items := make([]map[string]any, 0, len(board.Widgets))
		for _, placement := range board.Widgets {
			def, ok := modules.DashboardWidgetForSurface(placement.WidgetKey, module.UISurface(surface))
			if !ok {
				continue
			}
			if !principalAllowsAll(ident, p, def.RequiredPermissions) {
				continue
			}
			items = append(items, map[string]any{
				"id":               placement.ID,
				"title":            firstNonEmptyString(strings.TrimSpace(placement.Title), def.Title),
				"kind":             firstNonEmptyString(strings.TrimSpace(placement.Kind), def.RendererKind),
				"widget_key":       placement.WidgetKey,
				"width":            placement.Width,
				"height":           placement.Height,
				"order":            placement.Order,
				"refresh_override": placement.RefreshOverride,
				"filters":          placement.Filters,
				"definition":       def,
			})
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"surface": surface,
			"board":   board,
			"widgets": items,
		})
	})

	mux.HandleFunc("GET /ui/data/dashboard/demo", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("dashboard demo is not allowed"))
			return
		}

		latest := analyticsSvc.Snapshot()
		comparison, _ := analyticsSvc.Compare(analytics.Query{})
		trends := analyticsSvc.Trends(7)
		documentTypes := dashboardDocumentTypeRows(latest)
		locationPoints := dashboardLocationPoints(latest)

		respondJSON(w, http.StatusOK, map[string]any{
			"generated_at": latest.GeneratedAt,
			"overview": map[string]any{
				"submitted_documents":    latest.Documents.Submitted,
				"approved_documents":     latest.Documents.Approved,
				"pending_approvals":      latest.Workflow.PendingApprovals,
				"approval_rate_percent":  latest.Workflow.ApprovalRate * 100,
				"projection_coverage":    latest.Coverage.ProjectionCoverage * 100,
				"document_summary_count": latest.Coverage.DocumentSummaries,
			},
			"comparison": comparison,
			"trends": map[string]any{
				"documents": dashboardTrendRows(trends),
			},
			"tables": map[string]any{
				"document_types": documentTypes,
			},
			"charts": map[string]any{
				"document_types": documentTypes,
			},
			"map": map[string]any{
				"branches": locationPoints,
			},
		})
	})
}

func dashboardTrendRows(points []analytics.TrendPoint) []map[string]any {
	rows := make([]map[string]any, 0, len(points))
	for _, point := range points {
		rows = append(rows, map[string]any{
			"label":               point.GeneratedAt.Format("02 Jan 15:04"),
			"submitted_documents": point.SubmittedDocuments,
			"approved_documents":  point.ApprovedDocuments,
			"pending_approvals":   point.PendingApprovals,
			"dead_letters":        point.DeadLetters,
			"generated_at":        point.GeneratedAt.Format(time.RFC3339),
			"snapshot_id":         point.SnapshotID,
		})
	}
	return rows
}

func dashboardDocumentTypeRows(snapshot analytics.Snapshot) []map[string]any {
	keys := make([]string, 0, len(snapshot.Segments.ByDocumentType))
	for key := range snapshot.Segments.ByDocumentType {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		kpi := snapshot.Segments.ByDocumentType[key]
		rows = append(rows, map[string]any{
			"document_type": key,
			"created":       kpi.Created,
			"draft":         kpi.Draft,
			"submitted":     kpi.Submitted,
			"approved":      kpi.Approved,
			"rejected":      kpi.Rejected,
			"cancelled":     kpi.Cancelled,
		})
	}
	return rows
}

func dashboardLocationPoints(snapshot analytics.Snapshot) []map[string]any {
	keys := make([]string, 0, len(snapshot.Segments.ByLocation))
	for key := range snapshot.Segments.ByLocation {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	points := make([]map[string]any, 0, len(keys))
	for index, key := range keys {
		kpi := snapshot.Segments.ByLocation[key]
		lat, lng := dashboardCoordinateForIndex(index)
		points = append(points, map[string]any{
			"location_id": key,
			"label":       dashboardLocationLabel(key),
			"submitted":   kpi.Submitted,
			"approved":    kpi.Approved,
			"latitude":    lat,
			"longitude":   lng,
		})
	}
	return points
}

func dashboardCoordinateForIndex(index int) (float64, float64) {
	coords := [][2]float64{
		{-6.2088, 106.8456},
		{-6.1745, 106.8227},
		{-6.2615, 106.8106},
		{-6.2297, 106.6894},
		{-6.1456, 106.8019},
	}
	if index < len(coords) {
		selected := coords[index]
		return selected[0], selected[1]
	}

	// Spread additional points across a simple deterministic grid instead of
	// wrapping back onto the initial coordinates.
	extra := index - len(coords)
	row := extra / 4
	col := extra % 4
	baseLat := -6.30 + float64(row)*0.055
	baseLng := 106.66 + float64(col)*0.07
	if row%2 == 1 {
		baseLng += 0.02
	}
	return baseLat, baseLng
}

func dashboardLocationLabel(locationID string) string {
	trimmed := strings.TrimSpace(locationID)
	if trimmed == "" {
		return "Unscoped"
	}
	if trimmed == "loc_hq" {
		return "Head Office"
	}
	trimmed = strings.ReplaceAll(trimmed, "_", " ")
	trimmed = strings.ReplaceAll(trimmed, "-", " ")
	parts := strings.Fields(trimmed)
	for index, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		parts[index] = string(runes)
	}
	return strings.Join(parts, " ")
}
