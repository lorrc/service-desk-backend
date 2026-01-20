INSERT INTO permissions (code) VALUES ('admin:access')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'admin:access'
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;
