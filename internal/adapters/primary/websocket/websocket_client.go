package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
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
}

// ReadPump pumps messages from the websocket connection to the hub.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		// No need to explicitly close conn here, writePump defer handles it
		// _ = c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait)) // Ignore error on initial set
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait)) // Ignore error on pong
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			// Log unexpected close errors, otherwise just break (normal closure)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error for client %s: %v", c.UserID, err)
			}
			break // Exit loop on any error, including normal closure
		}

		// Handle incoming messages from the client
		c.handleIncomingMessage(message)
	}
}

// WritePump pumps messages from the hub to the websocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close() // Best effort close
	}()

	for {
		select {
		case event, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait)) // Ignore error on deadline set
			if !ok {
				// The hub closed the channel. Send close message.
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{}) // Best effort
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Error getting next writer for client %s: %v", c.UserID, err)
				return // Assume connection is broken
			}
			// ** FIX: Added error check for Encode **
			if err := json.NewEncoder(w).Encode(event); err != nil {
				log.Printf("Error encoding JSON for client %s: %v", c.UserID, err)
				// Don't return yet, try to close the writer
			}

			// ** FIX: Added error check for Close **
			if err := w.Close(); err != nil {
				log.Printf("Error closing writer for client %s: %v", c.UserID, err)
				return // Assume connection is broken
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait)) // Ignore error
			// ** FIX: Added error check for WriteMessage (Ping) **
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Error sending ping to client %s: %v", c.UserID, err)
				return // Assume connection is broken
			}
		}
	}
}

// --- Incoming Message Handling ---

// ClientMessage ... (ClientMessage, SubscribePayload, handleIncomingMessage remain the same) ...
// ClientMessage is the structure for messages sent *from* the client.
type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type SubscribePayload struct {
	TicketID int64 `json:"ticketId"`
}

func (c *Client) handleIncomingMessage(message []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error unmarshalling client message from %s: %v", c.UserID, err)
		return
	}

	switch msg.Type {
	case "SUBSCRIBE_TO_TICKET":
		var p SubscribePayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			log.Printf("Error unmarshalling SUBSCRIBE payload from %s: %v", c.UserID, err)
			return
		}
		// Basic validation could be added here (e.g., check if ticket exists)
		c.Hub.subscribeClientToTicket(c, p.TicketID)

	case "UNSUBSCRIBE_FROM_TICKET":
		var p SubscribePayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			log.Printf("Error unmarshalling UNSUBSCRIBE payload from %s: %v", c.UserID, err)
			return
		}
		c.Hub.unsubscribeClientFromTicket(c, p.TicketID)

		// Could add handling for other client messages here
	default:
		log.Printf("Received unknown message type '%s' from client %s", msg.Type, c.UserID)
	}
}
