ALTER TABLE artifact_contents
    ADD COLUMN IF NOT EXISTS storage_provider TEXT NOT NULL DEFAULT 'postgres',
    ADD COLUMN IF NOT EXISTS storage_key TEXT NOT NULL DEFAULT '';

ALTER TABLE artifact_contents
    ALTER COLUMN content DROP NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'artifact_contents_storage_provider_check'
    ) THEN
        ALTER TABLE artifact_contents
            ADD CONSTRAINT artifact_contents_storage_provider_check
            CHECK (storage_provider IN ('postgres', 'filesystem'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'artifact_contents_storage_shape_check'
    ) THEN
        ALTER TABLE artifact_contents
            ADD CONSTRAINT artifact_contents_storage_shape_check
            CHECK (
                (storage_provider = 'postgres' AND content IS NOT NULL AND storage_key = '')
                OR
                (storage_provider = 'filesystem' AND content IS NULL AND length(trim(storage_key)) > 0)
            );
    END IF;
END $$;
