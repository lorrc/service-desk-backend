package ports

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrTicketNotFound = errors.New("ticket not found")
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

// TicketRepository defines the operations required for persisting and retrieving Tickets.
// This is a Secondary Port (Driven Port).
type TicketRepository interface {
	// Create persists a new ticket entity. It returns the persisted entity (including the generated ID).
	Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error)

	// GetByID retrieves a single ticket by its ID.
	GetByID(ctx context.Context, id int64) (*domain.Ticket, error)

	// Update persists changes to an existing ticket entity.
	Update(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error)

	// List retrieves a collection of tickets.
	// Note: In Phase 1, we implement basic listing. Filtering and pagination will be added in Phase 2.
	List(ctx context.Context) ([]*domain.Ticket, error)

	// ListByRequester retrieves tickets created by a specific user.
	ListByRequester(ctx context.Context, requesterID uuid.UUID) ([]*domain.Ticket, error)
}
