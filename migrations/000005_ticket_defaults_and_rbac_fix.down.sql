-- Roll back ticket defaults and remove the added permission/grants

-- Revert defaults to previous lowercase values
ALTER TABLE tickets
    ALTER COLUMN status SET DEFAULT 'open',
    ALTER COLUMN priority SET DEFAULT 'medium';

-- Lowercase values that were uppercased
UPDATE tickets
SET status = LOWER(status)
WHERE status IS NOT NULL;

UPDATE tickets
SET priority = LOWER(priority)
WHERE priority IS NOT NULL;

-- Remove role grants for tickets:list:all
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'tickets:list:all');

-- Remove the permission
DELETE FROM permissions WHERE code = 'tickets:list:all';
