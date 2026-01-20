DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'comments'
      AND column_name = 'id'
      AND data_type = 'uuid'
  ) THEN
    CREATE SEQUENCE IF NOT EXISTS comments_id_bigint_seq;
    ALTER TABLE comments ADD COLUMN IF NOT EXISTS id_bigint BIGINT;

    UPDATE comments
    SET id_bigint = nextval('comments_id_bigint_seq')
    WHERE id_bigint IS NULL;

    PERFORM setval('comments_id_bigint_seq', (SELECT COALESCE(MAX(id_bigint), 0) FROM comments));

    ALTER TABLE comments ALTER COLUMN id_bigint SET DEFAULT nextval('comments_id_bigint_seq');
    ALTER SEQUENCE comments_id_bigint_seq OWNED BY comments.id_bigint;

    ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_pkey;
    ALTER TABLE comments DROP COLUMN id;
    ALTER TABLE comments RENAME COLUMN id_bigint TO id;
    ALTER TABLE comments ADD PRIMARY KEY (id);
  END IF;
END $$;
