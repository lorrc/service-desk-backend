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
		ID:        dbTicket.ID,
		Title:     dbTicket.Title,
		Status:    domain.TicketStatus(dbTicket.Status),
		Priority:  domain.TicketPriority(dbTicket.Priority),
		CreatedAt: dbTicket.CreatedAt.Time,
	}

	if dbTicket.Description != nil {
		domainTicket.Description = *dbTicket.Description
	}
	// Safely convert pgtype.UUID to uuid.UUID for RequesterID
	if dbTicket.RequesterID.Valid {
		// Explicit type cast from [16]byte to uuid.UUID
		domainTicket.RequesterID = uuid.UUID(dbTicket.RequesterID.Bytes)
	}

	// Handle nullable AssigneeID with the correct type cast
	if dbTicket.AssigneeID.Valid {
		// 1. Cast the [16]byte to uuid.UUID
		assigneeUUID := uuid.UUID(dbTicket.AssigneeID.Bytes)
		// 2. Assign the pointer of the correctly typed variable
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

	var description *string
	if ticket.Description != "" {
		description = &ticket.Description
	}

	params := db.CreateTicketParams{
		Title:       ticket.Title,
		Description: description, // <-- This is the fix
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
	params := db.UpdateTicketParams{
		ID:     ticket.ID,
		Status: string(ticket.Status),
		AssigneeID: pgtype.UUID{
			Bytes: [16]byte{},
			Valid: ticket.AssigneeID != nil,
		},
		UpdatedAt: pgtype.Timestamptz{
			Time:  time.Time{},
			Valid: ticket.UpdatedAt != nil,
		},
	}

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

// ListByRequester retrieves tickets created by a specific user.
func (r *TicketRepository) ListByRequester(ctx context.Context, requesterID uuid.UUID) ([]*domain.Ticket, error) {
	dbTickets, err := r.q.ListTicketsByRequesterID(ctx, pgtype.UUID{Bytes: requesterID, Valid: true})
	if err != nil {
		return nil, err
	}

	domainTickets := make([]*domain.Ticket, len(dbTickets))
	for i, dbTicket := range dbTickets {
		domainTickets[i] = mapDBTicketToDomain(dbTicket)
	}

	return domainTickets, nil
}

// List retrieves all tickets.
func (r *TicketRepository) List(ctx context.Context) ([]*domain.Ticket, error) {
	dbTickets, err := r.q.ListTickets(ctx)
	if err != nil {
		return nil, err
	}

	domainTickets := make([]*domain.Ticket, len(dbTickets))
	for i, dbTicket := range dbTickets {
		domainTickets[i] = mapDBTicketToDomain(dbTicket)
	}

	return domainTickets, nil
}
