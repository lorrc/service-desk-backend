package services

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type AdminService struct {
	userRepo      ports.UserRepository
	authRepo      ports.AuthorizationRepository
	authzSvc      ports.AuthorizationService
	analyticsRepo ports.AnalyticsRepository
}

var _ ports.AdminService = (*AdminService)(nil)

func NewAdminService(
	userRepo ports.UserRepository,
	authRepo ports.AuthorizationRepository,
	authzSvc ports.AuthorizationService,
	analyticsRepo ports.AnalyticsRepository,
) ports.AdminService {
	return &AdminService{
		userRepo:      userRepo,
		authRepo:      authRepo,
		authzSvc:      authzSvc,
		analyticsRepo: analyticsRepo,
	}
}

func (s *AdminService) ListUsers(ctx context.Context, actorID, orgID uuid.UUID) ([]*domain.UserSummary, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}

	return s.userRepo.ListByOrganization(ctx, orgID)
}

func (s *AdminService) UpdateUserRole(ctx context.Context, actorID, orgID, userID uuid.UUID, role string) error {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OrganizationID != orgID {
		return apperrors.ErrForbidden
	}

	return s.authRepo.SetUserRole(ctx, userID, role)
}

func (s *AdminService) UpdateUserStatus(ctx context.Context, actorID, orgID, userID uuid.UUID, isActive bool) error {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return err
	}
	if userID == actorID && !isActive {
		return apperrors.ErrForbidden
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OrganizationID != orgID {
		return apperrors.ErrForbidden
	}

	return s.userRepo.SetActive(ctx, userID, isActive)
}

func (s *AdminService) ResetUserPassword(ctx context.Context, actorID, orgID, userID uuid.UUID) (string, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return "", err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user.OrganizationID != orgID {
		return "", apperrors.ErrForbidden
	}

	temporaryPassword, err := generateTemporaryPassword(12)
	if err != nil {
		return "", err
	}

	hashedPassword, err := domain.HashPassword(temporaryPassword)
	if err != nil {
		return "", err
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		return "", err
	}

	return temporaryPassword, nil
}

func (s *AdminService) GetAnalyticsOverview(ctx context.Context, actorID, orgID uuid.UUID, days int) (*domain.AnalyticsOverview, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}

	return s.analyticsRepo.GetOverview(ctx, orgID, days)
}

func (s *AdminService) requireAdmin(ctx context.Context, actorID uuid.UUID) error {
	allowed, err := s.authzSvc.Can(ctx, actorID, "admin:access")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.ErrForbidden
	}
	return nil
}

func generateTemporaryPassword(length int) (string, error) {
	const upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lower = "abcdefghijklmnopqrstuvwxyz"
	const digits = "0123456789"
	const all = upper + lower + digits

	if length < 8 {
		length = 8
	}

	password := make([]byte, length)

	sets := []string{upper, lower, digits}
	for i := 0; i < len(sets); i++ {
		char, err := randomChar(sets[i])
		if err != nil {
			return "", err
		}
		password[i] = char
	}

	for i := len(sets); i < length; i++ {
		char, err := randomChar(all)
		if err != nil {
			return "", err
		}
		password[i] = char
	}

	for i := len(password) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())
		password[i], password[j] = password[j], password[i]
	}

	return string(password), nil
}

func randomChar(source string) (byte, error) {
	max := big.NewInt(int64(len(source)))
	index, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}
	return source[index.Int64()], nil
}
