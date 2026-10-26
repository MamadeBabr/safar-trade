package handler_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/hossen/safar-trade/internal/handler"
	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/service"
	"github.com/hossen/safar-trade/internal/ws"
)

type mockAuthService struct {
	validateFn func(token string) (*service.JWTClaims, error)
}

func (m *mockAuthService) Register(ctx context.Context, req model.RegisterRequest, userAgent, ip string) (*model.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthService) Login(ctx context.Context, req model.LoginRequest, userAgent, ip string) (*model.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthService) RefreshToken(ctx context.Context, req model.RefreshTokenRequest, userAgent, ip string) (*model.TokenPair, error) {
	return nil, nil
}
func (m *mockAuthService) Logout(ctx context.Context, refreshToken string, userID int64) error {
	return nil
}
func (m *mockAuthService) ValidateAccessToken(token string) (*service.JWTClaims, error) {
	if m.validateFn != nil {
		return m.validateFn(token)
	}
	return nil, service.ErrInvalidToken
}

type mockSubscriptionService struct {
	hasActiveFn func(ctx context.Context, userID int64) (bool, error)
}

func (m *mockSubscriptionService) ListPackages(ctx context.Context) ([]model.PackageResponse, error) {
	return nil, nil
}
func (m *mockSubscriptionService) GetActiveSubscription(ctx context.Context, userID int64) (*model.SubscriptionResponse, error) {
	return nil, nil
}
func (m *mockSubscriptionService) Subscribe(ctx context.Context, userID int64, req model.SubscribeRequest) (*model.SubscriptionResponse, error) {
	return nil, nil
}
func (m *mockSubscriptionService) Cancel(ctx context.Context, userID int64) error {
	return nil
}
func (m *mockSubscriptionService) HasActiveSubscription(ctx context.Context, userID int64) (bool, error) {
	if m.hasActiveFn != nil {
		return m.hasActiveFn(ctx, userID)
	}
	return false, nil
}

func TestWSHandler_HandshakeAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := ws.NewHub(logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	authSvc := &mockAuthService{
		validateFn: func(token string) (*service.JWTClaims, error) {
			if token == "valid-pro-token" {
				return &service.JWTClaims{UserID: 1, Email: "pro@example.com"}, nil
			}
			if token == "valid-free-token" {
				return &service.JWTClaims{UserID: 2, Email: "free@example.com"}, nil
			}
			return nil, service.ErrInvalidToken
		},
	}

	subSvc := &mockSubscriptionService{
		hasActiveFn: func(ctx context.Context, userID int64) (bool, error) {
			return userID == 1, nil // Only User 1 has active subscription
		},
	}

	wsHandler := handler.NewWSHandler(hub, authSvc, subSvc, logger)
	mux := http.NewServeMux()
	wsHandler.RegisterRoutes(mux)

	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws"

	// 1. Connection without token -> 401 Unauthorized
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected handshake to fail without token")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// 2. Connection with free user (no subscription) -> 403 Forbidden
	_, resp, err = websocket.DefaultDialer.Dial(wsURL+"?token=valid-free-token", nil)
	if err == nil {
		t.Fatal("expected handshake to fail for unsubscribed user")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", resp.StatusCode)
	}

	// 3. Connection with pro user (active subscription) -> 101 Switching Protocols
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL+"?token=valid-pro-token", nil)
	if err != nil {
		t.Fatalf("expected successful handshake for pro user, got error: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	// Read initial welcome event
	var welcomeEvent ws.ServerEvent
	if err := conn.ReadJSON(&welcomeEvent); err != nil {
		t.Fatalf("failed to read welcome event: %v", err)
	}
	if welcomeEvent.Type != "status" {
		t.Errorf("expected event type 'status', got %s", welcomeEvent.Type)
	}

	// 4. Send subscribe command
	subscribeCmd := ws.ClientCommand{
		Action: "subscribe",
		Topics: []string{"candles:BTCUSDT:1m", "alerts"},
	}
	if err := conn.WriteJSON(subscribeCmd); err != nil {
		t.Fatalf("failed to send subscribe command: %v", err)
	}

	// Read subscription confirmation
	var statusEvent ws.ServerEvent
	if err := conn.ReadJSON(&statusEvent); err != nil {
		t.Fatalf("failed to read status confirmation: %v", err)
	}
	if statusEvent.Type != "status" {
		t.Errorf("expected status event, got %s", statusEvent.Type)
	}

	// 5. Broadcast a candle and verify receipt
	candleData := map[string]any{
		"symbol": "BTCUSDT",
		"price":  "65000",
	}
	hub.BroadcastCandle("BTCUSDT", "1m", candleData)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var candleEvent ws.ServerEvent
	if err := conn.ReadJSON(&candleEvent); err != nil {
		t.Fatalf("failed to read broadcast candle event: %v", err)
	}
	if candleEvent.Type != "candle" || candleEvent.Topic != "candles:BTCUSDT:1m" {
		t.Errorf("unexpected candle event: %+v", candleEvent)
	}

	// 6. Broadcast an alert and verify receipt
	hub.BroadcastAlert("warning", "Economic Alert", "CPI in 10 mins", nil)

	var alertEvent ws.ServerEvent
	if err := conn.ReadJSON(&alertEvent); err != nil {
		t.Fatalf("failed to read broadcast alert event: %v", err)
	}
	if alertEvent.Type != "alert" || alertEvent.Topic != "alerts" {
		t.Errorf("unexpected alert event: %+v", alertEvent)
	}
}
