package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AuthorizationRepository handles database operations for RBAC.
type AuthorizationRepository struct {
	pool *pgxpool.Pool
}

// Ensure implementation matches the interface.
var _ ports.AuthorizationRepository = (*AuthorizationRepository)(nil)

// NewAuthorizationRepository creates a new repository for authorization queries.
func NewAuthorizationRepository(pool *pgxpool.Pool) ports.AuthorizationRepository {
	return &AuthorizationRepository{pool: pool}
}

// GetUserPermissions fetches all distinct permissions for a given user ID.
func (r *AuthorizationRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	query := `
		SELECT DISTINCT p.code
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		INNER JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1
	`

	rows, err := r.pool.Query(ctx, query, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		permissions = append(permissions, code)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}
