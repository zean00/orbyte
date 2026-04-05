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

	mux.HandleFunc("GET /ui/data/dashboard/sales-demo", func(w http.ResponseWriter, r *http.Request) {
		p, ok := requireInteractivePrincipal(w, r)
		if !ok {
			return
		}
		if !principalAllowsAll(ident, p, []string{"analytics.read"}) {
			respondError(w, shared.Forbidden("dashboard sales demo is not allowed"))
			return
		}

		latest := analyticsSvc.Snapshot()
		points := dashboardSalesBranchRows(latest)
		totalSales := 0.0
		targetSales := 0.0
		for _, point := range points {
			totalSales += point.NetSales
			targetSales += point.TargetSales
		}
		targetAttainment := 0.0
		if targetSales > 0 {
			targetAttainment = (totalSales / targetSales) * 100
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"generated_at": latest.GeneratedAt,
			"overview": map[string]any{
				"net_sales":                 totalSales,
				"target_sales":              targetSales,
				"target_attainment_percent": targetAttainment,
				"best_branch":               dashboardBestSalesBranch(points),
			},
			"trends": map[string]any{
				"sales": dashboardSalesTrendRows(analyticsSvc.Trends(7)),
			},
			"tables": map[string]any{
				"branches": dashboardSalesTableRows(points),
			},
			"charts": map[string]any{
				"branches": dashboardSalesChartRows(points),
			},
			"map": map[string]any{
				"branches": dashboardSalesMapRows(points),
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

type dashboardSalesBranch struct {
	LocationID  string
	Label       string
	Latitude    float64
	Longitude   float64
	NetSales    float64
	TargetSales float64
	Orders      int
	ApprovalPct float64
}

func dashboardSalesBranchRows(snapshot analytics.Snapshot) []dashboardSalesBranch {
	keys := make([]string, 0, len(snapshot.Segments.ByLocation))
	for key := range snapshot.Segments.ByLocation {
		if !strings.HasPrefix(strings.TrimSpace(key), "loc_demo_") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]dashboardSalesBranch, 0, len(keys))
	for index, key := range keys {
		kpi := snapshot.Segments.ByLocation[key]
		lat, lng := dashboardCoordinateForIndex(index)
		baseSales, targetSales := dashboardSyntheticSalesForLocation(key, kpi)
		approvalPct := 0.0
		if kpi.Submitted > 0 {
			approvalPct = (float64(kpi.Approved) / float64(kpi.Submitted)) * 100
		}
		rows = append(rows, dashboardSalesBranch{
			LocationID:  key,
			Label:       dashboardLocationLabel(key),
			Latitude:    lat,
			Longitude:   lng,
			NetSales:    baseSales,
			TargetSales: targetSales,
			Orders:      kpi.Submitted + kpi.Approved + kpi.Draft,
			ApprovalPct: approvalPct,
		})
	}
	return rows
}

func dashboardSyntheticSalesForLocation(locationID string, kpi analytics.DocumentKPI) (float64, float64) {
	switch strings.TrimSpace(locationID) {
	case "loc_demo_central":
		return 7900000, 10800000
	case "loc_demo_west":
		return 9800000, 11800000
	case "loc_demo_east":
		return 15800000, 14900000
	default:
		baseSales := float64(kpi.Submitted*1250000 + kpi.Approved*1850000 + 3500000)
		targetSales := baseSales * 1.1
		return baseSales, targetSales
	}
}

func dashboardSalesTrendRows(points []analytics.TrendPoint) []map[string]any {
	rows := make([]map[string]any, 0, len(points))
	for index, point := range points {
		netSales := float64(point.SubmittedDocuments*1450000 + point.ApprovedDocuments*2100000 + (index+1)*900000)
		rows = append(rows, map[string]any{
			"label":       point.GeneratedAt.Format("02 Jan"),
			"net_sales":   netSales,
			"orders":      point.SubmittedDocuments + point.ApprovedDocuments,
			"generated":   point.GeneratedAt.Format(time.RFC3339),
			"snapshot_id": point.SnapshotID,
		})
	}
	return rows
}

func dashboardSalesTableRows(points []dashboardSalesBranch) []map[string]any {
	rows := make([]map[string]any, 0, len(points))
	for _, point := range points {
		gap := point.NetSales - point.TargetSales
		rows = append(rows, map[string]any{
			"branch":             point.Label,
			"net_sales":          point.NetSales,
			"target_sales":       point.TargetSales,
			"orders":             point.Orders,
			"approval_percent":   point.ApprovalPct,
			"variance_to_target": gap,
		})
	}
	return rows
}

func dashboardSalesChartRows(points []dashboardSalesBranch) []map[string]any {
	rows := make([]map[string]any, 0, len(points))
	for _, point := range points {
		rows = append(rows, map[string]any{
			"branch":    point.Label,
			"net_sales": point.NetSales,
		})
	}
	return rows
}

func dashboardSalesMapRows(points []dashboardSalesBranch) []map[string]any {
	rows := make([]map[string]any, 0, len(points))
	for _, point := range points {
		rows = append(rows, map[string]any{
			"location_id": point.LocationID,
			"label":       point.Label,
			"latitude":    point.Latitude,
			"longitude":   point.Longitude,
			"net_sales":   point.NetSales,
		})
	}
	return rows
}

func dashboardBestSalesBranch(points []dashboardSalesBranch) string {
	best := ""
	bestValue := -1.0
	for _, point := range points {
		if point.NetSales > bestValue {
			best = point.Label
			bestValue = point.NetSales
		}
	}
	return best
}
