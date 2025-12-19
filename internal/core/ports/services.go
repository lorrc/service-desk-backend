package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

// AuthService defines the port for authentication business logic.
type AuthService interface {
	Register(ctx context.Context, fullName, email, password string, orgID uuid.UUID) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, error)
}

// AuthorizationService defines the port for checking user permissions.
type AuthorizationService interface {
	Can(ctx context.Context, userID uuid.UUID, permission string) (bool, error)
}

// CreateTicketParams defines the required input for creating a new ticket.
type CreateTicketParams struct {
	Title       string
	Description string
	Priority    domain.TicketPriority
	RequesterID uuid.UUID
}

// UpdateStatusParams defines the input for changing a ticket's status.
type UpdateStatusParams struct {
	TicketID int64
	Status   domain.TicketStatus
	ActorID  uuid.UUID
}

// AssignTicketParams defines the input for assigning a ticket.
type AssignTicketParams struct {
	TicketID   int64
	AssigneeID uuid.UUID
	ActorID    uuid.UUID
}

// CreateCommentParams defines the input for creating a comment.
type CreateCommentParams struct {
	TicketID int64
	ActorID  uuid.UUID
	Body     string
}

// GetCommentsParams defines the input for retrieving comments.
type GetCommentsParams struct {
	TicketID int64
	ActorID  uuid.UUID
}

// ListTicketsParams defines the input for listing tickets.
type ListTicketsParams struct {
	ViewerID uuid.UUID
	Limit    int
	Offset   int
	Status   *string
	Priority *string
}

// NotificationParams defines the input for sending a notification.
type NotificationParams struct {
	RecipientUserID uuid.UUID
	Subject         string
	Message         string
	TicketID        int64
}

// TicketService defines the core business operations for managing tickets.
type TicketService interface {
	CreateTicket(ctx context.Context, params CreateTicketParams) (*domain.Ticket, error)
	GetTicket(ctx context.Context, ticketID int64, viewerID uuid.UUID) (*domain.Ticket, error)
	UpdateStatus(ctx context.Context, params UpdateStatusParams) (*domain.Ticket, error)
	AssignTicket(ctx context.Context, params AssignTicketParams) (*domain.Ticket, error)
	ListTickets(ctx context.Context, params ListTicketsParams) ([]*domain.Ticket, error)
	Shutdown()
}

// CommentService defines the port for comment-related business logic.
type CommentService interface {
	CreateComment(ctx context.Context, params CreateCommentParams) (*domain.Comment, error)
	GetCommentsForTicket(ctx context.Context, params GetCommentsParams) ([]*domain.Comment, error)
}

// Notifier defines the port for sending asynchronous notifications.
type Notifier interface {
	Notify(ctx context.Context, params NotificationParams)
}

// EventBroadcaster defines the port for broadcasting real-time events.
type EventBroadcaster interface {
	Broadcast(event domain.Event) error
}
