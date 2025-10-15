package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type UserRepository struct {
	q db.Querier
}

var _ ports.UserRepository = (*UserRepository)(nil)

func NewUserRepository(pool *pgxpool.Pool) ports.UserRepository {
	return &UserRepository{
		q: db.New(pool),
	}
}

func mapDBUserToDomain(dbUser db.User) *domain.User {
	return &domain.User{
		ID:             dbUser.ID.Bytes,
		OrganizationID: dbUser.OrganizationID.Bytes,
		FullName:       dbUser.FullName,
		Email:          dbUser.Email,
		PasswordHash:   dbUser.PasswordHash,
		CreatedAt:      dbUser.CreatedAt.Time,
	}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	params := db.CreateUserParams{
		OrganizationID: pgtype.UUID{Bytes: user.OrganizationID, Valid: true},
		FullName:       user.FullName,
		Email:          user.Email,
		PasswordHash:   user.PasswordHash,
	}

	createdUser, err := r.q.CreateUser(ctx, params)
	if err != nil {
		// A more robust implementation would check for the specific unique violation error code
		return nil, err
	}

	return mapDBUserToDomain(createdUser), nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	dbUser, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrUserNotFound
		}
		return nil, err
	}
	return mapDBUserToDomain(dbUser), nil
}
