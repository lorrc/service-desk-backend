package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// TicketEventRepository handles persistence for ticket events.
type TicketEventRepository struct {
	pool *pgxpool.Pool
}

var _ ports.TicketEventRepository = (*TicketEventRepository)(nil)

// NewTicketEventRepository creates a new ticket event repository.
func NewTicketEventRepository(pool *pgxpool.Pool) ports.TicketEventRepository {
	return &TicketEventRepository{pool: pool}
}

func mapDBTicketEventToDomain(dbEvent db.TicketEvent) *domain.Event {
	var actorID uuid.UUID
	if dbEvent.ActorID.Valid {
		actorID = uuid.UUID(dbEvent.ActorID.Bytes)
	}

	return &domain.Event{
		ID:        dbEvent.ID,
		TicketID:  dbEvent.TicketID,
		Type:      domain.EventType(dbEvent.Type),
		Payload:   json.RawMessage(dbEvent.Payload),
		ActorID:   actorID,
		CreatedAt: dbEvent.CreatedAt.Time,
	}
}

// Create persists a new ticket event.
func (r *TicketEventRepository) Create(ctx context.Context, event *domain.Event) (*domain.Event, error) {
	q := db.New(GetDBTX(ctx, r.pool))
	params := db.CreateTicketEventParams{
		TicketID: event.TicketID,
		Type:     string(event.Type),
		Payload:  []byte(event.Payload),
		ActorID:  pgtype.UUID{Bytes: event.ActorID, Valid: true},
	}

	dbEvent, err := q.CreateTicketEvent(ctx, params)
	if err != nil {
		return nil, err
	}

	return mapDBTicketEventToDomain(dbEvent), nil
}

// ListByTicketID retrieves events for a ticket after a cursor.
func (r *TicketEventRepository) ListByTicketID(ctx context.Context, ticketID int64, afterID int64, limit int) ([]*domain.Event, error) {
	q := db.New(GetDBTX(ctx, r.pool))
	params := db.ListTicketEventsParams{
		TicketID: ticketID,
		ID:       afterID,
		Limit:    int32(limit),
	}

	dbEvents, err := q.ListTicketEvents(ctx, params)
	if err != nil {
		return nil, err
	}

	events := make([]*domain.Event, 0, len(dbEvents))
	for _, dbEvent := range dbEvents {
		events = append(events, mapDBTicketEventToDomain(dbEvent))
	}

	return events, nil
}
