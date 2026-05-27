CREATE TABLE IF NOT EXISTS execution_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sandbox_id UUID NOT NULL REFERENCES sandboxes(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'canceled', 'timed_out')),
    command TEXT[] NOT NULL CHECK (array_length(command, 1) IS NOT NULL),
    timeout_seconds INTEGER NOT NULL CHECK (timeout_seconds > 0),
    exit_code INTEGER,
    stdout TEXT NOT NULL DEFAULT '',
    stderr TEXT NOT NULL DEFAULT '',
    output_truncated BOOLEAN NOT NULL DEFAULT false,
    error TEXT NOT NULL DEFAULT '',
    runtime_ref JSONB,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS execution_tasks_sandbox_id_idx ON execution_tasks(sandbox_id);
CREATE INDEX IF NOT EXISTS execution_tasks_project_id_idx ON execution_tasks(project_id);
CREATE INDEX IF NOT EXISTS execution_tasks_status_idx ON execution_tasks(status);
CREATE INDEX IF NOT EXISTS execution_tasks_created_at_idx ON execution_tasks(created_at);

CREATE UNIQUE INDEX IF NOT EXISTS sandboxes_project_id_id_key ON sandboxes(project_id, id);
ALTER TABLE execution_tasks
    DROP CONSTRAINT IF EXISTS execution_tasks_project_sandbox_fkey,
    ADD CONSTRAINT execution_tasks_project_sandbox_fkey
    FOREIGN KEY (project_id, sandbox_id)
    REFERENCES sandboxes(project_id, id)
    ON DELETE CASCADE;

DROP TRIGGER IF EXISTS execution_tasks_set_updated_at ON execution_tasks;
CREATE TRIGGER execution_tasks_set_updated_at
BEFORE UPDATE ON execution_tasks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
