DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'comments'
      AND column_name = 'id'
      AND data_type = 'bigint'
  ) THEN
    ALTER TABLE comments ADD COLUMN IF NOT EXISTS id_uuid UUID;

    UPDATE comments
    SET id_uuid = md5(random()::text || clock_timestamp()::text || id::text)::uuid
    WHERE id_uuid IS NULL;

    ALTER TABLE comments ALTER COLUMN id_uuid SET DEFAULT (
      md5(random()::text || clock_timestamp()::text)::uuid
    );

    ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_pkey;
    ALTER TABLE comments DROP COLUMN id;
    ALTER TABLE comments RENAME COLUMN id_uuid TO id;
    ALTER TABLE comments ADD PRIMARY KEY (id);

    DROP SEQUENCE IF EXISTS comments_id_bigint_seq;
    DROP SEQUENCE IF EXISTS comments_id_seq;
  END IF;
END $$;
