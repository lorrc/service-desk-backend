-- name: CreateUser :one
INSERT INTO users (organization_id, full_name, email, hashed_password)
VALUES ($1, $2, $3, $4)
    RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;