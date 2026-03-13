BEGIN;

CREATE TABLE IF NOT EXISTS organizations (
    organization_id TEXT PRIMARY KEY,
    organization_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS locations (
    location_id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations (organization_id),
    location_key TEXT NOT NULL,
    name TEXT NOT NULL,
    location_type TEXT NOT NULL,
    status TEXT NOT NULL,
    parent_location_id TEXT NULL REFERENCES locations (location_id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (organization_id, location_key)
);

CREATE TABLE IF NOT EXISTS users (
    user_id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    authentication_subject TEXT NULL UNIQUE,
    status TEXT NOT NULL,
    default_location_id TEXT NULL REFERENCES locations (location_id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS user_credentials (
    user_id TEXT PRIMARY KEY REFERENCES users (user_id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    password_changed_at TIMESTAMPTZ NOT NULL,
    failed_attempt_count INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS roles (
    role_id TEXT PRIMARY KEY,
    role_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    scope_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS permissions (
    permission_key TEXT PRIMARY KEY,
    module_key TEXT NOT NULL,
    action_kind TEXT NOT NULL,
    resource_kind TEXT NOT NULL,
    description TEXT NULL
);

CREATE TABLE IF NOT EXISTS role_bindings (
    role_binding_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (user_id),
    role_id TEXT NOT NULL REFERENCES roles (role_id),
    scope_type TEXT NOT NULL,
    scope_id TEXT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ NULL,
    status TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id TEXT NOT NULL REFERENCES roles (role_id),
    permission_key TEXT NOT NULL REFERENCES permissions (permission_key),
    PRIMARY KEY (role_id, permission_key)
);

CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (user_id),
    status TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    authentication_method TEXT NULL,
    client_metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    current_location_scope TEXT NULL REFERENCES locations (location_id),
    revoked_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS auth_login_failures (
    failure_id TEXT PRIMARY KEY,
    throttle_key TEXT NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS service_principals (
    service_principal_id TEXT PRIMARY KEY,
    principal_key TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    allowed_operation_types_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    credential_ref TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS configuration_entries (
    configuration_key TEXT NOT NULL,
    module_key TEXT NOT NULL,
    category_key TEXT NOT NULL,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL DEFAULT '',
    value_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL,
    description TEXT,
    PRIMARY KEY (configuration_key, scope_type, scope_id)
);

CREATE TABLE IF NOT EXISTS model_definitions (
    model_key TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    owner_module_key TEXT NULL,
    version_key TEXT NOT NULL,
    create_permission_key TEXT NULL,
    list_permission_key TEXT NULL,
    read_permission_key TEXT NULL,
    update_permission_key TEXT NULL,
    default_sort TEXT NULL,
    fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    relations_json JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE IF NOT EXISTS model_records (
    model_key TEXT NOT NULL REFERENCES model_definitions (model_key) ON DELETE CASCADE,
    record_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    values_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (model_key, record_id)
);

CREATE TABLE IF NOT EXISTS installed_modules (
    module_key TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS document_definitions (
    document_type TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    workflow_key TEXT NULL,
    numbering_key TEXT NULL,
    owner_module_key TEXT NULL,
    allowed_link_types_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_attachment_types_json JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE IF NOT EXISTS document_extension_definitions (
    document_type TEXT NOT NULL REFERENCES document_definitions (document_type),
    module_key TEXT NOT NULL,
    display_name TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    read_permission_key TEXT NULL,
    write_permission_key TEXT NULL,
    PRIMARY KEY (document_type, module_key)
);

CREATE TABLE IF NOT EXISTS document_records (
    document_id TEXT PRIMARY KEY,
    document_type TEXT NOT NULL REFERENCES document_definitions (document_type),
    status TEXT NOT NULL,
    version INTEGER NOT NULL,
    etag TEXT NOT NULL,
    organization_id TEXT NOT NULL REFERENCES organizations (organization_id),
    location_id TEXT NULL REFERENCES locations (location_id),
    number TEXT NULL,
    created_by TEXT NOT NULL REFERENCES users (user_id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL REFERENCES users (user_id),
    updated_at TIMESTAMPTZ NOT NULL,
    submitted_by TEXT NULL REFERENCES users (user_id),
    submitted_at TIMESTAMPTZ NULL,
    schema_version TEXT NOT NULL,
    payload_json JSONB NOT NULL,
    content_hash TEXT NULL,
    total_amount_minor BIGINT NOT NULL DEFAULT 0,
    total_amount_currency TEXT NOT NULL DEFAULT 'IDR'
);

CREATE TABLE IF NOT EXISTS document_lines (
    document_line_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES document_records (document_id) ON DELETE CASCADE,
    line_no INTEGER NOT NULL,
    line_type TEXT NOT NULL,
    line_schema_ref TEXT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    amount_minor BIGINT NOT NULL DEFAULT 0,
    amount_currency TEXT NOT NULL DEFAULT 'IDR'
);

CREATE TABLE IF NOT EXISTS document_links (
    link_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES document_records (document_id) ON DELETE CASCADE,
    linked_document_id TEXT NOT NULL REFERENCES document_records (document_id) ON DELETE CASCADE,
    link_type TEXT NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS document_attachments (
    attachment_id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES document_records (document_id) ON DELETE CASCADE,
    attachment_type TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_events (
    audit_event_id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    from_state TEXT NULL,
    to_state TEXT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    correlation_id TEXT NULL
);

CREATE TABLE IF NOT EXISTS domain_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    event_version INTEGER NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    actor_id TEXT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS outbox_records (
    outbox_id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES domain_events (event_id),
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    dispatched_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS dead_letter_records (
    dead_letter_id TEXT PRIMARY KEY,
    outbox_id TEXT NOT NULL REFERENCES outbox_records (outbox_id),
    event_id TEXT NOT NULL REFERENCES domain_events (event_id),
    event_type TEXT NOT NULL,
    reason TEXT NOT NULL,
    attempt_count INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS search_document_summaries (
    document_id TEXT PRIMARY KEY REFERENCES document_records (document_id),
    document_type TEXT NOT NULL,
    status TEXT NOT NULL,
    version INTEGER NOT NULL,
    etag TEXT NOT NULL,
    organization_id TEXT NOT NULL REFERENCES organizations (organization_id),
    location_id TEXT NULL REFERENCES locations (location_id),
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS integration_systems (
    system_key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    adapter_key TEXT NOT NULL,
    description TEXT NULL,
    settings_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS integration_submissions (
    submission_id TEXT PRIMARY KEY,
    external_system_key TEXT NOT NULL REFERENCES integration_systems (system_key),
    operation_type TEXT NOT NULL,
    status TEXT NOT NULL,
    document_id TEXT NULL REFERENCES document_records (document_id),
    correlation_id TEXT NULL,
    external_reference TEXT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    processed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_integration_submissions_system_status
    ON integration_submissions (external_system_key, status, created_at);

CREATE TABLE IF NOT EXISTS workflow_definitions (
    workflow_key TEXT PRIMARY KEY,
    states_json JSONB NOT NULL,
    actions_json JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_tasks (
    task_id TEXT PRIMARY KEY,
    workflow_key TEXT NOT NULL REFERENCES workflow_definitions (workflow_key),
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    task_type TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_approvals (
    approval_id TEXT PRIMARY KEY,
    workflow_key TEXT NOT NULL REFERENCES workflow_definitions (workflow_key),
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    status TEXT NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_snapshots (
    snapshot_id TEXT PRIMARY KEY,
    generated_at TIMESTAMPTZ NOT NULL,
    window_key TEXT NOT NULL,
    payload_json JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_rollups (
    rollup_id TEXT PRIMARY KEY,
    granularity TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    bucket_end TIMESTAMPTZ NOT NULL,
    snapshot_count INTEGER NOT NULL,
    payload_json JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_document_facts (
    snapshot_id TEXT NOT NULL REFERENCES analytics_snapshots (snapshot_id),
    captured_at TIMESTAMPTZ NOT NULL,
    location_id TEXT NULL,
    document_type TEXT NULL,
    created_count INTEGER NOT NULL,
    draft_count INTEGER NOT NULL,
    submitted_count INTEGER NOT NULL,
    approved_count INTEGER NOT NULL,
    rejected_count INTEGER NOT NULL,
    cancelled_count INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_document_type_dim (
    document_type TEXT PRIMARY KEY,
    display_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_location_dim (
    location_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_report_definitions (
    report_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    dimension TEXT NOT NULL,
    format TEXT NOT NULL,
    window_key TEXT NOT NULL,
    location_id TEXT NULL,
    document_type TEXT NULL,
    delivery_channel TEXT NULL,
    delivery_target TEXT NULL,
    schedule_key TEXT NOT NULL,
    next_run_at TIMESTAMPTZ NOT NULL,
    enabled BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_report_runs (
    report_run_id TEXT PRIMARY KEY,
    report_id TEXT NOT NULL REFERENCES analytics_report_definitions (report_id),
    format TEXT NOT NULL,
    status TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    row_count INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_report_artifacts (
    artifact_id TEXT PRIMARY KEY,
    report_id TEXT NOT NULL REFERENCES analytics_report_definitions (report_id),
    report_run_id TEXT NOT NULL REFERENCES analytics_report_runs (report_run_id),
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    content_bytes BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_report_deliveries (
    delivery_id TEXT PRIMARY KEY,
    artifact_id TEXT NOT NULL REFERENCES analytics_report_artifacts (artifact_id),
    channel TEXT NOT NULL,
    recipient TEXT NULL,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    delivered_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS analytics_report_delivery_dead_letters (
    dead_letter_id TEXT PRIMARY KEY,
    artifact_id TEXT NOT NULL REFERENCES analytics_report_artifacts (artifact_id),
    channel TEXT NOT NULL,
    recipient TEXT NULL,
    reason TEXT NOT NULL,
    attempt_count INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_workflow_facts (
    snapshot_id TEXT NOT NULL REFERENCES analytics_snapshots (snapshot_id),
    captured_at TIMESTAMPTZ NOT NULL,
    pending_approvals INTEGER NOT NULL,
    open_tasks INTEGER NOT NULL,
    completed_tasks INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_reliability_facts (
    snapshot_id TEXT NOT NULL REFERENCES analytics_snapshots (snapshot_id),
    captured_at TIMESTAMPTZ NOT NULL,
    outbox_pending INTEGER NOT NULL,
    dead_letters INTEGER NOT NULL,
    dispatch_success BIGINT NOT NULL,
    dispatch_retries BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_locations_organization_status ON locations (organization_id, status);
CREATE INDEX IF NOT EXISTS idx_users_status ON users (status);
CREATE INDEX IF NOT EXISTS idx_user_credentials_locked_until ON user_credentials (locked_until);
CREATE INDEX IF NOT EXISTS idx_auth_login_failures_key_attempted ON auth_login_failures (throttle_key, attempted_at);
CREATE INDEX IF NOT EXISTS idx_document_records_type_status ON document_records (document_type, status);
CREATE INDEX IF NOT EXISTS idx_document_records_org_location ON document_records (organization_id, location_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_target ON audit_events (target_type, target_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_domain_events_aggregate ON domain_events (aggregate_type, aggregate_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_outbox_records_status_created ON outbox_records (status, created_at);
CREATE INDEX IF NOT EXISTS idx_dead_letter_records_created ON dead_letter_records (created_at);
CREATE INDEX IF NOT EXISTS idx_search_document_summaries_type_status ON search_document_summaries (document_type, status);
CREATE INDEX IF NOT EXISTS idx_search_document_summaries_org_location ON search_document_summaries (organization_id, location_id);
CREATE INDEX IF NOT EXISTS idx_workflow_tasks_target ON workflow_tasks (target_type, target_id, created_at);
CREATE INDEX IF NOT EXISTS idx_workflow_approvals_target ON workflow_approvals (target_type, target_id, requested_at);
CREATE INDEX IF NOT EXISTS idx_analytics_snapshots_generated_at ON analytics_snapshots (generated_at);
CREATE INDEX IF NOT EXISTS idx_analytics_snapshots_window_generated ON analytics_snapshots (window_key, generated_at);
CREATE INDEX IF NOT EXISTS idx_analytics_rollups_granularity_start ON analytics_rollups (granularity, bucket_start);
CREATE INDEX IF NOT EXISTS idx_analytics_document_facts_captured_at ON analytics_document_facts (captured_at);
CREATE INDEX IF NOT EXISTS idx_analytics_document_facts_location ON analytics_document_facts (location_id, captured_at);
CREATE INDEX IF NOT EXISTS idx_analytics_document_facts_doc_type ON analytics_document_facts (document_type, captured_at);
CREATE INDEX IF NOT EXISTS idx_analytics_workflow_facts_captured_at ON analytics_workflow_facts (captured_at);
CREATE INDEX IF NOT EXISTS idx_analytics_reliability_facts_captured_at ON analytics_reliability_facts (captured_at);
CREATE INDEX IF NOT EXISTS idx_analytics_document_type_dim_name ON analytics_document_type_dim (display_name);
CREATE INDEX IF NOT EXISTS idx_analytics_location_dim_name ON analytics_location_dim (display_name);
CREATE INDEX IF NOT EXISTS idx_analytics_report_definitions_next_run ON analytics_report_definitions (enabled, next_run_at);
CREATE INDEX IF NOT EXISTS idx_analytics_report_runs_generated_at ON analytics_report_runs (generated_at);
CREATE INDEX IF NOT EXISTS idx_analytics_report_artifacts_created_at ON analytics_report_artifacts (created_at);
CREATE INDEX IF NOT EXISTS idx_analytics_report_deliveries_created_at ON analytics_report_deliveries (created_at);
CREATE INDEX IF NOT EXISTS idx_analytics_report_delivery_dead_letters_created_at ON analytics_report_delivery_dead_letters (created_at);

INSERT INTO organizations (organization_id, organization_key, name, status, created_at, updated_at)
VALUES ('org_default', 'default', 'Default Organization', 'active', NOW(), NOW())
ON CONFLICT (organization_id) DO NOTHING;

INSERT INTO locations (location_id, organization_id, location_key, name, location_type, status, parent_location_id, created_at, updated_at)
VALUES ('loc_hq', 'org_default', 'hq', 'Head Office', 'location', 'active', NULL, NOW(), NOW())
ON CONFLICT (location_id) DO NOTHING;

INSERT INTO users (user_id, username, status, default_location_id, created_at, updated_at)
VALUES ('user_admin', 'admin', 'active', 'loc_hq', NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO roles (role_id, role_key, name, scope_type, created_at, updated_at)
VALUES ('role_admin', 'platform_admin', 'Platform Administrator', 'deployment', NOW(), NOW())
ON CONFLICT (role_id) DO NOTHING;

INSERT INTO permissions (permission_key, module_key, action_kind, resource_kind, description)
VALUES
    ('document.create', 'document', 'create', 'document', 'Create document drafts'),
    ('document.submit', 'document', 'submit', 'document', 'Submit document drafts'),
    ('document.approve', 'document', 'approve', 'document', 'Approve submitted documents'),
    ('document.reject', 'document', 'reject', 'document', 'Reject submitted documents'),
    ('document.reopen', 'document', 'reopen', 'document', 'Reopen finalized documents'),
    ('document.cancel', 'document', 'cancel', 'document', 'Cancel draft or submitted documents'),
    ('document.read', 'document', 'read', 'document', 'Read documents'),
    ('document.list', 'document', 'list', 'document', 'List documents'),
    ('document.update_draft', 'document', 'update_draft', 'document', 'Update draft documents'),
    ('platform.context.read', 'platform', 'read', 'context', 'Read platform context'),
    ('module.read', 'module', 'read', 'module', 'Read installed modules'),
    ('module.manage', 'module', 'manage', 'module', 'Manage installed modules'),
    ('configuration.read', 'configuration', 'read', 'configuration', 'Read managed configuration'),
    ('configuration.manage', 'configuration', 'manage', 'configuration', 'Manage configuration'),
    ('audit.read', 'audit', 'read', 'audit_event', 'Read audit events'),
    ('event.read', 'eventing', 'read', 'domain_event', 'Read domain events'),
    ('outbox.read', 'eventing', 'read', 'outbox', 'Read outbox records'),
    ('outbox.dispatch', 'eventing', 'dispatch', 'outbox', 'Dispatch outbox records'),
    ('deadletter.read', 'eventing', 'read', 'dead_letter', 'Read dead letters'),
    ('metrics.read', 'monitoring', 'read', 'metrics', 'Read metrics'),
    ('monitoring.read', 'monitoring', 'read', 'dashboard', 'Read monitoring views'),
    ('analytics.read', 'analytics', 'read', 'analytics', 'Read analytics views'),
    ('analytics.manage_reports', 'analytics', 'manage_reports', 'analytics_report', 'Manage analytics reports'),
    ('analytics.deliver_reports', 'analytics', 'deliver_reports', 'analytics_report', 'Deliver analytics reports'),
    ('identity.manage_sessions', 'identity', 'manage', 'session', 'Manage sessions'),
    ('identity.manage_users', 'identity', 'manage', 'user', 'Manage users')
ON CONFLICT (permission_key) DO NOTHING;

INSERT INTO role_bindings (role_binding_id, user_id, role_id, scope_type, scope_id, effective_from, effective_to, status)
VALUES ('rb_admin', 'user_admin', 'role_admin', 'deployment', NULL, NOW(), NULL, 'active')
ON CONFLICT (role_binding_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_key)
VALUES
    ('role_admin', 'document.create'),
    ('role_admin', 'document.submit'),
    ('role_admin', 'document.approve'),
    ('role_admin', 'document.reject'),
    ('role_admin', 'document.reopen'),
    ('role_admin', 'document.cancel'),
    ('role_admin', 'document.read'),
    ('role_admin', 'document.list'),
    ('role_admin', 'document.update_draft'),
    ('role_admin', 'platform.context.read'),
    ('role_admin', 'module.read'),
    ('role_admin', 'module.manage'),
    ('role_admin', 'configuration.read'),
    ('role_admin', 'configuration.manage'),
    ('role_admin', 'audit.read'),
    ('role_admin', 'event.read'),
    ('role_admin', 'outbox.read'),
    ('role_admin', 'outbox.dispatch'),
    ('role_admin', 'deadletter.read'),
    ('role_admin', 'metrics.read'),
    ('role_admin', 'monitoring.read'),
    ('role_admin', 'analytics.read'),
    ('role_admin', 'analytics.manage_reports'),
    ('role_admin', 'analytics.deliver_reports'),
    ('role_admin', 'identity.manage_sessions'),
    ('role_admin', 'identity.manage_users')
ON CONFLICT (role_id, permission_key) DO NOTHING;

INSERT INTO configuration_entries (configuration_key, module_key, category_key, scope_type, scope_id, value_json, updated_at, updated_by, description)
VALUES
    ('platform.http', 'platform.core', 'platform', 'deployment', '', '{"address":":8080"}'::jsonb, NOW(), 'system', 'Platform HTTP listener settings.'),
    ('identity.auth', 'identity', 'security', 'deployment', '', jsonb_build_object(
        'password_min_length', 8,
        'session_ttl_minutes', 480,
        'session_refresh_window_minutes', 60,
        'login_rate_limit_attempts', 5,
        'login_rate_limit_window_seconds', 300,
        'trusted_origins', '[]'::jsonb
    ), NOW(), 'system', 'Authentication, session, and login throttling policy.')
ON CONFLICT (configuration_key, scope_type, scope_id) DO NOTHING;

INSERT INTO document_definitions (document_type, display_name, schema_version, workflow_key, numbering_key, owner_module_key)
VALUES ('generic_request', 'Generic Request', 'v1', 'generic_request_flow', NULL, 'documents')
ON CONFLICT (document_type) DO NOTHING;

INSERT INTO document_extension_definitions (document_type, module_key, display_name, schema_version, read_permission_key, write_permission_key)
VALUES ('generic_request', 'analytics', 'Analytics Extension', 'v1', NULL, NULL)
ON CONFLICT (document_type, module_key) DO NOTHING;

INSERT INTO workflow_definitions (workflow_key, states_json, actions_json)
VALUES (
    'generic_request_flow',
    '["draft", "submitted", "approved", "rejected"]'::jsonb,
    '[{"action":"submit","from_state":"draft","to_state":"submitted","permission_key":"document.submit","task_type":"review","create_approval":true},{"action":"approve","from_state":"submitted","to_state":"approved","permission_key":"document.approve"},{"action":"reject","from_state":"submitted","to_state":"rejected","permission_key":"document.reject"},{"action":"reopen","from_state":"rejected","to_state":"draft","permission_key":"document.reopen"},{"action":"reopen","from_state":"approved","to_state":"draft","permission_key":"document.reopen"},{"action":"cancel","from_state":"draft","to_state":"cancelled","permission_key":"document.cancel"},{"action":"cancel","from_state":"submitted","to_state":"cancelled","permission_key":"document.cancel"}]'::jsonb
)
ON CONFLICT (workflow_key) DO NOTHING;

COMMIT;
