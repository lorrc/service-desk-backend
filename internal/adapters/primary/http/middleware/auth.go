package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/lorrc/service-desk-backend/internal/auth"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// UserClaimsKey is the key used to store user claims in the request context.
const UserClaimsKey contextKey = "userClaims"

// JWTMiddleware validates the JWT token from the Authorization header.
func JWTMiddleware(tm *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Authorization header format must be Bearer {token}", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims, err := tm.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add the claims to the context for downstream handlers to use.
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
