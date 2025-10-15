package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type TicketService struct {
	ticketRepo ports.TicketRepository
}

var _ ports.TicketService = (*TicketService)(nil)

func NewTicketService(ticketRepo ports.TicketRepository) ports.TicketService {
	return &TicketService{ticketRepo: ticketRepo}
}

func (s *TicketService) CreateTicket(ctx context.Context, params ports.CreateTicketParams) (*domain.Ticket, error) {
	ticket, err := domain.NewTicket(params.Title, params.Description, params.Priority, params.RequesterID)
	if err != nil {
		return nil, err
	}

	return s.ticketRepo.Create(ctx, ticket)
}

func (s *TicketService) GetTicket(ctx context.Context, ticketID int64, viewerID uuid.UUID) (*domain.Ticket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err // Can be ErrTicketNotFound
	}

	// Authorization logic: For now, only the requester can view the ticket.
	// In a real app, this would be more complex (e.g., admins, assignees).
	if ticket.RequesterID != viewerID {
		return nil, ports.ErrForbidden
	}

	return ticket, nil
}

func (s *TicketService) UpdateStatus(ctx context.Context, params ports.UpdateStatusParams) (*domain.Ticket, error) {
	ticket, err := s.GetTicket(ctx, params.TicketID, params.ActorID) // Re-use GetTicket for initial auth check
	if err != nil {
		return nil, err
	}

	if err := ticket.UpdateStatus(params.Status); err != nil {
		return nil, err
	}

	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) AssignTicket(ctx context.Context, params ports.AssignTicketParams) (*domain.Ticket, error) {
	ticket, err := s.GetTicket(ctx, params.TicketID, params.ActorID) // Re-use GetTicket for initial auth check
	if err != nil {
		return nil, err
	}

	// More complex auth would be here (e.g., only admins can assign)

	if err := ticket.Assign(params.AssigneeID); err != nil {
		return nil, err
	}

	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) ListTickets(ctx context.Context, viewerID uuid.UUID) ([]*domain.Ticket, error) {
	// Not implemented in this phase
	return nil, nil
}
