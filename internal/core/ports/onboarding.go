package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

// OnboardingParams chooses exactly one organization creation or invitation path.
type OnboardingParams struct {
	FullName         string
	Email            string
	Password         string
	OrganizationName string
	InvitationToken  string
}

// OnboardingService controls organization creation and customer invitations.
type OnboardingService interface {
	Register(ctx context.Context, params OnboardingParams) (*domain.User, error)
	OrganizationName(ctx context.Context, orgID uuid.UUID) (string, error)
	PreviewInvitation(ctx context.Context, token string) (*domain.Invitation, error)
	CreateInvitation(ctx context.Context, actorID, orgID uuid.UUID, email string) (*domain.Invitation, string, error)
	ListInvitations(ctx context.Context, actorID, orgID uuid.UUID, limit, offset int) ([]*domain.Invitation, error)
	RevokeInvitation(ctx context.Context, actorID, orgID, invitationID uuid.UUID) error
}

// OrganizationRepository persists and reads organization identities.
type OrganizationRepository interface {
	Create(ctx context.Context, name string) (*domain.Organization, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// InvitationRepository stores token hashes and atomically consumes invitations.
type InvitationRepository interface {
	Create(ctx context.Context, invitation *domain.Invitation, tokenHash []byte) (*domain.Invitation, error)
	GetByToken(ctx context.Context, tokenHash []byte) (*domain.Invitation, error)
	Accept(ctx context.Context, tokenHash []byte, email string) (*domain.Invitation, error)
	List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Invitation, error)
	Revoke(ctx context.Context, orgID, invitationID uuid.UUID) error
}
