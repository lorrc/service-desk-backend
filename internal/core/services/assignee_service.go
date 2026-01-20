package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AssigneeService implements business logic for listing assignable users.
type AssigneeService struct {
	userRepo ports.UserRepository
	authzSvc ports.AuthorizationService
}

var _ ports.AssigneeService = (*AssigneeService)(nil)

// NewAssigneeService creates a new assignee service.
func NewAssigneeService(userRepo ports.UserRepository, authzSvc ports.AuthorizationService) ports.AssigneeService {
	return &AssigneeService{
		userRepo: userRepo,
		authzSvc: authzSvc,
	}
}

// ListAssignableUsers returns users eligible for assignment within the org.
func (s *AssigneeService) ListAssignableUsers(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) ([]*domain.User, error) {
	canAssign, err := s.authzSvc.Can(ctx, actorID, "tickets:assign")
	if err != nil {
		return nil, err
	}
	if !canAssign {
		return nil, apperrors.ErrForbidden
	}

	return s.userRepo.ListAssignableUsers(ctx, orgID)
}
