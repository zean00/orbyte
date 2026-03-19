CREATE TABLE IF NOT EXISTS feature_flag_definitions (
    flag_key TEXT PRIMARY KEY,
    module_key TEXT NOT NULL,
    description TEXT,
    allowed_scopes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_state BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS feature_flag_values (
    flag_key TEXT NOT NULL REFERENCES feature_flag_definitions(flag_key) ON DELETE CASCADE,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL,
    status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL,
    effective_from TIMESTAMPTZ,
    effective_to TIMESTAMPTZ,
    PRIMARY KEY (flag_key, scope_type, scope_id)
);
