-- Remove backfilled customer roles when they are the only role
DELETE FROM user_roles ur
USING roles r
WHERE ur.role_id = r.id
  AND r.name = 'customer'
  AND NOT EXISTS (
      SELECT 1 FROM user_roles ur2
      WHERE ur2.user_id = ur.user_id AND ur2.role_id <> ur.role_id
  );

-- Remove list-all permission assignments for admin/agent
DELETE FROM role_permissions rp
USING roles r, permissions p
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND r.name IN ('admin', 'agent')
  AND p.code = 'tickets:list:all';

-- Remove the permission itself
DELETE FROM permissions WHERE code = 'tickets:list:all';
