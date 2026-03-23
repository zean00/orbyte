CREATE TABLE IF NOT EXISTS deep_link_grants (
    deep_link_grant_id TEXT PRIMARY KEY,
    grant_kind TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    location_id TEXT NULL REFERENCES locations (location_id),
    allowed_permission_keys_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_actions_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    review_only BOOLEAN NOT NULL DEFAULT FALSE,
    require_step_up BOOLEAN NOT NULL DEFAULT FALSE,
    one_time BOOLEAN NOT NULL DEFAULT TRUE,
    title TEXT NULL,
    message TEXT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    activated_at TIMESTAMPTZ NULL,
    consumed_at TIMESTAMPTZ NULL,
    consumed_by_action TEXT NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_deep_link_grants_target_user
    ON deep_link_grants (target_type, target_id, user_id);

CREATE INDEX IF NOT EXISTS idx_deep_link_grants_status
    ON deep_link_grants (status);

CREATE INDEX IF NOT EXISTS idx_deep_link_grants_expires_at
    ON deep_link_grants (expires_at);
