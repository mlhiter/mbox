CREATE TABLE IF NOT EXISTS project_quota_policies (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    enforcement TEXT NOT NULL DEFAULT 'disabled' CHECK (enforcement IN ('disabled', 'enforced')),
    max_active_sandboxes INTEGER CHECK (max_active_sandboxes IS NULL OR max_active_sandboxes >= 0),
    max_retained_artifact_bytes BIGINT CHECK (max_retained_artifact_bytes IS NULL OR max_retained_artifact_bytes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS project_quota_policies_set_updated_at ON project_quota_policies;
CREATE TRIGGER project_quota_policies_set_updated_at
BEFORE UPDATE ON project_quota_policies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
