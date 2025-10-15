package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// TicketRepository is the secondary adapter for ticket persistence.
type TicketRepository struct {
	q db.Querier
}

// Ensure TicketRepository implements the ports.TicketRepository interface.
var _ ports.TicketRepository = (*TicketRepository)(nil)

// NewTicketRepository creates a new ticket repository.
func NewTicketRepository(pool *pgxpool.Pool) ports.TicketRepository {
	return &TicketRepository{
		q: db.New(pool),
	}
}

// mapDBTicketToDomain converts a database ticket model to a core domain model.
func mapDBTicketToDomain(dbTicket db.Ticket) *domain.Ticket {
	domainTicket := &domain.Ticket{
		ID:          dbTicket.ID,
		Title:       dbTicket.Title,
		Description: dbTicket.Description.String,
		Status:      domain.TicketStatus(dbTicket.Status),
		Priority:    domain.TicketPriority(dbTicket.Priority),
		CreatedAt:   dbTicket.CreatedAt.Time,
	}

	// Safely convert pgtype.UUID to uuid.UUID for RequesterID
	if dbTicket.RequesterID.Valid {
		domainTicket.RequesterID = dbTicket.RequesterID.Bytes
	}

	// Handle nullable AssigneeID
	if dbTicket.AssigneeID.Valid {
		assigneeUUID := dbTicket.AssigneeID.Bytes
		domainTicket.AssigneeID = &assigneeUUID
	}

	// Handle nullable UpdatedAt
	if dbTicket.UpdatedAt.Valid {
		domainTicket.UpdatedAt = &dbTicket.UpdatedAt.Time
	}

	return domainTicket
}

// Create persists a new ticket entity.
func (r *TicketRepository) Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	params := db.CreateTicketParams{
		Title:       ticket.Title,
		Description: pgtype.Text{String: ticket.Description, Valid: ticket.Description != ""},
		Priority:    string(ticket.Priority),
		RequesterID: pgtype.UUID{Bytes: ticket.RequesterID, Valid: true},
	}

	createdTicket, err := r.q.CreateTicket(ctx, params)
	if err != nil {
		return nil, err
	}

	return mapDBTicketToDomain(createdTicket), nil
}

// GetByID retrieves a single ticket by its ID.
func (r *TicketRepository) GetByID(ctx context.Context, id int64) (*domain.Ticket, error) {
	dbTicket, err := r.q.GetTicketByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrTicketNotFound
		}
		return nil, err
	}
	return mapDBTicketToDomain(dbTicket), nil
}

// Update persists changes to an existing ticket entity.
func (r *TicketRepository) Update(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	// Prepare params for the update query
	params := db.UpdateTicketParams{
		ID:     ticket.ID,
		Status: string(ticket.Status),
		// Safely handle nullable assignee ID
		AssigneeID: pgtype.UUID{
			Bytes: [16]byte{}, // Default zero value
			Valid: ticket.AssigneeID != nil,
		},
		// Safely handle nullable updated at timestamp
		UpdatedAt: pgtype.Timestamptz{
			Time:  time.Time{}, // Default zero value
			Valid: ticket.UpdatedAt != nil,
		},
	}

	// Only set the Bytes and Time values if the pointers are not nil
	if ticket.AssigneeID != nil {
		params.AssigneeID.Bytes = *ticket.AssigneeID
	}
	if ticket.UpdatedAt != nil {
		params.UpdatedAt.Time = *ticket.UpdatedAt
	}

	updatedTicket, err := r.q.UpdateTicket(ctx, params)
	if err != nil {
		return nil, err
	}
	return mapDBTicketToDomain(updatedTicket), nil
}

// List is not implemented in this phase.
func (r *TicketRepository) List(ctx context.Context) ([]*domain.Ticket, error) {
	return nil, errors.New("not implemented")
}

// ListByRequester is not implemented in this phase.
func (r *TicketRepository) ListByRequester(ctx context.Context, requesterID uuid.UUID) ([]*domain.Ticket, error) {
	return nil, errors.New("not implemented")
}
