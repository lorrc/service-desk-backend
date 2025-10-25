package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type TicketHandler struct {
	ticketService  ports.TicketService
	commentHandler *CommentHandler // Inject CommentHandler
}

// NewTicketHandler now accepts a CommentHandler.
func NewTicketHandler(ticketService ports.TicketService, commentHandler *CommentHandler) *TicketHandler {
	return &TicketHandler{
		ticketService:  ticketService,
		commentHandler: commentHandler,
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
		r.Mount("/comments", h.commentHandler.Router())
	})
}

// --- Request/Response DTOs ---

type CreateTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

type UpdateStatusRequest struct {
	Status string `json:"status"`
}

type AssignTicketRequest struct {
	AssigneeID string `json:"assigneeId"`
}

// --- Handlers ---

func (h *TicketHandler) HandleListTickets(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(mw.UserClaimsKey).(*auth.Claims)
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Not authorized"})
		return
	}

	// Parse pagination and filter query parameters
	limit := getIntQueryParam(r, "limit", 25)
	offset := getIntQueryParam(r, "offset", 0)
	status := getStringQueryParam(r, "status")
	priority := getStringQueryParam(r, "priority")

	// Enforce a max limit to prevent abuse
	if limit > 100 {
		limit = 100
	}

	params := ports.ListTicketsParams{
		ViewerID: claims.UserID,
		Limit:    limit,
		Offset:   offset,
		Status:   status,
		Priority: priority,
	}

	tickets, err := h.ticketService.ListTickets(r.Context(), params)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve tickets"})
		return
	}

	WriteJSON(w, http.StatusOK, tickets)
}

func (h *TicketHandler) HandleCreateTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(mw.UserClaimsKey).(*auth.Claims)
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Not authorized"})
		return
	}

	var req CreateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
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
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to create ticket"})
		return
	}

	WriteJSON(w, http.StatusCreated, ticket)
}

func (h *TicketHandler) HandleGetTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(mw.UserClaimsKey).(*auth.Claims)
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Not authorized"})
		return
	}

	ticketIDStr := chi.URLParam(r, "ticketID")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid ticket ID"})
		return
	}

	ticket, err := h.ticketService.GetTicket(r.Context(), ticketID, claims.UserID)
	if err != nil {
		if errors.Is(err, ports.ErrTicketNotFound) {
			WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Ticket not found"})
		} else if errors.Is(err, ports.ErrForbidden) {
			WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "Access denied"})
		} else {
			WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "An unexpected error occurred"})
		}
		return
	}

	WriteJSON(w, http.StatusOK, ticket)
}

func (h *TicketHandler) HandleUpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(mw.UserClaimsKey).(*auth.Claims)
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Not authorized"})
		return
	}

	ticketID, err := strconv.ParseInt(chi.URLParam(r, "ticketID"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid ticket ID"})
		return
	}

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	params := ports.UpdateStatusParams{
		TicketID: ticketID,
		Status:   domain.TicketStatus(req.Status),
		ActorID:  claims.UserID,
	}

	ticket, err := h.ticketService.UpdateStatus(r.Context(), params)
	if err != nil {
		if errors.Is(err, ports.ErrTicketNotFound) {
			WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Ticket not found"})
		} else if errors.Is(err, domain.ErrInvalidStatusTransition) {
			WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		} else {
			WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to update ticket status"})
		}
		return
	}

	WriteJSON(w, http.StatusOK, ticket)
}

func (h *TicketHandler) HandleAssignTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(mw.UserClaimsKey).(*auth.Claims)
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Not authorized"})
		return
	}

	ticketID, err := strconv.ParseInt(chi.URLParam(r, "ticketID"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid ticket ID"})
		return
	}

	var req AssignTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	assigneeID, err := uuid.Parse(req.AssigneeID)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid assignee ID format"})
		return
	}

	params := ports.AssignTicketParams{
		TicketID:   ticketID,
		AssigneeID: assigneeID,
		ActorID:    claims.UserID,
	}

	ticket, err := h.ticketService.AssignTicket(r.Context(), params)
	if err != nil {
		if errors.Is(err, ports.ErrTicketNotFound) {
			WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Ticket not found"})
		} else {
			WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to assign ticket"})
		}
		return
	}

	WriteJSON(w, http.StatusOK, ticket)
}

// getIntQueryParam safely parses an integer query parameter or returns a default value.
func getIntQueryParam(r *http.Request, key string, defaultValue int) int {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultValue
	}
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	if valInt < 0 {
		return defaultValue
	}
	return valInt
}

// getStringQueryParam safely parses a string query parameter, returning nil if it's empty.
func getStringQueryParam(r *http.Request, key string) *string {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return nil
	}
	return &valStr
}
