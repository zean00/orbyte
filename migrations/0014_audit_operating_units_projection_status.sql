ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS actor_kind TEXT NULL,
    ADD COLUMN IF NOT EXISTS organization_id TEXT NULL,
    ADD COLUMN IF NOT EXISTS location_id TEXT NULL,
    ADD COLUMN IF NOT EXISTS operating_unit_id TEXT NULL,
    ADD COLUMN IF NOT EXISTS request_id TEXT NULL,
    ADD COLUMN IF NOT EXISTS change_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_audit_events_actor ON audit_events (actor_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_correlation ON audit_events (correlation_id, occurred_at);

CREATE TABLE IF NOT EXISTS operating_units (
    operating_unit_id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations (organization_id),
    location_id TEXT NULL REFERENCES locations (location_id),
    operating_unit_key TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_operating_units_scope ON operating_units (organization_id, location_id, status);

CREATE TABLE IF NOT EXISTS search_projection_status (
    projection_key TEXT PRIMARY KEY,
    last_refresh_status TEXT NOT NULL,
    last_success_at TIMESTAMPTZ NULL,
    last_failure_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    last_rebuild_started_at TIMESTAMPTZ NULL,
    last_rebuild_finished_at TIMESTAMPTZ NULL,
    last_rebuild_count INTEGER NOT NULL DEFAULT 0,
    source_count INTEGER NOT NULL DEFAULT 0,
    projection_count INTEGER NOT NULL DEFAULT 0,
    stale_count INTEGER NOT NULL DEFAULT 0,
    missing_count INTEGER NOT NULL DEFAULT 0
);
