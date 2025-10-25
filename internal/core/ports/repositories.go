package ports

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrTicketNotFound = errors.New("ticket not found")
	// No need for ErrForbidden here, it's defined in the services port
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	// GetByID Add GetByID if it's missing from your file
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type TicketRepository interface {
	Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error)
	GetByID(ctx context.Context, id int64) (*domain.Ticket, error)
	Update(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error)
	ListPaginated(ctx context.Context, params ListTicketsRepoParams) ([]*domain.Ticket, error)
	ListByRequesterPaginated(ctx context.Context, params ListTicketsRepoParams) ([]*domain.Ticket, error)
}

// AuthorizationRepository defines the port for RBAC data access.
type AuthorizationRepository interface {
	GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// CommentRepository defines the port for comment persistence.
type CommentRepository interface {
	Create(ctx context.Context, comment *domain.Comment) (*domain.Comment, error)
	ListByTicketID(ctx context.Context, ticketID int64) ([]*domain.Comment, error)
}

type ListTicketsRepoParams struct {
	Limit       int32
	Offset      int32
	Status      pgtype.Text // <-- Corrected from *string
	Priority    pgtype.Text // <-- Corrected from *string
	RequesterID pgtype.UUID
}
