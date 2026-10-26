package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hossen/safar-trade/internal/model"
	tfCandle "github.com/hossen/safar-trade/internal/processing/candle"
	"github.com/hossen/safar-trade/internal/repository"
)

// DataHandler handles serving market data that is restricted to users with an active subscription.
type DataHandler struct {
	candleRepo tfCandle.Repository
	eventRepo  repository.EconomicEventRepository
}

// NewDataHandler constructs a DataHandler.
func NewDataHandler(
	candleRepo tfCandle.Repository,
	eventRepo repository.EconomicEventRepository,
) *DataHandler {
	return &DataHandler{
		candleRepo: candleRepo,
		eventRepo:  eventRepo,
	}
}

// CandleHistoryInput defines the query parameters for retrieving candlestick history.
type CandleHistoryInput struct {
	Symbol    string `query:"symbol" required:"true" example:"BTCUSDT" doc:"Trading pair symbol (e.g. BTCUSDT)"`
	Timeframe string `query:"timeframe" required:"true" example:"15m" doc:"Bar interval: 1m, 5m, 15m, 30m, 60m/1h, 240m/4h, 1d, 1w, 1M"`
	From      int64  `query:"from" default:"0" minimum:"0" example:"1700000000000" doc:"Start timestamp in ms (or seconds). Required if countback is not provided."`
	To        int64  `query:"to" default:"0" minimum:"0" example:"1700086400000" doc:"End timestamp in ms (or seconds). Defaults to current time for countback."`
	Countback int    `query:"countback" default:"0" minimum:"0" maximum:"1000" example:"100" doc:"Number of bars backwards from 'to'. Required if 'from' is omitted."`
}

