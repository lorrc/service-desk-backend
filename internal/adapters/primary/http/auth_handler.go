package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/validation"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// LoginRequest defines the expected JSON body for a login request.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validate validates the login request
func (r *LoginRequest) Validate() error {
	v := validation.NewValidator()

	v.Required("email", r.Email).
		Email("email", r.Email)

	v.Required("password", r.Password)

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// RegisterRequest defines the expected JSON body for a registration request.
type RegisterRequest struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validate validates the register request (basic validation, detailed validation in domain)
func (r *RegisterRequest) Validate() error {
	v := validation.NewValidator()

	v.Required("fullName", r.FullName)
	v.Required("email", r.Email).
		Email("email", r.Email)
	v.Required("password", r.Password)

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// AuthResponse defines the JSON response containing the authentication token.
type AuthResponse struct {
	Token string   `json:"token"`
	User  *UserDTO `json:"user"`
}

// UserDTO is a safe representation of the user (no password hash)
type UserDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	FullName       string `json:"fullName"`
	Email          string `json:"email"`
	CreatedAt      string `json:"createdAt"`
}

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	authService  ports.AuthService
	tokenManager *auth.TokenManager
	errorHandler *ErrorHandler
	logger       *slog.Logger
}

// NewAuthHandler creates a new AuthHandler with the necessary dependencies.
func NewAuthHandler(
	authService ports.AuthService,
	tokenManager *auth.TokenManager,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		tokenManager: tokenManager,
		errorHandler: errorHandler,
		logger:       logger.With("handler", "auth"),
	}
}

// RegisterRoutes registers auth routes
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/login", h.HandleLogin)
	r.Post("/register", h.HandleRegister)
}

// HandleLogin processes login requests
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	req, err := validation.DecodeAndValidate[LoginRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	user, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	token, err := h.tokenManager.GenerateToken(user.ID, user.OrganizationID)
	if err != nil {
		h.logger.Error("failed to generate token",
			"user_id", user.ID,
			"error", err,
		)
		h.errorHandler.Handle(w, r, err)
		return
	}

	h.logger.Info("user logged in",
		"user_id", user.ID,
		"email", user.Email,
	)

	WriteJSON(w, http.StatusOK, AuthResponse{
		Token: token,
		User:  toUserDTO(user),
	})
}

// HandleRegister processes new user registration requests.
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	req, err := validation.DecodeAndValidate[RegisterRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	// Register user (domain validation happens in the service)
	user, err := h.authService.Register(r.Context(), req.FullName, req.Email, req.Password, uuid.Nil)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	token, err := h.tokenManager.GenerateToken(user.ID, user.OrganizationID)
	if err != nil {
		h.logger.Error("failed to generate token after registration",
			"user_id", user.ID,
			"error", err,
		)
		h.errorHandler.Handle(w, r, err)
		return
	}

	h.logger.Info("user registered",
		"user_id", user.ID,
		"email", user.Email,
	)

	WriteJSON(w, http.StatusCreated, AuthResponse{
		Token: token,
		User:  toUserDTO(user),
	})
}

// toUserDTO converts a domain user to a safe DTO
func toUserDTO(user *domain.User) *UserDTO {
	return &UserDTO{
		ID:             user.ID.String(),
		OrganizationID: user.OrganizationID.String(),
		FullName:       user.FullName,
		Email:          user.Email,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
