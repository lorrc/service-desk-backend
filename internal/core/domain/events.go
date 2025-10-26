package domain

// EventType defines the type of real-time event.
type EventType string

const (
	EventCommentAdded  EventType = "COMMENT_ADDED"
	EventStatusUpdated EventType = "STATUS_UPDATED"
)

// Event is the payload sent over WebSocket.
type Event struct {
	Type     EventType   `json:"type"`
	Payload  interface{} `json:"payload"`
	TicketID int64       `json:"ticketId"` // Used for routing to specific ticket "rooms"
}
