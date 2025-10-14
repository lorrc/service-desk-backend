package http

import (
	"encoding/json"
	"net/http"
	// ... imports
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type AuthHandler struct {
	authService  ports.AuthService
	tokenManager *auth.TokenManager
}

// ... Constructor ...

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// 1. Decode request DTO (e.g., LoginRequestDTO)
	// ...

	// 2. Call the core service
	user, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 3. Generate JWT (Adapter responsibility)
	token, err := h.tokenManager.GenerateToken(user.ID, user.OrganizationID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// 4. Send response DTO (e.g., TokenResponseDTO)
	// ...
}

// ... Implement HandleRegister ...
