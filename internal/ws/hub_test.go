package ws_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hossen/safar-trade/internal/ws"
)

func TestHub_RegisterAndBroadcast(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := ws.NewHub(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Verify initial client count
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients, got %d", hub.ClientCount())
	}

	// Test broadcast does not panic with 0 clients
	hub.Broadcast("candles:BTCUSDT:1m", map[string]string{"price": "65000"})
	hub.BroadcastAlert("warning", "Test Alert", "This is a test", nil)

	time.Sleep(10 * time.Millisecond)
}

func TestHub_AlertPayload(t *testing.T) {
	alert := ws.AlertPayload{
		Level:    "high",
		Title:    "CPI Announcement",
		Message:  "CPI in 15 mins",
		Category: "economic_calendar",
	}

	if alert.Level != "high" {
		t.Errorf("expected level 'high', got %s", alert.Level)
	}

	event := ws.NewServerEvent("alert", "alerts", alert, "")
	if event.Type != "alert" {
		t.Errorf("expected type 'alert', got %s", event.Type)
	}
	if event.Timestamp == 0 {
		t.Errorf("expected non-zero timestamp")
	}
}
