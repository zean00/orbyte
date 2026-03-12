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
				_, _ = s.service.CaptureSnapshot()
				_ = s.service.RunDueReports(time.Now().UTC())
				_ = s.service.DeleteOlderThan(time.Now().UTC().Add(-s.retention))
				_ = s.service.CleanupReportData(time.Now().UTC().Add(-s.retention))
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}
