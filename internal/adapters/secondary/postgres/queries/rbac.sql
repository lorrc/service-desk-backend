-- name: GetUserPermissions :many
SELECT DISTINCT p.code
FROM permissions p
INNER JOIN role_permissions rp ON p.id = rp.permission_id
INNER JOIN user_roles ur ON rp.role_id = ur.role_id
WHERE ur.user_id = $1;

-- name: AssignRole :one
WITH role AS (
    SELECT id FROM roles WHERE name = sqlc.arg('role_name')
),
ins AS (
    INSERT INTO user_roles (user_id, role_id)
    SELECT sqlc.arg('user_id'), id FROM role
    ON CONFLICT DO NOTHING
    RETURNING role_id
)
SELECT
    CASE
        WHEN NOT EXISTS (SELECT 1 FROM role) THEN 'role_not_found'
        WHEN EXISTS (SELECT 1 FROM ins) THEN 'assigned'
        ELSE 'already_assigned'
    END AS status;

-- name: SetUserRole :one
WITH target_role AS (
    SELECT id FROM roles WHERE name = sqlc.arg('role_name')
),
deleted AS (
    DELETE FROM user_roles ur WHERE ur.user_id = sqlc.arg('user_id')
),
inserted AS (
    INSERT INTO user_roles (user_id, role_id)
    SELECT sqlc.arg('user_id'), id FROM target_role
    RETURNING role_id
)
SELECT
    CASE
        WHEN NOT EXISTS (SELECT 1 FROM target_role) THEN 'role_not_found'
        WHEN EXISTS (SELECT 1 FROM inserted) THEN 'assigned'
        ELSE 'not_assigned'
    END AS status;
