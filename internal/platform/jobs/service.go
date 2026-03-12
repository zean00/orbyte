package jobs

import (
	"fmt"
	"sync"
	"time"
)

type Job struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	StartedAt time.Time      `json:"started_at,omitempty"`
	EndedAt   time.Time      `json:"ended_at,omitempty"`
	Result    map[string]any `json:"result,omitempty"`
}

type Service struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

func NewService() *Service {
	return &Service{jobs: map[string]Job{}}
}

func (s *Service) Enqueue(name string, fn func() (map[string]any, error)) Job {
	now := time.Now().UTC()
	job := Job{
		ID:        fmt.Sprintf("job:%d", now.UnixNano()),
		Name:      name,
		Status:    "queued",
		CreatedAt: now,
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	go s.run(job.ID, fn)
	return job
}

func (s *Service) run(id string, fn func() (map[string]any, error)) {
	s.mu.Lock()
	job := s.jobs[id]
	job.Status = "running"
	job.StartedAt = time.Now().UTC()
	s.jobs[id] = job
	s.mu.Unlock()

	result, err := fn()

	s.mu.Lock()
	job = s.jobs[id]
	job.EndedAt = time.Now().UTC()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
	} else {
		job.Status = "succeeded"
		job.Result = result
	}
	s.jobs[id] = job
	s.mu.Unlock()
}

func (s *Service) Get(id string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}
