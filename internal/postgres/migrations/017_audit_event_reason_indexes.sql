CREATE INDEX IF NOT EXISTS audit_events_reason_created_at_idx
    ON audit_events((metadata ->> 'reason'), created_at DESC)
    WHERE metadata ? 'reason';

CREATE INDEX IF NOT EXISTS audit_events_project_reason_created_at_idx
    ON audit_events(project_id, (metadata ->> 'reason'), created_at DESC)
    WHERE metadata ? 'reason';
