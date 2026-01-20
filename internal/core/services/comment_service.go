package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// CommentService implements the business logic for comments.
type CommentService struct {
	commentRepo ports.CommentRepository
	ticketSvc   ports.TicketService
	authzSvc    ports.AuthorizationService
	notifier    ports.Notifier
	eventRepo   ports.TicketEventRepository
	txManager   ports.TransactionManager
}

// Ensure implementation matches the interface.
var _ ports.CommentService = (*CommentService)(nil)

// NewCommentService creates a new service for comment logic.
func NewCommentService(
	commentRepo ports.CommentRepository,
	ticketSvc ports.TicketService,
	authzSvc ports.AuthorizationService,
	notifier ports.Notifier,
	eventRepo ports.TicketEventRepository,
	txManager ports.TransactionManager,
) ports.CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		ticketSvc:   ticketSvc,
		authzSvc:    authzSvc,
		notifier:    notifier,
		eventRepo:   eventRepo,
		txManager:   txManager,
	}
}

// canUserAccessTicket is a helper to check if a user can view a ticket,
// which is a prerequisite for viewing or making comments.
func (s *CommentService) canUserAccessTicket(ctx context.Context, ticketID int64, actorID uuid.UUID) (bool, error) {
	// We re-use the GetTicket service method, as it already contains
	// the necessary ownership and RBAC logic ("tickets:read", "tickets:read:all").
	_, err := s.ticketSvc.GetTicket(ctx, ticketID, actorID)
	if err != nil {
		if errors.Is(err, apperrors.ErrForbidden) || errors.Is(err, apperrors.ErrTicketNotFound) {
			return false, apperrors.ErrForbidden // Return a generic Forbidden
		}
		return false, err // Other system error
	}
	return true, nil
}

// CreateComment adds a new comment to a ticket.
func (s *CommentService) CreateComment(ctx context.Context, params ports.CreateCommentParams) (*domain.Comment, error) {
	// 1. Check permission to create comments.
	canCreate, err := s.authzSvc.Can(ctx, params.ActorID, "comments:create")
	if err != nil {
		return nil, err
	}
	if !canCreate {
		return nil, apperrors.ErrForbidden
	}

	// 2. Check if the user can access the ticket they're trying to comment on.
	// We use GetTicket directly here to fetch the ticket object for the notification.
	ticket, err := s.ticketSvc.GetTicket(ctx, params.TicketID, params.ActorID)
	if err != nil {
		// GetTicket already returns ErrForbidden if access is denied
		return nil, err
	}

	// 3. Create the domain entity using the new params-based constructor.
	commentParams := domain.CommentParams{
		TicketID: params.TicketID,
		AuthorID: params.ActorID,
		Body:     params.Body,
	}
	comment, err := domain.NewComment(commentParams)
	if err != nil {
		return nil, err // e.g., validation error
	}

	// 4. Persist the comment and event atomically.
	var newComment *domain.Comment
	if err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		createdComment, err := s.commentRepo.Create(txCtx, comment)
		if err != nil {
			return err
		}

		payload, err := marshalEventPayload(domain.NewCommentSnapshot(createdComment))
		if err != nil {
			return err
		}

		event := &domain.Event{
			TicketID: createdComment.TicketID,
			Type:     domain.EventCommentAdded,
			Payload:  payload,
			ActorID:  params.ActorID,
		}

		if _, err := s.eventRepo.Create(txCtx, event); err != nil {
			return err
		}

		newComment = createdComment
		return nil
	}); err != nil {
		return nil, err
	}

	// 5. Send email notification (asynchronously)
	// We notify the requester *unless* they are the one who made the comment.
	if ticket.RequesterID != params.ActorID {
		go s.notifier.Notify(context.Background(), ports.NotificationParams{
			RecipientUserID: ticket.RequesterID,
			Subject:         fmt.Sprintf("A new comment was added to your ticket: #%d", ticket.ID),
			Message:         fmt.Sprintf("A new comment has been added to your ticket '%s'.", ticket.Title),
			TicketID:        ticket.ID,
		})
	}

	return newComment, nil
}

// GetCommentsForTicket retrieves all comments for a specific ticket.
func (s *CommentService) GetCommentsForTicket(ctx context.Context, params ports.GetCommentsParams) ([]*domain.Comment, error) {
	// 1. Check permission to read comments.
	canRead, err := s.authzSvc.Can(ctx, params.ActorID, "comments:read")
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, apperrors.ErrForbidden
	}

	// 2. Check if the user can access the ticket to read its comments.
	canAccess, err := s.canUserAccessTicket(ctx, params.TicketID, params.ActorID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, apperrors.ErrForbidden
	}

	// 3. Retrieve the comments.
	return s.commentRepo.ListByTicketID(ctx, params.TicketID)
}
