package services

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AccountService manages self-service account access and session revocation.
type AccountService struct {
	users         ports.UserRepository
	permissions   ports.AuthorizationService
	organizations ports.OrganizationRepository
}

// NewAccountService constructs account operations from persistence and authorization ports.
func NewAccountService(users ports.UserRepository, permissions ports.AuthorizationService, organizations ports.OrganizationRepository) ports.AccountService {
	return &AccountService{users: users, permissions: permissions, organizations: organizations}
}

func (s *AccountService) currentUser(ctx context.Context, userID, orgID uuid.UUID, version int64) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive || user.OrganizationID != orgID || user.SessionVersion != version {
		return nil, apperrors.ErrUnauthorized
	}
	return user, nil
}

// Profile returns the current identity and permission set within the authenticated organization.
func (s *AccountService) Profile(ctx context.Context, userID, orgID uuid.UUID, version int64) (*domain.Profile, error) {
	user, err := s.currentUser(ctx, userID, orgID, version)
	if err != nil {
		return nil, err
	}
	organization, err := s.organizations.GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	user.OrganizationName = organization.Name
	permissions, err := s.permissions.GetPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, err := s.users.GetRoles(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}
	sort.Strings(permissions)
	return &domain.Profile{User: user, Roles: roles, Permissions: permissions}, nil
}

// ChangePassword verifies the old password and atomically revokes existing sessions.
func (s *AccountService) ChangePassword(ctx context.Context, userID, orgID uuid.UUID, version int64, currentPassword, newPassword string) error {
	user, err := s.currentUser(ctx, userID, orgID, version)
	if err != nil {
		return err
	}
	errors := apperrors.NewValidationErrors()
	if !user.CheckPassword(currentPassword) {
		errors.Add("currentPassword", "Current password is incorrect")
	}
	for _, message := range domain.ValidatePassword(newPassword) {
		errors.Add("newPassword", message)
	}
	if currentPassword == newPassword {
		errors.Add("newPassword", "Choose a different password")
	}
	if errors.HasErrors() {
		return errors
	}
	hash, err := domain.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.users.ChangeOwnPassword(ctx, userID, orgID, version, user.HashedPassword, hash)
}

// SignOutEverywhere revokes every token issued for the current session version.
func (s *AccountService) SignOutEverywhere(ctx context.Context, userID, orgID uuid.UUID, version int64) error {
	return s.users.RevokeSessions(ctx, userID, orgID, version)
}
