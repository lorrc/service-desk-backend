package http

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
	defaultEventsLimit = 50
	maxEventsLimit     = 200
)

// TicketHandler handles HTTP requests for tickets
type TicketHandler struct {
	ticketService  ports.TicketService
	eventService   ports.EventService
	commentHandler *CommentHandler
	errorHandler   *ErrorHandler
	logger         *slog.Logger
}

// NewTicketHandler creates a new ticket handler
func NewTicketHandler(
	ticketService ports.TicketService,
	eventService ports.EventService,
	commentHandler *CommentHandler,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *TicketHandler {
	return &TicketHandler{
		ticketService:  ticketService,
		eventService:   eventService,
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
		r.Get("/events", h.HandleListTicketEvents)

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

// TicketDTO defines the JSON response for tickets.
type TicketDTO struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	RequesterID string  `json:"requesterId"`
	AssigneeID  *string `json:"assigneeId"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   *string `json:"updatedAt"`
	ClosedAt    *string `json:"closedAt"`
}

func toTicketDTO(ticket *domain.Ticket) TicketDTO {
	var assigneeID *string
	if ticket.AssigneeID != nil {
		value := ticket.AssigneeID.String()
		assigneeID = &value
	}

	var updatedAt *string
	if ticket.UpdatedAt != nil {
		value := ticket.UpdatedAt.Format(time.RFC3339)
		updatedAt = &value
	}

	var closedAt *string
	if ticket.ClosedAt != nil {
		value := ticket.ClosedAt.Format(time.RFC3339)
		closedAt = &value
	}

	return TicketDTO{
		ID:          ticket.ID,
		Title:       ticket.Title,
		Description: ticket.Description,
		Status:      string(ticket.Status),
		Priority:    string(ticket.Priority),
		RequesterID: ticket.RequesterID.String(),
		AssigneeID:  assigneeID,
		CreatedAt:   ticket.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   updatedAt,
		ClosedAt:    closedAt,
	}
}

func toTicketDTOs(tickets []*domain.Ticket) []TicketDTO {
	response := make([]TicketDTO, 0, len(tickets))
	for _, ticket := range tickets {
		response = append(response, toTicketDTO(ticket))
	}
	return response
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
	unassigned := validation.ParseBoolQueryParam(r, "unassigned", false)

	v := validation.NewValidator()

	var assigneeID *uuid.UUID
	if assigneeIDStr := r.URL.Query().Get("assigneeId"); assigneeIDStr != "" {
		parsedAssigneeID, err := uuid.Parse(assigneeIDStr)
		if err != nil {
			v.Custom("assigneeId", false, "Must be a valid UUID")
		} else {
			assigneeID = &parsedAssigneeID
		}
	}

	createdFrom, err := validation.ParseTimeQueryParam(r, "createdFrom")
	if err != nil {
		v.Custom("createdFrom", false, "Must be a valid date or timestamp")
	}

	createdTo, err := validation.ParseTimeQueryParam(r, "createdTo")
	if err != nil {
		v.Custom("createdTo", false, "Must be a valid date or timestamp")
	}

	var createdFromTime *time.Time
	if createdFrom != nil {
		createdFromTime = &createdFrom.Time
	}

	var createdToTime *time.Time
	if createdTo != nil {
		adjusted := createdTo.Time
		if createdTo.DateOnly {
			adjusted = adjusted.Add(24 * time.Hour)
		}
		createdToTime = &adjusted
	}

	if createdFromTime != nil && createdToTime != nil && createdFromTime.After(*createdToTime) {
		v.Custom("createdFrom", false, "Must be before createdTo")
	}

	if unassigned {
		assigneeID = nil
	}

	if v.HasErrors() {
		h.errorHandler.Handle(w, r, v.Errors())
		return
	}

	params := ports.ListTicketsParams{
		ViewerID:    claims.UserID,
		Limit:       pagination.Limit + 1,
		Offset:      pagination.Offset,
		Status:      status,
		Priority:    priority,
		AssigneeID:  assigneeID,
		Unassigned:  unassigned,
		CreatedFrom: createdFromTime,
		CreatedTo:   createdToTime,
	}

	tickets, err := h.ticketService.ListTickets(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	// Use simple pagination (without total count for performance)
	WritePaginatedSimple(w, toTicketDTOs(tickets), pagination.Limit, pagination.Offset)
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

	WriteCreated(w, toTicketDTO(ticket))
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

	WriteJSON(w, http.StatusOK, toTicketDTO(ticket))
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

	WriteJSON(w, http.StatusOK, toTicketDTO(ticket))
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

	WriteJSON(w, http.StatusOK, toTicketDTO(ticket))
}

// TicketEventsResponse defines the JSON response for ticket events.
type TicketEventsResponse struct {
	Data       []*domain.Event `json:"data"`
	NextCursor *int64          `json:"nextCursor,omitempty"`
}

// HandleListTicketEvents handles GET /tickets/{ticketID}/events
func (h *TicketHandler) HandleListTicketEvents(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	ticketID, err := h.parseTicketID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	afterID, limit, err := h.parseEventQuery(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	params := ports.ListTicketEventsParams{
		TicketID: ticketID,
		ViewerID: claims.UserID,
		AfterID:  afterID,
		Limit:    limit,
	}

	events, err := h.eventService.ListTicketEvents(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	var nextCursor *int64
	if len(events) > 0 {
		cursor := events[len(events)-1].ID
		nextCursor = &cursor
	}

	WriteJSON(w, http.StatusOK, TicketEventsResponse{
		Data:       events,
		NextCursor: nextCursor,
	})
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

func (h *TicketHandler) parseEventQuery(r *http.Request) (int64, int, error) {
	v := validation.NewValidator()

	afterID := int64(0)
	if afterStr := r.URL.Query().Get("after"); afterStr != "" {
		parsed, err := strconv.ParseInt(afterStr, 10, 64)
		if err != nil || parsed < 0 {
			v.Custom("after", false, "after must be a positive integer")
		} else {
			afterID = parsed
		}
	}

	limit := defaultEventsLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 {
			v.Custom("limit", false, "limit must be a positive integer")
		} else {
			limit = parsed
		}
	}

	if limit > maxEventsLimit {
		v.Custom("limit", false, "limit exceeds maximum")
	}

	if v.HasErrors() {
		return 0, 0, v.Errors()
	}

	return afterID, limit, nil
}
