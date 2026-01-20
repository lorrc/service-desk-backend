package postgres

import (
	"context"
	"errors"
	"time"

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
	q    db.Querier
	pool *pgxpool.Pool
}

var _ ports.UserRepository = (*UserRepository)(nil)

// NewUserRepository creates a new user repository.
func NewUserRepository(pool *pgxpool.Pool) ports.UserRepository {
	return &UserRepository{
		q:    db.New(pool),
		pool: pool,
	}
}

// mapDBUserToDomain converts a database user model to a domain model.
func mapDBUserToDomain(dbUser db.User) *domain.User {
	return &domain.User{
		ID:             dbUser.ID.Bytes,
		OrganizationID: dbUser.OrganizationID.Bytes,
		FullName:       dbUser.FullName,
		Email:          dbUser.Email,
		HashedPassword: dbUser.HashedPassword,
		CreatedAt:      dbUser.CreatedAt.Time,
		IsActive:       dbUser.IsActive,
		LastActiveAt:   toTimePtr(dbUser.LastActiveAt),
	}
}

func toTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	value := ts.Time
	return &value
}

// Create persists a new user to the database.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	params := db.CreateUserParams{
		OrganizationID: pgtype.UUID{Bytes: user.OrganizationID, Valid: true},
		FullName:       user.FullName,
		Email:          user.Email,
		HashedPassword: user.HashedPassword,
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

// CountUsers returns the total number of users.
func (r *UserRepository) CountUsers(ctx context.Context) (int64, error) {
	return r.q.CountUsers(ctx)
}

// ListAssignableUsers returns users eligible for ticket assignment in the same org.
func (r *UserRepository) ListAssignableUsers(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	const listAssignableUsers = `
SELECT DISTINCT u.id, u.organization_id, u.full_name, u.email, u.hashed_password, u.created_at, u.is_active, u.last_active_at
FROM users u
JOIN user_roles ur ON u.id = ur.user_id
JOIN roles r ON ur.role_id = r.id
WHERE u.organization_id = $1
  AND u.is_active = TRUE
  AND r.name IN ('admin', 'agent')
ORDER BY u.full_name, u.email
`

	rows, err := r.pool.Query(ctx, listAssignableUsers, pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user := &domain.User{}
		var lastActive pgtype.Timestamptz
		if err := rows.Scan(
			&user.ID,
			&user.OrganizationID,
			&user.FullName,
			&user.Email,
			&user.HashedPassword,
			&user.CreatedAt,
			&user.IsActive,
			&lastActive,
		); err != nil {
			return nil, err
		}
		user.LastActiveAt = toTimePtr(lastActive)
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.UserSummary, error) {
	const listUsers = `
SELECT u.id,
       u.organization_id,
       u.full_name,
       u.email,
       u.created_at,
       u.is_active,
       u.last_active_at,
       COALESCE(array_agg(r.name ORDER BY r.name) FILTER (WHERE r.name IS NOT NULL), '{}') AS roles
FROM users u
LEFT JOIN user_roles ur ON u.id = ur.user_id
LEFT JOIN roles r ON ur.role_id = r.id
WHERE u.organization_id = $1
GROUP BY u.id
ORDER BY u.full_name, u.email
`

	rows, err := r.pool.Query(ctx, listUsers, pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*domain.UserSummary, 0)
	for rows.Next() {
		var (
			id           uuid.UUID
			organization uuid.UUID
			fullName     string
			email        string
			createdAt    time.Time
			isActive     bool
			lastActive   pgtype.Timestamptz
			roles        []string
		)

		if err := rows.Scan(
			&id,
			&organization,
			&fullName,
			&email,
			&createdAt,
			&isActive,
			&lastActive,
			&roles,
		); err != nil {
			return nil, err
		}

		if roles == nil {
			roles = []string{}
		}

		users = append(users, &domain.UserSummary{
			ID:             id,
			OrganizationID: organization,
			FullName:       fullName,
			Email:          email,
			Roles:          roles,
			IsActive:       isActive,
			CreatedAt:      createdAt,
			LastActiveAt:   toTimePtr(lastActive),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepository) SetActive(ctx context.Context, userID uuid.UUID, isActive bool) error {
	tag, err := r.pool.Exec(ctx, "UPDATE users SET is_active = $2 WHERE id = $1", pgtype.UUID{Bytes: userID, Valid: true}, isActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	tag, err := r.pool.Exec(ctx, "UPDATE users SET hashed_password = $2 WHERE id = $1", pgtype.UUID{Bytes: userID, Valid: true}, hashedPassword)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdateLastActive(ctx context.Context, userID uuid.UUID, at time.Time) error {
	tag, err := r.pool.Exec(ctx, "UPDATE users SET last_active_at = $2 WHERE id = $1", pgtype.UUID{Bytes: userID, Valid: true}, pgtype.Timestamptz{Time: at.UTC(), Valid: true})
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrUserNotFound
	}
	return nil
}
