package http

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/validation"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

const (
	maxTicketsPerPage = 100
)

// TicketHandler handles HTTP requests for tickets
type TicketHandler struct {
	ticketService  ports.TicketService
	commentHandler *CommentHandler
	errorHandler   *ErrorHandler
	logger         *slog.Logger
}

// NewTicketHandler creates a new ticket handler
func NewTicketHandler(
	ticketService ports.TicketService,
	commentHandler *CommentHandler,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *TicketHandler {
	return &TicketHandler{
		ticketService:  ticketService,
		commentHandler: commentHandler,
		errorHandler:   errorHandler,
		logger:         logger.With("handler", "ticket"),
	}
}

// Router sets up a new chi Router for all ticket-related routes.
func (h *TicketHandler) Router() http.Handler {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// RegisterRoutes sets up the routing for all ticket endpoints.
func (h *TicketHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.HandleListTickets)
	r.Post("/", h.HandleCreateTicket)

	// Routes for a specific ticket
	r.Route("/{ticketID}", func(r chi.Router) {
		r.Get("/", h.HandleGetTicket)
		r.Patch("/status", h.HandleUpdateTicketStatus)
		r.Patch("/assignee", h.HandleAssignTicket)

		// Mount the comment routes nested under /tickets/{ticketID}
		if h.commentHandler != nil {
			r.Mount("/comments", h.commentHandler.Router())
		}
	})
}

// --- Request/Response DTOs ---

// CreateTicketRequest defines the expected JSON body for creating a ticket
type CreateTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

// Validate validates the create ticket request
func (r *CreateTicketRequest) Validate() error {
	v := validation.NewValidator()

	v.Required("title", r.Title).
		MaxLength("title", r.Title, domain.MaxTitleLength)

	v.MaxLength("description", r.Description, domain.MaxDescriptionLength)

	v.Required("priority", r.Priority).
		OneOf("priority", r.Priority, []string{"LOW", "MEDIUM", "HIGH"})

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// UpdateStatusRequest defines the expected JSON body for status updates
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

// Validate validates the update status request
func (r *UpdateStatusRequest) Validate() error {
	v := validation.NewValidator()

	v.Required("status", r.Status).
		OneOf("status", r.Status, []string{"OPEN", "IN_PROGRESS", "CLOSED"})

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// AssignTicketRequest defines the expected JSON body for assigning a ticket
type AssignTicketRequest struct {
	AssigneeID string `json:"assigneeId"`
}

// Validate validates the assign ticket request
func (r *AssignTicketRequest) Validate() error {
	v := validation.NewValidator()

	v.Required("assigneeId", r.AssigneeID).
		UUID("assigneeId", r.AssigneeID)

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// --- Handlers ---

// HandleListTickets handles GET /tickets
func (h *TicketHandler) HandleListTickets(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	// Parse pagination
	pagination := validation.ParsePagination(r, maxTicketsPerPage)

	// Parse optional filters
	status := validation.ParseStringQueryParam(r, "status")
	priority := validation.ParseStringQueryParam(r, "priority")

	params := ports.ListTicketsParams{
		ViewerID: claims.UserID,
		Limit:    pagination.Limit,
		Offset:   pagination.Offset,
		Status:   status,
		Priority: priority,
	}

	tickets, err := h.ticketService.ListTickets(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	// Use simple pagination (without total count for performance)
	WritePaginatedSimple(w, tickets, pagination.Limit, pagination.Offset)
}

// HandleCreateTicket handles POST /tickets
func (h *TicketHandler) HandleCreateTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	req, err := validation.DecodeAndValidate[CreateTicketRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	params := ports.CreateTicketParams{
		Title:       req.Title,
		Description: req.Description,
		Priority:    domain.TicketPriority(req.Priority),
		RequesterID: claims.UserID,
	}

	ticket, err := h.ticketService.CreateTicket(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	h.logger.Info("ticket created",
		"ticket_id", ticket.ID,
		"user_id", claims.UserID,
	)

	WriteCreated(w, ticket)
}

// HandleGetTicket handles GET /tickets/{ticketID}
func (h *TicketHandler) HandleGetTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	ticketID, err := h.parseTicketID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	ticket, err := h.ticketService.GetTicket(r.Context(), ticketID, claims.UserID)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, ticket)
}

// HandleUpdateTicketStatus handles PATCH /tickets/{ticketID}/status
func (h *TicketHandler) HandleUpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	ticketID, err := h.parseTicketID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	req, err := validation.DecodeAndValidate[UpdateStatusRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	params := ports.UpdateStatusParams{
		TicketID: ticketID,
		Status:   domain.TicketStatus(req.Status),
		ActorID:  claims.UserID,
	}

	ticket, err := h.ticketService.UpdateStatus(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	h.logger.Info("ticket status updated",
		"ticket_id", ticketID,
		"new_status", req.Status,
		"user_id", claims.UserID,
	)

	WriteJSON(w, http.StatusOK, ticket)
}

// HandleAssignTicket handles PATCH /tickets/{ticketID}/assignee
func (h *TicketHandler) HandleAssignTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	ticketID, err := h.parseTicketID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	req, err := validation.DecodeAndValidate[AssignTicketRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	assigneeID, err := uuid.Parse(req.AssigneeID)
	if err != nil {
		// This shouldn't happen since we validated the UUID format
		h.errorHandler.Handle(w, r, err)
		return
	}

	params := ports.AssignTicketParams{
		TicketID:   ticketID,
		AssigneeID: assigneeID,
		ActorID:    claims.UserID,
	}

	ticket, err := h.ticketService.AssignTicket(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	h.logger.Info("ticket assigned",
		"ticket_id", ticketID,
		"assignee_id", assigneeID,
		"user_id", claims.UserID,
	)

	WriteJSON(w, http.StatusOK, ticket)
}

// --- Helper methods ---

// getClaims extracts and validates user claims from the request context
func (h *TicketHandler) getClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
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
func (h *TicketHandler) parseTicketID(r *http.Request) (int64, error) {
	ticketIDStr := chi.URLParam(r, "ticketID")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil || ticketID <= 0 {
		v := validation.NewValidator()
		v.Custom("ticketID", false, "Invalid ticket ID")
		return 0, v.Errors()
	}
	return ticketID, nil
}
