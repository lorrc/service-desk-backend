package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AuthService manages credentials and atomic account registration.
type AuthService struct {
	userRepo     ports.UserRepository
	authRepo     ports.AuthorizationRepository
	txManager    ports.TransactionManager
	defaultOrgID uuid.UUID
}

var _ ports.AuthService = (*AuthService)(nil)

// NewAuthService creates an authentication service.
func NewAuthService(userRepo ports.UserRepository, authRepo ports.AuthorizationRepository, txManager ports.TransactionManager, defaultOrgID uuid.UUID) ports.AuthService {
	return &AuthService{userRepo: userRepo, authRepo: authRepo, txManager: txManager, defaultOrgID: defaultOrgID}
}

// Register creates a user and their role in the same transaction.
// Public registration callers must supply the customer role.
func (s *AuthService) Register(ctx context.Context, fullName, email, password, role string, orgID uuid.UUID) (*domain.User, error) {
	if orgID == uuid.Nil {
		orgID = s.defaultOrgID
	}
	if role == "" {
		role = "customer"
	}
	user, err := domain.NewUser(domain.UserRegistrationParams{FullName: fullName, Email: email, Password: password}, orgID)
	if err != nil {
		return nil, err
	}

	var created *domain.User
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		var err error
		created, err = createAccount(txCtx, s.userRepo, s.authRepo, user, role)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func createAccount(ctx context.Context, users ports.UserRepository, roles ports.AuthorizationRepository, user *domain.User, role string) (*domain.User, error) {
	created, err := users.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	if err := roles.AssignRole(ctx, created.ID, role); err != nil {
		return nil, fmt.Errorf("assign user role: %w", err)
	}
	return created, nil
}

// BootstrapAdmin idempotently provisions the configured administrator.
// Locking the organization serializes bootstrap across application replicas.
func (s *AuthService) BootstrapAdmin(ctx context.Context, fullName, email, password string) error {
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.userRepo.LockOrganization(txCtx, s.defaultOrgID); err != nil {
			return err
		}
		user, err := s.userRepo.GetByEmail(txCtx, email)
		if errors.Is(err, apperrors.ErrUserNotFound) {
			user, err = domain.NewUser(domain.UserRegistrationParams{FullName: fullName, Email: email, Password: password}, s.defaultOrgID)
			if err != nil {
				return err
			}
			_, err = createAccount(txCtx, s.userRepo, s.authRepo, user, "admin")
			return err
		}
		if err != nil {
			return err
		}
		if user.OrganizationID != s.defaultOrgID {
			return apperrors.ErrForbidden
		}
		// Existing accounts keep their current password and active status.
		err = s.authRepo.AssignRole(txCtx, user.ID, "admin")
		if errors.Is(err, apperrors.ErrRoleAlreadyAssigned) {
			return nil
		}
		return err
	})
}

// Login authenticates an active user and records their last login.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, error) {
	if email == "" {
		return nil, apperrors.ErrEmailRequired
	}
	if password == "" {
		return nil, apperrors.ErrPasswordRequired
	}
	user, err := s.userRepo.GetByEmail(ctx, email)
	canonical := strings.ToLower(strings.TrimSpace(email))
	if errors.Is(err, apperrors.ErrUserNotFound) && canonical != email {
		user, err = s.userRepo.GetByEmail(ctx, canonical)
	}
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, err
	}
	if !user.CheckPassword(password) {
		return nil, apperrors.ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, apperrors.ErrUserInactive
	}
	now := time.Now().UTC()
	if err := s.userRepo.UpdateLastActive(ctx, user.ID, now); err != nil {
		return nil, err
	}
	user.LastActiveAt = &now
	return user, nil
}
