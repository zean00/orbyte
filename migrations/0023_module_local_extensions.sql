CREATE TABLE IF NOT EXISTS module_local_extension_activations (
    base_module_key TEXT NOT NULL REFERENCES installed_modules (module_key) ON DELETE CASCADE,
    extension_module_key TEXT NOT NULL REFERENCES installed_modules (module_key) ON DELETE CASCADE,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL,
    PRIMARY KEY (base_module_key, scope_type, scope_id)
);

CREATE INDEX IF NOT EXISTS idx_module_local_extension_extension
    ON module_local_extension_activations (extension_module_key);
