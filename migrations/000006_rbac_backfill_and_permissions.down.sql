-- The permission and admin/agent grants belong to migration 5.
-- Customer-role backfills cannot be distinguished from legitimate grants made
-- before or after this migration. Preserve them when returning to version 5.
SELECT 1;
