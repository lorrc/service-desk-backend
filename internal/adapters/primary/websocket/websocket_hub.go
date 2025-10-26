package websocket

import (
	"log"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// Hub maintains the set of active Clients and broadcasts messages to them.
type Hub struct {
	Clients    map[uuid.UUID]map[*Client]bool
	Rooms      map[int64]map[*Client]bool
	Hbroadcast chan domain.Event // Changed field name
	Register   chan *Client
	Unregister chan *Client
}

// Ensure Hub implements the EventBroadcaster interface.
var _ ports.EventBroadcaster = (*Hub)(nil)

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[uuid.UUID]map[*Client]bool),
		Rooms:      make(map[int64]map[*Client]bool),
		Hbroadcast: make(chan domain.Event, 256), // Use a buffer
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// Broadcast sends an event to the hub's internal broadcast channel.
// This method implements the ports.EventBroadcaster interface.
// ** FIX: Added 'error' return type (returning nil) **
func (h *Hub) Broadcast(event domain.Event) error {
	select {
	case h.Hbroadcast <- event: // Use the renamed field
		return nil // Successfully queued
	default:
		// Channel is full, log and potentially return an error or drop
		log.Printf("WARNING: Hub broadcast channel full. Dropping event: %+v", event)
		// Depending on requirements, you might return an error here:
		// return errors.New("broadcast channel is full")
		return nil // For now, just log and continue
	}
}

// Run starts the hub's event loop. This MUST be run as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			if h.Clients[client.UserID] == nil {
				h.Clients[client.UserID] = make(map[*Client]bool)
			}
			h.Clients[client.UserID][client] = true
			log.Printf("Client registered: %s", client.UserID)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case event := <-h.Hbroadcast: // Use the renamed field
			log.Printf("Broadcasting event %s for ticket %d", event.Type, event.TicketID)
			room, ok := h.Rooms[event.TicketID]
			if !ok {
				// log.Println("...no Clients in room.") // Can be noisy, optionally remove
				continue
			}

			for client := range room {
				select {
				case client.Send <- event: // Use capitalized Send
				default:
					// If the client's send buffer is full, assume they are slow/stuck
					log.Printf("Client %s send buffer full, unregistering.", client.UserID)
					h.unregisterClient(client) // Unregister the slow client
				}
			}
		}
	}
}

// ... (rest of the Hub methods: unregisterClient, subscribeClientToTicket, unsubscribeClientFromTicket remain the same) ...
func (h *Hub) unregisterClient(client *Client) {
	// 1. Remove from the global user map
	if userClients, ok := h.Clients[client.UserID]; ok {
		if _, ok := userClients[client]; ok {
			delete(userClients, client)
			if len(userClients) == 0 {
				delete(h.Clients, client.UserID)
			}
		}
	}

	// 2. Remove from all subscribed rooms
	for ticketID := range client.Subscriptions {
		if room, ok := h.Rooms[ticketID]; ok {
			if _, ok := room[client]; ok {
				delete(room, client)
				if len(room) == 0 {
					delete(h.Rooms, ticketID)
				}
			}
		}
	}

	// 3. Close the send channel
	// Check if channel is already closed to prevent panic
	select {
	case <-client.Send:
		// Channel is already closed or empty
	default:
		close(client.Send)
	}
	log.Printf("Client unregistered: %s", client.UserID)
}

func (h *Hub) subscribeClientToTicket(client *Client, ticketID int64) {
	if h.Rooms[ticketID] == nil {
		h.Rooms[ticketID] = make(map[*Client]bool)
	}
	h.Rooms[ticketID][client] = true
	client.Subscriptions[ticketID] = true
	log.Printf("Client %s subscribed to ticket %d", client.UserID, ticketID)
}

func (h *Hub) unsubscribeClientFromTicket(client *Client, ticketID int64) {
	if room, ok := h.Rooms[ticketID]; ok {
		if _, ok := room[client]; ok {
			delete(room, client)
			if len(room) == 0 {
				delete(h.Rooms, ticketID)
			}
		}
	}
	delete(client.Subscriptions, ticketID)
	log.Printf("Client %s unsubscribed from ticket %d", client.UserID, ticketID)
}
