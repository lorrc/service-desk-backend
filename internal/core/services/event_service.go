package services

import (
	"context"

	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// EventService handles ticket event queries.
type EventService struct {
	eventRepo ports.TicketEventRepository
	ticketSvc ports.TicketService
}

var _ ports.EventService = (*EventService)(nil)

// NewEventService creates a new event service.
func NewEventService(
	eventRepo ports.TicketEventRepository,
	ticketSvc ports.TicketService,
) ports.EventService {
	return &EventService{
		eventRepo: eventRepo,
		ticketSvc: ticketSvc,
	}
}

// ListTicketEvents retrieves events for a ticket after the given cursor.
func (s *EventService) ListTicketEvents(ctx context.Context, params ports.ListTicketEventsParams) ([]*domain.Event, error) {
	// Reuse ticket service authorization logic.
	if _, err := s.ticketSvc.GetTicket(ctx, params.TicketID, params.ViewerID); err != nil {
		return nil, err
	}

	return s.eventRepo.ListByTicketID(ctx, params.TicketID, params.AfterID, params.Limit)
}
