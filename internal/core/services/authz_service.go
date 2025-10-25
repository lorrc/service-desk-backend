package services

import (
	"context"

	"github.com/google/uuid"
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
	userPermissions, err := s.authRepo.GetUserPermissions(ctx, userID)
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
