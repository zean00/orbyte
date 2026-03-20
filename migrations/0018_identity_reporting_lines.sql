CREATE TABLE IF NOT EXISTS user_reporting_lines (
    reporting_line_id TEXT PRIMARY KEY,
    subject_user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    manager_user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    relationship_type TEXT NOT NULL,
    organization_id TEXT NULL REFERENCES organizations(organization_id) ON DELETE CASCADE,
    location_id TEXT NULL REFERENCES locations(location_id) ON DELETE CASCADE,
    operating_unit_id TEXT NULL REFERENCES operating_units(operating_unit_id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reporting_lines_subject_active
    ON user_reporting_lines(subject_user_id, relationship_type, status, effective_from);

CREATE INDEX IF NOT EXISTS idx_reporting_lines_manager_active
    ON user_reporting_lines(manager_user_id, status, effective_from);
