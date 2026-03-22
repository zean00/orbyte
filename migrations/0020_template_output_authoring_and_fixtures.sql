ALTER TABLE template_output_versions
    ADD COLUMN IF NOT EXISTS change_note TEXT,
    ADD COLUMN IF NOT EXISTS cloned_from_version INTEGER,
    ADD COLUMN IF NOT EXISTS last_previewed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_render_status TEXT,
    ADD COLUMN IF NOT EXISTS last_render_error TEXT,
    ADD COLUMN IF NOT EXISTS last_rendered_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS template_output_fixtures (
    fixture_key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    target_kind TEXT NOT NULL,
    template_key TEXT,
    source_type TEXT,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_template_output_fixtures_lookup
    ON template_output_fixtures (target_kind, template_key);
