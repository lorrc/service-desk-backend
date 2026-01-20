package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AuthorizationService implements the business logic for RBAC.
type AuthorizationService struct {
	authRepo ports.AuthorizationRepository
}

// Ensure implementation matches the interface.
var _ ports.AuthorizationService = (*AuthorizationService)(nil)

// NewAuthorizationService creates a new service for authorization logic.
func NewAuthorizationService(authRepo ports.AuthorizationRepository) ports.AuthorizationService {
	return &AuthorizationService{
		authRepo: authRepo,
	}
}

// Can checks if a user has a specific permission.
func (s *AuthorizationService) Can(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	userPermissions, err := s.ensurePermissions(ctx, userID)
	if err != nil {
		// If there's an error fetching permissions (e.g., db down), deny access.
		return false, err
	}

	// Check if the required permission is in the user's list of permissions.
	for _, p := range userPermissions {
		if p == permission {
			return true, nil
		}
	}

	// If the loop finishes, the user does not have the permission.
	return false, nil
}

// GetPermissions returns all permissions for a user.
func (s *AuthorizationService) GetPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return s.ensurePermissions(ctx, userID)
}

func (s *AuthorizationService) ensurePermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	permissions, err := s.authRepo.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(permissions) == 0 {
		if err := s.authRepo.AssignRole(ctx, userID, "customer"); err != nil && !errors.Is(err, apperrors.ErrRoleAlreadyAssigned) {
			return nil, err
		}

		permissions, err = s.authRepo.GetUserPermissions(ctx, userID)
		if err != nil {
			return nil, err
		}
	}

	if permissions == nil {
		return []string{}, nil
	}

	return permissions, nil
}
