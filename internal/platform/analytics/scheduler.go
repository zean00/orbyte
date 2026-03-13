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
				s.enqueueTick(time.Now().UTC())
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scheduler) enqueueTick(now time.Time) {
	if s.service == nil || s.service.jobs == nil {
		s.runTickInline(now)
		return
	}
	bucket := now.Truncate(s.interval).Format(time.RFC3339)
	cutoff := now.Add(-s.retention).Format(time.RFC3339)
	s.enqueueOrRun(JobCaptureSnapshot, nil, JobCaptureSnapshot+":"+bucket, func() {
		_, _ = s.service.CaptureSnapshot()
	})
	s.enqueueOrRun(JobRunDueReports, map[string]any{"now": now.Format(time.RFC3339)}, JobRunDueReports+":"+bucket, func() {
		_ = s.service.RunDueReports(now)
	})
	s.enqueueOrRun(JobCleanupSnapshots, map[string]any{"cutoff": cutoff}, JobCleanupSnapshots+":"+bucket, func() {
		_ = s.service.DeleteOlderThan(now.Add(-s.retention))
	})
	s.enqueueOrRun(JobCleanupReports, map[string]any{"cutoff": cutoff}, JobCleanupReports+":"+bucket, func() {
		_ = s.service.CleanupReportData(now.Add(-s.retention))
	})
}

func (s *Scheduler) enqueueOrRun(name string, payload map[string]any, dedupKey string, fallback func()) {
	if _, err := s.service.jobs.EnqueueUnique(name, payload, dedupKey); err != nil && fallback != nil {
		fallback()
	}
}

func (s *Scheduler) runTickInline(now time.Time) {
	_, _ = s.service.CaptureSnapshot()
	_ = s.service.RunDueReports(now)
	_ = s.service.DeleteOlderThan(now.Add(-s.retention))
	_ = s.service.CleanupReportData(now.Add(-s.retention))
}
