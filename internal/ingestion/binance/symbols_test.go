package binance

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSymbolFetcherSpot(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"symbols": [
				{"symbol": "BTCUSDT", "status": "TRADING", "quoteAsset": "USDT", "isSpotTradingAllowed": true},
				{"symbol": "ETHBTC", "status": "TRADING", "quoteAsset": "BTC", "isSpotTradingAllowed": true},
				{"symbol": "SOLUSDT", "status": "BREAK", "quoteAsset": "USDT", "isSpotTradingAllowed": false}
			]
		}`))
	}))
	defer mockServer.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fetcher := &SymbolFetcher{
		httpClient: mockServer.Client(),
		logger:     logger,
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, mockServer.URL, nil)
	resp, err := fetcher.httpClient.Do(req)
	if err != nil {
		t.Fatalf("failed to fetch from mock server: %v", err)
	}
	defer resp.Body.Close()

	var data spotExchangeInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var symbols []string
	for _, s := range data.Symbols {
		if s.Status == "TRADING" && s.QuoteAsset == "USDT" {
			symbols = append(symbols, s.Symbol)
		}
	}

	if len(symbols) != 1 || symbols[0] != "BTCUSDT" {
		t.Errorf("expected [BTCUSDT], got %v", symbols)
	}
}
