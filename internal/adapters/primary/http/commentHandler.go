package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// CommentHandler handles HTTP requests for comments.
type CommentHandler struct {
	commentService ports.CommentService
}

// NewCommentHandler creates a new CommentHandler.
func NewCommentHandler(commentService ports.CommentService) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
	}
}

// Router sets up a new chi Router for comment routes.
func (h *CommentHandler) Router() http.Handler {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// RegisterRoutes registers the comment-specific endpoints.
// These routes are relative to /api/v1/tickets/{ticketID}/comments
func (h *CommentHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.HandleCreateComment)
	r.Get("/", h.HandleListComments)
}

// --- Request DTOs ---

type CreateCommentRequest struct {
	Body string `json:"body"`
}

// --- Handlers ---

// HandleCreateComment handles requests to create a new comment.
func (h *CommentHandler) HandleCreateComment(w http.ResponseWriter, r *http.Request) {
	// 1. Get claims from context
	claims, ok := r.Context().Value(mw.UserClaimsKey).(*auth.Claims)
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Not authorized"})
		return
	}

	// 2. Get ticketID from URL param
	ticketIDStr := chi.URLParam(r, "ticketID")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid ticket ID"})
		return
	}

	// 3. Decode request body
	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	// 4. Call service
	params := ports.CreateCommentParams{
		TicketID: ticketID,
		ActorID:  claims.UserID,
		Body:     req.Body,
	}
	comment, err := h.commentService.CreateComment(r.Context(), params)
	if err != nil {
		if errors.Is(err, ports.ErrForbidden) {
			WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "You do not have permission to comment on this ticket"})
		} else if errors.Is(err, domain.ErrCommentBodyRequired) {
			WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		} else {
			WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create comment"})
		}
		return
	}

	// 5. Send response
	WriteJSON(w, http.StatusCreated, comment)
}

// HandleListComments handles requests to list comments for a ticket.
func (h *CommentHandler) HandleListComments(w http.ResponseWriter, r *http.Request) {
	// 1. Get claims from context
	claims, ok := r.Context().Value(mw.UserClaimsKey).(*auth.Claims)
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Not authorized"})
		return
	}

	// 2. Get ticketID from URL param
	ticketIDStr := chi.URLParam(r, "ticketID")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid ticket ID"})
		return
	}

	// 3. Call service
	params := ports.GetCommentsParams{
		TicketID: ticketID,
		ActorID:  claims.UserID,
	}
	comments, err := h.commentService.GetCommentsForTicket(r.Context(), params)
	if err != nil {
		if errors.Is(err, ports.ErrForbidden) {
			WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "You do not have permission to view comments for this ticket"})
		} else {
			WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve comments"})
		}
		return
	}

	// 4. Send response
	WriteJSON(w, http.StatusOK, comments)
}
