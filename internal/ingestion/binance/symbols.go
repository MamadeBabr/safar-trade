package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	SpotExchangeInfoURL    = "https://api.binance.com/api/v3/exchangeInfo"
	FuturesExchangeInfoURL = "https://fapi.binance.com/fapi/v1/exchangeInfo"
)

// DefaultFallbackSymbols contains top high-volume USDT trading pairs used if the REST API is unreachable.
var DefaultFallbackSymbols = []string{
	"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT",
	"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT", "SUIUSDT",
	"NEARUSDT", "APTUSDT", "DOTUSDT", "PEPEUSDT", "SHIBUSDT",
	"LTCUSDT", "TRXUSDT", "TONUSDT", "MATICUSDT", "UNIUSDT",
}

type spotExchangeInfoResponse struct {
	Symbols []struct {
		Symbol               string `json:"symbol"`
		Status               string `json:"status"`
		QuoteAsset           string `json:"quoteAsset"`
		IsSpotTradingAllowed bool   `json:"isSpotTradingAllowed"`
	} `json:"symbols"`
}

type futuresExchangeInfoResponse struct {
	Symbols []struct {
		Symbol         string `json:"symbol"`
		Status         string `json:"status"`
		ContractStatus string `json:"contractStatus"`
		QuoteAsset     string `json:"quoteAsset"`
		ContractType   string `json:"contractType"`
	} `json:"symbols"`
}

// SymbolFetcher retrieves actively traded symbols from official Binance REST APIs.
type SymbolFetcher struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// NewSymbolFetcher creates a new SymbolFetcher instance.
func NewSymbolFetcher(logger *slog.Logger) *SymbolFetcher {
	return &SymbolFetcher{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// GetActiveSymbols retrieves active USDT trading symbols for the given market ("spot" or "futures").
func (f *SymbolFetcher) GetActiveSymbols(ctx context.Context, market string) ([]string, error) {
	var symbols []string
	var err error

	f.logger.Info("fetching active symbols from Binance official REST API", slog.String("market", market))

	switch strings.ToLower(market) {
	case "spot":
		symbols, err = f.fetchSpotSymbols(ctx)
	case "futures":
		symbols, err = f.fetchFuturesSymbols(ctx)
	default:
		return nil, fmt.Errorf("unknown market: %s (supported: spot, futures)", market)
	}

	if err != nil || len(symbols) == 0 {
		f.logger.Warn("failed to fetch symbols from Binance REST API, using default top USDT symbols fallback",
			slog.String("market", market),
			slog.Any("error", err),
			slog.Int("fallback_count", len(DefaultFallbackSymbols)),
		)
		return DefaultFallbackSymbols, nil
	}

	return symbols, nil
}

func (f *SymbolFetcher) fetchSpotSymbols(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SpotExchangeInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create spot exchangeInfo request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spot exchangeInfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spot exchangeInfo returned HTTP %d", resp.StatusCode)
	}

	var data spotExchangeInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode spot exchangeInfo: %w", err)
	}

	var symbols []string
	for _, s := range data.Symbols {
		if s.Status == "TRADING" && s.QuoteAsset == "USDT" {
			symbols = append(symbols, s.Symbol)
		}
	}

	f.logger.Info("fetched active Binance spot symbols", slog.Int("count", len(symbols)))
	return symbols, nil
}

func (f *SymbolFetcher) fetchFuturesSymbols(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, FuturesExchangeInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create futures exchangeInfo request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch futures exchangeInfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("futures exchangeInfo returned HTTP %d", resp.StatusCode)
	}

	var data futuresExchangeInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode futures exchangeInfo: %w", err)
	}

	var symbols []string
	for _, s := range data.Symbols {
		status := s.Status
		if status == "" {
			status = s.ContractStatus
		}
		if status == "TRADING" && s.QuoteAsset == "USDT" {
			symbols = append(symbols, s.Symbol)
		}
	}

	f.logger.Info("fetched active Binance futures symbols", slog.Int("count", len(symbols)))
	return symbols, nil
}
