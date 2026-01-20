package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// AssigneeDTO represents a user that can be assigned to tickets.
type AssigneeDTO struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
}

// AssigneeHandler handles HTTP requests for assignable users.
type AssigneeHandler struct {
	assigneeService ports.AssigneeService
	errorHandler    *ErrorHandler
	logger          *slog.Logger
}

// NewAssigneeHandler creates a new AssigneeHandler.
func NewAssigneeHandler(
	assigneeService ports.AssigneeService,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *AssigneeHandler {
	return &AssigneeHandler{
		assigneeService: assigneeService,
		errorHandler:    errorHandler,
		logger:          logger.With("handler", "assignees"),
	}
}

// RegisterRoutes registers the /assignees routes.
func (h *AssigneeHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.HandleListAssignees)
}

// HandleListAssignees handles GET /assignees.
func (h *AssigneeHandler) HandleListAssignees(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	users, err := h.assigneeService.ListAssignableUsers(r.Context(), claims.UserID, claims.OrgID)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteList(w, mapAssignees(users))
}

func mapAssignees(users []*domain.User) []AssigneeDTO {
	assignees := make([]AssigneeDTO, 0, len(users))
	for _, user := range users {
		assignees = append(assignees, AssigneeDTO{
			ID:       user.ID.String(),
			FullName: user.FullName,
			Email:    user.Email,
		})
	}
	return assignees
}

// getClaims extracts and validates user claims from the request context.
func (h *AssigneeHandler) getClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
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
