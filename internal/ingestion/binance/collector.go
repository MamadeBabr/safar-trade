package binance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
	redisStore "github.com/hossen/safar-trade/internal/storage/redis"
)

// CollectorConfig holds the runtime parameters for the Binance ingestion collector.
type CollectorConfig struct {
	SpotWSURL           string
	FuturesWSURL        string
	FuturesSymbolSuffix string
	FlushInterval       time.Duration
	ExplicitSymbols     []string // If non-empty, use this list instead of auto-discovering
}

// Collector coordinates the Binance Spot & Futures ingestion pipeline.
type Collector struct {
	cfg         CollectorConfig
	repo        repository.CandleRepository
	candleStore redisStore.CandleStore
	logger      *slog.Logger

	fetcher       *SymbolFetcher
	spotStream    *Stream
	futuresStream *Stream
	spotBuffer    *LiveBuffer
	futuresBuffer *LiveBuffer
}

// NewCollector constructs a new Binance Ingestion Collector.
func NewCollector(
	cfg CollectorConfig,
	repo repository.CandleRepository,
	candleStore redisStore.CandleStore,
	logger *slog.Logger,
) *Collector {
	if cfg.SpotWSURL == "" {
		cfg.SpotWSURL = "wss://stream.binance.com:9443/stream"
	}
	if cfg.FuturesWSURL == "" {
		cfg.FuturesWSURL = "wss://fstream.binance.com/stream"
	}
	if cfg.FuturesSymbolSuffix == "" {
		cfg.FuturesSymbolSuffix = "_FUTURES"
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 3 * time.Second
	}

	return &Collector{
		cfg:         cfg,
		repo:        repo,
		candleStore: candleStore,
		logger:      logger,
		fetcher:     NewSymbolFetcher(logger),
	}
}

// Start discovers symbols, connects WebSocket streams, and starts live buffering.
func (c *Collector) Start(ctx context.Context) error {
	c.logger.Info("initializing Binance collector for Spot and Futures")

	// 1. Resolve symbols
	var spotSymbols, futuresSymbols []string
	var err error

	if len(c.cfg.ExplicitSymbols) > 0 {
		spotSymbols = c.cfg.ExplicitSymbols
		futuresSymbols = c.cfg.ExplicitSymbols
		c.logger.Info("using explicit symbol configuration", slog.Int("count", len(spotSymbols)))
	} else {
		// Auto-discover active symbols from official Binance REST APIs
		spotSymbols, err = c.fetcher.GetActiveSymbols(ctx, "spot")
		if err != nil {
			return fmt.Errorf("failed to fetch Binance spot symbols: %w", err)
		}

		futuresSymbols, err = c.fetcher.GetActiveSymbols(ctx, "futures")
		if err != nil {
			return fmt.Errorf("failed to fetch Binance futures symbols: %w", err)
		}
	}

	// 2. Initialize database batch buffers
	c.spotBuffer = NewLiveBuffer(c.repo, c.logger.With(slog.String("buffer", "spot")), c.cfg.FlushInterval, 500)
	c.futuresBuffer = NewLiveBuffer(c.repo, c.logger.With(slog.String("buffer", "futures")), c.cfg.FlushInterval, 500)

	c.spotBuffer.StartFlushing(ctx)
	c.futuresBuffer.StartFlushing(ctx)

	// 3. Initialize WebSocket streams
	c.spotStream = NewStream(StreamOptions{
		URL:          c.cfg.SpotWSURL,
		Market:       "spot",
		Symbols:      spotSymbols,
		SymbolSuffix: "",
		OnCandle: func(candle *model.Candle) {
			c.handleCandle(candle, c.spotBuffer)
		},
	}, c.logger)

	c.futuresStream = NewStream(StreamOptions{
		URL:          c.cfg.FuturesWSURL,
		Market:       "futures",
		Symbols:      futuresSymbols,
		SymbolSuffix: c.cfg.FuturesSymbolSuffix,
		OnCandle: func(candle *model.Candle) {
			c.handleCandle(candle, c.futuresBuffer)
		},
	}, c.logger)

	c.spotStream.Start(ctx)
	c.futuresStream.Start(ctx)

	c.logger.Info("Binance collector successfully started",
		slog.Int("spot_symbols", len(spotSymbols)),
		slog.Int("futures_symbols", len(futuresSymbols)),
	)
	return nil
}

// Stop gracefully stops streams and flushes all remaining cached candles to PostgreSQL.
func (c *Collector) Stop() {
	c.logger.Info("stopping Binance collector...")

	if c.spotStream != nil {
		c.spotStream.Stop()
	}
	if c.futuresStream != nil {
		c.futuresStream.Stop()
	}

	if c.spotBuffer != nil {
		c.spotBuffer.StopFlushing()
	}
	if c.futuresBuffer != nil {
		c.futuresBuffer.StopFlushing()
	}

	c.logger.Info("Binance collector shutdown complete")
}

func (c *Collector) handleCandle(candle *model.Candle, buffer *LiveBuffer) {
	// 1. Immediately persist the latest candle to Redis
	if c.candleStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := c.candleStore.SetLatestCandle(ctx, candle); err != nil {
			c.logger.Debug("failed to persist latest candle to redis",
				slog.String("symbol", candle.Symbol),
				slog.Any("error", err),
			)
		}
		cancel()
	}

	// 2. Persist to PostgreSQL via batch buffer when closed (or closed candle updates)
	if candle.IsClosed && buffer != nil {
		buffer.Push(*candle)
	}
}
