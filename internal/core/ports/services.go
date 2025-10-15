package ports

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

// Custom errors for services and repositories
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrForbidden          = errors.New("action forbidden")
)

type AuthService interface {
	Register(ctx context.Context, fullName, email, password string, orgID uuid.UUID) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, error)
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
	// ActorID is the user performing the action (crucial for authorization and auditing).
	ActorID uuid.UUID
}

// AssignTicketParams defines the input for assigning a ticket.
type AssignTicketParams struct {
	TicketID int64
	// AssigneeID is the user the ticket is being assigned to.
	AssigneeID uuid.UUID
	// ActorID is the user performing the action.
	ActorID uuid.UUID
}

// TicketService defines the core business operations for managing the ticket lifecycle.
// This is a Primary Port (Driver Port).
type TicketService interface {
	// CreateTicket handles the use case for submitting a new ticket.
	CreateTicket(ctx context.Context, params CreateTicketParams) (*domain.Ticket, error)

	// GetTicket handles retrieving a specific ticket.
	// The viewerID is included so the service can enforce authorization rules
	// (e.g., ensuring the viewer is the requester, assignee, or an admin).
	GetTicket(ctx context.Context, ticketID int64, viewerID uuid.UUID) (*domain.Ticket, error)

	// UpdateStatus handles the use case for changing a ticket's state.
	UpdateStatus(ctx context.Context, params UpdateStatusParams) (*domain.Ticket, error)

	// AssignTicket handles the use case for assigning a ticket to an agent.
	AssignTicket(ctx context.Context, params AssignTicketParams) (*domain.Ticket, error)

	// ListTickets handles retrieving a list of tickets.
	// The viewerID is included so the service can scope the results based on the user's role
	// (e.g., customers see only their own tickets, agents see their assigned tickets).
	ListTickets(ctx context.Context, viewerID uuid.UUID) ([]*domain.Ticket, error)
}
