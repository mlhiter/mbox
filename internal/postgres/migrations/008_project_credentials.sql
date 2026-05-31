CREATE TABLE IF NOT EXISTS project_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slug TEXT NOT NULL CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'),
    type TEXT NOT NULL CHECK (type IN ('git', 'registry', 'kubernetes', 'ssh', 'generic')),
    target TEXT NOT NULL DEFAULT '',
    secret_ref JSONB NOT NULL,
    usage TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (jsonb_typeof(secret_ref) = 'object' AND secret_ref ? 'name' AND length(trim(secret_ref->>'name')) > 0)
);

CREATE INDEX IF NOT EXISTS project_credentials_project_id_idx ON project_credentials(project_id);
CREATE INDEX IF NOT EXISTS project_credentials_type_idx ON project_credentials(type);
CREATE UNIQUE INDEX IF NOT EXISTS project_credentials_project_slug_key
    ON project_credentials(project_id, slug);

DROP TRIGGER IF EXISTS project_credentials_set_updated_at ON project_credentials;
CREATE TRIGGER project_credentials_set_updated_at
BEFORE UPDATE ON project_credentials
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
