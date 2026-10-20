package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// OnboardingService creates organizations and controls admission through invitations.
type OnboardingService struct {
	users         ports.UserRepository
	roles         ports.AuthorizationRepository
	permissions   ports.AuthorizationService
	organizations ports.OrganizationRepository
	invitations   ports.InvitationRepository
	transactions  ports.TransactionManager
}

// NewOnboardingService constructs transactional registration and invitation operations.
func NewOnboardingService(users ports.UserRepository, roles ports.AuthorizationRepository, permissions ports.AuthorizationService, organizations ports.OrganizationRepository, invitations ports.InvitationRepository, transactions ports.TransactionManager) ports.OnboardingService {
	return &OnboardingService{users: users, roles: roles, permissions: permissions, organizations: organizations, invitations: invitations, transactions: transactions}
}

// Register creates a founder administrator or admits an invited customer atomically.
func (s *OnboardingService) Register(ctx context.Context, params ports.OnboardingParams) (*domain.User, error) {
	name := strings.TrimSpace(params.OrganizationName)
	token := strings.TrimSpace(params.InvitationToken)
	validation := apperrors.NewValidationErrors()
	if (name == "") == (token == "") {
		validation.Add("organizationName", "Provide an organization name or an invitation, but not both")
		validation.Add("invitationToken", "Provide an organization name or an invitation, but not both")
	}
	if len(name) > 255 {
		validation.Add("organizationName", "Organization name must be 255 bytes or less")
	}
	email, err := canonicalEmail(params.Email)
	if err != nil {
		validation.Add("email", "Enter a valid email address")
	}
	if validation.HasErrors() {
		return nil, validation
	}
	user, err := domain.NewUser(domain.UserRegistrationParams{FullName: strings.TrimSpace(params.FullName), Email: email, Password: params.Password}, uuid.Nil)
	if err != nil {
		return nil, err
	}
	var created *domain.User
	err = s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
		role := "customer"
		organizationName := name
		if token != "" {
			hash, err := invitationHash(token)
			if err != nil {
				return err
			}
			invitation, err := s.invitations.Accept(txCtx, hash, email)
			if errors.Is(err, apperrors.ErrNotFound) {
				return invalidInvitation()
			}
			if err != nil {
				return err
			}
			user.OrganizationID = invitation.OrganizationID
			organizationName = invitation.OrganizationName
		} else {
			organization, err := s.organizations.Create(txCtx, name)
			if err != nil {
				return err
			}
			user.OrganizationID = organization.ID
			role = "admin"
		}
		exists, err := s.users.EmailExists(txCtx, email)
		if err != nil {
			return err
		}
		if exists {
			return apperrors.ErrUserExists
		}
		created, err = createAccount(txCtx, s.users, s.roles, user, role)
		if err != nil {
			return err
		}
		created.OrganizationName = organizationName
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// OrganizationName resolves the display name from persistent organization identity.
func (s *OnboardingService) OrganizationName(ctx context.Context, orgID uuid.UUID) (string, error) {
	organization, err := s.organizations.GetByID(ctx, orgID)
	if err != nil {
		return "", err
	}
	return organization.Name, nil
}

// PreviewInvitation returns only the recipient and organization for a usable token.
func (s *OnboardingService) PreviewInvitation(ctx context.Context, token string) (*domain.Invitation, error) {
	hash, err := invitationHash(strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	invitation, err := s.invitations.GetByToken(ctx, hash)
	if errors.Is(err, apperrors.ErrNotFound) {
		return nil, invalidInvitation()
	}
	return invitation, err
}

// CreateInvitation issues a seven-day, single-use customer invitation.
func (s *OnboardingService) CreateInvitation(ctx context.Context, actorID, orgID uuid.UUID, email string) (*domain.Invitation, string, error) {
	if err := s.requireAdmin(ctx, actorID, orgID); err != nil {
		return nil, "", err
	}
	email, err := canonicalEmail(email)
	if err != nil {
		return nil, "", err
	}
	exists, err := s.users.EmailExists(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if exists {
		return nil, "", apperrors.ErrUserExists
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	hash := sha256.Sum256(secret)
	now := time.Now().UTC()
	invitation, err := s.invitations.Create(ctx, &domain.Invitation{
		OrganizationID: orgID, Email: email, CreatedBy: actorID,
		CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour),
	}, hash[:])
	if err != nil {
		return nil, "", err
	}
	return invitation, token, nil
}

// ListInvitations returns a bounded page plus one lookahead row within the actor's organization.
func (s *OnboardingService) ListInvitations(ctx context.Context, actorID, orgID uuid.UUID, limit, offset int) ([]*domain.Invitation, error) {
	if err := s.requireAdmin(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 2147483647 {
		validation := apperrors.NewValidationErrors()
		validation.Add("pagination", "Limit must be between 1 and 100 and offset must fit a nonnegative 32-bit integer")
		return nil, validation
	}
	return s.invitations.List(ctx, orgID, limit+1, offset)
}

// RevokeInvitation prevents future use without changing a completed acceptance.
func (s *OnboardingService) RevokeInvitation(ctx context.Context, actorID, orgID, invitationID uuid.UUID) error {
	if err := s.requireAdmin(ctx, actorID, orgID); err != nil {
		return err
	}
	if err := s.invitations.Revoke(ctx, orgID, invitationID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return apperrors.NewNotFoundError(err, "Invitation not found")
		}
		return err
	}
	return nil
}

func (s *OnboardingService) requireAdmin(ctx context.Context, actorID, orgID uuid.UUID) error {
	user, err := s.users.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if !user.IsActive || user.OrganizationID != orgID {
		return apperrors.ErrForbidden
	}
	allowed, err := s.permissions.Can(ctx, actorID, "admin:access")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.ErrForbidden
	}
	return nil
}

func canonicalEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > domain.MaxEmailLength {
		validation := apperrors.NewValidationErrors()
		validation.Add("email", "Enter a valid email address")
		return "", validation
	}
	return email, nil
}

func invitationHash(token string) ([]byte, error) {
	if len(token) != base64.RawURLEncoding.EncodedLen(32) {
		return nil, invalidInvitation()
	}
	secret, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(secret) != 32 {
		return nil, invalidInvitation()
	}
	hash := sha256.Sum256(secret)
	return hash[:], nil
}

func invalidInvitation() error {
	validation := apperrors.NewValidationErrors()
	validation.Add("invitationToken", "This invitation is invalid or no longer available")
	return validation
}
