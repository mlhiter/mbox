CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (length(trim(action)) > 0),
    resource_type TEXT NOT NULL CHECK (length(trim(resource_type)) > 0),
    resource_id UUID,
    resource_name TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_events_project_created_at_idx
    ON audit_events(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_resource_idx
    ON audit_events(resource_type, resource_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_action_idx ON audit_events(action);
