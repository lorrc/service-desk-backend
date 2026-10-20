package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// InvitationRepository persists invitation metadata and hashed bearer secrets.
type InvitationRepository struct {
	pool *pgxpool.Pool
}

// NewInvitationRepository constructs the PostgreSQL invitation adapter.
func NewInvitationRepository(pool *pgxpool.Pool) ports.InvitationRepository {
	return &InvitationRepository{pool: pool}
}

const invitationColumns = `id, organization_id, email, created_by, created_at, expires_at, accepted_at, revoked_at`

func scanInvitation(row pgx.Row) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := row.Scan(&invitation.ID, &invitation.OrganizationID, &invitation.Email, &invitation.CreatedBy, &invitation.CreatedAt, &invitation.ExpiresAt, &invitation.AcceptedAt, &invitation.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return &invitation, err
}

// Create stores a new invitation and never persists its raw token.
func (r *InvitationRepository) Create(ctx context.Context, invitation *domain.Invitation, tokenHash []byte) (*domain.Invitation, error) {
	return scanInvitation(GetDBTX(ctx, r.pool).QueryRow(ctx, `INSERT INTO organization_invitations
		(organization_id, email, created_by, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, now() + $5::bigint * interval '1 second') RETURNING `+invitationColumns,
		invitation.OrganizationID, invitation.Email, invitation.CreatedBy, tokenHash, int64(invitation.ExpiresAt.Sub(invitation.CreatedAt).Seconds())))
}

// GetByToken reveals metadata only while the invitation remains usable.
func (r *InvitationRepository) GetByToken(ctx context.Context, tokenHash []byte) (*domain.Invitation, error) {
	invitation, err := scanInvitation(GetDBTX(ctx, r.pool).QueryRow(ctx, `SELECT `+invitationColumns+`
		FROM organization_invitations WHERE token_hash = $1 AND accepted_at IS NULL
		AND revoked_at IS NULL AND expires_at > clock_timestamp()`, tokenHash))
	if err != nil {
		return nil, err
	}
	err = GetDBTX(ctx, r.pool).QueryRow(ctx, "SELECT name FROM organizations WHERE id = $1", invitation.OrganizationID).Scan(&invitation.OrganizationName)
	return invitation, err
}

// Accept locks and consumes an invitation in the transaction that creates its account.
func (r *InvitationRepository) Accept(ctx context.Context, tokenHash []byte, email string) (*domain.Invitation, error) {
	if _, ok := TxFromContext(ctx); !ok {
		return nil, fmt.Errorf("invitation acceptance requires a transaction")
	}
	conn := GetDBTX(ctx, r.pool)
	invitation, err := scanInvitation(conn.QueryRow(ctx, `SELECT `+invitationColumns+`
		FROM organization_invitations WHERE token_hash = $1 FOR UPDATE`, tokenHash))
	if err != nil {
		return nil, err
	}
	// Check expiry after acquiring the lock so a waiter cannot redeem an expired invitation.
	result, err := conn.Exec(ctx, `UPDATE organization_invitations SET accepted_at = clock_timestamp()
		WHERE id = $1 AND email = $2 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > clock_timestamp()`,
		invitation.ID, email)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, apperrors.ErrNotFound
	}
	err = conn.QueryRow(ctx, "SELECT name FROM organizations WHERE id = $1", invitation.OrganizationID).Scan(&invitation.OrganizationName)
	return invitation, err
}

// List returns invitation history in a stable organization-scoped order.
func (r *InvitationRepository) List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Invitation, error) {
	rows, err := GetDBTX(ctx, r.pool).Query(ctx, `SELECT `+invitationColumns+`
		FROM organization_invitations WHERE organization_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invitations := make([]*domain.Invitation, 0)
	for rows.Next() {
		invitation, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

// Revoke is idempotent for known invitations and leaves terminal states unchanged.
func (r *InvitationRepository) Revoke(ctx context.Context, orgID, invitationID uuid.UUID) error {
	tag, err := GetDBTX(ctx, r.pool).Exec(ctx, `UPDATE organization_invitations
		SET revoked_at = CASE WHEN accepted_at IS NULL AND revoked_at IS NULL AND expires_at > clock_timestamp()
			THEN clock_timestamp() ELSE revoked_at END
		WHERE id = $1 AND organization_id = $2`, invitationID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
