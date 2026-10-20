package http

import (
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/validation"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// PermissionsResponse defines the JSON response for user permissions.
type PermissionsResponse struct {
	Permissions []string `json:"permissions"`
}

// MeHandler handles HTTP requests for the authenticated user.
type MeHandler struct {
	authzService   ports.AuthorizationService
	accountService ports.AccountService
	errorHandler   *ErrorHandler
	logger         *slog.Logger
}

// NewMeHandler creates a new MeHandler.
func NewMeHandler(
	authzService ports.AuthorizationService,
	accountService ports.AccountService,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *MeHandler {
	return &MeHandler{
		authzService:   authzService,
		accountService: accountService,
		errorHandler:   errorHandler,
		logger:         logger.With("handler", "me"),
	}
}

// RegisterRoutes registers the /me routes.
func (h *MeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/permissions", h.HandlePermissions)
	r.Get("/", h.HandleProfile)
	r.Post("/password", h.HandleChangePassword)
	r.Post("/sign-out-everywhere", h.HandleSignOutEverywhere)
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

// ProfileResponse exposes the authenticated identity and current access rights.
type ProfileResponse struct {
	*UserDTO
	IsActive     bool     `json:"isActive"`
	LastSignInAt *string  `json:"lastSignInAt"`
	Roles        []string `json:"roles"`
	Permissions  []string `json:"permissions"`
}

// HandleProfile handles GET /me without exposing credential or session-version fields.
func (h *MeHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}
	profile, err := h.accountService.Profile(r.Context(), claims.UserID, claims.OrgID, claims.SessionVersion)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	var lastSignIn *string
	if profile.User.LastActiveAt != nil {
		value := profile.User.LastActiveAt.Format(time.RFC3339)
		lastSignIn = &value
	}
	WriteJSON(w, http.StatusOK, ProfileResponse{UserDTO: toUserDTO(profile.User), IsActive: profile.User.IsActive, LastSignInAt: lastSignIn, Roles: profile.Roles, Permissions: profile.Permissions})
}

// ChangePasswordRequest supplies the current password and its proposed replacement.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// Validate checks required fields; the account service checks password policy and identity.
func (r *ChangePasswordRequest) Validate() error {
	v := validation.NewValidator()
	v.Required("currentPassword", r.CurrentPassword)
	v.Required("newPassword", r.NewPassword)
	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// HandleChangePassword handles POST /me/password and revokes sessions on success.
func (h *MeHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}
	request, err := validation.DecodeAndValidate[ChangePasswordRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	if err := request.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	if err := h.accountService.ChangePassword(r.Context(), claims.UserID, claims.OrgID, claims.SessionVersion, request.CurrentPassword, request.NewPassword); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	WriteNoContent(w)
}

// HandleSignOutEverywhere handles POST /me/sign-out-everywhere, including the calling session.
func (h *MeHandler) HandleSignOutEverywhere(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}
	if err := h.accountService.SignOutEverywhere(r.Context(), claims.UserID, claims.OrgID, claims.SessionVersion); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	WriteNoContent(w)
}
