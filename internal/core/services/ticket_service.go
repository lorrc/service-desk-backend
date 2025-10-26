package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/lorrc/service-desk-backend/internal/core/utils"
)

type TicketService struct {
	ticketRepo  ports.TicketRepository
	authzSvc    ports.AuthorizationService
	notifier    ports.Notifier
	broadcaster ports.EventBroadcaster
}

var _ ports.TicketService = (*TicketService)(nil)

func NewTicketService(ticketRepo ports.TicketRepository, authzSvc ports.AuthorizationService, notifier ports.Notifier, broadcaster ports.EventBroadcaster) ports.TicketService {
	return &TicketService{
		ticketRepo:  ticketRepo,
		authzSvc:    authzSvc,
		notifier:    notifier,
		broadcaster: broadcaster,
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
	// 1. Authorization Check
	canUpdate, err := s.authzSvc.Can(ctx, params.ActorID, "tickets:update:status")
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, ports.ErrForbidden
	}

	// 2. Fetch and update domain entity
	ticket, err := s.ticketRepo.GetByID(ctx, params.TicketID)
	if err != nil {
		return nil, err
	}

	if err := ticket.UpdateStatus(params.Status); err != nil {
		return nil, err
	}

	updatedTicket, err := s.ticketRepo.Update(ctx, ticket)
	if err != nil {
		return nil, err
	}

	// 3. Send email notification (asynchronously)
	if ticket.RequesterID != params.ActorID {
		go s.notifier.Notify(ctx, ports.NotificationParams{
			RecipientUserID: ticket.RequesterID,
			Subject:         fmt.Sprintf("Your ticket status has been updated: #%d", ticket.ID),
			Message:         fmt.Sprintf("The status of your ticket '%s' was changed to %s.", ticket.Title, ticket.Status),
			TicketID:        ticket.ID,
		})
	}

	// 4. Hbroadcast real-time event (asynchronously)
	event := domain.Event{
		Type:     domain.EventStatusUpdated,
		Payload:  updatedTicket,
		TicketID: updatedTicket.ID,
	}
	go s.broadcaster.Broadcast(event)

	return updatedTicket, nil
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

func (s *TicketService) ListTickets(ctx context.Context, params ports.ListTicketsParams) ([]*domain.Ticket, error) {
	// 1. RBAC Check
	canListAll, err := s.authzSvc.Can(ctx, params.ViewerID, "tickets:list:all")
	if err != nil {
		return nil, err
	}

	repoParams := ports.ListTicketsRepoParams{
		Limit:    int32(params.Limit),
		Offset:   int32(params.Offset),
		Status:   utils.ToNullString(params.Status),   // Use helper
		Priority: utils.ToNullString(params.Priority), // Use helper
	}

	// 3. Call the correct repository method based on permissions
	if canListAll {
		return s.ticketRepo.ListPaginated(ctx, repoParams)
	}

	// Default: scope query to the user who made the request
	repoParams.RequesterID = pgtype.UUID{Bytes: params.ViewerID, Valid: true}
	return s.ticketRepo.ListByRequesterPaginated(ctx, repoParams)
}

// ... (rest of TicketService) ...
