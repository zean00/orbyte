package analytics

import (
	"testing"
	"time"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func TestAttachRuntimeAndJobTime(t *testing.T) {
	svc := NewService(document.NewService(), workflow.NewService(), eventing.NewService(), search.NewService(), audit.NewService(), observability.NewService())
	jobSvc := jobs.NewService()
	svc.AttachRuntime(jobSvc)

	claimed, err := jobTime(map[string]any{"cutoff": "2026-03-19T10:00:00Z"}, "cutoff")
	if err != nil || claimed.Format(time.RFC3339) != "2026-03-19T10:00:00Z" {
		t.Fatalf("unexpected parsed job time: %s err=%v", claimed, err)
	}
	if _, err := jobTime(map[string]any{}, "cutoff"); err == nil {
		t.Fatal("expected missing job time to fail")
	}
	if _, err := jobTime(map[string]any{"cutoff": "bad"}, "cutoff"); err == nil {
		t.Fatal("expected invalid job time to fail")
	}
}

func TestMemoryRepositoryRecentAndRollups(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if err := repo.SaveSnapshot(Snapshot{ID: "s" + string(rune('1'+i)), GeneratedAt: now.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatalf("save snapshot failed: %v", err)
		}
	}
	recent := repo.ListRecent(2)
	if len(recent) != 2 || recent[0].ID != "s2" || recent[1].ID != "s3" {
		t.Fatalf("unexpected recent snapshots: %+v", recent)
	}

	if err := repo.UpsertRollup(Rollup{ID: "r1", Granularity: "daily", BucketStart: now}); err != nil {
		t.Fatalf("upsert rollup failed: %v", err)
	}
	if err := repo.UpsertRollup(Rollup{ID: "r2", Granularity: "daily", BucketStart: now.Add(24 * time.Hour)}); err != nil {
		t.Fatalf("upsert rollup failed: %v", err)
	}
	rollups := repo.ListRollups("daily", 1)
	if len(rollups) != 1 || rollups[0].ID != "r2" {
		t.Fatalf("unexpected listed rollups: %+v", rollups)
	}
}
