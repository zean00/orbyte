package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"orbyte/internal/modules"
	"orbyte/internal/platform/analytics"
	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/store"
)

type dashboardSeedManifest struct {
	SeededAt       string `json:"seeded_at"`
	OrganizationID string `json:"organization_id"`
	Surface        string `json:"surface"`
	BoardID        string `json:"board_id"`
	BoardName      string `json:"board_name"`
	BoardPath      string `json:"board_path"`
	DataPath       string `json:"data_path"`
	Widgets        []struct {
		Key   string `json:"key"`
		Title string `json:"title"`
		Kind  string `json:"kind"`
	} `json:"widgets"`
	Snapshots []struct {
		ID                 string `json:"id"`
		GeneratedAt        string `json:"generated_at"`
		SubmittedDocuments int    `json:"submitted_documents"`
		ApprovedDocuments  int    `json:"approved_documents"`
		PendingApprovals   int    `json:"pending_approvals"`
	} `json:"snapshots"`
}

func TestSeedDashboardSyntheticScenario(t *testing.T) {
	if os.Getenv("DASHBOARD_SEED") != "1" {
		t.Skip("set DASHBOARD_SEED=1 to seed the postgres-backed dashboard scenario")
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed dashboard seed")
	}

	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	manifests, err := modules.ForProfile(modules.ProfileAll)
	if err != nil {
		t.Fatalf("load business manifests: %v", err)
	}
	graph := constructServiceGraph(postgres, manifests)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, manifests, testBootstrapAdminPassword); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}

	actorID := "user_admin"
	orgID := "org_default"
	suffix := time.Now().UTC().Format("20060102150405")
	now := time.Now().UTC()

	for _, item := range []struct {
		id   string
		key  string
		name string
	}{
		{id: "loc_demo_west", key: "demo_west", name: "West Branch"},
		{id: "loc_demo_central", key: "demo_central", name: "Central Branch"},
		{id: "loc_demo_east", key: "demo_east", name: "East Branch"},
	} {
		if _, err := postgres.DB.ExecContext(context.Background(), `
			INSERT INTO locations (location_id, organization_id, location_key, name, location_type, status, parent_location_id, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,NULL,$7,$8)
			ON CONFLICT (location_id) DO UPDATE SET
				organization_id = EXCLUDED.organization_id,
				location_key = EXCLUDED.location_key,
				name = EXCLUDED.name,
				location_type = EXCLUDED.location_type,
				status = EXCLUDED.status,
				updated_at = EXCLUDED.updated_at
		`, item.id, orgID, item.key, item.name, "store", "active", now, now); err != nil {
			t.Fatalf("upsert location %s: %v", item.id, err)
		}
	}

	type documentSeed struct {
		documentType string
		locationID   string
		status       string
		title        string
	}

	batches := [][]documentSeed{
		{
			{documentType: "generic_request", locationID: "loc_hq", status: "submitted", title: "HQ launch review " + suffix},
			{documentType: "sales_order", locationID: "loc_demo_west", status: "draft", title: "West branch walk-in orders " + suffix},
			{documentType: "invoice", locationID: "loc_demo_east", status: "approved", title: "East branch receivables " + suffix},
		},
		{
			{documentType: "generic_request", locationID: "loc_hq", status: "approved", title: "HQ vendor escalation " + suffix},
			{documentType: "purchase_request", locationID: "loc_demo_west", status: "submitted", title: "West replenishment request " + suffix},
			{documentType: "sales_order", locationID: "loc_demo_central", status: "approved", title: "Central branch trade sale " + suffix},
		},
		{
			{documentType: "generic_request", locationID: "loc_demo_west", status: "submitted", title: "West exception queue " + suffix},
			{documentType: "invoice", locationID: "loc_demo_central", status: "rejected", title: "Central billing dispute " + suffix},
			{documentType: "purchase_request", locationID: "loc_demo_east", status: "approved", title: "East replenishment release " + suffix},
			{documentType: "sales_order", locationID: "loc_hq", status: "submitted", title: "HQ corporate sale " + suffix},
		},
	}

	for _, batch := range batches {
		for _, item := range batch {
			record, err := graph.documents.Create(item.documentType, orgID, item.locationID, actorID, map[string]any{
				"title": item.title,
			})
			if err != nil {
				t.Fatalf("create %s: %v", item.documentType, err)
			}
			switch item.status {
			case "submitted":
				record, err = graph.docActions.Submit(record.Header.ID, application.ActingContext{ActorID: actorID, EffectiveUserID: actorID}, record.Header.Version, record.Header.ETag)
				if err != nil {
					t.Fatalf("submit %s: %v", item.documentType, err)
				}
			case "approved":
				if item.documentType == "generic_request" {
					record, err = graph.docActions.Submit(record.Header.ID, application.ActingContext{ActorID: actorID, EffectiveUserID: actorID}, record.Header.Version, record.Header.ETag)
					if err != nil {
						t.Fatalf("submit %s for approval: %v", item.documentType, err)
					}
					record, err = graph.docActions.Approve(record.Header.ID, application.ActingContext{ActorID: actorID, EffectiveUserID: actorID}, record.Header.Version, record.Header.ETag)
					if err != nil {
						t.Fatalf("approve %s: %v", item.documentType, err)
					}
				} else {
					record.Header.Status = "approved"
					record.Header.UpdatedAt = time.Now().UTC()
					record.Header.UpdatedBy = actorID
					if err := graph.documents.Save(record); err != nil {
						t.Fatalf("save approved %s: %v", item.documentType, err)
					}
				}
			case "rejected":
				record.Header.Status = "rejected"
				record.Header.UpdatedAt = time.Now().UTC()
				record.Header.UpdatedBy = actorID
				if err := graph.documents.Save(record); err != nil {
					t.Fatalf("save rejected %s: %v", item.documentType, err)
				}
			}
		}
		if _, err := graph.analytics.CaptureSnapshot(); err != nil {
			t.Fatalf("capture snapshot: %v", err)
		}
		time.Sleep(15 * time.Millisecond)
	}

	board, err := graph.analytics.SaveDashboard(analytics.Dashboard{
		Name:        "Operations Demo Board " + suffix,
		Description: "Synthetic business dashboard board for renderer verification across metric, gauge, chart, table, and map widgets.",
		Surface:     "dashboard",
		IsDefault:   true,
		Visibility:  "private",
		Status:      "active",
		RuntimeScope: analytics.RuntimeScope{
			ScopeType: "deployment",
		},
		Widgets: []analytics.DashboardWidget{
			{WidgetKey: "analytics.demo.submitted_documents", Title: "Submitted Documents", Kind: "metric", Width: 3, Height: 1, Order: 1},
			{WidgetKey: "analytics.demo.approval_rate", Title: "Approval Rate", Kind: "gauge", Width: 3, Height: 2, Order: 2},
			{WidgetKey: "analytics.demo.submission_trend", Title: "Submission Trend", Kind: "chart_line", Width: 6, Height: 2, Order: 3},
			{WidgetKey: "analytics.demo.document_mix", Title: "Document Type Mix", Kind: "chart_bar", Width: 6, Height: 2, Order: 4},
			{WidgetKey: "analytics.demo.document_breakdown", Title: "Document Breakdown", Kind: "table", Width: 6, Height: 2, Order: 5},
			{WidgetKey: "analytics.demo.branch_map", Title: "Branch Performance", Kind: "map", Width: 6, Height: 2, Order: 6},
		},
	})
	if err != nil {
		t.Fatalf("save dashboard board: %v", err)
	}

	manifest := dashboardSeedManifest{
		SeededAt:       time.Now().UTC().Format(time.RFC3339),
		OrganizationID: orgID,
		Surface:        "dashboard",
		BoardID:        board.ID,
		BoardName:      board.Name,
		BoardPath:      "/ui/dashboard",
		DataPath:       "/ui/data/dashboard/demo",
	}
	for _, widget := range board.Widgets {
		manifest.Widgets = append(manifest.Widgets, struct {
			Key   string `json:"key"`
			Title string `json:"title"`
			Kind  string `json:"kind"`
		}{
			Key:   widget.WidgetKey,
			Title: widget.Title,
			Kind:  widget.Kind,
		})
	}
	for _, snapshot := range graph.analytics.ListRecent(7) {
		manifest.Snapshots = append(manifest.Snapshots, struct {
			ID                 string `json:"id"`
			GeneratedAt        string `json:"generated_at"`
			SubmittedDocuments int    `json:"submitted_documents"`
			ApprovedDocuments  int    `json:"approved_documents"`
			PendingApprovals   int    `json:"pending_approvals"`
		}{
			ID:                 snapshot.ID,
			GeneratedAt:        snapshot.GeneratedAt.Format(time.RFC3339),
			SubmittedDocuments: snapshot.Documents.Submitted,
			ApprovedDocuments:  snapshot.Documents.Approved,
			PendingApprovals:   snapshot.Workflow.PendingApprovals,
		})
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestPath := filepath.Join(os.TempDir(), "orbyte-dashboard-seed.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Logf("seeded dashboard demo scenario: %s", manifestPath)
	t.Logf("board=%s data_path=%s widgets=%d snapshots=%d", manifest.BoardName, manifest.DataPath, len(manifest.Widgets), len(manifest.Snapshots))
}
