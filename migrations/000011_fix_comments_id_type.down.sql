DO $$
BEGIN
  -- Migration 3 already uses BIGSERIAL, so the canonical schema needs no undo.
  -- UUID conversions, including conversions made by earlier releases of this
  -- migration, own this distinct sequence and discarded the original UUIDs.
  IF to_regclass(pg_get_serial_sequence('comments', 'id')) =
     to_regclass('comments_id_bigint_seq') THEN
    RAISE EXCEPTION
      'Migration 11 converted legacy UUID comment IDs and is irreversible. Restore a pre-conversion backup to recover the original UUIDs.';
  END IF;
END $$;
