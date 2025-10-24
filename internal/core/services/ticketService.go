package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type TicketService struct {
	ticketRepo ports.TicketRepository
	authzSvc   ports.AuthorizationService
}

var _ ports.TicketService = (*TicketService)(nil)

// NewTicketService now requires an AuthorizationService.
func NewTicketService(ticketRepo ports.TicketRepository, authzSvc ports.AuthorizationService) ports.TicketService {
	return &TicketService{
		ticketRepo: ticketRepo,
		authzSvc:   authzSvc,
	}
}

func (s *TicketService) CreateTicket(ctx context.Context, params ports.CreateTicketParams) (*domain.Ticket, error) {
	// Authorization Check
	canCreate, err := s.authzSvc.Can(ctx, params.RequesterID, "tickets:create")
	if err != nil {
		return nil, err
	}
	if !canCreate {
		return nil, ports.ErrForbidden
	}

	ticket, err := domain.NewTicket(params.Title, params.Description, params.Priority, params.RequesterID)
	if err != nil {
		return nil, err
	}

	return s.ticketRepo.Create(ctx, ticket)
}

func (s *TicketService) GetTicket(ctx context.Context, ticketID int64, viewerID uuid.UUID) (*domain.Ticket, error) {
	// Authorization Check
	canRead, err := s.authzSvc.Can(ctx, viewerID, "tickets:read")
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, ports.ErrForbidden
	}

	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	// Additional logic: A non-admin can only see their own tickets.
	// We'll need a more advanced check here in the future. For now, this is a good start.
	isOwner := ticket.RequesterID == viewerID
	if !isOwner {
		// Check if the user is an agent/admin who can view all tickets
		canReadAll, _ := s.authzSvc.Can(ctx, viewerID, "tickets:read:all") // Hypothetical permission
		if !canReadAll {
			return nil, ports.ErrForbidden
		}
	}

	return ticket, nil
}

func (s *TicketService) UpdateStatus(ctx context.Context, params ports.UpdateStatusParams) (*domain.Ticket, error) {
	// Authorization Check
	canUpdate, err := s.authzSvc.Can(ctx, params.ActorID, "tickets:update:status")
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, ports.ErrForbidden
	}

	ticket, err := s.ticketRepo.GetByID(ctx, params.TicketID)
	if err != nil {
		return nil, err
	}

	if err := ticket.UpdateStatus(params.Status); err != nil {
		return nil, err
	}

	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) AssignTicket(ctx context.Context, params ports.AssignTicketParams) (*domain.Ticket, error) {
	// Authorization Check
	canAssign, err := s.authzSvc.Can(ctx, params.ActorID, "tickets:assign")
	if err != nil {
		return nil, err
	}
	if !canAssign {
		return nil, ports.ErrForbidden
	}

	ticket, err := s.ticketRepo.GetByID(ctx, params.TicketID)
	if err != nil {
		return nil, err
	}

	if err := ticket.Assign(params.AssigneeID); err != nil {
		return nil, err
	}

	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) ListTickets(ctx context.Context, viewerID uuid.UUID) ([]*domain.Ticket, error) {
	// RBAC logic for listing tickets.
	// Can the user see all tickets, or just their own?
	canListAll, err := s.authzSvc.Can(ctx, viewerID, "tickets:list:all") // Hypothetical permission
	if err != nil {
		return nil, err
	}

	if canListAll {
		return s.ticketRepo.List(ctx)
	}

	// Default to listing only tickets requested by the user.
	return s.ticketRepo.ListByRequester(ctx, viewerID)
}
