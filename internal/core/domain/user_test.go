package domain_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectValid bool
	}{
		// Valid passwords
		{"valid password", "Password1", true},
		{"valid with special char", "Password1!", true},
		{"valid longer password", "MySecurePassword123", true},

		// Too short
		{"too short", "Pass1", false},
		{"7 chars", "Passwo1", false},

		// Missing uppercase
		{"no uppercase", "password1", false},

		// Missing lowercase
		{"no lowercase", "PASSWORD1", false},

		// Missing number
		{"no number", "Password", false},

		// Too long
		{"too long", strings.Repeat("P", 129), false},

		// Edge cases
		{"exactly 8 chars valid", "Passwor1", true},
		{"exactly 128 chars valid", strings.Repeat("P", 60) + strings.Repeat("a", 60) + "1234567A", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := domain.ValidatePassword(tt.password)
			if tt.expectValid {
				assert.Empty(t, errors, "expected password to be valid, got errors: %v", errors)
			} else {
				assert.NotEmpty(t, errors, "expected password to be invalid")
			}
		})
	}
}

func TestIsPasswordValid(t *testing.T) {
	assert.True(t, domain.IsPasswordValid("Password1"))
	assert.False(t, domain.IsPasswordValid("weak"))
}

func TestHashPassword(t *testing.T) {
	t.Run("valid password", func(t *testing.T) {
		hash, err := domain.HashPassword("Password1")
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.NotEqual(t, "Password1", hash) // Should be hashed
	})

	t.Run("weak password fails", func(t *testing.T) {
		hash, err := domain.HashPassword("weak")
		assert.Error(t, err)
		assert.Equal(t, apperrors.ErrPasswordTooWeak, err)
		assert.Empty(t, hash)
	})
}

func TestUser_CheckPassword(t *testing.T) {
	password := "Password1"
	hash, err := domain.HashPassword(password)
	require.NoError(t, err)

	user := &domain.User{
		ID:           uuid.New(),
		PasswordHash: hash,
	}

	t.Run("correct password", func(t *testing.T) {
		assert.True(t, user.CheckPassword(password))
	})

	t.Run("incorrect password", func(t *testing.T) {
		assert.False(t, user.CheckPassword("WrongPassword1"))
	})

	t.Run("empty password", func(t *testing.T) {
		assert.False(t, user.CheckPassword(""))
	})
}

func TestUserRegistrationParams_Validate(t *testing.T) {
	tests := []struct {
		name        string
		params      domain.UserRegistrationParams
		expectError bool
		errorFields []string
	}{
		{
			name: "valid params",
			params: domain.UserRegistrationParams{
				FullName: "John Doe",
				Email:    "john@example.com",
				Password: "Password1",
			},
			expectError: false,
		},
		{
			name: "empty full name",
			params: domain.UserRegistrationParams{
				FullName: "",
				Email:    "john@example.com",
				Password: "Password1",
			},
			expectError: true,
			errorFields: []string{"fullName"},
		},
		{
			name: "full name too long",
			params: domain.UserRegistrationParams{
				FullName: strings.Repeat("a", 256),
				Email:    "john@example.com",
				Password: "Password1",
			},
			expectError: true,
			errorFields: []string{"fullName"},
		},
		{
			name: "empty email",
			params: domain.UserRegistrationParams{
				FullName: "John Doe",
				Email:    "",
				Password: "Password1",
			},
			expectError: true,
			errorFields: []string{"email"},
		},
		{
			name: "invalid email format",
			params: domain.UserRegistrationParams{
				FullName: "John Doe",
				Email:    "not-an-email",
				Password: "Password1",
			},
			expectError: true,
			errorFields: []string{"email"},
		},
		{
			name: "weak password",
			params: domain.UserRegistrationParams{
				FullName: "John Doe",
				Email:    "john@example.com",
				Password: "weak",
			},
			expectError: true,
			errorFields: []string{"password"},
		},
		{
			name: "multiple errors",
			params: domain.UserRegistrationParams{
				FullName: "",
				Email:    "invalid",
				Password: "weak",
			},
			expectError: true,
			errorFields: []string{"fullName", "email", "password"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()

			if tt.expectError {
				require.Error(t, err)

				var validationErr *apperrors.ValidationErrors
				if assert.ErrorAs(t, err, &validationErr) {
					for _, field := range tt.errorFields {
						assert.Contains(t, validationErr.Errors, field,
							"expected error for field %q", field)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewUser(t *testing.T) {
	orgID := uuid.New()

	t.Run("valid user creation", func(t *testing.T) {
		params := domain.UserRegistrationParams{
			FullName: "John Doe",
			Email:    "john@example.com",
			Password: "Password1",
		}

		user, err := domain.NewUser(params, orgID)
		require.NoError(t, err)
		require.NotNil(t, user)

		assert.NotEqual(t, uuid.Nil, user.ID)
		assert.Equal(t, orgID, user.OrganizationID)
		assert.Equal(t, params.FullName, user.FullName)
		assert.Equal(t, params.Email, user.Email)
		assert.NotEmpty(t, user.PasswordHash)
		assert.NotEqual(t, params.Password, user.PasswordHash)
		assert.False(t, user.CreatedAt.IsZero())
	})

	t.Run("invalid params", func(t *testing.T) {
		params := domain.UserRegistrationParams{
			FullName: "",
			Email:    "invalid",
			Password: "weak",
		}

		user, err := domain.NewUser(params, orgID)
		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestDefaultPasswordRequirements(t *testing.T) {
	reqs := domain.DefaultPasswordRequirements()

	assert.Equal(t, 8, reqs.MinLength)
	assert.True(t, reqs.RequireUppercase)
	assert.True(t, reqs.RequireLowercase)
	assert.True(t, reqs.RequireNumber)
	assert.False(t, reqs.RequireSpecial) // Optional by default
}
