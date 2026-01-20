package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType defines the type of ticket event.
type EventType string

const (
	EventCommentAdded   EventType = "COMMENT_ADDED"
	EventStatusUpdated  EventType = "STATUS_UPDATED"
	EventTicketCreated  EventType = "TICKET_CREATED"
	EventTicketAssigned EventType = "TICKET_ASSIGNED"
)

// Event represents a persisted ticket event.
type Event struct {
	ID        int64           `json:"id"`
	TicketID  int64           `json:"ticketId"`
	Type      EventType       `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ActorID   uuid.UUID       `json:"actorId"`
	CreatedAt time.Time       `json:"createdAt"`
}
