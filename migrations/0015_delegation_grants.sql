ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS on_behalf_of_user_id TEXT NULL,
    ADD COLUMN IF NOT EXISTS delegation_grant_id TEXT NULL;

CREATE INDEX IF NOT EXISTS idx_audit_events_on_behalf_of ON audit_events (on_behalf_of_user_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_delegation_grant ON audit_events (delegation_grant_id, occurred_at);

CREATE TABLE IF NOT EXISTS delegation_grants (
    delegation_grant_id TEXT PRIMARY KEY,
    grantor_user_id TEXT NOT NULL REFERENCES users (user_id),
    delegate_kind TEXT NULL,
    delegate_id TEXT NULL,
    delegate_user_id TEXT NULL REFERENCES users (user_id),
    status TEXT NOT NULL,
    location_id TEXT NOT NULL REFERENCES locations (location_id),
    allowed_permission_keys_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_document_types_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    reason TEXT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ NULL,
    accepted_by_kind TEXT NULL,
    accepted_by_id TEXT NULL,
    accepted_by_user_id TEXT NULL REFERENCES users (user_id),
    revoked_at TIMESTAMPTZ NULL,
    revoked_by_user_id TEXT NULL REFERENCES users (user_id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE delegation_grants
    ADD COLUMN IF NOT EXISTS delegate_kind TEXT NULL,
    ADD COLUMN IF NOT EXISTS delegate_id TEXT NULL,
    ADD COLUMN IF NOT EXISTS accepted_by_kind TEXT NULL,
    ADD COLUMN IF NOT EXISTS accepted_by_id TEXT NULL;

UPDATE delegation_grants
SET delegate_kind = 'user'
WHERE delegate_kind IS NULL AND delegate_user_id IS NOT NULL;

UPDATE delegation_grants
SET delegate_id = delegate_user_id
WHERE (delegate_id IS NULL OR delegate_id = '') AND delegate_user_id IS NOT NULL;

UPDATE delegation_grants
SET accepted_by_kind = 'user'
WHERE accepted_by_kind IS NULL AND accepted_by_user_id IS NOT NULL;

UPDATE delegation_grants
SET accepted_by_id = accepted_by_user_id
WHERE (accepted_by_id IS NULL OR accepted_by_id = '') AND accepted_by_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_delegation_grants_grantor ON delegation_grants (grantor_user_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_delegation_grants_delegate ON delegation_grants (delegate_user_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_delegation_grants_delegate_target ON delegation_grants (delegate_kind, delegate_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_delegation_grants_location ON delegation_grants (location_id, status, expires_at);
