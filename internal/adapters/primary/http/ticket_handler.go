package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	maxTicketsPerPage  = 100
	defaultEventsLimit = 50
	maxEventsLimit     = 200
)

// TicketHandler handles HTTP requests for tickets
type TicketHandler struct {
	ticketService  ports.TicketService
	eventService   ports.EventService
	userLookup     ports.UserLookupService
	commentHandler *CommentHandler
	errorHandler   *ErrorHandler
	logger         *slog.Logger
}

// NewTicketHandler creates a new ticket handler
func NewTicketHandler(
	ticketService ports.TicketService,
	eventService ports.EventService,
	userLookup ports.UserLookupService,
	commentHandler *CommentHandler,
	errorHandler *ErrorHandler,
	logger *slog.Logger,
) *TicketHandler {
	return &TicketHandler{
		ticketService:  ticketService,
		eventService:   eventService,
		userLookup:     userLookup,
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
	r.Get("/counts", h.HandleTicketCounts)

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
	ID          int64        `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	Priority    string       `json:"priority"`
	RequesterID string       `json:"requesterId"`
	Requester   *UserInfoDTO `json:"requester,omitempty"`
	AssigneeID  *string      `json:"assigneeId"`
	Assignee    *UserInfoDTO `json:"assignee,omitempty"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   *string      `json:"updatedAt"`
	ClosedAt    *string      `json:"closedAt"`
}

func toTicketDTO(ticket *domain.Ticket, userInfoByID map[uuid.UUID]UserInfoDTO) TicketDTO {
	var assigneeID *string
	if ticket.AssigneeID != nil {
		value := ticket.AssigneeID.String()
		assigneeID = &value
	}

	var requester *UserInfoDTO
	if userInfo, ok := userInfoByID[ticket.RequesterID]; ok {
		value := userInfo
		requester = &value
	}

	var assignee *UserInfoDTO
	if ticket.AssigneeID != nil {
		if userInfo, ok := userInfoByID[*ticket.AssigneeID]; ok {
			value := userInfo
			assignee = &value
		}
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
		Requester:   requester,
		AssigneeID:  assigneeID,
		Assignee:    assignee,
		CreatedAt:   ticket.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   updatedAt,
		ClosedAt:    closedAt,
	}
}

func toTicketDTOs(tickets []*domain.Ticket, userInfoByID map[uuid.UUID]UserInfoDTO) []TicketDTO {
	response := make([]TicketDTO, 0, len(tickets))
	for _, ticket := range tickets {
		response = append(response, toTicketDTO(ticket, userInfoByID))
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

	query, createdFromTime, createdToTime, err := parseTicketBaseline(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	active, err := parseOptionalBool(r, "active")
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	if status != nil {
		v.OneOf("status", *status, []string{"OPEN", "IN_PROGRESS", "CLOSED"})
	}
	if priority != nil {
		v.OneOf("priority", *priority, []string{"LOW", "MEDIUM", "HIGH"})
	}

	if unassigned {
		assigneeID = nil
	}

	if v.HasErrors() {
		h.errorHandler.Handle(w, r, v.Errors())
		return
	}

	params := ports.ListTicketsParams{
		Query: query, Active: active != nil && *active,
		OrganizationID: claims.OrgID,
		ViewerID:       claims.UserID,
		Limit:          pagination.Limit,
		Offset:         pagination.Offset,
		Status:         status,
		Priority:       priority,
		AssigneeID:     assigneeID,
		Unassigned:     unassigned,
		CreatedFrom:    createdFromTime,
		CreatedTo:      createdToTime,
	}

	tickets, err := h.ticketService.ListTickets(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	userInfoByID, err := buildUserInfoDTOMap(
		r.Context(),
		h.userLookup,
		claims.OrgID,
		collectTicketUserIDs(tickets),
	)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	// Use simple pagination (without total count for performance)
	WritePaginatedSimple(w, toTicketDTOs(tickets, userInfoByID), pagination.Limit, pagination.Offset)
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
		OrganizationID: claims.OrgID,
		Title:          req.Title,
		Description:    req.Description,
		Priority:       domain.TicketPriority(req.Priority),
		RequesterID:    claims.UserID,
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

	userInfoByID, err := buildUserInfoDTOMap(
		r.Context(),
		h.userLookup,
		claims.OrgID,
		collectTicketUserIDs([]*domain.Ticket{ticket}),
	)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteCreated(w, toTicketDTO(ticket, userInfoByID))
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

	ticket, err := h.ticketService.GetTicket(r.Context(), ticketID, claims.UserID, claims.OrgID)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	userInfoByID, err := buildUserInfoDTOMap(
		r.Context(),
		h.userLookup,
		claims.OrgID,
		collectTicketUserIDs([]*domain.Ticket{ticket}),
	)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, toTicketDTO(ticket, userInfoByID))
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
		OrganizationID: claims.OrgID,
		TicketID:       ticketID,
		Status:         domain.TicketStatus(req.Status),
		ActorID:        claims.UserID,
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

	userInfoByID, err := buildUserInfoDTOMap(
		r.Context(),
		h.userLookup,
		claims.OrgID,
		collectTicketUserIDs([]*domain.Ticket{ticket}),
	)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, toTicketDTO(ticket, userInfoByID))
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
		OrganizationID: claims.OrgID,
		TicketID:       ticketID,
		AssigneeID:     assigneeID,
		ActorID:        claims.UserID,
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

	userInfoByID, err := buildUserInfoDTOMap(
		r.Context(),
		h.userLookup,
		claims.OrgID,
		collectTicketUserIDs([]*domain.Ticket{ticket}),
	)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, toTicketDTO(ticket, userInfoByID))
}

// TicketEventDTO enriches factual snapshots with organization-scoped display identities.
type TicketEventDTO struct {
	*domain.Event
	Actor    *UserInfoDTO `json:"actor,omitempty"`
	Assignee *UserInfoDTO `json:"assignee,omitempty"`
}

// TicketEventsResponse carries one bounded event page in the requested direction.
type TicketEventsResponse struct {
	Data       []TicketEventDTO `json:"data"`
	NextCursor *int64           `json:"nextCursor,omitempty"`
	HasMore    bool             `json:"hasMore"`
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

	direction := r.URL.Query().Get("direction")
	backward := direction == "backward"
	v := validation.NewValidator()
	if direction != "" {
		v.OneOf("direction", direction, []string{"forward", "backward"})
	}
	var beforeID int64
	if before := r.URL.Query().Get("before"); before != "" {
		parsed, parseErr := strconv.ParseInt(before, 10, 64)
		if parseErr != nil || parsed < 1 || !backward {
			v.Custom("before", false, "before requires backward direction and a positive cursor")
		} else {
			beforeID = parsed
		}
	}
	if backward && r.URL.Query().Has("after") {
		v.Custom("after", false, "after cannot be used for backward history")
	}
	if v.HasErrors() {
		h.errorHandler.Handle(w, r, v.Errors())
		return
	}

	params := ports.ListTicketEventsParams{
		OrganizationID: claims.OrgID,
		TicketID:       ticketID,
		ViewerID:       claims.UserID,
		AfterID:        afterID, BeforeID: beforeID, Backward: backward,
		Limit: limit + 1,
	}

	events, err := h.eventService.ListTicketEvents(r.Context(), params)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	ids := make([]uuid.UUID, 0, len(events)*2)
	assigneeIDs := make(map[int64]uuid.UUID)
	for _, event := range events {
		ids = append(ids, event.ActorID)
		if event.Type != domain.EventCommentAdded {
			var snapshot struct {
				AssigneeID *uuid.UUID `json:"assigneeId"`
			}
			if json.Unmarshal(event.Payload, &snapshot) == nil && snapshot.AssigneeID != nil {
				ids = append(ids, *snapshot.AssigneeID)
				assigneeIDs[event.ID] = *snapshot.AssigneeID
			}
		}
	}
	people, err := buildUserInfoDTOMap(r.Context(), h.userLookup, claims.OrgID, ids)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	data := make([]TicketEventDTO, 0, len(events))
	for _, event := range events {
		entry := TicketEventDTO{Event: event}
		if person, ok := people[event.ActorID]; ok {
			entry.Actor = &person
		}
		if id, ok := assigneeIDs[event.ID]; ok {
			if person, found := people[id]; found {
				entry.Assignee = &person
			}
		}
		data = append(data, entry)
	}
	var nextCursor *int64
	if len(events) > 0 {
		cursor := events[len(events)-1].ID
		nextCursor = &cursor
	}
	WriteJSON(w, http.StatusOK, TicketEventsResponse{Data: data, NextCursor: nextCursor, HasMore: hasMore})
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

func collectTicketUserIDs(tickets []*domain.Ticket) []uuid.UUID {
	userIDs := make([]uuid.UUID, 0, len(tickets)*2)
	for _, ticket := range tickets {
		userIDs = append(userIDs, ticket.RequesterID)
		if ticket.AssigneeID != nil {
			userIDs = append(userIDs, *ticket.AssigneeID)
		}
	}
	return userIDs
}

// HandleTicketCounts returns aggregates for the authenticated viewer's ticket scope.
func (h *TicketHandler) HandleTicketCounts(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}
	query, from, to, err := parseTicketBaseline(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	counts, err := h.ticketService.GetTicketCounts(r.Context(), ports.TicketCountsParams{ViewerID: claims.UserID, OrganizationID: claims.OrgID, Query: query, CreatedFrom: from, CreatedTo: to})
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, counts)
}

// parseTicketBaseline is shared by ticket lists and every triage count.
func parseTicketBaseline(r *http.Request) (string, *time.Time, *time.Time, error) {
	v := validation.NewValidator()
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	v.MaxLength("q", query, 200)
	from, err := validation.ParseTimeQueryParam(r, "createdFrom")
	if err != nil {
		v.Custom("createdFrom", false, "Must be a valid date or timestamp")
	}
	to, err := validation.ParseTimeQueryParam(r, "createdTo")
	if err != nil {
		v.Custom("createdTo", false, "Must be a valid date or timestamp")
	}
	var start, end *time.Time
	if from != nil {
		start = &from.Time
	}
	if to != nil {
		value := to.Time
		if to.DateOnly {
			value = value.AddDate(0, 0, 1)
		}
		end = &value
	}
	if start != nil && end != nil && !start.Before(*end) {
		v.Custom("createdFrom", false, "Must be before createdTo")
	}
	if v.HasErrors() {
		return "", nil, nil, v.Errors()
	}
	return query, start, end, nil
}

func parseOptionalBool(r *http.Request, name string) (*bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	if raw != "true" && raw != "false" {
		v := validation.NewValidator()
		v.Custom(name, false, "Must be true or false")
		return nil, v.Errors()
	}
	value := raw == "true"
	return &value, nil
}
