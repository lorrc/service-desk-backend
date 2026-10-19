package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// TicketService manages authorized ticket changes and their event history.
type TicketService struct {
	ticketRepo ports.TicketRepository
	userRepo   ports.UserRepository
	authzSvc   ports.AuthorizationService
	notifier   ports.Notifier
	eventRepo  ports.TicketEventRepository
	txManager  ports.TransactionManager
	wg         sync.WaitGroup
}

var _ ports.TicketService = (*TicketService)(nil)

// NewTicketService creates a ticket service.
func NewTicketService(ticketRepo ports.TicketRepository, authzSvc ports.AuthorizationService, notifier ports.Notifier, eventRepo ports.TicketEventRepository, txManager ports.TransactionManager, userRepo ports.UserRepository) ports.TicketService {
	return &TicketService{ticketRepo: ticketRepo, userRepo: userRepo, authzSvc: authzSvc, notifier: notifier, eventRepo: eventRepo, txManager: txManager}
}

func (s *TicketService) requirePermission(ctx context.Context, actorID, orgID uuid.UUID, permission string) error {
	if actorID == uuid.Nil || orgID == uuid.Nil {
		return apperrors.ErrForbidden
	}
	allowed, err := s.authzSvc.Can(ctx, actorID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.ErrForbidden
	}
	return nil
}

