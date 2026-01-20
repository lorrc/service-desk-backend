package http

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// UserInfoDTO represents a lightweight user reference in responses.
type UserInfoDTO struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
}

func toUserInfoDTO(user domain.UserInfo) UserInfoDTO {
	return UserInfoDTO{
		ID:       user.ID.String(),
		FullName: user.FullName,
		Email:    user.Email,
	}
}

func buildUserInfoDTOMap(
	ctx context.Context,
	userLookup ports.UserLookupService,
	orgID uuid.UUID,
	userIDs []uuid.UUID,
) (map[uuid.UUID]UserInfoDTO, error) {
	if userLookup == nil || len(userIDs) == 0 {
		return map[uuid.UUID]UserInfoDTO{}, nil
	}

	users, err := userLookup.GetUserInfo(ctx, orgID, userIDs)
	if err != nil {
		return nil, err
	}

	mapped := make(map[uuid.UUID]UserInfoDTO, len(users))
	for id, user := range users {
		mapped[id] = toUserInfoDTO(user)
	}

	return mapped, nil
}
