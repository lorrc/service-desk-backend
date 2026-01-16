package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AuthorizationRepository handles database operations for RBAC.
type AuthorizationRepository struct {
	q db.Querier
}

// Ensure implementation matches the interface.
var _ ports.AuthorizationRepository = (*AuthorizationRepository)(nil)

// NewAuthorizationRepository creates a new repository for authorization queries.
func NewAuthorizationRepository(pool *pgxpool.Pool) ports.AuthorizationRepository {
	return &AuthorizationRepository{
		q: db.New(pool),
	}
}

// GetUserPermissions fetches all distinct permissions for a given user ID.
func (r *AuthorizationRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	pgUUID := pgtype.UUID{Bytes: userID, Valid: true}
	permissions, err := r.q.GetUserPermissions(ctx, pgUUID)
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

// AssignRole assigns a role to a user by role name.
func (r *AuthorizationRepository) AssignRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	params := db.AssignRoleParams{
		UserID:   pgtype.UUID{Bytes: userID, Valid: true},
		RoleName: roleName,
	}

	status, err := r.q.AssignRole(ctx, params)
	if err != nil {
		return err
	}
	switch status {
	case "assigned":
		return nil
	case "already_assigned":
		return apperrors.ErrRoleAlreadyAssigned
	case "role_not_found":
		return apperrors.ErrRoleNotFound
	default:
		return fmt.Errorf("unexpected role assignment status: %s", status)
	}
}
