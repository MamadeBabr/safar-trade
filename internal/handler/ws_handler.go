package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/hossen/safar-trade/internal/service"
	"github.com/hossen/safar-trade/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allows cross-origin browser WebSocket connections
		return true
	},
}

// WSHandler manages WebSocket upgrade requests and subscription authentication.
type WSHandler struct {
	hub         *ws.Hub
	authService service.AuthService
	subService  service.SubscriptionService
	logger      *slog.Logger
}

// NewWSHandler creates a new WebSocket handler instance.
func NewWSHandler(
	hub *ws.Hub,
	authService service.AuthService,
	subService service.SubscriptionService,
	logger *slog.Logger,
) *WSHandler {
	return &WSHandler{
		hub:         hub,
		authService: authService,
		subService:  subService,
		logger:      logger,
	}
}

// ServeHTTP handles the WebSocket handshake, JWT validation, and subscription status check.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Extract JWT token from query parameter "?token=" or "Authorization" header
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			tokenStr = parts[1]
		}
	}

	if tokenStr == "" {
		WriteError(w, http.StatusUnauthorized, "missing authentication token: provide ?token=<jwt> or Authorization header", nil)
		return
	}

	// 2. Validate JWT token
	claims, err := h.authService.ValidateAccessToken(tokenStr)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid or expired access token", nil)
		return
	}

	// 3. Verify active subscription (cached in Redis with PostgreSQL fallback)
	hasActive, err := h.subService.HasActiveSubscription(r.Context(), claims.UserID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to verify subscription status", nil)
		return
	}

	if !hasActive {
		WriteError(w, http.StatusForbidden, "active subscription required to connect to realtime stream", map[string]string{
			"reason": "subscription_required",
			"action": "please subscribe to an active plan at /api/v1/packages",
		})
		return
	}

	// 4. Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket connection", slog.Any("error", err), slog.Int64("user_id", claims.UserID))
		return
	}

	// 5. Register client with Hub and launch read/write worker pumps
	client := ws.NewClient(h.hub, conn, claims.UserID, h.logger)
	h.hub.Register(client)

	// Send initial welcome status event
	client.SendEvent(ws.NewServerEvent(
		"status",
		"",
		map[string]any{
			"user_id":   claims.UserID,
			"role":      claims.Role,
			"protocols": []string{"subscribe", "unsubscribe", "ping"},
			"topics":    []string{"candles:<symbol>:<timeframe>", "alerts"},
		},
		"connected to safar-trade realtime streaming service",
	))

	go client.WritePump()
	go client.ReadPump()
}

// RegisterRoutes attaches the WebSocket endpoint to the HTTP multiplexer.
func (h *WSHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/ws", h)
}
