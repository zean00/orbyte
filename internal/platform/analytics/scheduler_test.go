package analytics

import (
	"context"
	"errors"
	"sync/atomic"
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

func TestSchedulerReportsFailureWhenEnqueueFails(t *testing.T) {
	svc := NewServiceWithRepository(document.NewService(), workflow.NewService(), eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), observability.NewService(), nil), search.NewService(), audit.NewService(), observability.NewService(), NewMemoryRepository())
	svc.AttachJobs(jobs.NewServiceWithRepository(failingSchedulerJobRepo{}))
	scheduler := NewScheduler(svc, 10*time.Millisecond, time.Hour)
	var successes atomic.Int32
	var failures atomic.Int32
	scheduler.SetHealthHooks(func() {
		successes.Add(1)
	}, func(error) {
		failures.Add(1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	time.Sleep(25 * time.Millisecond)
	cancel()
	scheduler.Stop()

	if failures.Load() == 0 {
		t.Fatal("expected scheduler failure hook to run")
	}
	if successes.Load() != 0 {
		t.Fatalf("expected no success hook on failed ticks, got %d", successes.Load())
	}
}

type failingSchedulerJobRepo struct{}

func (failingSchedulerJobRepo) Enqueue(job jobs.Job) (jobs.Job, bool, error) {
	return jobs.Job{}, false, errors.New("enqueue failed")
}

func (failingSchedulerJobRepo) Get(string) (jobs.Job, bool) { return jobs.Job{}, false }

func (failingSchedulerJobRepo) List() []jobs.Job { return nil }

func (failingSchedulerJobRepo) ClaimPending(time.Time, time.Duration, int) []jobs.Job { return nil }

func (failingSchedulerJobRepo) RenewLease(string, time.Time, time.Duration) error { return nil }

func (failingSchedulerJobRepo) MarkSucceeded(string, map[string]any, time.Time) error { return nil }

func (failingSchedulerJobRepo) MarkFailed(string, string, string, time.Time) error { return nil }

func (failingSchedulerJobRepo) Requeue(string, time.Time) error { return nil }

func TestSchedulerUsesSharedJobDedupAcrossInstances(t *testing.T) {
	repo := NewMemoryRepository()
	jobRepo := jobs.NewMemoryRepository()
	obs := observability.NewService()
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "analytics.scheduler.enqueued.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "analytics.scheduler.already_claimed.total", Type: "counter"})
	obs.RegisterMetricDefinition(observability.MetricDefinition{Key: "analytics.scheduler.enqueue_failed.total", Type: "counter"})
	svcA := NewServiceWithRepository(document.NewService(), workflow.NewService(), eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil), search.NewService(), audit.NewService(), obs, repo)
	svcB := NewServiceWithRepository(document.NewService(), workflow.NewService(), eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil), search.NewService(), audit.NewService(), obs, repo)
	jobsSvc := jobs.NewServiceWithRepository(jobRepo)
	svcA.AttachJobs(jobsSvc)
	svcB.AttachJobs(jobsSvc)
	schedulerA := NewScheduler(svcA, time.Minute, time.Hour)
	schedulerB := NewScheduler(svcB, time.Minute, time.Hour)
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
	schedulerA.enqueueTick(now)
	schedulerB.enqueueTick(now)

	summary := jobsSvc.Summary()
	if summary.Queued != 4 {
		t.Fatalf("expected one queued job per scheduler task, got %+v", summary)
	}
	if got := obs.Snapshot().Counters["analytics.scheduler.already_claimed.total"]; got == 0 {
		t.Fatalf("expected dedup claim metric, got %+v", obs.Snapshot().Counters)
	}
}
