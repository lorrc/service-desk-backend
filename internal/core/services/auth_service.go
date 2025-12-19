package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AuthService implements authentication business logic
type AuthService struct {
	userRepo     ports.UserRepository
	defaultOrgID uuid.UUID
}

var _ ports.AuthService = (*AuthService)(nil)

// NewAuthService creates a new authentication service
func NewAuthService(userRepo ports.UserRepository, defaultOrgID uuid.UUID) ports.AuthService {
	return &AuthService{
		userRepo:     userRepo,
		defaultOrgID: defaultOrgID,
	}
}

// Register creates a new user account with validated credentials
func (s *AuthService) Register(ctx context.Context, fullName, email, password string, orgID uuid.UUID) (*domain.User, error) {
	// Validate registration parameters
	params := domain.UserRegistrationParams{
		FullName: fullName,
		Email:    email,
		Password: password,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	// Check if user already exists
	_, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return nil, apperrors.ErrUserExists
	}
	if !errors.Is(err, apperrors.ErrUserNotFound) {
		return nil, err // An actual DB error occurred
	}

	// Determine organization ID
	targetOrgID := orgID
	if targetOrgID == uuid.Nil {
		targetOrgID = s.defaultOrgID
	}

	// Create user with validated params
	user, err := domain.NewUser(params, targetOrgID)
	if err != nil {
		return nil, err
	}

	// Persist the user
	return s.userRepo.Create(ctx, user)
}

// Login authenticates a user with email and password
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, error) {
	// Basic validation
	if email == "" {
		return nil, apperrors.ErrEmailRequired
	}
	if password == "" {
		return nil, apperrors.ErrPasswordRequired
	}

	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			// Don't reveal whether email exists
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, err
	}

	// Verify password
	if !user.CheckPassword(password) {
		return nil, apperrors.ErrInvalidCredentials
	}

	return user, nil
}
