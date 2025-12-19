package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// UserRepository is the postgres adapter for user persistence.
type UserRepository struct {
	pool *pgxpool.Pool
}

var _ ports.UserRepository = (*UserRepository)(nil)

// NewUserRepository creates a new user repository.
func NewUserRepository(pool *pgxpool.Pool) ports.UserRepository {
	return &UserRepository{pool: pool}
}

// Create persists a new user to the database.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	query := `
		INSERT INTO users (id, organization_id, full_name, email, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, organization_id, full_name, email, password_hash, created_at
	`

	// Generate a new UUID if not provided
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	var dbUser domain.User
	var orgID, userID pgtype.UUID

	err := r.pool.QueryRow(ctx, query,
		pgtype.UUID{Bytes: user.ID, Valid: true},
		pgtype.UUID{Bytes: user.OrganizationID, Valid: true},
		user.FullName,
		user.Email,
		user.PasswordHash,
		user.CreatedAt,
	).Scan(&userID, &orgID, &dbUser.FullName, &dbUser.Email, &dbUser.PasswordHash, &dbUser.CreatedAt)

	if err != nil {
		return nil, err
	}

	dbUser.ID = userID.Bytes
	dbUser.OrganizationID = orgID.Bytes
	return &dbUser, nil
}

// GetByEmail retrieves a user by email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, organization_id, full_name, email, password_hash, created_at FROM users WHERE email = $1`

	var user domain.User
	var orgID, userID pgtype.UUID

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&userID, &orgID, &user.FullName, &user.Email, &user.PasswordHash, &user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, err
	}

	user.ID = userID.Bytes
	user.OrganizationID = orgID.Bytes
	return &user, nil
}

// GetByID retrieves a user by their ID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT id, organization_id, full_name, email, password_hash, created_at FROM users WHERE id = $1`

	var user domain.User
	var orgID, userID pgtype.UUID

	err := r.pool.QueryRow(ctx, query, pgtype.UUID{Bytes: id, Valid: true}).Scan(
		&userID, &orgID, &user.FullName, &user.Email, &user.PasswordHash, &user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, err
	}

	user.ID = userID.Bytes
	user.OrganizationID = orgID.Bytes
	return &user, nil
}
