package websocket

import (
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// Hub maintains the set of active Clients and broadcasts messages to them.
type Hub struct {
	// Clients maps user IDs to their active connections
	// A single user can have multiple connections (multiple tabs/devices)
	clients map[uuid.UUID]map[*Client]bool

	// Rooms maps ticket IDs to subscribed clients
	rooms map[int64]map[*Client]bool

	// Broadcast channel for events
	broadcast chan domain.Event

	// Register requests from clients
	Register chan *Client

	// Unregister requests from clients
	Unregister chan *Client

	// mu protects the clients and rooms maps
	mu sync.RWMutex

	// logger for the hub
	logger *slog.Logger
}

// Ensure Hub implements the EventBroadcaster interface.
var _ ports.EventBroadcaster = (*Hub)(nil)

// NewHub creates a new WebSocket hub
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		rooms:      make(map[int64]map[*Client]bool),
		broadcast:  make(chan domain.Event, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		logger:     logger.With("component", "websocket_hub"),
	}
}

// Broadcast sends an event to the hub's internal broadcast channel.
// This method implements the ports.EventBroadcaster interface.
func (h *Hub) Broadcast(event domain.Event) error {
	select {
	case h.broadcast <- event:
		return nil
	default:
		h.logger.Warn("broadcast channel full, dropping event",
			"event_type", event.Type,
			"ticket_id", event.TicketID,
		)
		return nil
	}
}

// Run starts the hub's event loop. This MUST be run as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case event := <-h.broadcast:
			h.broadcastEvent(event)
		}
	}
}

// registerClient adds a client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.UserID] == nil {
		h.clients[client.UserID] = make(map[*Client]bool)
	}
	h.clients[client.UserID][client] = true

	h.logger.Info("client registered",
		"user_id", client.UserID,
		"total_connections", len(h.clients[client.UserID]),
	)
}

// unregisterClient removes a client from the hub and all rooms
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Get subscriptions before removing from maps
	subscriptions := client.GetSubscriptions()

	// 1. Remove from the global user map
	if userClients, ok := h.clients[client.UserID]; ok {
		if _, exists := userClients[client]; exists {
			delete(userClients, client)
			if len(userClients) == 0 {
				delete(h.clients, client.UserID)
			}
		}
	}

	// 2. Remove from all subscribed rooms
	for _, ticketID := range subscriptions {
		if room, ok := h.rooms[ticketID]; ok {
			delete(room, client)
			if len(room) == 0 {
				delete(h.rooms, ticketID)
			}
		}
	}

	// 3. Safely close the send channel
	client.CloseSend()

	h.logger.Info("client unregistered",
		"user_id", client.UserID,
	)
}

// broadcastEvent sends an event to all clients subscribed to the ticket
func (h *Hub) broadcastEvent(event domain.Event) {
	h.mu.RLock()
	room, ok := h.rooms[event.TicketID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Copy the client list to avoid holding the lock while sending
	clients := make([]*Client, 0, len(room))
	for client := range room {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	h.logger.Debug("broadcasting event",
		"event_type", event.Type,
		"ticket_id", event.TicketID,
		"client_count", len(clients),
	)

	// Send to each client
	for _, client := range clients {
		select {
		case client.Send <- event:
			// Successfully queued
		default:
			// Client's send buffer is full, unregister them
			h.logger.Warn("client send buffer full, unregistering",
				"user_id", client.UserID,
			)
			h.Unregister <- client
		}
	}
}

// subscribeClientToTicket adds a client to a ticket's room
func (h *Hub) subscribeClientToTicket(client *Client, ticketID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[ticketID] == nil {
		h.rooms[ticketID] = make(map[*Client]bool)
	}
	h.rooms[ticketID][client] = true
	client.AddSubscription(ticketID)

	h.logger.Debug("client subscribed to ticket",
		"user_id", client.UserID,
		"ticket_id", ticketID,
	)
}

// unsubscribeClientFromTicket removes a client from a ticket's room
func (h *Hub) unsubscribeClientFromTicket(client *Client, ticketID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[ticketID]; ok {
		delete(room, client)
		if len(room) == 0 {
			delete(h.rooms, ticketID)
		}
	}
	client.RemoveSubscription(ticketID)

	h.logger.Debug("client unsubscribed from ticket",
		"user_id", client.UserID,
		"ticket_id", ticketID,
	)
}

// GetClientCount returns the total number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, userClients := range h.clients {
		count += len(userClients)
	}
	return count
}

// GetRoomCount returns the number of active rooms
func (h *Hub) GetRoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// GetClientsInRoom returns the number of clients subscribed to a ticket
func (h *Hub) GetClientsInRoom(ticketID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, ok := h.rooms[ticketID]; ok {
		return len(room)
	}
	return 0
}

// IsUserConnected checks if a user has any active connections
func (h *Hub) IsUserConnected(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[userID]
	return ok && len(clients) > 0
}

// SendToUser sends an event directly to a specific user (all their connections)
func (h *Hub) SendToUser(userID uuid.UUID, event domain.Event) {
	h.mu.RLock()
	clients, ok := h.clients[userID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Copy client list
	clientList := make([]*Client, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	h.mu.RUnlock()

	// Send to all user's connections
	for _, client := range clientList {
		select {
		case client.Send <- event:
		default:
			// Buffer full, skip this connection
		}
	}
}
