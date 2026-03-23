CREATE TABLE IF NOT EXISTS notification_items (
    notification_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(user_id),
    category TEXT,
    channel TEXT,
    status TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    target_type TEXT,
    target_id TEXT,
    deep_link_path TEXT,
    action_link_path TEXT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    read_at TIMESTAMPTZ,
    dismissed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notification_items_user_created
    ON notification_items (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notification_items_user_status
    ON notification_items (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notification_items_target
    ON notification_items (target_type, target_id, created_at DESC);
