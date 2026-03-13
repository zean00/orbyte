package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Enqueue(job Job) (Job, bool, error) {
	if stringsTrim(job.DedupKey) != "" {
		if existing, ok := r.findByDedupKey(job.DedupKey); ok {
			return existing, false, nil
		}
	}
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return Job{}, false, err
	}
	const query = `
		INSERT INTO job_records (
			job_id, job_name, dedup_key, status, attempt_count, last_error,
			payload_json, result_json, created_at, started_at, ended_at, lease_expires_at
		) VALUES ($1,$2,NULLIF($3,''),$4,$5,NULLIF($6,''),$7,'{}'::jsonb,$8,NULL,NULL,NULL)`
	if _, err := r.db.ExecContext(context.Background(), query, job.ID, job.Name, job.DedupKey, job.Status, job.AttemptCount, job.Error, payload, job.CreatedAt); err != nil {
		if stringsTrim(job.DedupKey) != "" {
			if existing, ok := r.findByDedupKey(job.DedupKey); ok {
				return existing, false, nil
			}
		}
		return Job{}, false, err
	}
	return cloneJob(job), true, nil
}

func (r *PostgresRepository) findByDedupKey(dedupKey string) (Job, bool) {
	const query = `
		SELECT job_id, job_name, COALESCE(dedup_key,''), status, attempt_count, COALESCE(last_error,''),
			COALESCE(payload_json,'{}'::jsonb), COALESCE(result_json,'{}'::jsonb),
			created_at, started_at, ended_at, lease_expires_at
		FROM job_records
		WHERE dedup_key = $1`
	return scanJob(r.db.QueryRowContext(context.Background(), query, dedupKey))
}

func (r *PostgresRepository) Get(id string) (Job, bool) {
	const query = `
		SELECT job_id, job_name, COALESCE(dedup_key,''), status, attempt_count, COALESCE(last_error,''),
			COALESCE(payload_json,'{}'::jsonb), COALESCE(result_json,'{}'::jsonb),
			created_at, started_at, ended_at, lease_expires_at
		FROM job_records
		WHERE job_id = $1`
	return scanJob(r.db.QueryRowContext(context.Background(), query, id))
}

func (r *PostgresRepository) List() []Job {
	const query = `
		SELECT job_id, job_name, COALESCE(dedup_key,''), status, attempt_count, COALESCE(last_error,''),
			COALESCE(payload_json,'{}'::jsonb), COALESCE(result_json,'{}'::jsonb),
			created_at, started_at, ended_at, lease_expires_at
		FROM job_records
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Job, 0)
	for rows.Next() {
		if job, ok := scanJobFromRows(rows); ok {
			items = append(items, job)
		}
	}
	return items
}

func (r *PostgresRepository) ClaimPending(now time.Time, lease time.Duration, limit int) []Job {
	if limit <= 0 {
		limit = 20
	}
	const query = `
		WITH claimed AS (
			SELECT job_id
			FROM job_records
			WHERE status IN ('queued', 'running')
				AND (lease_expires_at IS NULL OR lease_expires_at <= $1)
			ORDER BY created_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE job_records
		SET status = 'running',
			attempt_count = attempt_count + 1,
			started_at = COALESCE(started_at, $1),
			ended_at = NULL,
			lease_expires_at = $3
		WHERE job_id IN (SELECT job_id FROM claimed)
		RETURNING job_id, job_name, COALESCE(dedup_key,''), status, attempt_count, COALESCE(last_error,''),
			COALESCE(payload_json,'{}'::jsonb), COALESCE(result_json,'{}'::jsonb),
			created_at, started_at, ended_at, lease_expires_at`
	rows, err := r.db.QueryContext(context.Background(), query, now, limit, now.Add(lease))
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Job, 0, limit)
	for rows.Next() {
		job, ok := scanJobFromRows(rows)
		if ok {
			items = append(items, job)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) RenewLease(id string, now time.Time, lease time.Duration) error {
	const query = `
		UPDATE job_records
		SET lease_expires_at = $1
		WHERE job_id = $2 AND status = 'running'`
	_, err := r.db.ExecContext(context.Background(), query, now.Add(lease), id)
	return err
}

func (r *PostgresRepository) MarkSucceeded(id string, result map[string]any, endedAt time.Time) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	const query = `
		UPDATE job_records
		SET status = $1, last_error = NULL, result_json = $2, ended_at = $3, lease_expires_at = NULL
		WHERE job_id = $4`
	_, err = r.db.ExecContext(context.Background(), query, StatusSucceeded, body, endedAt, id)
	return err
}

func (r *PostgresRepository) MarkFailed(id string, status string, lastError string, endedAt time.Time) error {
	const query = `
		UPDATE job_records
		SET status = $1, last_error = NULLIF($2,''), ended_at = $3, lease_expires_at = NULL
		WHERE job_id = $4`
	_, err := r.db.ExecContext(context.Background(), query, status, lastError, endedAt, id)
	return err
}

type jobScanner interface {
	Scan(dest ...any) error
}

func scanJob(scanner jobScanner) (Job, bool) {
	var (
		job       Job
		payload   []byte
		result    []byte
		startedAt sql.NullTime
		endedAt   sql.NullTime
		leaseAt   sql.NullTime
	)
	if err := scanner.Scan(&job.ID, &job.Name, &job.DedupKey, &job.Status, &job.AttemptCount, &job.Error, &payload, &result, &job.CreatedAt, &startedAt, &endedAt, &leaseAt); err != nil {
		return Job{}, false
	}
	_ = json.Unmarshal(payload, &job.Payload)
	_ = json.Unmarshal(result, &job.Result)
	if startedAt.Valid {
		job.StartedAt = startedAt.Time
	}
	if endedAt.Valid {
		job.EndedAt = endedAt.Time
	}
	if leaseAt.Valid {
		job.LeaseExpiresAt = leaseAt.Time
	}
	return job, true
}

func scanJobFromRows(rows *sql.Rows) (Job, bool) {
	return scanJob(rows)
}

func stringsTrim(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\n' || value[0] == '\t') {
		value = value[1:]
	}
	for len(value) > 0 && (value[len(value)-1] == ' ' || value[len(value)-1] == '\n' || value[len(value)-1] == '\t') {
		value = value[:len(value)-1]
	}
	return value
}
