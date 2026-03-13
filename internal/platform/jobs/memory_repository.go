package jobs

import (
	"sort"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	jobs      map[string]Job
	dedupKeys map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		jobs:      map[string]Job{},
		dedupKeys: map[string]string{},
	}
}

func (r *MemoryRepository) Enqueue(job Job) (Job, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job.DedupKey != "" {
		if existingID, ok := r.dedupKeys[job.DedupKey]; ok {
			if existing, ok := r.jobs[existingID]; ok {
				return existing, false, nil
			}
		}
	}
	r.jobs[job.ID] = cloneJob(job)
	if job.DedupKey != "" {
		r.dedupKeys[job.DedupKey] = job.ID
	}
	return cloneJob(job), true, nil
}

func (r *MemoryRepository) Get(id string) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.jobs[id]
	return cloneJob(job), ok
}

func (r *MemoryRepository) List() []Job {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]Job, 0, len(r.jobs))
	for _, job := range r.jobs {
		items = append(items, cloneJob(job))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *MemoryRepository) ClaimPending(now time.Time, lease time.Duration, limit int) []Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	ids := make([]string, 0, len(r.jobs))
	for id := range r.jobs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return r.jobs[ids[i]].CreatedAt.Before(r.jobs[ids[j]].CreatedAt)
	})
	items := make([]Job, 0, limit)
	for _, id := range ids {
		job := r.jobs[id]
		if job.Status != StatusQueued && !(job.Status == StatusRunning && !job.LeaseExpiresAt.IsZero() && !job.LeaseExpiresAt.After(now)) {
			continue
		}
		job.Status = StatusRunning
		job.AttemptCount++
		if job.StartedAt.IsZero() {
			job.StartedAt = now
		}
		job.EndedAt = time.Time{}
		job.LeaseExpiresAt = now.Add(lease)
		r.jobs[id] = job
		items = append(items, cloneJob(job))
		if len(items) >= limit {
			break
		}
	}
	return items
}

func (r *MemoryRepository) MarkSucceeded(id string, result map[string]any, endedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return nil
	}
	job.Status = StatusSucceeded
	job.Error = ""
	job.Result = cloneMap(result)
	job.EndedAt = endedAt
	job.LeaseExpiresAt = time.Time{}
	r.jobs[id] = job
	return nil
}

func (r *MemoryRepository) RenewLease(id string, now time.Time, lease time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok || job.Status != StatusRunning {
		return nil
	}
	job.LeaseExpiresAt = now.Add(lease)
	r.jobs[id] = job
	return nil
}

func (r *MemoryRepository) MarkFailed(id string, status string, lastError string, endedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return nil
	}
	job.Status = status
	job.Error = lastError
	job.EndedAt = endedAt
	job.LeaseExpiresAt = time.Time{}
	r.jobs[id] = job
	return nil
}
