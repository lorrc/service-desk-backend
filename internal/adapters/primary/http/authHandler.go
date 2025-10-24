package http

import (
	"encoding/json"
	"errors"
	"net/http"
	// ... imports

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// LoginRequest defines the expected JSON body for a login request.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest defines the expected JSON body for a registration request.
type RegisterRequest struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse defines the JSON response containing the authentication token.
type AuthResponse struct {
	Token string `json:"token"`
}

type AuthHandler struct {
	authService  ports.AuthService
	tokenManager *auth.TokenManager
}

// NewAuthHandler creates a new AuthHandler with the necessary dependencies.
func NewAuthHandler(authService ports.AuthService, tokenManager *auth.TokenManager) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		tokenManager: tokenManager,
	}
}

func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/login", h.HandleLogin)
	r.Post("/register", h.HandleRegister)
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	user, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ports.ErrInvalidCredentials) {
			WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid credentials"})
		} else {
			WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "An unexpected error occurred"})
		}
		return
	}

	// Correctly pass OrganizationID to the token generator
	token, err := h.tokenManager.GenerateToken(user.ID, user.OrganizationID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate token"})
		return
	}

	WriteJSON(w, http.StatusOK, AuthResponse{Token: token})
}

// HandleRegister processes new user registration requests.
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	// For simplicity, new users are registered to a default organization.
	// This logic resides in the auth service.
	user, err := h.authService.Register(r.Context(), req.FullName, req.Email, req.Password, uuid.Nil)
	if err != nil {
		if errors.Is(err, ports.ErrUserExists) {
			WriteJSON(w, http.StatusConflict, ErrorResponse{Error: "A user with this email already exists"})
		} else {
			WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to register user"})
		}
		return
	}

	// Correctly pass OrganizationID to the token generator
	token, err := h.tokenManager.GenerateToken(user.ID, user.OrganizationID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate token"})
		return
	}

	WriteJSON(w, http.StatusCreated, AuthResponse{Token: token})
}
