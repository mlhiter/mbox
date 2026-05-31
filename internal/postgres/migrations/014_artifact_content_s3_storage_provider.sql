ALTER TABLE artifact_contents
    DROP CONSTRAINT IF EXISTS artifact_contents_storage_provider_check;

ALTER TABLE artifact_contents
    ADD CONSTRAINT artifact_contents_storage_provider_check
    CHECK (storage_provider IN ('postgres', 'filesystem', 's3'));

ALTER TABLE artifact_contents
    DROP CONSTRAINT IF EXISTS artifact_contents_storage_shape_check;

ALTER TABLE artifact_contents
    ADD CONSTRAINT artifact_contents_storage_shape_check
    CHECK (
        (storage_provider = 'postgres' AND content IS NOT NULL AND storage_key = '')
        OR
        (storage_provider IN ('filesystem', 's3') AND content IS NULL AND length(trim(storage_key)) > 0)
    );
