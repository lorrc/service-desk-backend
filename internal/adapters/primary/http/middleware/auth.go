package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/auth"
)

// UserClaimsKey is the key used to store user claims in the request context.
type contextKey string

const UserClaimsKey contextKey = "userClaims"

// SessionValidator checks whether an issued token still represents an active session.
type SessionValidator interface {
	ValidateSession(ctx context.Context, userID, orgID uuid.UUID, version int64) (bool, error)
}

// JWTMiddleware validates the JWT token from the Authorization header.
func JWTMiddleware(tm *auth.TokenManager, sessions SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "Authorization header is required", "UNAUTHORIZED")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeJSONError(w, http.StatusUnauthorized, "Authorization header format must be Bearer {token}", "INVALID_AUTH_FORMAT")
				return
			}

			tokenString := parts[1]
			claims, err := tm.ValidateToken(tokenString)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "Invalid or expired token", "INVALID_TOKEN")
				return
			}

			valid, err := sessions.ValidateSession(r.Context(), claims.UserID, claims.OrgID, claims.SessionVersion)
			if err != nil {
				writeJSONError(w, http.StatusServiceUnavailable, "Authentication temporarily unavailable", "AUTH_UNAVAILABLE")
				return
			}
			if !valid {
				writeJSONError(w, http.StatusUnauthorized, "Session is no longer valid", "INVALID_TOKEN")
				return
			}

			// Add the claims to the context for downstream handlers to use.
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)

			// Also add user ID and org ID to context for logging
			ctx = context.WithValue(ctx, contextKey("user_id"), claims.UserID.String())
			ctx = context.WithValue(ctx, contextKey("org_id"), claims.OrgID.String())

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims retrieves user claims from the context
func GetClaims(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(UserClaimsKey).(*auth.Claims)
	return claims, ok
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `","code":"` + code + `"}`))
}
