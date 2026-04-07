CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events (action, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_request ON audit_events (request_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_scope ON audit_events (organization_id, location_id, operating_unit_id, occurred_at);
