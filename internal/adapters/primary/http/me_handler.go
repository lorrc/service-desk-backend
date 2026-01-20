package http

import (
	"log/slog"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// PermissionsResponse defines the JSON response for user permissions.
type PermissionsResponse struct {
	Permissions []string `json:"permissions"`
}

// MeHandler handles HTTP requests for the authenticated user.
type MeHandler struct {
	authzService ports.AuthorizationService
	errorHandler *ErrorHandler
	logger       *slog.Logger
}

// NewMeHandler creates a new MeHandler.
func NewMeHandler(
	authzService ports.AuthorizationService,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *MeHandler {
	return &MeHandler{
		authzService: authzService,
		errorHandler: errorHandler,
		logger:       logger.With("handler", "me"),
	}
}

// RegisterRoutes registers the /me routes.
func (h *MeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/permissions", h.HandlePermissions)
}

// HandlePermissions handles GET /me/permissions.
func (h *MeHandler) HandlePermissions(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	permissions, err := h.authzService.GetPermissions(r.Context(), claims.UserID)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if permissions == nil {
		permissions = []string{}
	}

	sort.Strings(permissions)

	WriteJSON(w, http.StatusOK, PermissionsResponse{
		Permissions: permissions,
	})
}

// getClaims extracts and validates user claims from the request context.
func (h *MeHandler) getClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := mw.GetClaims(r.Context())
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error: "Not authorized",
			Code:  "UNAUTHORIZED",
		})
		return nil, false
	}
	return claims, true
}
