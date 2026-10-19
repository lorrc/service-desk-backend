package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// UserLookupService provides lightweight user details for display.
type UserLookupService struct {
	userRepo ports.UserRepository
}

var _ ports.UserLookupService = (*UserLookupService)(nil)

// NewUserLookupService creates a new UserLookupService.
func NewUserLookupService(userRepo ports.UserRepository) ports.UserLookupService {
	return &UserLookupService{
		userRepo: userRepo,
	}
}

// GetUserInfo returns user details for the provided IDs within an organization.
func (s *UserLookupService) GetUserInfo(
	ctx context.Context,
	orgID uuid.UUID,
	userIDs []uuid.UUID,
) (map[uuid.UUID]domain.UserInfo, error) {
	return s.userRepo.GetUserInfo(ctx, orgID, userIDs)
}
