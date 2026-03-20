CREATE TABLE IF NOT EXISTS analytics_dashboards (
    dashboard_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private',
    scope_type TEXT,
    scope_id TEXT,
    owner_user_id TEXT,
    organization_id TEXT,
    location_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    payload_json JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_analytics_dashboards_scope ON analytics_dashboards (scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_analytics_dashboards_owner ON analytics_dashboards (owner_user_id);

CREATE TABLE IF NOT EXISTS analytics_saved_metrics (
    metric_id TEXT PRIMARY KEY,
    metric_key TEXT NOT NULL,
    name TEXT NOT NULL,
    scope_type TEXT,
    scope_id TEXT,
    owner_user_id TEXT,
    organization_id TEXT,
    location_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    payload_json JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_analytics_saved_metrics_scope ON analytics_saved_metrics (scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_analytics_saved_metrics_owner ON analytics_saved_metrics (owner_user_id);

CREATE TABLE IF NOT EXISTS analytics_saved_queries (
    query_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    scope_type TEXT,
    scope_id TEXT,
    owner_user_id TEXT,
    organization_id TEXT,
    location_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    payload_json JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_analytics_saved_queries_scope ON analytics_saved_queries (scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_analytics_saved_queries_owner ON analytics_saved_queries (owner_user_id);
