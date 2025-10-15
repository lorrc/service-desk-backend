package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type TicketRepository struct {
	q db.Querier
}

var _ ports.TicketRepository = (*TicketRepository)(nil)

func NewTicketRepository(pool *pgxpool.Pool) ports.TicketRepository {
	return &TicketRepository{
		q: db.New(pool),
	}
}

func mapDBTicketToDomain(dbTicket db.Ticket) *domain.Ticket {
	domainTicket := &domain.Ticket{
		ID:          dbTicket.ID,
		Title:       dbTicket.Title,
		Description: dbTicket.Description.String,
		Status:      domain.TicketStatus(dbTicket.Status),
		Priority:    domain.TicketPriority(dbTicket.Priority),
		CreatedAt:   dbTicket.CreatedAt.Time,
	}

	// Convert pgtype.UUID to uuid.UUID for RequesterID
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

func (r *TicketRepository) Update(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	params := db.UpdateTicketParams{
		ID:     ticket.ID,
		Status: string(ticket.Status),
		AssigneeID: pgtype.UUID{
			Valid: ticket.AssigneeID != nil,
		},
		UpdatedAt: pgtype.Timestamptz{Time: *ticket.UpdatedAt, Valid: ticket.UpdatedAt != nil},
	}
	if ticket.AssigneeID != nil {
		params.AssigneeID.Bytes = *ticket.AssigneeID
	}

	updatedTicket, err := r.q.UpdateTicket(ctx, params)
	if err != nil {
		return nil, err
	}
	return mapDBTicketToDomain(updatedTicket), nil
}

func (r *TicketRepository) List(ctx context.Context) ([]*domain.Ticket, error) {
	// This will be implemented in a later phase with filtering
	return nil, errors.New("not implemented")
}

func (r *TicketRepository) ListByRequester(ctx context.Context, requesterID uuid.UUID) ([]*domain.Ticket, error) {
	// This will be implemented in a later phase with filtering
	return nil, errors.New("not implemented")
}
