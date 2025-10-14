package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/lorrc/service-desk-backend/internal/auth"
)

// Define a custom type for context keys to avoid collisions
type contextKey string

const UserClaimsKey contextKey = "userClaims"

func JWTMiddleware(tm *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Authorization header format must be Bearer {token}", http.StatusUnauthorized)
				return
			}

			// 2. Validate token
			claims, err := tm.ValidateToken(parts[1])
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// 3. Add Claims to context
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
