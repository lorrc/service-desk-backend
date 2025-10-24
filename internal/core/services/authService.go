package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type AuthService struct {
	userRepo ports.UserRepository
}

var _ ports.AuthService = (*AuthService)(nil)

func NewAuthService(userRepo ports.UserRepository) ports.AuthService {
	return &AuthService{userRepo: userRepo}
}

// Register now correctly matches the ports.AuthService interface signature.
func (s *AuthService) Register(ctx context.Context, fullName, email, password string, orgID uuid.UUID) (*domain.User, error) {
	_, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return nil, ports.ErrUserExists
	}
	if !errors.Is(err, ports.ErrUserNotFound) {
		return nil, err // An actual DB error occurred
	}

	hashedPassword, err := domain.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Use the hardcoded default organization ID from the migration script,
	// ignoring the orgID parameter for this phase of the project.
	defaultOrgID, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")

	user := &domain.User{
		Email:          email,
		FullName:       fullName,
		PasswordHash:   hashedPassword,
		OrganizationID: defaultOrgID,
	}

	return s.userRepo.Create(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ports.ErrUserNotFound) {
			return nil, ports.ErrInvalidCredentials
		}
		return nil, err
	}

	if !user.CheckPassword(password) {
		return nil, ports.ErrInvalidCredentials
	}

	return user, nil
}
