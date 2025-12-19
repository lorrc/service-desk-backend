package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/lorrc/service-desk-backend/internal/core/utils"
)

// TicketService implements business logic for ticket management
type TicketService struct {
	ticketRepo  ports.TicketRepository
	authzSvc    ports.AuthorizationService
	notifier    ports.Notifier
	broadcaster ports.EventBroadcaster
	wg          sync.WaitGroup
}

var _ ports.TicketService = (*TicketService)(nil)

// NewTicketService creates a new ticket service
func NewTicketService(
	ticketRepo ports.TicketRepository,
	authzSvc ports.AuthorizationService,
	notifier ports.Notifier,
	broadcaster ports.EventBroadcaster,
) ports.TicketService {
	return &TicketService{
		ticketRepo:  ticketRepo,
		authzSvc:    authzSvc,
		notifier:    notifier,
		broadcaster: broadcaster,
	}
}

// CreateTicket handles the use case for submitting a new ticket
func (s *TicketService) CreateTicket(ctx context.Context, params ports.CreateTicketParams) (*domain.Ticket, error) {
	// 1. Authorization Check
	canCreate, err := s.authzSvc.Can(ctx, params.RequesterID, "tickets:create")
	if err != nil {
		return nil, err
	}
	if !canCreate {
		return nil, apperrors.ErrForbidden
	}

	// 2. Create domain entity with validation
	ticketParams := domain.TicketParams{
		Title:       params.Title,
		Description: params.Description,
		Priority:    params.Priority,
		RequesterID: params.RequesterID,
	}

	ticket, err := domain.NewTicket(ticketParams)
	if err != nil {
		return nil, err // Validation errors are returned here
	}

	// 3. Persist the ticket
	return s.ticketRepo.Create(ctx, ticket)
}

// GetTicket retrieves a specific ticket with authorization
func (s *TicketService) GetTicket(ctx context.Context, ticketID int64, viewerID uuid.UUID) (*domain.Ticket, error) {
	// 1. Basic Authorization Check
	canRead, err := s.authzSvc.Can(ctx, viewerID, "tickets:read")
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, apperrors.ErrForbidden
	}

	// 2. Fetch the ticket
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	// 3. Check ownership or elevated permissions
	isOwner := ticket.IsOwnedBy(viewerID)
	isAssignee := ticket.IsAssignedTo(viewerID)

	if !isOwner && !isAssignee {
		// Check if the user can view all tickets (admin/agent)
		canReadAll, _ := s.authzSvc.Can(ctx, viewerID, "tickets:read:all")
		if !canReadAll {
			return nil, apperrors.ErrForbidden
		}
	}

	return ticket, nil
}

// UpdateStatus changes a ticket's status with business rule enforcement
func (s *TicketService) UpdateStatus(ctx context.Context, params ports.UpdateStatusParams) (*domain.Ticket, error) {
	// 1. Authorization Check
	canUpdate, err := s.authzSvc.Can(ctx, params.ActorID, "tickets:update:status")
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, apperrors.ErrForbidden
	}

	// 2. Fetch and update domain entity
	ticket, err := s.ticketRepo.GetByID(ctx, params.TicketID)
	if err != nil {
		return nil, err
	}

	// 3. Apply status change (domain validates the transition)
	if err := ticket.UpdateStatus(params.Status); err != nil {
		return nil, err
	}

	// 4. Persist changes
	updatedTicket, err := s.ticketRepo.Update(ctx, ticket)
	if err != nil {
		return nil, err
	}

	// 5. Send notification (async, in background context)
	if ticket.RequesterID != params.ActorID {
		go s.notifyStatusUpdate(ticket, params.ActorID)
	}

	// 6. Broadcast real-time event (async)
	go s.broadcastStatusUpdate(updatedTicket)

	return updatedTicket, nil
}

// AssignTicket assigns a ticket to an agent
func (s *TicketService) AssignTicket(ctx context.Context, params ports.AssignTicketParams) (*domain.Ticket, error) {
	// 1. Authorization Check
	canAssign, err := s.authzSvc.Can(ctx, params.ActorID, "tickets:assign")
	if err != nil {
		return nil, err
	}
	if !canAssign {
		return nil, apperrors.ErrForbidden
	}

	// 2. Fetch and update domain entity
	ticket, err := s.ticketRepo.GetByID(ctx, params.TicketID)
	if err != nil {
		return nil, err
	}

	// 3. Apply assignment (domain validates business rules)
	if err := ticket.Assign(params.AssigneeID); err != nil {
		return nil, err
	}

	// 4. Persist changes
	return s.ticketRepo.Update(ctx, ticket)
}

// ListTickets retrieves tickets based on user permissions
func (s *TicketService) ListTickets(ctx context.Context, params ports.ListTicketsParams) ([]*domain.Ticket, error) {
	// 1. Check if user can see all tickets
	canListAll, err := s.authzSvc.Can(ctx, params.ViewerID, "tickets:list:all")
	if err != nil {
		return nil, err
	}

	fetchLimit := params.Limit + 1

	repoParams := ports.ListTicketsRepoParams{
		Limit:    int32(fetchLimit),
		Offset:   int32(params.Offset),
		Status:   utils.ToNullString(params.Status),
		Priority: utils.ToNullString(params.Priority),
	}

	// ... execute query ...
	// 3. Query based on permissions
	if canListAll {
		return s.ticketRepo.ListPaginated(ctx, repoParams)
	}

	// Default: scope query to the requesting user's tickets
	repoParams.RequesterID = pgtype.UUID{Bytes: params.ViewerID, Valid: true}
	return s.ticketRepo.ListByRequesterPaginated(ctx, repoParams)
}

// notifyStatusUpdate sends email notification for status changes
func (s *TicketService) notifyStatusUpdate(ticket *domain.Ticket, actorID uuid.UUID) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Use background context since the HTTP request may be done
		ctx := context.Background()

		s.notifier.Notify(ctx, ports.NotificationParams{
			RecipientUserID: ticket.RequesterID,
			Subject:         fmt.Sprintf("Your ticket status has been updated: #%d", ticket.ID),
			Message:         fmt.Sprintf("The status of your ticket '%s' was changed to %s.", ticket.Title, ticket.Status),
			TicketID:        ticket.ID,
		})
	}()
}

// broadcastStatusUpdate sends real-time event for status changes
func (s *TicketService) broadcastStatusUpdate(ticket *domain.Ticket) {
	event := domain.Event{
		Type:     domain.EventStatusUpdated,
		Payload:  ticket,
		TicketID: ticket.ID,
	}
	_ = s.broadcaster.Broadcast(event)
}

func (s *TicketService) Shutdown() {
	s.wg.Wait()
}
