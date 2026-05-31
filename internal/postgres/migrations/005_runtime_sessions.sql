CREATE TABLE IF NOT EXISTS runtime_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sandbox_id UUID NOT NULL REFERENCES sandboxes(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('terminal', 'ide', 'notebook', 'browser', 'command', 'custom')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended', 'failed')),
    client TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    runtime_ref JSONB,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE INDEX IF NOT EXISTS runtime_sessions_project_id_idx ON runtime_sessions(project_id);
CREATE INDEX IF NOT EXISTS runtime_sessions_sandbox_id_idx ON runtime_sessions(sandbox_id);
CREATE INDEX IF NOT EXISTS runtime_sessions_status_idx ON runtime_sessions(status);
CREATE INDEX IF NOT EXISTS runtime_sessions_type_idx ON runtime_sessions(type);
CREATE INDEX IF NOT EXISTS runtime_sessions_started_at_idx ON runtime_sessions(started_at);
CREATE UNIQUE INDEX IF NOT EXISTS sandboxes_project_id_id_key ON sandboxes(project_id, id);

ALTER TABLE runtime_sessions
    DROP CONSTRAINT IF EXISTS runtime_sessions_project_sandbox_fkey,
    ADD CONSTRAINT runtime_sessions_project_sandbox_fkey
    FOREIGN KEY (project_id, sandbox_id)
    REFERENCES sandboxes(project_id, id)
    ON DELETE CASCADE;

DROP TRIGGER IF EXISTS runtime_sessions_set_updated_at ON runtime_sessions;
CREATE TRIGGER runtime_sessions_set_updated_at
BEFORE UPDATE ON runtime_sessions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
