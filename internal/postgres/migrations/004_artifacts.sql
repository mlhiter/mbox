CREATE TABLE IF NOT EXISTS artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sandbox_id UUID NOT NULL REFERENCES sandboxes(id) ON DELETE CASCADE,
    task_id UUID REFERENCES execution_tasks(id) ON DELETE SET NULL,
    kind TEXT NOT NULL CHECK (kind IN ('file', 'directory', 'log', 'report', 'screenshot', 'image', 'link', 'other')),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    uri TEXT NOT NULL CHECK (length(trim(uri)) > 0),
    content_type TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT CHECK (size_bytes IS NULL OR size_bytes >= 0),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS artifacts_project_id_idx ON artifacts(project_id);
CREATE INDEX IF NOT EXISTS artifacts_sandbox_id_idx ON artifacts(sandbox_id);
CREATE INDEX IF NOT EXISTS artifacts_task_id_idx ON artifacts(task_id);
CREATE INDEX IF NOT EXISTS artifacts_kind_idx ON artifacts(kind);
CREATE INDEX IF NOT EXISTS artifacts_created_at_idx ON artifacts(created_at);

CREATE UNIQUE INDEX IF NOT EXISTS execution_tasks_project_sandbox_id_key ON execution_tasks(project_id, sandbox_id, id);
ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_project_sandbox_fkey,
    ADD CONSTRAINT artifacts_project_sandbox_fkey
    FOREIGN KEY (project_id, sandbox_id)
    REFERENCES sandboxes(project_id, id)
    ON DELETE CASCADE,
    DROP CONSTRAINT IF EXISTS artifacts_project_sandbox_task_fkey,
    ADD CONSTRAINT artifacts_project_sandbox_task_fkey
    FOREIGN KEY (project_id, sandbox_id, task_id)
    REFERENCES execution_tasks(project_id, sandbox_id, id)
    ON DELETE SET NULL (task_id);

DROP TRIGGER IF EXISTS artifacts_set_updated_at ON artifacts;
CREATE TRIGGER artifacts_set_updated_at
BEFORE UPDATE ON artifacts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