// RegisterRoutes registers OpenAPI 3.1 endpoints that require both authentication and an active subscription.
func (h *DataHandler) RegisterRoutes(api huma.API, subRequiredMW huma.Middlewares) {
	// 1. Candlestick Market History (Protected by active subscription)
	huma.Register(api, huma.Operation{
		OperationID: "get-market-candles",
		Method:      http.MethodGet,
		Path:        "/api/v1/data/candles",
		Summary:     "Get candlestick history & live bar (Subscription required)",
		Description: "Retrieves historical candles for a specific timeframe by range (from/to) or countback, and serves the live in-progress candle from Redis alongside historical data.",
		Tags:        []string{"Protected Market Data"},
		Middlewares: subRequiredMW,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *CandleHistoryInput) (*struct {
		Body struct {
			Symbol     string                     `json:"symbol" example:"BTCUSDT"`
			Timeframe  string                     `json:"timeframe" example:"15m"`
			Count      int                        `json:"count" example:"100"`
			Candles    []tfCandle.TimeframeCandle `json:"candles"`
			LiveCandle *tfCandle.TimeframeCandle  `json:"live_candle,omitempty"`
		}
	}, error) {
		symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
		if symbol == "" {
			return nil, huma.Error400BadRequest("symbol is required")
		}

		tf, ok := tfCandle.ParseTimeframe(strings.TrimSpace(input.Timeframe))
		if !ok {
			return nil, huma.Error400BadRequest(fmt.Sprintf(
				"invalid timeframe %q. Supported intervals: 1m, 5m, 15m, 30m, 60m (1h), 240m (4h), 1d, 1w, 1M",
				input.Timeframe,
			))
		}

		// Validation: enforce "from" and "to" OR "countback"
		hasRange := input.From > 0 && input.To > 0
		hasCountback := input.Countback > 0

		if !hasRange && !hasCountback {
			return nil, huma.Error400BadRequest("invalid request: either 'from' and 'to' or 'countback' must be provided")
		}

		from := input.From
		to := input.To

		// Normalize timestamps: if caller passed seconds (< 1e11), convert to milliseconds
		if from > 0 && from < 100_000_000_000 {
			from *= 1000
		}
		if to > 0 && to < 100_000_000_000 {
			to *= 1000
		}

		var candles []tfCandle.TimeframeCandle
		var err error

		if hasCountback {
			toTime := time.Now().UnixMilli()
			if to > 0 {
				toTime = to
			}
			count := input.Countback
			if count > 1000 {
				count = 1000
			}
			candles, err = h.candleRepo.GetHistoryByCountback(ctx, tf, symbol, toTime, count)
		} else {
			if from > to {
				return nil, huma.Error400BadRequest("'from' timestamp cannot be greater than 'to' timestamp")
			}
			candles, err = h.candleRepo.GetHistoryByRange(ctx, tf, symbol, from, to)
		}

		if err != nil {
			return nil, huma.Error500InternalServerError("failed to retrieve candlestick history: " + err.Error())
		}

		// Read live unfinalized candle from Redis
		liveCandle, _ := h.candleRepo.GetLiveCandle(ctx, tf, symbol)

		return &struct {
			Body struct {
				Symbol     string                   `json:"symbol" example:"BTCUSDT"`
				Timeframe  string                   `json:"timeframe" example:"15m"`
				Count      int                      `json:"count" example:"100"`
				Candles    []tfCandle.TimeframeCandle `json:"candles"`
				LiveCandle *tfCandle.TimeframeCandle  `json:"live_candle,omitempty"`
			}
		}{
			Body: struct {
				Symbol     string                   `json:"symbol" example:"BTCUSDT"`
				Timeframe  string                   `json:"timeframe" example:"15m"`
				Count      int                      `json:"count" example:"100"`
				Candles    []tfCandle.TimeframeCandle `json:"candles"`
				LiveCandle *tfCandle.TimeframeCandle  `json:"live_candle,omitempty"`
			}{
				Symbol:     symbol,
				Timeframe:  string(tf),
				Count:      len(candles),
				Candles:    candles,
				LiveCandle: liveCandle,
			},
		}, nil
	})

	// 2. Economic Calendar Events (Protected by active subscription)
	huma.Register(api, huma.Operation{
		OperationID: "get-economic-events",
		Method:      http.MethodGet,
		Path:        "/api/v1/data/economic-events",
		Summary:     "Get economic calendar events (Subscription required)",
		Description: "Returns high-impact forex macroeconomic events and indicators. Restricted to active subscribers.",
		Tags:        []string{"Protected Market Data"},
		Middlewares: subRequiredMW,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct {
		Limit  int32 `query:"limit" default:"50" minimum:"1" maximum:"200" doc:"Page size limit"`
		Offset int32 `query:"offset" default:"0" minimum:"0" doc:"Page offset"`
	}) (*struct {
		Body struct {
			Count  int                   `json:"count"`
			Events []model.EconomicEvent `json:"events"`
		}
	}, error) {
		limit := input.Limit
		if limit <= 0 || limit > 200 {
			limit = 50
		}

		events, err := h.eventRepo.List(ctx, limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to retrieve economic events: " + err.Error())
		}

		return &struct {
			Body struct {
				Count  int                   `json:"count"`
				Events []model.EconomicEvent `json:"events"`
			}
		}{
			Body: struct {
				Count  int                   `json:"count"`
				Events []model.EconomicEvent `json:"events"`
			}{
				Count:  len(events),
				Events: events,
			},
		}, nil
	})

	// 3. Supported Symbols (Public)
	huma.Register(api, huma.Operation{
		OperationID: "get-supported-symbols",
		Method:      http.MethodGet,
		Path:        "/api/v1/data/symbols",
		Summary:     "Get supported trading symbols",
		Description: "Returns active cryptocurrency trading pairs available for live streaming and chart history.",
		Tags:        []string{"Market Data"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Symbols []string `json:"symbols"`
		}
	}, error) {
		symbols := []string{
			"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT",
			"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT", "SUIUSDT",
			"NEARUSDT", "APTUSDT", "DOTUSDT", "PEPEUSDT", "SHIBUSDT",
			"LTCUSDT", "TRXUSDT", "TONUSDT", "MATICUSDT", "UNIUSDT",
		}
		return &struct {
			Body struct {
				Symbols []string `json:"symbols"`
			}
		}{
			Body: struct {
				Symbols []string `json:"symbols"`
			}{
				Symbols: symbols,
			},
		}, nil
	})

	// 4. Live Tickers Snapshot (Public)
	huma.Register(api, huma.Operation{
		OperationID: "get-live-tickers",
		Method:      http.MethodGet,
		Path:        "/api/v1/data/tickers",
		Summary:     "Get live price snapshot for multiple symbols",
		Description: "Fetches the latest live candle for each requested symbol from Redis cache.",
		Tags:        []string{"Market Data"},
	}, func(ctx context.Context, input *struct {
		Symbols string `query:"symbols" example:"BTCUSDT,ETHUSDT,SOLUSDT" doc:"Optional comma-separated symbol whitelist (defaults to top pairs)"`
	}) (*struct {
		Body struct {
			Tickers map[string]*tfCandle.TimeframeCandle `json:"tickers"`
		}
	}, error) {
		var list []string
		if input.Symbols != "" {
			for _, s := range strings.Split(input.Symbols, ",") {
				if trimmed := strings.ToUpper(strings.TrimSpace(s)); trimmed != "" {
					list = append(list, trimmed)
				}
			}
		}
		if len(list) == 0 {
			list = []string{
				"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT",
				"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT", "SUIUSDT",
			}
		}

		tickers := make(map[string]*tfCandle.TimeframeCandle, len(list))
		for _, s := range list {
			candle, err := h.candleRepo.GetLiveCandle(ctx, tfCandle.TF1m, s)
			if err == nil && candle != nil {
				tickers[s] = candle
			}
		}

		return &struct {
			Body struct {
				Tickers map[string]*tfCandle.TimeframeCandle `json:"tickers"`
			}
		}{
			Body: struct {
				Tickers map[string]*tfCandle.TimeframeCandle `json:"tickers"`
			}{
				Tickers: tickers,
			},
		}, nil
	})
}
