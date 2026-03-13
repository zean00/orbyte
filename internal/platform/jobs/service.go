package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	StatusQueued     = "queued"
	StatusRunning    = "running"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusDeadLetter = "dead_letter"
	maxAttempts      = 3
)

type Job struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	DedupKey       string         `json:"dedup_key,omitempty"`
	Status         string         `json:"status"`
	Error          string         `json:"error,omitempty"`
	AttemptCount   int            `json:"attempt_count"`
	CreatedAt      time.Time      `json:"created_at"`
	StartedAt      time.Time      `json:"started_at,omitempty"`
	EndedAt        time.Time      `json:"ended_at,omitempty"`
	LeaseExpiresAt time.Time      `json:"lease_expires_at,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
}

type Handler func(context.Context, map[string]any) (map[string]any, error)

type Service struct {
	repo         Repository
	pollInterval time.Duration
	lease        time.Duration
	limit        int

	mu        sync.RWMutex
	handlers  map[string]Handler
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	onSuccess func()
	onFailure func(error)
}

type Summary struct {
	Queued     int `json:"queued"`
	Running    int `json:"running"`
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
	DeadLetter int `json:"dead_letter"`
}

type processResult struct {
	Claimed   int
	Succeeded int
	Failed    int
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	if repo == nil {
		repo = NewMemoryRepository()
	}
	return &Service{
		repo:         repo,
		pollInterval: 100 * time.Millisecond,
		lease:        5 * time.Second,
		limit:        20,
		handlers:     map[string]Handler{},
	}
}

func (s *Service) RegisterHandler(name string, handler Handler) {
	if name == "" || handler == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[name] = handler
}

func (s *Service) SetHealthHooks(onSuccess func(), onFailure func(error)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSuccess = onSuccess
	s.onFailure = onFailure
}

func (s *Service) Enqueue(name string, payload map[string]any) (Job, error) {
	return s.EnqueueUnique(name, payload, "")
}

func (s *Service) EnqueueUnique(name string, payload map[string]any, dedupKey string) (Job, error) {
	job, _, err := s.EnqueueUniqueDetailed(name, payload, dedupKey)
	return job, err
}

func (s *Service) EnqueueUniqueDetailed(name string, payload map[string]any, dedupKey string) (Job, bool, error) {
	now := time.Now().UTC()
	job := Job{
		ID:        fmt.Sprintf("job:%d", now.UnixNano()),
		Name:      name,
		DedupKey:  dedupKey,
		Status:    StatusQueued,
		CreatedAt: now,
		Payload:   cloneMap(payload),
	}
	return s.repo.Enqueue(job)
}

func (s *Service) Get(id string) (Job, bool) {
	return s.repo.Get(id)
}

func (s *Service) List() []Job {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.List()
}

func (s *Service) Summary() Summary {
	summary := Summary{}
	for _, job := range s.List() {
		switch job.Status {
		case StatusQueued:
			summary.Queued++
		case StatusRunning:
			summary.Running++
		case StatusSucceeded:
			summary.Succeeded++
		case StatusFailed:
			summary.Failed++
		case StatusDeadLetter:
			summary.DeadLetter++
		}
	}
	return summary
}

func (s *Service) Start(parent context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go s.loop(ctx)
}

func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *Service) loop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		result, err := s.processOnce(ctx)
		if err != nil {
			s.reportFailure(err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(minDuration(s.pollInterval, 250*time.Millisecond)):
			}
		} else if result.Failed > 0 {
			s.reportFailure(fmt.Errorf("%d job handler(s) failed", result.Failed))
		} else if result.Succeeded > 0 {
			s.reportSuccess()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) reportSuccess() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.onSuccess != nil {
		s.onSuccess()
	}
}

func (s *Service) reportFailure(err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.onFailure != nil {
		s.onFailure(err)
	}
}

func (s *Service) processOnce(ctx context.Context) (processResult, error) {
	outcome := processResult{}
	items := s.repo.ClaimPending(time.Now().UTC(), s.lease, s.limit)
	outcome.Claimed = len(items)
	for _, job := range items {
		handler := s.handler(job.Name)
		if handler == nil {
			if err := s.repo.MarkFailed(job.ID, StatusDeadLetter, "job handler is not registered", time.Now().UTC()); err != nil {
				return outcome, err
			}
			outcome.Failed++
			continue
		}
		jobCtx, stopRenew := s.startLeaseHeartbeat(ctx, job.ID)
		handlerResult, err := handler(jobCtx, cloneMap(job.Payload))
		renewErr := stopRenew()
		if err == nil && renewErr != nil {
			err = renewErr
		}
		if err != nil {
			status := StatusFailed
			if job.AttemptCount >= maxAttempts {
				status = StatusDeadLetter
			} else {
				status = StatusQueued
			}
			if markErr := s.repo.MarkFailed(job.ID, status, err.Error(), time.Now().UTC()); markErr != nil {
				return outcome, markErr
			}
			outcome.Failed++
			continue
		}
		if err := s.repo.MarkSucceeded(job.ID, handlerResult, time.Now().UTC()); err != nil {
			return outcome, err
		}
		outcome.Succeeded++
	}
	return outcome, nil
}

func (s *Service) startLeaseHeartbeat(parent context.Context, jobID string) (context.Context, func() error) {
	if s == nil || s.repo == nil || s.lease <= 0 {
		return parent, func() error { return nil }
	}
	interval := s.lease / 2
	if interval <= 0 {
		interval = time.Second
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.repo.RenewLease(jobID, time.Now().UTC(), s.lease); err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
			}
		}
	}()
	return ctx, func() error {
		cancel()
		<-done
		select {
		case err := <-errCh:
			return err
		default:
			return nil
		}
	}
}

func (s *Service) handler(name string) Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handlers[name]
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneJob(job Job) Job {
	job.Payload = cloneMap(job.Payload)
	job.Result = cloneMap(job.Result)
	return job
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
