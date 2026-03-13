package analytics

import (
	"context"
	"time"
)

type Scheduler struct {
	service   *Service
	interval  time.Duration
	retention time.Duration
	cancel    context.CancelFunc
	onSuccess func()
	onFailure func(error)
}

func NewScheduler(service *Service, interval, retention time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	return &Scheduler{service: service, interval: interval, retention: retention}
}

func (s *Scheduler) SetHealthHooks(onSuccess func(), onFailure func(error)) {
	if s == nil {
		return
	}
	s.onSuccess = onSuccess
	s.onFailure = onFailure
}

func (s *Scheduler) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.enqueueTick(time.Now().UTC()); err != nil {
					if s.onFailure != nil {
						s.onFailure(err)
					}
					continue
				}
				if s.onSuccess != nil {
					s.onSuccess()
				}
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scheduler) enqueueTick(now time.Time) error {
	if s.service == nil || s.service.jobs == nil {
		s.runTickInline(now)
		return nil
	}
	bucket := now.Truncate(s.interval).Format(time.RFC3339)
	cutoff := now.Add(-s.retention).Format(time.RFC3339)
	var tickErr error
	s.enqueueOrRun(JobCaptureSnapshot, nil, JobCaptureSnapshot+":"+bucket, func() {
		_, _ = s.service.CaptureSnapshot()
	}, &tickErr)
	s.enqueueOrRun(JobRunDueReports, map[string]any{"now": now.Format(time.RFC3339)}, JobRunDueReports+":"+bucket, func() {
		_ = s.service.RunDueReports(now)
	}, &tickErr)
	s.enqueueOrRun(JobCleanupSnapshots, map[string]any{"cutoff": cutoff}, JobCleanupSnapshots+":"+bucket, func() {
		_ = s.service.DeleteOlderThan(now.Add(-s.retention))
	}, &tickErr)
	s.enqueueOrRun(JobCleanupReports, map[string]any{"cutoff": cutoff}, JobCleanupReports+":"+bucket, func() {
		_ = s.service.CleanupReportData(now.Add(-s.retention))
	}, &tickErr)
	return tickErr
}

func (s *Scheduler) enqueueOrRun(name string, payload map[string]any, dedupKey string, fallback func(), tickErr *error) {
	job, created, err := s.service.jobs.EnqueueUniqueDetailed(name, payload, dedupKey)
	if err == nil {
		if created {
			s.service.observeCounter("analytics.scheduler.enqueued.total")
		} else if job.ID != "" {
			s.service.observeCounter("analytics.scheduler.already_claimed.total")
		}
		return
	}
	s.service.observeCounter("analytics.scheduler.enqueue_failed.total")
	if tickErr != nil && *tickErr == nil {
		*tickErr = err
	}
	if fallback != nil {
		fallback()
	}
}

func (s *Scheduler) runTickInline(now time.Time) {
	_, _ = s.service.CaptureSnapshot()
	_ = s.service.RunDueReports(now)
	_ = s.service.DeleteOlderThan(now.Add(-s.retention))
	_ = s.service.CleanupReportData(now.Add(-s.retention))
}
