CREATE INDEX IF NOT EXISTS audit_events_request_id_created_at_idx
    ON audit_events((metadata ->> 'requestId'), created_at DESC)
    WHERE metadata ? 'requestId';

CREATE INDEX IF NOT EXISTS audit_events_project_request_id_created_at_idx
    ON audit_events(project_id, (metadata ->> 'requestId'), created_at DESC)
    WHERE metadata ? 'requestId';
