package services

import (
	"context"
	"errors"
	"fmt" // Added for error wrapping

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AuthService implements authentication business logic
type AuthService struct {
	userRepo     ports.UserRepository
	authRepo     ports.AuthorizationRepository // <--- ADDED: Dependency for role assignment
	defaultOrgID uuid.UUID
}

var _ ports.AuthService = (*AuthService)(nil)

// NewAuthService creates a new authentication service
// We updated the constructor to require authRepo
func NewAuthService(
	userRepo ports.UserRepository,
	authRepo ports.AuthorizationRepository, // <--- ADDED: Inject dependency
	defaultOrgID uuid.UUID,
) ports.AuthService {
	return &AuthService{
		userRepo:     userRepo,
		authRepo:     authRepo, // <--- ADDED: Assign dependency
		defaultOrgID: defaultOrgID,
	}
}

// Register creates a new user account with validated credentials and assigns a default role
func (s *AuthService) Register(ctx context.Context, fullName, email, password, role string, orgID uuid.UUID) (*domain.User, error) {
	// 1. Validate registration parameters
	params := domain.UserRegistrationParams{
		FullName: fullName,
		Email:    email,
		Password: password,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	// 2. Check if user already exists
	_, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return nil, apperrors.ErrUserExists
	}
	if !errors.Is(err, apperrors.ErrUserNotFound) {
		return nil, err // An actual DB error occurred
	}

	// 3. Determine organization ID
	targetOrgID := orgID
	if targetOrgID == uuid.Nil {
		targetOrgID = s.defaultOrgID
	}

	// 4. Determine if this is the first user
	userCount, err := s.userRepo.CountUsers(ctx)
	if err != nil {
		return nil, err
	}

	// 5. Create user domain object
	user, err := domain.NewUser(params, targetOrgID)
	if err != nil {
		return nil, err
	}

	// 6. Persist the user
	createdUser, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// 7. Assign Role
	// First user is auto-promoted to admin.
	assignRole := "customer"
	if userCount == 0 {
		assignRole = "admin"
	} else if role != "" {
		assignRole = role
	}

	err = s.authRepo.AssignRole(ctx, createdUser.ID, assignRole)
	if err != nil {
		// Note: In a production environment, you should wrap steps 5 & 6 in a database transaction
		// or delete the user here if role assignment fails to prevent "ghost users".
		return nil, fmt.Errorf("user created but failed to assign role: %w", err)
	}

	return createdUser, nil
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
