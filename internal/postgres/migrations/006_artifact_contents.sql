CREATE TABLE IF NOT EXISTS artifact_contents (
    artifact_id UUID PRIMARY KEY REFERENCES artifacts(id) ON DELETE CASCADE,
    content BYTEA NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    source_uri TEXT NOT NULL CHECK (length(trim(source_uri)) > 0),
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS artifact_contents_sha256_idx ON artifact_contents(sha256);
CREATE INDEX IF NOT EXISTS artifact_contents_captured_at_idx ON artifact_contents(captured_at);

DROP TRIGGER IF EXISTS artifact_contents_set_updated_at ON artifact_contents;
CREATE TRIGGER artifact_contents_set_updated_at
BEFORE UPDATE ON artifact_contents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
