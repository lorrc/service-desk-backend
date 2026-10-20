package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// TicketRepository is the secondary adapter for ticket persistence.
type TicketRepository struct {
	pool *pgxpool.Pool
}

// Ensure TicketRepository implements the ports.TicketRepository interface.
var _ ports.TicketRepository = (*TicketRepository)(nil)

// NewTicketRepository creates a new ticket repository.
func NewTicketRepository(pool *pgxpool.Pool) ports.TicketRepository {
	return &TicketRepository{
		pool: pool,
	}
}

// mapDBTicketToDomain converts a database ticket model to a core domain model.
func mapDBTicketToDomain(dbTicket db.Ticket) *domain.Ticket {
	domainTicket := &domain.Ticket{
		ID:          dbTicket.ID,
		Title:       dbTicket.Title,
		Description: fromText(dbTicket.Description),
		Status:      domain.TicketStatus(dbTicket.Status),
		Priority:    domain.TicketPriority(dbTicket.Priority),
		CreatedAt:   dbTicket.CreatedAt.Time,
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
	if dbTicket.ClosedAt.Valid {
		domainTicket.ClosedAt = &dbTicket.ClosedAt.Time
	}

	return domainTicket
}

// mapDBTicketListToDomain is a helper to map slices of tickets.
func mapDBTicketListToDomain(dbTickets []db.Ticket) []*domain.Ticket {
	domainTickets := make([]*domain.Ticket, len(dbTickets))
	for i, dbTicket := range dbTickets {
		domainTickets[i] = mapDBTicketToDomain(dbTicket)
	}
	return domainTickets
}

// Create persists a new ticket entity.
func (r *TicketRepository) Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	q := db.New(GetDBTX(ctx, r.pool))
	params := db.CreateTicketParams{
		Title:       ticket.Title,
		Description: toText(ticket.Description),
		Status:      string(ticket.Status),
		Priority:    string(ticket.Priority),
		RequesterID: pgtype.UUID{Bytes: ticket.RequesterID, Valid: true},
	}

	createdTicket, err := q.CreateTicket(ctx, params)
	if err != nil {
		return nil, err
	}
	return mapDBTicketToDomain(createdTicket), nil
}

// GetByID retrieves a single ticket by its ID.
func (r *TicketRepository) GetByID(ctx context.Context, id int64, orgID uuid.UUID) (*domain.Ticket, error) {
	q := db.New(GetDBTX(ctx, r.pool))
	dbTicket, err := q.GetTicketByID(ctx, db.GetTicketByIDParams{ID: id, OrganizationID: pgtype.UUID{Bytes: orgID, Valid: true}})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrTicketNotFound
		}
		return nil, err
	}
	return mapDBTicketToDomain(dbTicket), nil
}

// Update persists changes to an existing ticket entity.
func (r *TicketRepository) Update(ctx context.Context, ticket *domain.Ticket, orgID uuid.UUID) (*domain.Ticket, error) {
	q := db.New(GetDBTX(ctx, r.pool))
	params := db.UpdateTicketParams{
		OrganizationID: pgtype.UUID{Bytes: orgID, Valid: true},
		ID:             ticket.ID,
		Status:         string(ticket.Status),
		AssigneeID: pgtype.UUID{
			Bytes: [16]byte{},
			Valid: ticket.AssigneeID != nil,
		},
		UpdatedAt: pgtype.Timestamptz{
			Time:  time.Time{},
			Valid: ticket.UpdatedAt != nil,
		},
		ClosedAt: pgtype.Timestamptz{
			Time:  time.Time{},
			Valid: ticket.ClosedAt != nil,
		},
	}

	if ticket.AssigneeID != nil {
		params.AssigneeID.Bytes = *ticket.AssigneeID
	}
	if ticket.UpdatedAt != nil {
		params.UpdatedAt.Time = *ticket.UpdatedAt
	} else {
		params.UpdatedAt.Time = time.Now().UTC()
		params.UpdatedAt.Valid = true
	}
	if ticket.ClosedAt != nil {
		params.ClosedAt.Time = *ticket.ClosedAt
	}

	updatedTicket, err := q.UpdateTicket(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrTicketNotFound
		}
		return nil, err
	}
	return mapDBTicketToDomain(updatedTicket), nil
}

// GetForUpdate locks a ticket until the surrounding transaction completes.
func (r *TicketRepository) GetForUpdate(ctx context.Context, id int64, orgID uuid.UUID) (*domain.Ticket, error) {
	if _, ok := TxFromContext(ctx); !ok {
		return nil, fmt.Errorf("ticket lock requires a transaction")
	}
	ticket, err := db.New(GetDBTX(ctx, r.pool)).GetTicketForUpdate(ctx, db.GetTicketForUpdateParams{ID: id, OrganizationID: pgtype.UUID{Bytes: orgID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrTicketNotFound
	}
	if err != nil {
		return nil, err
	}
	return mapDBTicketToDomain(ticket), nil
}

// ListPaginated retrieves tickets within the authorized organization and requester scope.
func (r *TicketRepository) ListPaginated(ctx context.Context, params ports.ListTicketsRepoParams) ([]*domain.Ticket, error) {
	rows, err := db.New(GetDBTX(ctx, r.pool)).ListTicketsPaginated(ctx, db.ListTicketsPaginatedParams{
		OrganizationID: pgtype.UUID{Bytes: params.OrganizationID, Valid: true},
		RequesterID:    nullableUUID(params.RequesterID), Active: params.Active, Search: params.Query,
		Limit: int32(params.Limit), Offset: int32(params.Offset),
		Status: nullableText(params.Status), Priority: nullableText(params.Priority),
		AssigneeID: nullableUUID(params.AssigneeID), Unassigned: params.Unassigned,
		CreatedFrom: nullableTime(params.CreatedFrom), CreatedTo: nullableTime(params.CreatedTo),
	})
	if err != nil {
		return nil, err
	}
	return mapDBTicketListToDomain(rows), nil
}

// GetCounts aggregates the complete authorized ticket set.
func (r *TicketRepository) GetCounts(ctx context.Context, params ports.TicketCountsRepoParams) (*domain.TicketCounts, error) {
	counts, err := db.New(GetDBTX(ctx, r.pool)).GetTicketCounts(ctx, db.GetTicketCountsParams{
		OrganizationID: pgtype.UUID{Bytes: params.OrganizationID, Valid: true}, RequesterID: nullableUUID(params.RequesterID), ViewerID: pgtype.UUID{Bytes: params.ViewerID, Valid: true}, Search: params.Query, CreatedFrom: nullableTime(params.CreatedFrom), CreatedTo: nullableTime(params.CreatedTo),
	})
	if err != nil {
		return nil, err
	}
	return &domain.TicketCounts{All: counts.All, Unassigned: counts.Unassigned, AssignedToMe: counts.AssignedToMe, Open: counts.Open, InProgress: counts.InProgress, Closed: counts.Closed, HighPriority: counts.HighPriority}, nil
}

func nullableUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func nullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}
