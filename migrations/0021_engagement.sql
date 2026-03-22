CREATE TABLE IF NOT EXISTS engagement_programs (
    program_key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    subject_type TEXT,
    status TEXT NOT NULL,
    published_version INTEGER,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT
);

CREATE TABLE IF NOT EXISTS engagement_program_versions (
    program_key TEXT NOT NULL,
    version_no INTEGER NOT NULL,
    status TEXT NOT NULL,
    change_note TEXT,
    rules_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,
    published_by TEXT,
    last_error TEXT,
    last_replay_id TEXT,
    PRIMARY KEY (program_key, version_no)
);

CREATE TABLE IF NOT EXISTS engagement_journal_entries (
    entry_id TEXT PRIMARY KEY,
    program_key TEXT NOT NULL,
    version_no INTEGER NOT NULL,
    subject_id TEXT NOT NULL,
    account_key TEXT NOT NULL,
    entry_type TEXT NOT NULL,
    amount INTEGER NOT NULL,
    rule_key TEXT,
    event_id TEXT,
    event_type TEXT,
    correlation_id TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_engagement_journal_program_subject_account
    ON engagement_journal_entries (program_key, subject_id, account_key, occurred_at);

CREATE TABLE IF NOT EXISTS engagement_balances (
    program_key TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    account_key TEXT NOT NULL,
    balance INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (program_key, subject_id, account_key)
);

CREATE TABLE IF NOT EXISTS engagement_qualifications (
    program_key TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    tier_key TEXT,
    score INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (program_key, subject_id)
);

CREATE TABLE IF NOT EXISTS engagement_achievements (
    grant_id TEXT PRIMARY KEY,
    program_key TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    achievement_key TEXT NOT NULL,
    rule_key TEXT,
    event_id TEXT,
    granted_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_engagement_achievements_program_subject
    ON engagement_achievements (program_key, subject_id, granted_at);

CREATE UNIQUE INDEX IF NOT EXISTS uq_engagement_achievements_program_subject_key
    ON engagement_achievements (program_key, subject_id, achievement_key);

CREATE TABLE IF NOT EXISTS engagement_processed_events (
    idempotency_key TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS engagement_consumers (
    consumer_id TEXT PRIMARY KEY,
    program_key TEXT NOT NULL,
    version_no INTEGER NOT NULL,
    event_types_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    processed INTEGER NOT NULL DEFAULT 0,
    last_event_id TEXT,
    last_event_at TIMESTAMPTZ,
    last_error TEXT,
    status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS engagement_replay_runs (
    replay_run_id TEXT PRIMARY KEY,
    program_key TEXT NOT NULL,
    version_no INTEGER NOT NULL,
    status TEXT NOT NULL,
    matching_events INTEGER NOT NULL DEFAULT 0,
    processed INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_by TEXT,
    validation_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    job_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_engagement_replay_runs_program_started
    ON engagement_replay_runs (program_key, started_at DESC);
