CREATE TABLE IF NOT EXISTS job_records (
    job_id TEXT PRIMARY KEY,
    job_name TEXT NOT NULL,
    dedup_key TEXT UNIQUE,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_job_records_status_lease_created
    ON job_records (status, lease_expires_at, created_at);
