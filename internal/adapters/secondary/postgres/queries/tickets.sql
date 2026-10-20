-- name: CreateTicket :one
INSERT INTO tickets (title, description, status, priority, requester_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTicketByID :one
SELECT t.* FROM tickets t
JOIN users u ON u.id = t.requester_id
WHERE t.id = $1 AND u.organization_id = $2;

-- name: GetTicketForUpdate :one
SELECT t.* FROM tickets t
JOIN users u ON u.id = t.requester_id
WHERE t.id = $1 AND u.organization_id = $2
FOR UPDATE OF t;

-- name: UpdateTicket :one
UPDATE tickets t
SET status = $2, assignee_id = $3, updated_at = $4, closed_at = $5
FROM users u
WHERE t.id = $1 AND u.id = t.requester_id AND u.organization_id = $6
RETURNING t.*;

-- name: ListTicketsPaginated :many
SELECT t.* FROM tickets t
JOIN users u ON u.id = t.requester_id
WHERE u.organization_id = sqlc.arg('organization_id')
  AND (t.requester_id = sqlc.narg('requester_id') OR sqlc.narg('requester_id') IS NULL)
  AND (NOT sqlc.arg('active')::boolean OR t.status <> 'CLOSED')
  AND (sqlc.arg('search')::text = '' OR strpos(lower(t.title || ' ' || coalesce(t.description, '') || ' ' || t.id::text), lower(sqlc.arg('search')::text)) > 0)
  AND (t.status = sqlc.narg('status') OR sqlc.narg('status') IS NULL)
  AND (t.priority = sqlc.narg('priority') OR sqlc.narg('priority') IS NULL)
  AND (NOT sqlc.arg('unassigned')::boolean OR t.assignee_id IS NULL)
  AND (t.assignee_id = sqlc.narg('assignee_id') OR sqlc.narg('assignee_id') IS NULL)
  AND (t.created_at >= sqlc.narg('created_from') OR sqlc.narg('created_from') IS NULL)
  AND (t.created_at < sqlc.narg('created_to') OR sqlc.narg('created_to') IS NULL)
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTicketCounts :one
SELECT
  COUNT(*) AS all,
  COUNT(*) FILTER (WHERE t.assignee_id IS NULL) AS unassigned,
  COUNT(*) FILTER (WHERE t.assignee_id = sqlc.arg('viewer_id')) AS assigned_to_me,
  COUNT(*) FILTER (WHERE t.status = 'OPEN') AS open,
  COUNT(*) FILTER (WHERE t.status = 'IN_PROGRESS') AS in_progress,
  COUNT(*) FILTER (WHERE t.status = 'CLOSED') AS closed,
  COUNT(*) FILTER (WHERE t.priority = 'HIGH' AND t.status <> 'CLOSED') AS high_priority
FROM tickets t
JOIN users u ON u.id = t.requester_id
WHERE u.organization_id = sqlc.arg('organization_id')
  AND (t.requester_id = sqlc.narg('requester_id') OR sqlc.narg('requester_id') IS NULL)
  AND (sqlc.arg('search')::text = '' OR strpos(lower(t.title || ' ' || coalesce(t.description, '') || ' ' || t.id::text), lower(sqlc.arg('search')::text)) > 0)
  AND (t.created_at >= sqlc.narg('created_from') OR sqlc.narg('created_from') IS NULL)
  AND (t.created_at < sqlc.narg('created_to') OR sqlc.narg('created_to') IS NULL);
