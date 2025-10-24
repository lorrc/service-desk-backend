-- name: CreateTicket :one
INSERT INTO tickets (title, description, priority, requester_id)
VALUES ($1, $2, $3, $4)
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

-- name: ListTicketsByRequesterID :many
SELECT * FROM tickets
WHERE requester_id = $1
ORDER BY created_at DESC;

-- name: ListTickets :many
SELECT * FROM tickets
ORDER BY created_at DESC;