ALTER TABLE domain_events
    ADD COLUMN IF NOT EXISTS schema_version TEXT,
    ADD COLUMN IF NOT EXISTS correlation_id TEXT,
    ADD COLUMN IF NOT EXISTS organization_id TEXT,
    ADD COLUMN IF NOT EXISTS location_id TEXT,
    ADD COLUMN IF NOT EXISTS module_key TEXT;

ALTER TABLE dead_letter_records
    ADD COLUMN IF NOT EXISTS sink_name TEXT;

CREATE TABLE IF NOT EXISTS outbox_deliveries (
    delivery_id TEXT PRIMARY KEY,
    outbox_id TEXT NOT NULL REFERENCES outbox_records (outbox_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL REFERENCES domain_events (event_id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    sink_name TEXT NOT NULL,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dispatched_at TIMESTAMPTZ,
    UNIQUE (outbox_id, sink_name)
);

CREATE INDEX IF NOT EXISTS idx_outbox_deliveries_status_created ON outbox_deliveries (status, created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_deliveries_outbox ON outbox_deliveries (outbox_id);
