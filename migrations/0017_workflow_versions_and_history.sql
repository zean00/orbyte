ALTER TABLE workflow_definitions
    ADD COLUMN IF NOT EXISTS version_no INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE workflow_tasks
    ADD COLUMN IF NOT EXISTS workflow_version INTEGER NOT NULL DEFAULT 1;

ALTER TABLE workflow_approvals
    ADD COLUMN IF NOT EXISTS workflow_version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS workflow_definition_versions (
    workflow_key TEXT NOT NULL,
    version_no INTEGER NOT NULL,
    status TEXT NOT NULL,
    states_json JSONB NOT NULL,
    actions_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NULL,
    published_at TIMESTAMPTZ NULL,
    published_by TEXT NULL,
    PRIMARY KEY (workflow_key, version_no)
);

INSERT INTO workflow_definition_versions (
    workflow_key, version_no, status, states_json, actions_json, created_at, updated_at, published_at
)
SELECT workflow_key, COALESCE(version_no, 1), 'published', states_json, actions_json, NOW(), COALESCE(updated_at, NOW()), NOW()
FROM workflow_definitions
ON CONFLICT (workflow_key, version_no) DO NOTHING;

CREATE TABLE IF NOT EXISTS workflow_history (
    history_id TEXT PRIMARY KEY,
    workflow_key TEXT NOT NULL,
    workflow_version INTEGER NOT NULL DEFAULT 1,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    action TEXT NOT NULL,
    from_state TEXT NULL,
    to_state TEXT NULL,
    actor_id TEXT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    decision_code TEXT NULL,
    decision_reason TEXT NULL,
    assignment_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_workflow_definition_versions_key_status
    ON workflow_definition_versions (workflow_key, status, version_no);

CREATE INDEX IF NOT EXISTS idx_workflow_history_target
    ON workflow_history (target_type, target_id, occurred_at);
