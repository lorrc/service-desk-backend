-- name: CreateUser :one
INSERT INTO users (organization_id, full_name, email, hashed_password)
VALUES ($1, $2, $3, $4)
    RETURNING id, organization_id, full_name, email, hashed_password, created_at, is_active, last_active_at, session_version;

-- name: GetUserByEmail :one
SELECT id, organization_id, full_name, email, hashed_password, created_at, is_active, last_active_at, session_version FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT id, organization_id, full_name, email, hashed_password, created_at, is_active, last_active_at, session_version FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserInfo :many
SELECT id, full_name, email FROM users
WHERE organization_id = $1 AND id = ANY($2::uuid[]);

-- name: ValidateSession :one
SELECT EXISTS (
  SELECT 1 FROM users
  WHERE id = $1 AND organization_id = $2 AND session_version = $3 AND is_active
);
