package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims defines the structured data we store in the JWT
type Claims struct {
	UserID         uuid.UUID `json:"user_id"`
	OrgID          uuid.UUID `json:"org_id"`
	SessionVersion int64     `json:"session_version"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secretKey []byte
	accessTTL time.Duration
}

func NewTokenManager(secret string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{
		secretKey: []byte(secret),
		accessTTL: accessTTL,
	}
}

// GenerateToken creates a new JWT access token
func (tm *TokenManager) GenerateToken(userID, orgID uuid.UUID, sessionVersion int64) (string, error) {
	if userID == uuid.Nil || orgID == uuid.Nil || sessionVersion < 1 {
		return "", errors.New("invalid session identity")
	}
	ttl := tm.accessTTL
	if ttl <= 0 {
		ttl = time.Hour
	}

	expirationTime := time.Now().Add(ttl)
	claims := &Claims{
		UserID:         userID,
		OrgID:          orgID,
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
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
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())

	if err != nil {
		return nil, err
	}

	if !token.Valid || claims.UserID == uuid.Nil || claims.OrgID == uuid.Nil || claims.SessionVersion < 1 {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
