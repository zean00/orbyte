ALTER TABLE integration_systems
    ADD COLUMN IF NOT EXISTS connector_key TEXT NULL;

ALTER TABLE integration_submissions
    ADD COLUMN IF NOT EXISTS endpoint_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS contract_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS contract_version INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS intent TEXT NULL,
    ADD COLUMN IF NOT EXISTS mode TEXT NULL,
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT NULL;

CREATE TABLE IF NOT EXISTS integration_endpoints (
    endpoint_key TEXT PRIMARY KEY,
    system_key TEXT NOT NULL REFERENCES integration_systems (system_key),
    name TEXT NOT NULL,
    direction TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT NOT NULL,
    connector_key TEXT NOT NULL,
    description TEXT NULL,
    settings_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS integration_contracts (
    contract_key TEXT NOT NULL,
    version INTEGER NOT NULL,
    name TEXT NOT NULL,
    direction TEXT NOT NULL,
    intent TEXT NOT NULL,
    status TEXT NOT NULL,
    description TEXT NULL,
    schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (contract_key, version)
);

CREATE TABLE IF NOT EXISTS integration_mappings (
    mapping_key TEXT PRIMARY KEY,
    system_key TEXT NOT NULL REFERENCES integration_systems (system_key),
    endpoint_key TEXT NULL REFERENCES integration_endpoints (endpoint_key),
    contract_key TEXT NOT NULL,
    contract_version INTEGER NOT NULL,
    direction TEXT NOT NULL,
    status TEXT NOT NULL,
    rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (contract_key, contract_version) REFERENCES integration_contracts (contract_key, version)
);

CREATE TABLE IF NOT EXISTS integration_submission_attempts (
    attempt_id TEXT PRIMARY KEY,
    submission_id TEXT NOT NULL REFERENCES integration_submissions (submission_id),
    attempt_number INTEGER NOT NULL,
    status TEXT NOT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    request_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS integration_dead_letters (
    dead_letter_id TEXT PRIMARY KEY,
    submission_id TEXT NOT NULL REFERENCES integration_submissions (submission_id),
    external_system_key TEXT NOT NULL REFERENCES integration_systems (system_key),
    endpoint_key TEXT NULL REFERENCES integration_endpoints (endpoint_key),
    contract_key TEXT NULL,
    contract_version INTEGER NOT NULL DEFAULT 0,
    intent TEXT NULL,
    status TEXT NOT NULL,
    last_error TEXT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_integration_submissions_contract_status
    ON integration_submissions (contract_key, contract_version, status, created_at);

CREATE INDEX IF NOT EXISTS idx_integration_submission_attempts_submission
    ON integration_submission_attempts (submission_id, attempt_number);

CREATE INDEX IF NOT EXISTS idx_integration_dead_letters_system_status
    ON integration_dead_letters (external_system_key, status, created_at);
