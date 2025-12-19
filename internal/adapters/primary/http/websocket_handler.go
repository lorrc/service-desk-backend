package http

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	wsAdapter "github.com/lorrc/service-desk-backend/internal/adapters/primary/websocket"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/config"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

// WebSocketHandler handles WebSocket connection upgrades
type WebSocketHandler struct {
	hub      *wsAdapter.Hub
	tm       *auth.TokenManager
	upgrader websocket.Upgrader
	logger   *slog.Logger
}

// WebSocketConfig holds configuration for the WebSocket handler
type WebSocketConfig struct {
	AllowedOrigins  []string
	ReadBufferSize  int
	WriteBufferSize int
	IsDevelopment   bool
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	hub *wsAdapter.Hub,
	tm *auth.TokenManager,
	cfg *config.Config,
	logger *slog.Logger,
) *WebSocketHandler {
	handler := &WebSocketHandler{
		hub:    hub,
		tm:     tm,
		logger: logger,
	}

	handler.upgrader = websocket.Upgrader{
		ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
		WriteBufferSize: cfg.WebSocket.WriteBufferSize,
		CheckOrigin:     handler.makeOriginChecker(cfg),
	}

	return handler
}

// makeOriginChecker creates an origin checking function based on configuration
func (h *WebSocketHandler) makeOriginChecker(cfg *config.Config) func(r *http.Request) bool {
	allowedOrigins := cfg.WebSocket.AllowedOrigins

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// In development mode, allow all origins (but log a warning)
		if cfg.IsDevelopment() {
			if origin != "" {
				h.logger.Warn("allowing websocket connection in development mode",
					"origin", origin,
					"remote_addr", r.RemoteAddr,
				)
			}
			return true
		}

		// No origin header (same-origin request or non-browser client)
		if origin == "" {
			return true
		}

		// Check against allowed origins
		parsedOrigin, err := url.Parse(origin)
		if err != nil {
			h.logger.Warn("failed to parse websocket origin",
				"origin", origin,
				"error", err,
			)
			return false
		}

		originHost := parsedOrigin.Host

		for _, allowed := range allowedOrigins {
			// Support wildcard subdomains like "*.example.com"
			if strings.HasPrefix(allowed, "*.") {
				suffix := allowed[1:] // Remove the "*", keep ".example.com"
				if strings.HasSuffix(originHost, suffix) || originHost == allowed[2:] {
					return true
				}
			} else if originHost == allowed {
				return true
			}
		}

		h.logger.Warn("websocket connection rejected due to origin",
			"origin", origin,
			"remote_addr", r.RemoteAddr,
			"allowed_origins", allowedOrigins,
		)
		return false
	}
}

// ServeHTTP handles WebSocket connection requests
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := GetRequestID(r.Context())

	// 1. Authenticate the connection via query parameter
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		h.logger.Warn("websocket connection rejected: missing token",
			"request_id", requestID,
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "Missing authentication token", http.StatusUnauthorized)
		return
	}

	claims, err := h.tm.ValidateToken(tokenString)
	if err != nil {
		h.logger.Warn("websocket connection rejected: invalid token",
			"request_id", requestID,
			"remote_addr", r.RemoteAddr,
			"error", err,
		)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// 2. Upgrade the connection
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket connection",
			"request_id", requestID,
			"user_id", claims.UserID,
			"error", err,
		)
		return
	}

	h.logger.Info("websocket connection established",
		"request_id", requestID,
		"user_id", claims.UserID,
		"remote_addr", r.RemoteAddr,
	)

	// 3. Create and register the new client
	client := &wsAdapter.Client{
		Hub:           h.hub,
		Conn:          conn,
		Send:          make(chan domain.Event, 256),
		UserID:        claims.UserID,
		Subscriptions: make(map[int64]bool),
	}
	client.Hub.Register <- client

	// 4. Start the I/O pumps in new goroutines
	go client.WritePump()
	go client.ReadPump()
}
