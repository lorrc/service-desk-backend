package http

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/validation"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// CommentHandler handles HTTP requests for comments.
type CommentHandler struct {
	commentService ports.CommentService
	errorHandler   *ErrorHandler
	logger         *slog.Logger
}

// NewCommentHandler creates a new CommentHandler.
func NewCommentHandler(
	commentService ports.CommentService,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
		errorHandler:   errorHandler,
		logger:         logger.With("handler", "comment"),
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

// CreateCommentRequest defines the expected JSON body for creating a comment
type CreateCommentRequest struct {
	Body string `json:"body"`
}

// Validate validates the create comment request
func (r *CreateCommentRequest) Validate() error {
	v := validation.NewValidator()

	v.Required("body", r.Body).
		MaxLength("body", r.Body, domain.MaxCommentBodyLength)

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// CommentDTO defines the JSON response for comments.
type CommentDTO struct {
	ID        string `json:"id"`
	TicketID  int64  `json:"ticketId"`
	AuthorID  string `json:"authorId"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

func toCommentDTO(comment *domain.Comment) CommentDTO {
	return CommentDTO{
		ID:        strconv.FormatInt(comment.ID, 10),
		TicketID:  comment.TicketID,
		AuthorID:  comment.AuthorID.String(),
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt.Format(time.RFC3339),
	}
}

func toCommentDTOs(comments []*domain.Comment) []CommentDTO {
	response := make([]CommentDTO, 0, len(comments))
	for _, comment := range comments {
		response = append(response, toCommentDTO(comment))
	}
	return response
}

// --- Handlers ---

// HandleCreateComment handles requests to create a new comment.
func (h *CommentHandler) HandleCreateComment(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	ticketID, err := h.parseTicketID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	req, err := validation.DecodeAndValidate[CreateCommentRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	params := ports.CreateCommentParams{
		TicketID: ticketID,
		ActorID:  claims.UserID,
		Body:     req.Body,
	}

	comment, err := h.commentService.CreateComment(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	h.logger.Info("comment created",
		"comment_id", comment.ID,
		"ticket_id", ticketID,
		"user_id", claims.UserID,
	)

	WriteCreated(w, toCommentDTO(comment))
}

// HandleListComments handles requests to list comments for a ticket.
func (h *CommentHandler) HandleListComments(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	ticketID, err := h.parseTicketID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	params := ports.GetCommentsParams{
		TicketID: ticketID,
		ActorID:  claims.UserID,
	}

	comments, err := h.commentService.GetCommentsForTicket(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteList(w, toCommentDTOs(comments))
}

// --- Helper methods ---

// getClaims extracts and validates user claims from the request context
func (h *CommentHandler) getClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
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

// parseTicketID extracts and validates the ticket ID from the URL
func (h *CommentHandler) parseTicketID(r *http.Request) (int64, error) {
	ticketIDStr := chi.URLParam(r, "ticketID")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil || ticketID <= 0 {
		v := validation.NewValidator()
		v.Custom("ticketID", false, "Invalid ticket ID")
		return 0, v.Errors()
	}
	return ticketID, nil
}
