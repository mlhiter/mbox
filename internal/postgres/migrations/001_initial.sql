CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slug TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'),
    repository_url TEXT NOT NULL DEFAULT '',
    default_namespace TEXT NOT NULL CHECK (length(trim(default_namespace)) > 0),
    default_template_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS environment_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slug TEXT NOT NULL CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'),
    image TEXT NOT NULL CHECK (length(trim(image)) > 0),
    startup_command TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    working_dir TEXT NOT NULL DEFAULT '/workspace',
    cpu_request TEXT NOT NULL DEFAULT '',
    memory_request TEXT NOT NULL DEFAULT '',
    storage_request TEXT NOT NULL DEFAULT '',
    exposed_ports JSONB NOT NULL DEFAULT '[]'::jsonb,
    env JSONB NOT NULL DEFAULT '{}'::jsonb,
    secret_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    network_policy TEXT NOT NULL DEFAULT 'default',
    lifecycle_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE projects
    ADD CONSTRAINT projects_default_template_id_fkey
    FOREIGN KEY (default_template_id)
    REFERENCES environment_templates(id)
    ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS sandboxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    template_id UUID NOT NULL REFERENCES environment_templates(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slug TEXT NOT NULL CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'stopped', 'failed', 'deleted')),
    namespace TEXT NOT NULL CHECK (length(trim(namespace)) > 0),
    service_account_name TEXT NOT NULL CHECK (length(trim(service_account_name)) > 0),
    runtime_ref JSONB,
    ports JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS environment_templates_project_id_idx ON environment_templates(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS environment_templates_global_slug_key
    ON environment_templates(slug)
    WHERE project_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS environment_templates_project_slug_key
    ON environment_templates(project_id, slug)
    WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS sandboxes_project_id_idx ON sandboxes(project_id);
CREATE INDEX IF NOT EXISTS sandboxes_template_id_idx ON sandboxes(template_id);
CREATE INDEX IF NOT EXISTS sandboxes_status_idx ON sandboxes(status);
CREATE INDEX IF NOT EXISTS sandboxes_deleted_at_idx ON sandboxes(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS sandboxes_active_project_slug_key
    ON sandboxes(project_id, slug)
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS projects_set_updated_at ON projects;
CREATE TRIGGER projects_set_updated_at
BEFORE UPDATE ON projects
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS environment_templates_set_updated_at ON environment_templates;
CREATE TRIGGER environment_templates_set_updated_at
BEFORE UPDATE ON environment_templates
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS sandboxes_set_updated_at ON sandboxes;
CREATE TRIGGER sandboxes_set_updated_at
BEFORE UPDATE ON sandboxes
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
