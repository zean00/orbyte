CREATE TABLE IF NOT EXISTS template_output_versions (
    template_key TEXT NOT NULL,
    version_no INTEGER NOT NULL,
    status TEXT NOT NULL,
    renderer_kind TEXT NOT NULL,
    body TEXT NOT NULL,
    style TEXT,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT,
    published_at TIMESTAMPTZ,
    published_by TEXT,
    PRIMARY KEY (template_key, version_no)
);

CREATE TABLE IF NOT EXISTS template_output_bindings (
    binding_id TEXT PRIMARY KEY,
    template_key TEXT NOT NULL,
    scope_type TEXT NOT NULL,
    scope_id TEXT,
    target_kind TEXT NOT NULL,
    target_key TEXT NOT NULL,
    purpose TEXT,
    channel TEXT,
    is_default BOOLEAN NOT NULL DEFAULT TRUE,
    is_official BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_template_output_bindings_lookup
    ON template_output_bindings (target_kind, target_key, scope_type, scope_id);
