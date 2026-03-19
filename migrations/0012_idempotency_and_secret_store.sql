CREATE TABLE IF NOT EXISTS idempotency_records (
    operation_key TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    actor_id TEXT,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL,
    response_code INTEGER NOT NULL,
    response_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (operation_key, idempotency_key)
);

CREATE TABLE IF NOT EXISTS secret_store (
    secret_ref TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    secret_value TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
