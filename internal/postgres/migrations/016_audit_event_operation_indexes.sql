CREATE INDEX IF NOT EXISTS audit_events_operation_created_at_idx
    ON audit_events((metadata ->> 'operation'), created_at DESC)
    WHERE metadata ? 'operation';

CREATE INDEX IF NOT EXISTS audit_events_project_operation_created_at_idx
    ON audit_events(project_id, (metadata ->> 'operation'), created_at DESC)
    WHERE metadata ? 'operation';
