-- Add missing permission used by ticket listing logic
INSERT INTO permissions (code) VALUES ('tickets:list:all')
ON CONFLICT DO NOTHING;

-- Ensure admin and agent have list-all capability
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('admin', 'agent') AND p.code = 'tickets:list:all'
ON CONFLICT DO NOTHING;

-- Backfill users without roles to the customer role
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.name = 'customer'
WHERE NOT EXISTS (
    SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id
)
ON CONFLICT DO NOTHING;