// CreateTicket creates a ticket and its initial event atomically.
func (s *TicketService) CreateTicket(ctx context.Context, params ports.CreateTicketParams) (*domain.Ticket, error) {
	if err := s.requirePermission(ctx, params.RequesterID, params.OrganizationID, "tickets:create"); err != nil {
		return nil, err
	}
	requester, err := s.userRepo.GetByID(ctx, params.RequesterID)
	if err != nil {
		return nil, err
	}
	if !requester.IsActive || requester.OrganizationID != params.OrganizationID {
		return nil, apperrors.ErrForbidden
	}
	ticket, err := domain.NewTicket(domain.TicketParams{Title: params.Title, Description: params.Description, Priority: params.Priority, RequesterID: params.RequesterID})
	if err != nil {
		return nil, err
	}
	var created *domain.Ticket
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		var err error
		created, err = s.ticketRepo.Create(txCtx, ticket)
		if err != nil {
			return err
		}
		return s.recordEvent(txCtx, created, params.RequesterID, domain.EventTicketCreated)
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// GetTicket retrieves a ticket within the viewer's organization and access rights.
func (s *TicketService) GetTicket(ctx context.Context, ticketID int64, viewerID, orgID uuid.UUID) (*domain.Ticket, error) {
	if err := s.requirePermission(ctx, viewerID, orgID, "tickets:read"); err != nil {
		return nil, err
	}
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID, orgID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTicketAccess(ctx, ticket, viewerID, orgID); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *TicketService) requireTicketAccess(ctx context.Context, ticket *domain.Ticket, viewerID, orgID uuid.UUID) error {
	if ticket.IsOwnedBy(viewerID) || ticket.IsAssignedTo(viewerID) {
		return nil
	}
	return s.requirePermission(ctx, viewerID, orgID, "tickets:read:all")
}

// UpdateStatus validates the transition against the locked, current ticket state.
func (s *TicketService) UpdateStatus(ctx context.Context, params ports.UpdateStatusParams) (*domain.Ticket, error) {
	if err := s.requirePermission(ctx, params.ActorID, params.OrganizationID, "tickets:update:status"); err != nil {
		return nil, err
	}
	updated, err := s.changeTicket(ctx, params.TicketID, params.ActorID, params.OrganizationID, domain.EventStatusUpdated,
		func(ticket *domain.Ticket) error { return ticket.UpdateStatus(params.Status) })
	if err != nil {
		return nil, err
	}
	if updated.RequesterID != params.ActorID {
		s.notifyStatusUpdate(updated)
	}
	return updated, nil
}

// AssignTicket only assigns active, eligible users within the same organization.
func (s *TicketService) AssignTicket(ctx context.Context, params ports.AssignTicketParams) (*domain.Ticket, error) {
	if err := s.requirePermission(ctx, params.ActorID, params.OrganizationID, "tickets:assign"); err != nil {
		return nil, err
	}
	assignee, err := s.userRepo.GetByID(ctx, params.AssigneeID)
	if err != nil {
		return nil, err
	}
	if !assignee.IsActive || assignee.OrganizationID != params.OrganizationID {
		return nil, apperrors.ErrForbidden
	}
	if err := s.requirePermission(ctx, assignee.ID, params.OrganizationID, "tickets:assign"); err != nil {
		return nil, err
	}
	return s.changeTicket(ctx, params.TicketID, params.ActorID, params.OrganizationID, domain.EventTicketAssigned,
		func(ticket *domain.Ticket) error { return ticket.Assign(params.AssigneeID) })
}

func (s *TicketService) changeTicket(ctx context.Context, id int64, actorID, orgID uuid.UUID, eventType domain.EventType, change func(*domain.Ticket) error) (*domain.Ticket, error) {
	if err := s.requirePermission(ctx, actorID, orgID, "tickets:read"); err != nil {
		return nil, err
	}
	var updated *domain.Ticket
	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		ticket, err := s.ticketRepo.GetForUpdate(txCtx, id, orgID)
		if err != nil {
			return err
		}
		if err := s.requireTicketAccess(txCtx, ticket, actorID, orgID); err != nil {
			return err
		}
		if err := change(ticket); err != nil {
			return err
		}
		updated, err = s.ticketRepo.Update(txCtx, ticket, orgID)
		if err != nil {
			return err
		}
		return s.recordEvent(txCtx, updated, actorID, eventType)
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *TicketService) recordEvent(ctx context.Context, ticket *domain.Ticket, actorID uuid.UUID, eventType domain.EventType) error {
	payload, err := marshalEventPayload(domain.NewTicketSnapshot(ticket))
	if err != nil {
		return err
	}
	_, err = s.eventRepo.Create(ctx, &domain.Event{TicketID: ticket.ID, Type: eventType, Payload: payload, ActorID: actorID})
	return err
}

func (s *TicketService) requesterScope(ctx context.Context, viewerID, orgID uuid.UUID) (*uuid.UUID, error) {
	if err := s.requirePermission(ctx, viewerID, orgID, "tickets:read"); err != nil {
		return nil, err
	}
	all, err := s.authzSvc.Can(ctx, viewerID, "tickets:list:all")
	if err != nil {
		return nil, err
	}
	if all {
		return nil, nil
	}
	return &viewerID, nil
}

// ListTickets lists tickets in the same scope used by dashboard counts.
func (s *TicketService) ListTickets(ctx context.Context, params ports.ListTicketsParams) ([]*domain.Ticket, error) {
	requesterID, err := s.requesterScope(ctx, params.ViewerID, params.OrganizationID)
	if err != nil {
		return nil, err
	}
	if params.Limit <= 0 || params.Limit > 100 || params.Offset < 0 || params.Offset > 2147483647 {
		errs := apperrors.NewValidationErrors()
		errs.Add("pagination", "Limit must be between 1 and 100 and offset must fit a nonnegative 32-bit integer")
		return nil, errs
	}
	return s.ticketRepo.ListPaginated(ctx, ports.ListTicketsRepoParams{
		OrganizationID: params.OrganizationID, RequesterID: requesterID,
		Limit: params.Limit + 1, Offset: params.Offset,
		Status: params.Status, Priority: params.Priority, AssigneeID: params.AssigneeID,
		Unassigned: params.Unassigned, CreatedFrom: params.CreatedFrom, CreatedTo: params.CreatedTo,
	})
}

// GetTicketCounts counts all visible tickets rather than a single page.
func (s *TicketService) GetTicketCounts(ctx context.Context, viewerID, orgID uuid.UUID) (*domain.TicketCounts, error) {
	requesterID, err := s.requesterScope(ctx, viewerID, orgID)
	if err != nil {
		return nil, err
	}
	return s.ticketRepo.GetCounts(ctx, orgID, requesterID)
}

func (s *TicketService) notifyStatusUpdate(ticket *domain.Ticket) {
	s.wg.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.notifier.Notify(ctx, ports.NotificationParams{
			RecipientUserID: ticket.RequesterID,
			Subject:         fmt.Sprintf("Your ticket status has been updated: #%d", ticket.ID),
			Message:         fmt.Sprintf("The status of your ticket '%s' was changed to %s.", ticket.Title, ticket.Status),
			TicketID:        ticket.ID,
		})
	})
}

// Shutdown waits for pending status notifications.
func (s *TicketService) Shutdown() { s.wg.Wait() }
