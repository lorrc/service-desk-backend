package ports

import (
	"context"

	"github.com/your-org/service-desk-backend/internal/core/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

// ... Define TicketRepository ...
