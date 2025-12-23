package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRepos is a helper to create repos for a test.
func newTestRepos(t *testing.T) (ports.TicketRepository, ports.UserRepository) {
	require.NotNil(t, testPool, "testPool is nil. TestMain may not have run.")

	userRepo := NewUserRepository(testPool)
	ticketRepo := NewTicketRepository(testPool)

	return ticketRepo, userRepo
}

func TestUserRepository_CreateGet(t *testing.T) {
	ctx := context.Background()
	_, userRepo := newTestRepos(t)

	// 1. Create a new user
	orgID := uuid.New()
	newUser := &domain.User{
		ID:             uuid.New(),
		FullName:       "Test User",
		Email:          "test.user@example.com",
		HashedPassword: "hashedpassword",
		OrganizationID: orgID,
	}

	createdUser, err := userRepo.Create(ctx, newUser)
	require.NoError(t, err, "Failed to create user")

	// 2. Get the user by email
	foundUser, err := userRepo.GetByEmail(ctx, "test.user@example.com")
	require.NoError(t, err, "Failed to get user by email")

	// 3. Assert values are correct
	assert.Equal(t, createdUser.ID, foundUser.ID)
	assert.Equal(t, "Test User", foundUser.FullName)
	assert.Equal(t, "test.user@example.com", foundUser.Email)

	// 4. Get the user by ID
	foundUserByID, err := userRepo.GetByID(ctx, createdUser.ID)
	require.NoError(t, err, "Failed to get user by ID")
	assert.Equal(t, createdUser.ID, foundUserByID.ID)
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	ctx := context.Background()
	_, userRepo := newTestRepos(t)

	_, err := userRepo.GetByEmail(ctx, "nonexistent@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrUserNotFound)
}
