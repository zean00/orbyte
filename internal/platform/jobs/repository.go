package jobs

import "time"

type Repository interface {
	Enqueue(job Job) (Job, bool, error)
	Get(id string) (Job, bool)
	ClaimPending(now time.Time, lease time.Duration, limit int) []Job
	RenewLease(id string, now time.Time, lease time.Duration) error
	MarkSucceeded(id string, result map[string]any, endedAt time.Time) error
	MarkFailed(id string, status string, lastError string, endedAt time.Time) error
}
