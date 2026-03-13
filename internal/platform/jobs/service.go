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

	mu       sync.RWMutex
	handlers map[string]Handler
	cancel   context.CancelFunc
	wg       sync.WaitGroup
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

func (s *Service) Enqueue(name string, payload map[string]any) (Job, error) {
	return s.EnqueueUnique(name, payload, "")
}

func (s *Service) EnqueueUnique(name string, payload map[string]any, dedupKey string) (Job, error) {
	now := time.Now().UTC()
	job := Job{
		ID:        fmt.Sprintf("job:%d", now.UnixNano()),
		Name:      name,
		DedupKey:  dedupKey,
		Status:    StatusQueued,
		CreatedAt: now,
		Payload:   cloneMap(payload),
	}
	stored, _, err := s.repo.Enqueue(job)
	return stored, err
}

func (s *Service) Get(id string) (Job, bool) {
	return s.repo.Get(id)
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
		if err := s.processOnce(ctx); err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(minDuration(s.pollInterval, 250*time.Millisecond)):
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) processOnce(ctx context.Context) error {
	items := s.repo.ClaimPending(time.Now().UTC(), s.lease, s.limit)
	for _, job := range items {
		handler := s.handler(job.Name)
		if handler == nil {
			if err := s.repo.MarkFailed(job.ID, StatusDeadLetter, "job handler is not registered", time.Now().UTC()); err != nil {
				return err
			}
			continue
		}
		jobCtx, stopRenew := s.startLeaseHeartbeat(ctx, job.ID)
		result, err := handler(jobCtx, cloneMap(job.Payload))
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
				return markErr
			}
			continue
		}
		if err := s.repo.MarkSucceeded(job.ID, result, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
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
