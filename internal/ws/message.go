package ws

import "time"

// ClientCommand represents an incoming JSON message sent by a WebSocket client.
type ClientCommand struct {
	Action string   `json:"action"` // "subscribe", "unsubscribe", "ping"
	Topics []string `json:"topics"` // e.g. ["candles:BTCUSDT:1m", "alerts"]
}

// ServerEvent represents an outgoing JSON message pushed to a WebSocket client.
type ServerEvent struct {
	Type      string `json:"type"`            // "candle", "alert", "status", "pong", "error"
	Topic     string `json:"topic,omitempty"` // e.g. "candles:BTCUSDT:1m" or "alerts"
	Data      any    `json:"data,omitempty"`
	Message   string `json:"message,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// NewServerEvent constructs a standardized server message with current UTC timestamp.
func NewServerEvent(eventType, topic string, data any, msg string) ServerEvent {
	return ServerEvent{
		Type:      eventType,
		Topic:     topic,
		Data:      data,
		Message:   msg,
		Timestamp: time.Now().UnixMilli(),
	}
}

// AlertPayload defines the structured notification pushed over the "alerts" topic.
type AlertPayload struct {
	ID        string         `json:"id,omitempty"`
	Level     string         `json:"level"` // "info", "warning", "critical"
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Category  string         `json:"category,omitempty"` // "economic_calendar", "price_alert", "subscription", "system"
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp int64          `json:"timestamp"`
}

// CandlePayload represents the real-time candlestick update pushed over WebSocket.
type CandlePayload struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe,omitempty"`
	O         string `json:"o"`
	H         string `json:"h"`
	L         string `json:"l"`
	C         string `json:"c"`
	V         string `json:"v"`
	T         int64  `json:"t"`
	IsClosed  bool   `json:"is_closed"`
}
