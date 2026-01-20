-- name: CreateTicketEvent :one
INSERT INTO ticket_events (ticket_id, type, payload, actor_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListTicketEvents :many
SELECT * FROM ticket_events
WHERE ticket_id = $1
  AND id > $2
ORDER BY id ASC
LIMIT $3;
