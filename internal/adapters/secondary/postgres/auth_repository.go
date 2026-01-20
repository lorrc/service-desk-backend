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
	q    db.Querier
	dbtx db.DBTX
}

// Ensure implementation matches the interface.
var _ ports.AuthorizationRepository = (*AuthorizationRepository)(nil)

// NewAuthorizationRepository creates a new repository for authorization queries.
func NewAuthorizationRepository(pool *pgxpool.Pool) ports.AuthorizationRepository {
	return &AuthorizationRepository{
		q:    db.New(pool),
		dbtx: pool,
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

	for attempt := 0; attempt < 2; attempt++ {
		status, err := r.q.AssignRole(ctx, params)
		if err != nil {
			return err
		}
		switch status {
		case "assigned":
			return nil
		case "already_assigned":
			if attempt == 0 {
				if err := r.ensureRBACDefaults(ctx); err != nil {
					return err
				}
			}
			return apperrors.ErrRoleAlreadyAssigned
		case "role_not_found":
			if attempt == 0 {
				if err := r.ensureRBACDefaults(ctx); err != nil {
					return err
				}
				continue
			}
			return apperrors.ErrRoleNotFound
		default:
			return fmt.Errorf("unexpected role assignment status: %s", status)
		}
	}

	return apperrors.ErrRoleNotFound
}

// SetUserRole replaces any existing roles for a user with the provided role.
func (r *AuthorizationRepository) SetUserRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	params := db.SetUserRoleParams{
		UserID:   pgtype.UUID{Bytes: userID, Valid: true},
		RoleName: roleName,
	}

	for attempt := 0; attempt < 2; attempt++ {
		status, err := r.q.SetUserRole(ctx, params)
		if err != nil {
			return err
		}
		switch status {
		case "assigned":
			return nil
		case "role_not_found":
			if attempt == 0 {
				if err := r.ensureRBACDefaults(ctx); err != nil {
					return err
				}
				continue
			}
			return apperrors.ErrRoleNotFound
		default:
			return fmt.Errorf("unexpected role update status: %s", status)
		}
	}

	return apperrors.ErrRoleNotFound
}

func (r *AuthorizationRepository) EnsureRBACDefaults(ctx context.Context) error {
	return r.ensureRBACDefaults(ctx)
}

func (r *AuthorizationRepository) ensureRBACDefaults(ctx context.Context) error {
	statements := []string{
		`INSERT INTO permissions (code) VALUES
			('tickets:create'),
			('tickets:read'),
			('tickets:read:all'),
			('tickets:update:status'),
			('tickets:assign'),
			('tickets:list:all'),
			('comments:create'),
			('comments:read'),
			('admin:access')
		ON CONFLICT DO NOTHING;`,
		`INSERT INTO roles (name) VALUES ('admin'), ('agent'), ('customer')
		ON CONFLICT DO NOTHING;`,
		`INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'admin'
		ON CONFLICT DO NOTHING;`,
		`INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.name = 'agent' AND p.code IN (
			'tickets:create', 'tickets:read', 'tickets:read:all',
			'tickets:update:status', 'tickets:assign', 'tickets:list:all',
			'comments:create', 'comments:read'
		)
		ON CONFLICT DO NOTHING;`,
		`INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.name = 'customer' AND p.code IN (
			'tickets:create', 'tickets:read', 'comments:create', 'comments:read'
		)
		ON CONFLICT DO NOTHING;`,
	}

	for _, stmt := range statements {
		if _, err := r.dbtx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("seed rbac defaults: %w", err)
		}
	}

	return nil
}
