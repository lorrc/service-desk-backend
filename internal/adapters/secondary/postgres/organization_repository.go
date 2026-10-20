package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// OrganizationRepository persists workspace identities.
type OrganizationRepository struct {
	pool *pgxpool.Pool
}

// NewOrganizationRepository constructs the PostgreSQL organization adapter.
func NewOrganizationRepository(pool *pgxpool.Pool) ports.OrganizationRepository {
	return &OrganizationRepository{pool: pool}
}

// Create inserts a fresh organization without resolving or claiming an existing name.
func (r *OrganizationRepository) Create(ctx context.Context, name string) (*domain.Organization, error) {
	var organization domain.Organization
	err := GetDBTX(ctx, r.pool).QueryRow(ctx, "INSERT INTO organizations (name) VALUES ($1) RETURNING id, name", name).Scan(&organization.ID, &organization.Name)
	return &organization, err
}

// GetByID returns an organization's display identity.
func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	var organization domain.Organization
	err := GetDBTX(ctx, r.pool).QueryRow(ctx, "SELECT id, name FROM organizations WHERE id = $1", id).Scan(&organization.ID, &organization.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return &organization, err
}
