CREATE TABLE IF NOT EXISTS project_policies (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    enforcement TEXT NOT NULL DEFAULT 'disabled' CHECK (enforcement IN ('disabled', 'enforced')),
    allowed_image_prefixes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    allowed_service_accounts TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    allowed_secret_refs TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS project_policies_set_updated_at ON project_policies;
CREATE TRIGGER project_policies_set_updated_at
BEFORE UPDATE ON project_policies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
