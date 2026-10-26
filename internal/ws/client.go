package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8192
	sendBufferSize = 256
)

// Client represents a single connected WebSocket client.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID int64
	logger *slog.Logger

	mu     sync.RWMutex
	topics map[string]bool
}

// NewClient constructs a new Client connection wrapper.
func NewClient(hub *Hub, conn *websocket.Conn, userID int64, logger *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, sendBufferSize),
		userID: userID,
		logger: logger,
		topics: make(map[string]bool),
	}
}

// SubscribedTopics returns a copy of all topics currently subscribed to by this client.
func (c *Client) SubscribedTopics() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	topics := make([]string, 0, len(c.topics))
	for t := range c.topics {
		topics = append(topics, t)
	}
	return topics
}

// HasTopic returns true if the client is subscribed to the given topic.
func (c *Client) HasTopic(topic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.topics[topic]
}

// addTopic tracks a topic subscription internally.
func (c *Client) addTopic(topic string) {
	c.mu.Lock()
	c.topics[topic] = true
	c.mu.Unlock()
}

// removeTopic removes a topic subscription internally.
func (c *Client) removeTopic(topic string) {
	c.mu.Lock()
	delete(c.topics, topic)
	c.mu.Unlock()
}

// ReadPump handles incoming WebSocket frames from the client.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Debug("websocket read error", slog.Any("error", err), slog.Int64("user_id", c.userID))
			}
			break
		}

		var cmd ClientCommand
		if err := json.Unmarshal(payload, &cmd); err != nil {
			c.SendEvent(NewServerEvent("error", "", nil, "malformed JSON command"))
			continue
		}

		switch cmd.Action {
		case "subscribe":
			for _, topic := range cmd.Topics {
				if topic != "" {
					c.hub.Subscribe(c, topic)
					c.addTopic(topic)
				}
			}
			c.SendEvent(NewServerEvent("status", "", map[string]any{"subscribed": cmd.Topics}, "subscriptions updated"))

		case "unsubscribe":
			for _, topic := range cmd.Topics {
				if topic != "" {
					c.hub.Unsubscribe(c, topic)
					c.removeTopic(topic)
				}
			}
			c.SendEvent(NewServerEvent("status", "", map[string]any{"unsubscribed": cmd.Topics}, "unsubscribed successfully"))

		case "ping":
			c.SendEvent(NewServerEvent("pong", "", nil, "pong"))

		default:
			c.SendEvent(NewServerEvent("error", "", nil, "unknown action: "+cmd.Action))
		}
	}
}

// WritePump pushes queued messages to the WebSocket client and handles heartbeat pings.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Drain and batch any additional buffered messages to optimize throughput
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendEvent serializes and enqueues a ServerEvent. Non-blocking: drops if buffer is full.
func (c *Client) SendEvent(event ServerEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	select {
	case c.send <- data:
	default:
		// Send buffer full - slow consumer, drop message to prevent blocking the hub
		c.logger.Warn("dropping websocket message for slow consumer",
			slog.Int64("user_id", c.userID),
			slog.String("type", event.Type),
		)
	}
}
