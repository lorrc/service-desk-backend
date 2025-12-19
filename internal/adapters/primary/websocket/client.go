package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1024
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	Hub *Hub

	// The websocket connection.
	Conn *websocket.Conn

	// Buffered channel of outbound messages.
	Send chan domain.Event

	// User ID for this client.
	UserID uuid.UUID

	// Subscriptions maps ticket IDs to true.
	Subscriptions map[int64]bool

	// closeOnce ensures the Send channel is only closed once
	closeOnce sync.Once

	// mu protects Subscriptions map
	mu sync.RWMutex

	// logger for this client
	logger *slog.Logger
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, logger *slog.Logger) *Client {
	return &Client{
		Hub:           hub,
		Conn:          conn,
		Send:          make(chan domain.Event, 256),
		UserID:        userID,
		Subscriptions: make(map[int64]bool),
		logger:        logger.With("user_id", userID.String()),
	}
}

// CloseSend safely closes the Send channel exactly once
func (c *Client) CloseSend() {
	c.closeOnce.Do(func() {
		close(c.Send)
	})
}

// AddSubscription adds a subscription to a ticket
func (c *Client) AddSubscription(ticketID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Subscriptions[ticketID] = true
}

// RemoveSubscription removes a subscription from a ticket
func (c *Client) RemoveSubscription(ticketID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Subscriptions, ticketID)
}

// HasSubscription checks if the client is subscribed to a ticket
func (c *Client) HasSubscription(ticketID int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Subscriptions[ticketID]
}

// GetSubscriptions returns a copy of all subscriptions
func (c *Client) GetSubscriptions() []int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := make([]int64, 0, len(c.Subscriptions))
	for ticketID := range c.Subscriptions {
		subs = append(subs, ticketID)
	}
	return subs
}

// ReadPump pumps messages from the websocket connection to the hub.
// This method runs in its own goroutine.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		_ = c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		c.logger.Error("failed to set read deadline", "error", err)
		return
	}

	c.Conn.SetPongHandler(func(string) error {
		if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			c.logger.Error("failed to set read deadline in pong handler", "error", err)
		}
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				c.logger.Warn("websocket read error", "error", err)
			}
			break
		}

		c.handleIncomingMessage(message)
	}
}

// WritePump pumps messages from the hub to the websocket connection.
// This method runs in its own goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.Send:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.logger.Error("failed to set write deadline", "error", err)
				return
			}

			if !ok {
				// The hub closed the channel. Send close message.
				if err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					c.logger.Debug("failed to send close message", "error", err)
				}
				return
			}

			if err := c.writeJSON(event); err != nil {
				c.logger.Error("failed to write message", "error", err)
				return
			}

		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.logger.Error("failed to set write deadline for ping", "error", err)
				return
			}

			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Debug("failed to send ping", "error", err)
				return
			}
		}
	}
}

// writeJSON writes a JSON message to the websocket connection
func (c *Client) writeJSON(event domain.Event) error {
	w, err := c.Conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(w).Encode(event); err != nil {
		_ = w.Close()
		return err
	}

	return w.Close()
}

// --- Incoming Message Handling ---

// ClientMessage is the structure for messages sent from the client.
type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// SubscribePayload is the payload for subscribe/unsubscribe messages
type SubscribePayload struct {
	TicketID int64 `json:"ticketId"`
}

// handleIncomingMessage processes messages received from the client
func (c *Client) handleIncomingMessage(message []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		c.logger.Warn("failed to unmarshal client message", "error", err)
		return
	}

	switch msg.Type {
	case "SUBSCRIBE_TO_TICKET":
		c.handleSubscribe(msg.Payload)

	case "UNSUBSCRIBE_FROM_TICKET":
		c.handleUnsubscribe(msg.Payload)

	case "PING":
		// Client-side keep-alive, respond with pong
		c.sendPong()

	default:
		c.logger.Debug("received unknown message type", "type", msg.Type)
	}
}

func (c *Client) handleSubscribe(payload json.RawMessage) {
	var p SubscribePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		c.logger.Warn("failed to unmarshal subscribe payload", "error", err)
		return
	}

	if p.TicketID <= 0 {
		c.logger.Warn("invalid ticket ID in subscribe request", "ticket_id", p.TicketID)
		return
	}

	// TODO: Add authorization check - verify user has access to this ticket
	c.Hub.subscribeClientToTicket(c, p.TicketID)
}

func (c *Client) handleUnsubscribe(payload json.RawMessage) {
	var p SubscribePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		c.logger.Warn("failed to unmarshal unsubscribe payload", "error", err)
		return
	}

	c.Hub.unsubscribeClientFromTicket(c, p.TicketID)
}

func (c *Client) sendPong() {
	select {
	case c.Send <- domain.Event{Type: "PONG"}:
	default:
		// Channel full, skip pong response
	}
}
