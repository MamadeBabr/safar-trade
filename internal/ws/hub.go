package ws

import (
	"context"
	"log/slog"
	"sync"
)

type topicAction struct {
	client *Client
	topic  string
}

type broadcastMessage struct {
	topic string
	event ServerEvent
}

// Hub maintains the set of active clients, subscriptions to topics, and coordinates broadcasts.
type Hub struct {
	logger *slog.Logger

	mu          sync.RWMutex
	clients     map[*Client]bool
	topics      map[string]map[*Client]bool
	userClients map[int64]map[*Client]bool

	register   chan *Client
	unregister chan *Client
	subChan    chan topicAction
	unsubChan  chan topicAction
	broadcast  chan broadcastMessage
}

// NewHub creates a new WebSocket Hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		logger:      logger,
		clients:     make(map[*Client]bool),
		topics:      make(map[string]map[*Client]bool),
		userClients: make(map[int64]map[*Client]bool),
		register:    make(chan *Client, 128),
		unregister:  make(chan *Client, 128),
		subChan:     make(chan topicAction, 256),
		unsubChan:   make(chan topicAction, 256),
		broadcast:   make(chan broadcastMessage, 1024),
	}
}

// Run executes the central event routing loop until the context is canceled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("shutting down websocket hub")
			h.closeAll()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.userID > 0 {
				if h.userClients[client.userID] == nil {
					h.userClients[client.userID] = make(map[*Client]bool)
				}
				h.userClients[client.userID][client] = true
			}
			h.mu.Unlock()
			h.logger.Debug("websocket client registered", slog.Int64("user_id", client.userID))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)

				// Remove from user index
				if client.userID > 0 && h.userClients[client.userID] != nil {
					delete(h.userClients[client.userID], client)
					if len(h.userClients[client.userID]) == 0 {
						delete(h.userClients, client.userID)
					}
				}

				// Remove from all topics
				for topic := range client.topics {
					if clients, exists := h.topics[topic]; exists {
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.topics, topic)
						}
					}
				}
			}
			h.mu.Unlock()
			h.logger.Debug("websocket client unregistered", slog.Int64("user_id", client.userID))

		case action := <-h.subChan:
			h.mu.Lock()
			if h.topics[action.topic] == nil {
				h.topics[action.topic] = make(map[*Client]bool)
			}
			h.topics[action.topic][action.client] = true
			h.mu.Unlock()

		case action := <-h.unsubChan:
			h.mu.Lock()
			if clients, ok := h.topics[action.topic]; ok {
				delete(clients, action.client)
				if len(clients) == 0 {
					delete(h.topics, action.topic)
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			// Find all clients subscribed to this exact topic
			if clients, ok := h.topics[msg.topic]; ok {
				for client := range clients {
					client.SendEvent(msg.event)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a newly connected client to the Hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a disconnected client from the Hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Subscribe attaches a client to a specific topic channel.
func (h *Hub) Subscribe(client *Client, topic string) {
	h.subChan <- topicAction{client: client, topic: topic}
}

// Unsubscribe detaches a client from a specific topic channel.
func (h *Hub) Unsubscribe(client *Client, topic string) {
	h.unsubChan <- topicAction{client: client, topic: topic}
}

// Broadcast sends an event to all clients subscribed to `topic`. Non-blocking.
func (h *Hub) Broadcast(topic string, data any) {
	event := NewServerEvent("message", topic, data, "")
	select {
	case h.broadcast <- broadcastMessage{topic: topic, event: event}:
	default:
		// Drop message if broadcast channel is congested
		h.logger.Warn("hub broadcast channel full, dropping message", slog.String("topic", topic))
	}
}

// BroadcastCandle sends a formatted candlestick tick to `candles:<symbol>:<timeframe>` subscribers.
func (h *Hub) BroadcastCandle(symbol, timeframe string, candle any) {
	topic := "candles:" + symbol + ":" + timeframe
	event := NewServerEvent("candle", topic, candle, "")
	select {
	case h.broadcast <- broadcastMessage{topic: topic, event: event}:
	default:
		h.logger.Warn("hub candle broadcast dropped due to backpressure", slog.String("topic", topic))
	}
}

// BroadcastAlert broadcasts a priority alert to all clients subscribed to the "alerts" topic.
func (h *Hub) BroadcastAlert(level, title, message string, metadata map[string]any) {
	topic := "alerts"
	alert := AlertPayload{
		Level:    level,
		Title:    title,
		Message:  message,
		Metadata: metadata,
	}
	event := NewServerEvent("alert", topic, alert, "")
	select {
	case h.broadcast <- broadcastMessage{topic: topic, event: event}:
	default:
		h.logger.Warn("hub alert broadcast dropped due to backpressure", slog.String("title", title))
	}
}

// SendToUser pushes a targeted message exclusively to all connections for a specific user.
func (h *Hub) SendToUser(userID int64, event ServerEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.userClients[userID]; ok {
		for client := range clients {
			client.SendEvent(event)
		}
	}
}

// ClientCount returns the total number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		close(client.send)
		_ = client.conn.Close()
		delete(h.clients, client)
	}
	h.topics = make(map[string]map[*Client]bool)
	h.userClients = make(map[int64]map[*Client]bool)
}
