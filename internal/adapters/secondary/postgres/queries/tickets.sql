-- name: CreateTicket :one
INSERT INTO tickets (title, description, priority, requester_id)
VALUES ($1, $2, $3, $4)
    RETURNING *;