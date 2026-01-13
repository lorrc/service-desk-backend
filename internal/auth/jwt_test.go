package auth

import (
	"testing"
	"time"

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

	token, err := tm.GenerateToken(userID, orgID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := tm.ValidateToken(token)
	require.NoError(t, err)
	require.NotNil(t, claims.ExpiresAt)

	expectedExpiry := start.Add(ttl)
	assert.WithinDuration(t, expectedExpiry, claims.ExpiresAt.Time, 2*time.Second)
}
