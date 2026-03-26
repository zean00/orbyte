ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS login_step_up_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS approval_step_up_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS approval_step_up_until TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS user_totp_enrollments (
    user_id TEXT PRIMARY KEY REFERENCES users (user_id) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    issuer TEXT NULL,
    account_name TEXT NULL,
    login_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    approval_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMPTZ NULL,
    disabled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_totp_enrollments_verified
    ON user_totp_enrollments (verified_at);

CREATE TABLE IF NOT EXISTS auth_challenges (
    challenge_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    username TEXT NOT NULL,
    auth_method TEXT NULL,
    current_location_id TEXT NULL,
    status TEXT NOT NULL,
    purpose TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NULL,
    client_metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_auth_challenges_user_status
    ON auth_challenges (user_id, status);

CREATE INDEX IF NOT EXISTS idx_auth_challenges_expires
    ON auth_challenges (expires_at);
