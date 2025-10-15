
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
