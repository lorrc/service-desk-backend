package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type AnalyticsRepository struct {
	pool *pgxpool.Pool
}

var _ ports.AnalyticsRepository = (*AnalyticsRepository)(nil)

func NewAnalyticsRepository(pool *pgxpool.Pool) ports.AnalyticsRepository {
	return &AnalyticsRepository{pool: pool}
}

func (r *AnalyticsRepository) GetOverview(ctx context.Context, orgID uuid.UUID, days int) (*domain.AnalyticsOverview, error) {
	if days <= 0 {
		days = 30
	}

	statusCounts, err := r.fetchStatusCounts(ctx, orgID)
	if err != nil {
		return nil, err
	}

	workload, err := r.fetchWorkload(ctx, orgID)
	if err != nil {
		return nil, err
	}

	volume, err := r.fetchVolume(ctx, orgID, days)
	if err != nil {
		return nil, err
	}

	mttrHours, err := r.fetchMTTRHours(ctx, orgID)
	if err != nil {
		return nil, err
	}

	return &domain.AnalyticsOverview{
		StatusCounts: statusCounts,
		Workload:     workload,
		Volume:       volume,
		MTTRHours:    mttrHours,
	}, nil
}

func (r *AnalyticsRepository) fetchStatusCounts(ctx context.Context, orgID uuid.UUID) ([]domain.StatusCount, error) {
	const query = `
SELECT t.status, COUNT(*)
FROM tickets t
JOIN users ru ON t.requester_id = ru.id
WHERE ru.organization_id = $1
GROUP BY t.status
`

	rows, err := r.pool.Query(ctx, query, pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[domain.TicketStatus]int64{
		domain.StatusOpen:       0,
		domain.StatusInProgress: 0,
		domain.StatusClosed:     0,
	}

	for rows.Next() {
		var (
			status string
			count  int64
		)
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[domain.TicketStatus(status)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return []domain.StatusCount{
		{Status: domain.StatusOpen, Count: counts[domain.StatusOpen]},
		{Status: domain.StatusInProgress, Count: counts[domain.StatusInProgress]},
		{Status: domain.StatusClosed, Count: counts[domain.StatusClosed]},
	}, nil
}

func (r *AnalyticsRepository) fetchWorkload(ctx context.Context, orgID uuid.UUID) ([]domain.WorkloadItem, error) {
	const query = `
SELECT t.assignee_id, u.full_name, u.email, COUNT(*)
FROM tickets t
JOIN users ru ON t.requester_id = ru.id
LEFT JOIN users u ON t.assignee_id = u.id
WHERE ru.organization_id = $1
  AND t.status != 'CLOSED'
GROUP BY t.assignee_id, u.full_name, u.email
ORDER BY COUNT(*) DESC, u.full_name, u.email
`

	rows, err := r.pool.Query(ctx, query, pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.WorkloadItem, 0)
	for rows.Next() {
		var (
			assigneeID pgtype.UUID
			fullName   pgtype.Text
			email      pgtype.Text
			count      int64
		)
		if err := rows.Scan(&assigneeID, &fullName, &email, &count); err != nil {
			return nil, err
		}

		var idPtr *uuid.UUID
		if assigneeID.Valid {
			value := uuid.UUID(assigneeID.Bytes)
			idPtr = &value
		}

		items = append(items, domain.WorkloadItem{
			AssigneeID: idPtr,
			FullName:   textOrEmpty(fullName),
			Email:      textOrEmpty(email),
			Count:      count,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *AnalyticsRepository) fetchVolume(ctx context.Context, orgID uuid.UUID, days int) ([]domain.VolumePoint, error) {
	const query = `
WITH days AS (
  SELECT generate_series(
    date_trunc('day', NOW()) - ($2::int - 1) * interval '1 day',
    date_trunc('day', NOW()),
    interval '1 day'
  ) AS day
),
created AS (
  SELECT date_trunc('day', t.created_at) AS day, COUNT(*) AS created_count
  FROM tickets t
  JOIN users ru ON t.requester_id = ru.id
  WHERE ru.organization_id = $1
    AND t.created_at >= date_trunc('day', NOW()) - ($2::int - 1) * interval '1 day'
  GROUP BY 1
),
resolved AS (
  SELECT date_trunc('day', t.closed_at) AS day, COUNT(*) AS resolved_count
  FROM tickets t
  JOIN users ru ON t.requester_id = ru.id
  WHERE ru.organization_id = $1
    AND t.closed_at IS NOT NULL
    AND t.closed_at >= date_trunc('day', NOW()) - ($2::int - 1) * interval '1 day'
  GROUP BY 1
)
SELECT d.day,
       COALESCE(c.created_count, 0) AS created_count,
       COALESCE(r.resolved_count, 0) AS resolved_count
FROM days d
LEFT JOIN created c ON c.day = d.day
LEFT JOIN resolved r ON r.day = d.day
ORDER BY d.day
`

	rows, err := r.pool.Query(ctx, query, pgtype.UUID{Bytes: orgID, Valid: true}, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]domain.VolumePoint, 0)
	for rows.Next() {
		var (
			day           time.Time
			createdCount  int64
			resolvedCount int64
		)
		if err := rows.Scan(&day, &createdCount, &resolvedCount); err != nil {
			return nil, err
		}
		points = append(points, domain.VolumePoint{
			Day:           day,
			CreatedCount:  createdCount,
			ResolvedCount: resolvedCount,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return points, nil
}

func (r *AnalyticsRepository) fetchMTTRHours(ctx context.Context, orgID uuid.UUID) (float64, error) {
	const query = `
SELECT AVG(EXTRACT(EPOCH FROM (t.closed_at - t.created_at)))
FROM tickets t
JOIN users ru ON t.requester_id = ru.id
WHERE ru.organization_id = $1
  AND t.closed_at IS NOT NULL
`

	row := r.pool.QueryRow(ctx, query, pgtype.UUID{Bytes: orgID, Valid: true})
	var avgSeconds pgtype.Float8
	if err := row.Scan(&avgSeconds); err != nil {
		return 0, err
	}
	if !avgSeconds.Valid {
		return 0, nil
	}
	return avgSeconds.Float64 / 3600, nil
}

func textOrEmpty(text pgtype.Text) string {
	if text.Valid {
		return text.String
	}
	return ""
}
