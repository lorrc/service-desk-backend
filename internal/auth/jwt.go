package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims defines the structured data we store in the JWT
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	OrgID  uuid.UUID `json:"org_id"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secretKey []byte
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secretKey: []byte(secret)}
}

// GenerateToken creates a new JWT access token
func (tm *TokenManager) GenerateToken(userID, orgID uuid.UUID) (string, error) {
	expirationTime := time.Now().Add(1 * time.Hour) // 1 hour expiration
	claims := &Claims{
		UserID: userID,
		OrgID:  orgID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Subject:   userID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secretKey)
}

// ValidateToken parses and validates the token string
func (tm *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
