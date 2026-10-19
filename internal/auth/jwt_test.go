package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenManager_UsesConfiguredTTL(t *testing.T) {
	ttl := 2 * time.Hour
	tm := NewTokenManager("test-secret", ttl)

	userID := uuid.New()
	orgID := uuid.New()

	start := time.Now()

	token, err := tm.GenerateToken(userID, orgID, 1)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := tm.ValidateToken(token)
	require.NoError(t, err)
	require.NotNil(t, claims.ExpiresAt)

	expectedExpiry := start.Add(ttl)
	assert.WithinDuration(t, expectedExpiry, claims.ExpiresAt.Time, 2*time.Second)
}

func TestTokenManagerRejectsInvalidClaims(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing expiry", "expired", "missing version", "missing organization", "missing user", "wrong algorithm"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			claims := Claims{UserID: uuid.New(), OrgID: uuid.New(), SessionVersion: 1,
				RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
			method := jwt.SigningMethodHS256
			switch name {
			case "missing expiry":
				claims.ExpiresAt = nil
			case "expired":
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
			case "missing version":
				claims.SessionVersion = 0
			case "missing organization":
				claims.OrgID = uuid.Nil
			case "missing user":
				claims.UserID = uuid.Nil
			case "wrong algorithm":
				method = jwt.SigningMethodHS512
			}
			token, err := jwt.NewWithClaims(method, claims).SignedString([]byte("test-secret"))
			require.NoError(t, err)
			_, err = NewTokenManager("test-secret", time.Hour).ValidateToken(token)
			require.Error(t, err)
		})
	}
}

func TestTokenManagerRejectsInvalidIdentity(t *testing.T) {
	t.Parallel()
	manager := NewTokenManager("test-secret", time.Hour)
	for _, tc := range []struct {
		user, org uuid.UUID
		version   int64
	}{
		{uuid.Nil, uuid.New(), 1}, {uuid.New(), uuid.Nil, 1}, {uuid.New(), uuid.New(), 0},
	} {
		_, err := manager.GenerateToken(tc.user, tc.org, tc.version)
		require.Error(t, err)
	}
}
