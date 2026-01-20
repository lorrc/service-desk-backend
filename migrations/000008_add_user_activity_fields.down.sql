DROP INDEX IF EXISTS idx_users_is_active;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_active_at,
    DROP COLUMN IF EXISTS is_active;
