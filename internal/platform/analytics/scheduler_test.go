package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/jobs"
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

func TestSchedulerFallsBackWhenJobEnqueueFails(t *testing.T) {
	svc := NewServiceWithRepository(document.NewService(), workflow.NewService(), eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), observability.NewService(), nil), search.NewService(), audit.NewService(), observability.NewService(), NewMemoryRepository())
	svc.AttachJobs(jobs.NewServiceWithRepository(failingSchedulerJobRepo{}))
	scheduler := NewScheduler(svc, 10*time.Millisecond, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	time.Sleep(25 * time.Millisecond)
	cancel()
	scheduler.Stop()
	if len(svc.ListSnapshots()) == 0 {
		t.Fatal("expected fallback snapshot capture")
	}
}

type failingSchedulerJobRepo struct{}

func (failingSchedulerJobRepo) Enqueue(job jobs.Job) (jobs.Job, bool, error) {
	return jobs.Job{}, false, errors.New("enqueue failed")
}

func (failingSchedulerJobRepo) Get(string) (jobs.Job, bool) { return jobs.Job{}, false }

func (failingSchedulerJobRepo) ClaimPending(time.Time, time.Duration, int) []jobs.Job { return nil }

func (failingSchedulerJobRepo) RenewLease(string, time.Time, time.Duration) error { return nil }

func (failingSchedulerJobRepo) MarkSucceeded(string, map[string]any, time.Time) error { return nil }

func (failingSchedulerJobRepo) MarkFailed(string, string, string, time.Time) error { return nil }
