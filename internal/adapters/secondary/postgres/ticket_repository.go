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
	"github.com/lorrc/service-desk-backend/internal/core/utils" // <-- Import utils
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
		// ** FIX for Line 43 **
		// Use helper to convert pgtype.Text -> string
		Description: utils.FromString(dbTicket.Description),
	}

	if dbTicket.RequesterID.Valid {
		domainTicket.RequesterID = dbTicket.RequesterID.Bytes
	}
	if dbTicket.AssigneeID.Valid {
		assigneeUUID := uuid.UUID(dbTicket.AssigneeID.Bytes)
		domainTicket.AssigneeID = &assigneeUUID
	}
	if dbTicket.UpdatedAt.Valid {
		domainTicket.UpdatedAt = &dbTicket.UpdatedAt.Time
	}

	return domainTicket
}

// Create persists a new ticket entity.
func (r *TicketRepository) Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	params := db.CreateTicketParams{
		Title:    ticket.Title,
		Priority: string(ticket.Priority),
		// ** FIX for Create **
		// Use helper to convert string -> pgtype.Text
		Description: utils.ToString(ticket.Description),
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

// mapDBTicketListToDomain is a helper to map slices
func mapDBTicketListToDomain(dbTickets []db.Ticket) []*domain.Ticket {
	domainTickets := make([]*domain.Ticket, len(dbTickets))
	for i, dbTicket := range dbTickets {
		domainTickets[i] = mapDBTicketToDomain(dbTicket)
	}
	return domainTickets
}

// ListPaginated retrieves all tickets with pagination and optional filters.
func (r *TicketRepository) ListPaginated(ctx context.Context, params ports.ListTicketsRepoParams) ([]*domain.Ticket, error) {
	// The dbParams fields (pgtype.Text) now correctly match the ports.ListTicketsRepoParams fields
	dbParams := db.ListTicketsPaginatedParams{
		Limit:    params.Limit,
		Offset:   params.Offset,
		Status:   params.Status,
		Priority: params.Priority,
	}

	dbTickets, err := r.q.ListTicketsPaginated(ctx, dbParams)
	if err != nil {
		return nil, err
	}

	return mapDBTicketListToDomain(dbTickets), nil
}

// ListByRequesterPaginated retrieves tickets for a specific user with pagination and optional filters.
func (r *TicketRepository) ListByRequesterPaginated(ctx context.Context, params ports.ListTicketsRepoParams) ([]*domain.Ticket, error) {
	// The dbParams fields (pgtype.Text) now correctly match the ports.ListTicketsRepoParams fields
	dbParams := db.ListTicketsByRequesterPaginatedParams{
		RequesterID: params.RequesterID,
		Limit:       params.Limit,
		Offset:      params.Offset,
		Status:      params.Status,
		Priority:    params.Priority,
	}

	dbTickets, err := r.q.ListTicketsByRequesterPaginated(ctx, dbParams)
	if err != nil {
		return nil, err
	}

	return mapDBTicketListToDomain(dbTickets), nil
}
