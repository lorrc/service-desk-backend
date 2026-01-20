package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
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
	if len(userIDs) == 0 {
		return map[uuid.UUID]domain.UserInfo{}, nil
	}

	uniqueIDs := make(map[uuid.UUID]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id == uuid.Nil {
			continue
		}
		uniqueIDs[id] = struct{}{}
	}

	results := make(map[uuid.UUID]domain.UserInfo, len(uniqueIDs))
	for id := range uniqueIDs {
		user, err := s.userRepo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, apperrors.ErrUserNotFound) {
				continue
			}
			return nil, err
		}
		if user.OrganizationID != orgID {
			continue
		}

		results[id] = domain.UserInfo{
			ID:       user.ID,
			FullName: user.FullName,
			Email:    user.Email,
		}
	}

	return results, nil
}
