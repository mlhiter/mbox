CREATE INDEX IF NOT EXISTS audit_events_actor_created_at_idx
    ON audit_events(actor, created_at DESC)
    WHERE actor <> '';

CREATE INDEX IF NOT EXISTS audit_events_source_created_at_idx
    ON audit_events(source, created_at DESC)
    WHERE source <> '';
