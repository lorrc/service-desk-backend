package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

type AuthService interface {
	Register(ctx context.Context, fullName, email, password string, orgID uuid.UUID) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, error)
}

// ... Define TicketService ...
