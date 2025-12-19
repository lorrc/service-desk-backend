package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/mocks"
	"github.com/lorrc/service-desk-backend/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAuthService_Register(t *testing.T) {
	ctx := context.Background()
	testOrgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	t.Run("success", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		// User doesn't exist yet
		mockUserRepo.On("GetByEmail", ctx, "newuser@example.com").
			Return(nil, apperrors.ErrUserNotFound)

		// User will be created
		mockUserRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).
			Return(&domain.User{
				ID:             uuid.New(),
				OrganizationID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				FullName:       "New User",
				Email:          "newuser@example.com",
				CreatedAt:      time.Now(),
			}, nil)

		user, err := svc.Register(ctx, "New User", "newuser@example.com", "Password123", uuid.Nil)

		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "New User", user.FullName)
		assert.Equal(t, "newuser@example.com", user.Email)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("user already exists", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		existingUser := &domain.User{
			ID:    uuid.New(),
			Email: "existing@example.com",
		}
		mockUserRepo.On("GetByEmail", ctx, "existing@example.com").
			Return(existingUser, nil)

		user, err := svc.Register(ctx, "User", "existing@example.com", "Password123", uuid.Nil)

		assert.Nil(t, user)
		assert.ErrorIs(t, err, apperrors.ErrUserExists)
		mockUserRepo.AssertNotCalled(t, "Create")
	})

	t.Run("weak password", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		user, err := svc.Register(ctx, "User", "user@example.com", "weak", uuid.Nil)

		assert.Nil(t, user)
		assert.Error(t, err)
		// Validation error for weak password
		var validationErr *apperrors.ValidationErrors
		assert.ErrorAs(t, err, &validationErr)

		mockUserRepo.AssertNotCalled(t, "GetByEmail")
		mockUserRepo.AssertNotCalled(t, "Create")
	})

	t.Run("invalid email", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		user, err := svc.Register(ctx, "User", "invalid-email", "Password123", uuid.Nil)

		assert.Nil(t, user)
		assert.Error(t, err)

		mockUserRepo.AssertNotCalled(t, "GetByEmail")
		mockUserRepo.AssertNotCalled(t, "Create")
	})

	t.Run("empty full name", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		user, err := svc.Register(ctx, "", "user@example.com", "Password123", uuid.Nil)

		assert.Nil(t, user)
		assert.Error(t, err)

		mockUserRepo.AssertNotCalled(t, "GetByEmail")
		mockUserRepo.AssertNotCalled(t, "Create")
	})
}

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()
	testOrgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	t.Run("success", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		// Create a valid password hash
		hash, _ := domain.HashPassword("Password123")

		existingUser := &domain.User{
			ID:           uuid.New(),
			Email:        "user@example.com",
			FullName:     "Test User",
			PasswordHash: hash,
		}

		mockUserRepo.On("GetByEmail", ctx, "user@example.com").
			Return(existingUser, nil)

		user, err := svc.Login(ctx, "user@example.com", "Password123")

		require.NoError(t, err)
		assert.Equal(t, existingUser.ID, user.ID)
		assert.Equal(t, existingUser.Email, user.Email)
	})

	t.Run("user not found", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		mockUserRepo.On("GetByEmail", ctx, "unknown@example.com").
			Return(nil, apperrors.ErrUserNotFound)

		user, err := svc.Login(ctx, "unknown@example.com", "Password123")

		assert.Nil(t, user)
		// Should return generic invalid credentials, not reveal user doesn't exist
		assert.ErrorIs(t, err, apperrors.ErrInvalidCredentials)
	})

	t.Run("wrong password", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		hash, _ := domain.HashPassword("Password123")

		existingUser := &domain.User{
			ID:           uuid.New(),
			Email:        "user@example.com",
			PasswordHash: hash,
		}

		mockUserRepo.On("GetByEmail", ctx, "user@example.com").
			Return(existingUser, nil)

		user, err := svc.Login(ctx, "user@example.com", "WrongPassword123")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, apperrors.ErrInvalidCredentials)
	})

	t.Run("empty email", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		user, err := svc.Login(ctx, "", "Password123")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, apperrors.ErrEmailRequired)
		mockUserRepo.AssertNotCalled(t, "GetByEmail")
	})

	t.Run("empty password", func(t *testing.T) {
		mockUserRepo := mocks.NewMockUserRepository()
		svc := services.NewAuthService(mockUserRepo, testOrgID)

		user, err := svc.Login(ctx, "user@example.com", "")

		assert.Nil(t, user)
		assert.ErrorIs(t, err, apperrors.ErrPasswordRequired)
		mockUserRepo.AssertNotCalled(t, "GetByEmail")
	})
}
