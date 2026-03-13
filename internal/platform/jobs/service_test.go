package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestServiceProcessesQueuedJobs(t *testing.T) {
	svc := NewService()
	svc.RegisterHandler("search.refresh_document", func(_ context.Context, payload map[string]any) (map[string]any, error) {
		return map[string]any{"document_id": payload["document_id"]}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	job, err := svc.Enqueue("search.refresh_document", map[string]any{"document_id": "doc-1"})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	waitForStatus(t, svc, job.ID, StatusSucceeded)
	stored, ok := svc.Get(job.ID)
	if !ok {
		t.Fatal("expected stored job")
	}
	if stored.Result["document_id"] != "doc-1" {
		t.Fatalf("expected result payload, got %+v", stored.Result)
	}
}

func TestServiceDeadLettersUnknownHandlers(t *testing.T) {
	svc := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	job, err := svc.Enqueue("missing.handler", nil)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	waitForStatus(t, svc, job.ID, StatusDeadLetter)
}

func TestServiceRetriesFailedHandlers(t *testing.T) {
	svc := NewService()
	attempts := 0
	svc.RegisterHandler("analytics.recompute.current_state", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		attempts++
		return nil, errors.New("boom")
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	job, err := svc.Enqueue("analytics.recompute.current_state", nil)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	waitForStatus(t, svc, job.ID, StatusDeadLetter)
	if attempts != maxAttempts {
		t.Fatalf("expected %d attempts, got %d", maxAttempts, attempts)
	}
}

func TestServiceReportsFailureWhenHandlersFail(t *testing.T) {
	svc := NewService()
	var successes atomic.Int32
	var failures atomic.Int32
	svc.SetHealthHooks(func() {
		successes.Add(1)
	}, func(error) {
		failures.Add(1)
	})
	svc.RegisterHandler("analytics.recompute.current_state", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		return nil, errors.New("boom")
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	job, err := svc.Enqueue("analytics.recompute.current_state", nil)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	waitForStatus(t, svc, job.ID, StatusDeadLetter)
	if failures.Load() == 0 {
		t.Fatal("expected failure health hook to run")
	}
	if successes.Load() != 0 {
		t.Fatalf("expected no success health hook, got %d", successes.Load())
	}
}

func TestEnqueueUniqueReturnsExistingJob(t *testing.T) {
	svc := NewService()
	first, err := svc.EnqueueUnique("analytics.capture_snapshot", nil, "bucket:1")
	if err != nil {
		t.Fatalf("enqueue unique failed: %v", err)
	}
	second, err := svc.EnqueueUnique("analytics.capture_snapshot", nil, "bucket:1")
	if err != nil {
		t.Fatalf("enqueue duplicate unique failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected duplicate enqueue to return same job, got %s and %s", first.ID, second.ID)
	}
}

func TestServiceRenewsLeaseForLongRunningJobs(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(repo)
	svc.lease = 50 * time.Millisecond
	svc.pollInterval = 10 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	var renewals atomic.Int32

	svc.RegisterHandler("search.rebuild_large_index", func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		close(started)
		select {
		case <-release:
			return map[string]any{"ok": true}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	job, err := svc.Enqueue("search.rebuild_large_index", nil)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	<-started
	time.Sleep(120 * time.Millisecond)
	stored, ok := repo.Get(job.ID)
	if !ok {
		t.Fatal("expected stored job while running")
	}
	if stored.Status != StatusRunning {
		t.Fatalf("expected running job, got %+v", stored)
	}
	if time.Until(stored.LeaseExpiresAt) <= 0 {
		t.Fatalf("expected renewed lease to still be active, got %+v", stored)
	}
	claimed := repo.ClaimPending(time.Now().UTC(), svc.lease, 1)
	if len(claimed) != 0 {
		t.Fatalf("expected long-running job to remain unclaimable, got %+v", claimed)
	}
	renewals.Store(1)
	close(release)
	waitForStatus(t, svc, job.ID, StatusSucceeded)
	if renewals.Load() == 0 {
		t.Fatal("expected renewals to be observed during long-running job")
	}
}

func waitForStatus(t *testing.T, svc *Service, jobID string, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := svc.Get(jobID)
		if ok && job.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := svc.Get(jobID)
	t.Fatalf("expected status %s, got %+v", want, job)
}
