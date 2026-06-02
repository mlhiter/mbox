CREATE TABLE IF NOT EXISTS project_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    principal_type TEXT NOT NULL CHECK (principal_type IN ('user', 'service_account', 'automation')),
    principal TEXT NOT NULL CHECK (length(trim(principal)) > 0),
    role TEXT NOT NULL CHECK (role IN ('owner', 'operator', 'viewer')),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS project_members_project_id_idx ON project_members(project_id);
CREATE INDEX IF NOT EXISTS project_members_principal_idx ON project_members(principal_type, principal);
CREATE UNIQUE INDEX IF NOT EXISTS project_members_project_principal_key
    ON project_members(project_id, principal_type, principal);

DROP TRIGGER IF EXISTS project_members_set_updated_at ON project_members;
CREATE TRIGGER project_members_set_updated_at
BEFORE UPDATE ON project_members
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
