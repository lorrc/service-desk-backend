package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

// AuthService defines the port for authentication business logic.
type AuthService interface {
	Register(ctx context.Context, fullName, email, password, role string, orgID uuid.UUID) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, error)
}

// AuthorizationService defines the port for checking user permissions.
type AuthorizationService interface {
	Can(ctx context.Context, userID uuid.UUID, permission string) (bool, error)
	GetPermissions(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// AssigneeService defines the port for listing assignable users.
type AssigneeService interface {
	ListAssignableUsers(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) ([]*domain.User, error)
}

// AdminService defines the port for admin-only operations.
type AdminService interface {
	ListUsers(ctx context.Context, actorID, orgID uuid.UUID) ([]*domain.UserSummary, error)
	UpdateUserRole(ctx context.Context, actorID, orgID, userID uuid.UUID, role string) error
	UpdateUserStatus(ctx context.Context, actorID, orgID, userID uuid.UUID, isActive bool) error
	ResetUserPassword(ctx context.Context, actorID, orgID, userID uuid.UUID) (string, error)
	GetAnalyticsOverview(ctx context.Context, actorID, orgID uuid.UUID, days int) (*domain.AnalyticsOverview, error)
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
	ViewerID    uuid.UUID
	Limit       int
	Offset      int
	Status      *string
	Priority    *string
	AssigneeID  *uuid.UUID
	Unassigned  bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// ListTicketEventsParams defines the input for listing ticket events.
type ListTicketEventsParams struct {
	TicketID int64
	ViewerID uuid.UUID
	AfterID  int64
	Limit    int
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

// EventService defines the port for ticket event queries.
type EventService interface {
	ListTicketEvents(ctx context.Context, params ListTicketEventsParams) ([]*domain.Event, error)
}

// Notifier defines the port for sending asynchronous notifications.
type Notifier interface {
	Notify(ctx context.Context, params NotificationParams)
}

// TransactionManager defines the port for running atomic operations.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
