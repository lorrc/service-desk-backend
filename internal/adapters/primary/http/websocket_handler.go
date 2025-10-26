package http

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	wsAdapter "github.com/lorrc/service-desk-backend/internal/adapters/primary/websocket" // <-- Alias websocket package
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain" // <-- Need domain import
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

type WebSocketHandler struct {
	hub *wsAdapter.Hub // Use alias
	tm  *auth.TokenManager
}

func NewWebSocketHandler(hub *wsAdapter.Hub, tm *auth.TokenManager) *WebSocketHandler { // Use alias
	return &WebSocketHandler{
		hub: hub,
		tm:  tm,
	}
}

// ServeHTTP handles WebSocket connection requests.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate the connection
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		http.Error(w, "Missing authentication token", http.StatusUnauthorized)
		return
	}

	claims, err := h.tm.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// 2. Upgrade the connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket connection: %v", err)
		return
	}

	// 3. Create and register the new client
	// ** FIX IS HERE: Use capitalized field names **
	client := &wsAdapter.Client{ // Use alias
		Hub:           h.hub,
		Conn:          conn,
		Send:          make(chan domain.Event, 256),
		UserID:        claims.UserID,
		Subscriptions: make(map[int64]bool),
	}
	client.Hub.Register <- client // Use capitalized Hub

	// 4. Start the I/O pumps in new goroutines
	go client.WritePump()
	go client.ReadPump()
}
