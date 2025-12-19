-- name: CreateComment :one
INSERT INTO comments (ticket_id, author_id, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListCommentsByTicketID :many
SELECT * FROM comments
WHERE ticket_id = $1
ORDER BY created_at ASC;
