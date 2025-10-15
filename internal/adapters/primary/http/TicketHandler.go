package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type TicketHandler struct {
	ticketService ports.TicketService
}

func NewTicketHandler(ticketService ports.TicketService) *TicketHandler {
	return &TicketHandler{
		ticketService: ticketService,
	}
}

// RegisterRoutes sets up the routing for the ticket endpoints using chi.Router.
func (h *TicketHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.HandleCreateTicket)
	r.Get("/{ticketID}", h.HandleGetTicket)
}

// CreateTicketRequest defines the expected JSON body for creating a ticket.
type CreateTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

// HandleCreateTicket processes new ticket creation requests.
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

// HandleGetTicket processes requests to retrieve a single ticket.
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
		// Here you could check for specific errors like ErrTicketNotFound or ErrForbidden
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Ticket not found or access denied"})
		return
	}

	WriteJSON(w, http.StatusOK, ticket)
}
