package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// UserRepository is the postgres adapter for user persistence.
type UserRepository struct {
	q db.Querier
}

var _ ports.UserRepository = (*UserRepository)(nil)

// NewUserRepository creates a new user repository.
func NewUserRepository(pool *pgxpool.Pool) ports.UserRepository {
	return &UserRepository{
		q: db.New(pool),
	}
}

// mapDBUserToDomain converts a database user model to a domain model.
func mapDBUserToDomain(dbUser db.User) *domain.User {
	return &domain.User{
		ID:             dbUser.ID.Bytes,
		OrganizationID: dbUser.OrganizationID.Bytes,
		FullName:       dbUser.FullName,
		Email:          dbUser.Email,
		PasswordHash:   dbUser.HashedPassword,
		CreatedAt:      dbUser.CreatedAt.Time,
	}
}

// Create persists a new user to the database.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	params := db.CreateUserParams{
		OrganizationID: pgtype.UUID{Bytes: user.OrganizationID, Valid: true},
		FullName:       user.FullName,
		Email:          user.Email,
		HashedPassword: user.PasswordHash,
	}

	createdUser, err := r.q.CreateUser(ctx, params)
	if err != nil {
		// FIX: Check for Postgres Unique Violation (Code "23505")
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperrors.ErrUserExists
		}
		return nil, err
	}

	return mapDBUserToDomain(createdUser), nil
}

// GetByEmail retrieves a user by email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	dbUser, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, err
	}
	return mapDBUserToDomain(dbUser), nil
}

// GetByID retrieves a user by their ID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	dbUser, err := r.q.GetUserByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, err
	}
	return mapDBUserToDomain(dbUser), nil
}
