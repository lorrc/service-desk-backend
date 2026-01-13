INSERT INTO tickets (title, description, status, priority, requester_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTicketByID :one
SELECT * FROM tickets
WHERE id = $1 LIMIT 1;

-- name: UpdateTicket :one
UPDATE tickets
SET
    status = $2,
    assignee_id = $3,
    updated_at = $4
WHERE id = $1
RETURNING *;

-- name: ListTicketsPaginated :many
SELECT * FROM tickets
WHERE
    (status = sqlc.narg('status') OR sqlc.narg('status') IS NULL)
  AND
    (priority = sqlc.narg('priority') OR sqlc.narg('priority') IS NULL)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')
    OFFSET sqlc.arg('offset');

-- name: ListTicketsByRequesterPaginated :many
SELECT * FROM tickets
WHERE
    requester_id = sqlc.arg('requester_id')
  AND
    (status = sqlc.narg('status') OR sqlc.narg('status') IS NULL)
  AND
    (priority = sqlc.narg('priority') OR sqlc.narg('priority') IS NULL)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')
    OFFSET sqlc.arg('offset');
