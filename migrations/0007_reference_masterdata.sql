BEGIN;

CREATE TABLE IF NOT EXISTS reference_type_definitions (
    reference_type_key TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    owner_module_key TEXT NULL,
    value_type TEXT NOT NULL DEFAULT 'json',
    allowed_scopes_json JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE IF NOT EXISTS reference_records (
    reference_type_key TEXT NOT NULL REFERENCES reference_type_definitions (reference_type_key) ON DELETE CASCADE,
    reference_key TEXT NOT NULL,
    display_name TEXT NOT NULL,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    value_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    external_codes_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    effective_from TIMESTAMPTZ NULL,
    effective_to TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL,
    PRIMARY KEY (reference_type_key, reference_key, scope_type, scope_id)
);

COMMIT;
