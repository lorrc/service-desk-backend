-- name: CreateUser :one
INSERT INTO users (organization_id, full_name, email, hashed_password)
VALUES ($1, $2, $3, $4)
    RETURNING id, organization_id, full_name, email, hashed_password, created_at, is_active, last_active_at;

-- name: GetUserByEmail :one
SELECT id, organization_id, full_name, email, hashed_password, created_at, is_active, last_active_at FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT id, organization_id, full_name, email, hashed_password, created_at, is_active, last_active_at FROM users
WHERE id = $1 LIMIT 1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;
