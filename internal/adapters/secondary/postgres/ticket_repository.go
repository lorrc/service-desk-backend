package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/lorrc/service-desk-backend/internal/core/utils"
)

// TicketRepository is the postgres adapter for ticket persistence.
type TicketRepository struct {
	pool *pgxpool.Pool
}

var _ ports.TicketRepository = (*TicketRepository)(nil)

// NewTicketRepository creates a new ticket repository.
func NewTicketRepository(pool *pgxpool.Pool) ports.TicketRepository {
	return &TicketRepository{pool: pool}
}

// Create persists a new ticket to the database.
func (r *TicketRepository) Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	query := `
		INSERT INTO tickets (title, description, status, priority, requester_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, title, description, status, priority, requester_id, assignee_id, created_at, updated_at
	`

	return r.scanTicket(r.pool.QueryRow(ctx, query,
		ticket.Title,
		ticket.Description,
		string(ticket.Status),
		string(ticket.Priority),
		pgtype.UUID{Bytes: ticket.RequesterID, Valid: true},
		ticket.CreatedAt,
	))
}

// GetByID retrieves a ticket by its ID.
func (r *TicketRepository) GetByID(ctx context.Context, id int64) (*domain.Ticket, error) {
	query := `
		SELECT id, title, description, status, priority, requester_id, assignee_id, created_at, updated_at
		FROM tickets WHERE id = $1
	`

	ticket, err := r.scanTicket(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrTicketNotFound
		}
		return nil, err
	}
	return ticket, nil
}

// Update persists changes to an existing ticket.
func (r *TicketRepository) Update(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	query := `
		UPDATE tickets
		SET status = $2, assignee_id = $3, updated_at = $4
		WHERE id = $1
		RETURNING id, title, description, status, priority, requester_id, assignee_id, created_at, updated_at
	`

	var assigneeID pgtype.UUID
	if ticket.AssigneeID != nil {
		assigneeID = pgtype.UUID{Bytes: *ticket.AssigneeID, Valid: true}
	}

	var updatedAt pgtype.Timestamptz
	if ticket.UpdatedAt != nil {
		updatedAt = pgtype.Timestamptz{Time: *ticket.UpdatedAt, Valid: true}
	} else {
		updatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	}

	return r.scanTicket(r.pool.QueryRow(ctx, query,
		ticket.ID,
		string(ticket.Status),
		assigneeID,
		updatedAt,
	))
}

// ListPaginated retrieves all tickets with pagination and filters.
func (r *TicketRepository) ListPaginated(ctx context.Context, params ports.ListTicketsRepoParams) ([]*domain.Ticket, error) {
	query := `
		SELECT id, title, description, status, priority, requester_id, assignee_id, created_at, updated_at
		FROM tickets
		WHERE ($1::text IS NULL OR status = $1)
		  AND ($2::text IS NULL OR priority = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	var status, priority *string
	if params.Status.Valid {
		status = &params.Status.String
	}
	if params.Priority.Valid {
		priority = &params.Priority.String
	}

	rows, err := r.pool.Query(ctx, query, status, priority, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTickets(rows)
}

// ListByRequesterPaginated retrieves tickets for a specific requester with pagination.
func (r *TicketRepository) ListByRequesterPaginated(ctx context.Context, params ports.ListTicketsRepoParams) ([]*domain.Ticket, error) {
	query := `
		SELECT id, title, description, status, priority, requester_id, assignee_id, created_at, updated_at
		FROM tickets
		WHERE requester_id = $1
		  AND ($2::text IS NULL OR status = $2)
		  AND ($3::text IS NULL OR priority = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`

	var status, priority *string
	if params.Status.Valid {
		status = &params.Status.String
	}
	if params.Priority.Valid {
		priority = &params.Priority.String
	}

	rows, err := r.pool.Query(ctx, query, params.RequesterID, status, priority, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTickets(rows)
}

// scanTicket scans a single ticket row.
func (r *TicketRepository) scanTicket(row pgx.Row) (*domain.Ticket, error) {
	var ticket domain.Ticket
	var requesterID, assigneeID pgtype.UUID
	var description pgtype.Text
	var updatedAt pgtype.Timestamptz
	var status, priority string

	err := row.Scan(
		&ticket.ID,
		&ticket.Title,
		&description,
		&status,
		&priority,
		&requesterID,
		&assigneeID,
		&ticket.CreatedAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	ticket.Description = utils.FromString(description)
	ticket.Status = domain.TicketStatus(status)
	ticket.Priority = domain.TicketPriority(priority)
	ticket.RequesterID = requesterID.Bytes

	if assigneeID.Valid {
		id := uuid.UUID(assigneeID.Bytes)
		ticket.AssigneeID = &id
	}
	if updatedAt.Valid {
		ticket.UpdatedAt = &updatedAt.Time
	}

	return &ticket, nil
}

// scanTickets scans multiple ticket rows.
func (r *TicketRepository) scanTickets(rows pgx.Rows) ([]*domain.Ticket, error) {
	var tickets []*domain.Ticket

	for rows.Next() {
		var ticket domain.Ticket
		var requesterID, assigneeID pgtype.UUID
		var description pgtype.Text
		var updatedAt pgtype.Timestamptz
		var status, priority string

		err := rows.Scan(
			&ticket.ID,
			&ticket.Title,
			&description,
			&status,
			&priority,
			&requesterID,
			&assigneeID,
			&ticket.CreatedAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		ticket.Description = utils.FromString(description)
		ticket.Status = domain.TicketStatus(status)
		ticket.Priority = domain.TicketPriority(priority)
		ticket.RequesterID = requesterID.Bytes

		if assigneeID.Valid {
			id := uuid.UUID(assigneeID.Bytes)
			ticket.AssigneeID = &id
		}
		if updatedAt.Valid {
			ticket.UpdatedAt = &updatedAt.Time
		}

		tickets = append(tickets, &ticket)
	}

	return tickets, rows.Err()
}
