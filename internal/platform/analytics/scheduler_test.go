package analytics

import (
	"context"
	"testing"
	"time"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/observability"
	"clinic/internal/platform/search"
	"clinic/internal/platform/workflow"
)

func TestSchedulerCapturesSnapshots(t *testing.T) {
	svc := NewServiceWithRepository(document.NewService(), workflow.NewService(), eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), observability.NewService(), nil), search.NewService(), audit.NewService(), observability.NewService(), NewMemoryRepository())
	scheduler := NewScheduler(svc, 10*time.Millisecond, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	time.Sleep(25 * time.Millisecond)
	cancel()
	scheduler.Stop()
	if len(svc.ListSnapshots()) == 0 {
		t.Fatal("expected scheduled snapshot capture")
	}
}
