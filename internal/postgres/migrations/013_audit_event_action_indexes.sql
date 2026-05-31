CREATE INDEX IF NOT EXISTS audit_events_action_created_at_idx
    ON audit_events(action, created_at DESC);

CREATE INDEX IF NOT EXISTS audit_events_project_action_created_at_idx
    ON audit_events(project_id, action, created_at DESC);
