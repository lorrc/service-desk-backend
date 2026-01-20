ALTER TABLE tickets
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;

UPDATE tickets
SET closed_at = COALESCE(updated_at, created_at, NOW())
WHERE UPPER(status) = 'CLOSED'
  AND closed_at IS NULL;
