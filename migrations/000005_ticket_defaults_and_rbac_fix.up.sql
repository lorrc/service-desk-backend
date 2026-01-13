-- Align ticket status/priority casing with domain expectations and add missing permission

-- Ensure ticket defaults use uppercase values
ALTER TABLE tickets
    ALTER COLUMN status SET DEFAULT 'OPEN',
    ALTER COLUMN priority SET DEFAULT 'MEDIUM';

-- Backfill existing rows to uppercase to avoid validation mismatches
UPDATE tickets
SET status = UPPER(status)
WHERE status IS NOT NULL;

UPDATE tickets
SET priority = UPPER(priority)
WHERE priority IS NOT NULL;

-- Seed missing permission used by application code
INSERT INTO permissions (code) VALUES ('tickets:list:all')
ON CONFLICT DO NOTHING;

-- Grant list-all permission to admin and agent roles
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'tickets:list:all'
WHERE r.name IN ('admin', 'agent')
ON CONFLICT DO NOTHING;
