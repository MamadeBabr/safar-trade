package candle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hossen/safar-trade/internal/model"
)

// Processor orchestrates real-time multi-timeframe aggregation and batched database persistence.
type Processor interface {
	Process(candle *model.Candle)
	Start(ctx context.Context)
	Stop()
	RegisterListener(fn func(tf Timeframe, c *TimeframeCandle))
}

type candleProcessor struct {
	repo          Repository
	redisClient   *redis.Client
	logger        *slog.Logger
	flushInterval time.Duration

	// In-memory state: map[Timeframe]map[Symbol]*TimeframeCandle
	mu            sync.RWMutex
	activeCandles map[Timeframe]map[string]*TimeframeCandle

	// Queue of finalized (closed) timeframe candles awaiting PostgreSQL persistence
	queueMu        sync.Mutex
	finalizedQueue map[Timeframe][]TimeframeCandle

	listeners []func(tf Timeframe, c *TimeframeCandle)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var _ Processor = (*candleProcessor)(nil)

// NewProcessor initializes a new candle processing engine.
func NewProcessor(
	repo Repository,
	redisClient *redis.Client,
	logger *slog.Logger,
	flushInterval time.Duration,
) Processor {
	if flushInterval <= 0 {
		flushInterval = 3 * time.Second
	}

	active := make(map[Timeframe]map[string]*TimeframeCandle, len(AllTimeframes))
	queue := make(map[Timeframe][]TimeframeCandle, len(AllTimeframes))
	for _, tf := range AllTimeframes {
		active[tf] = make(map[string]*TimeframeCandle)
		queue[tf] = make([]TimeframeCandle, 0, 100)
	}

	return &candleProcessor{
		repo:           repo,
		redisClient:    redisClient,
		logger:         logger,
		flushInterval:  flushInterval,
		activeCandles:  active,
		finalizedQueue: queue,
	}
}

// Start launches the background batch persistence worker.
func (p *candleProcessor) Start(parentCtx context.Context) {
	p.ctx, p.cancel = context.WithCancel(parentCtx)
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				p.flushFinalized()
				return
			case <-ticker.C:
				p.flushFinalized()
			}
		}
	}()
}

// Stop cleanly terminates the processor and flushes all queued finalized candles.
func (p *candleProcessor) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

// Process updates all 8 timeframes immediately in-memory and in Redis.
// When a timeframe bucket rolls over, the completed candle is queued for PostgreSQL batch persistence.
func (p *candleProcessor) Process(candle *model.Candle) {
	if candle == nil || candle.Symbol == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, tf := range AllTimeframes {
		bucketStart := CalculateBucketTime(candle.OpenTime, tf)
		activeMap := p.activeCandles[tf]
		current, exists := activeMap[candle.Symbol]

		if !exists {
			// First candle seen for this symbol & timeframe
			newCandle := &TimeframeCandle{
				Symbol: candle.Symbol,
				O:      candle.Open,
				H:      candle.High,
				L:      candle.Low,
				C:      candle.Close,
				V:      candle.Volume,
				T:      bucketStart,
			}
			activeMap[candle.Symbol] = newCandle
			p.updateRedis(tf, newCandle)

			if tf == TF1m && candle.IsClosed {
				p.queueMu.Lock()
				p.finalizedQueue[tf] = append(p.finalizedQueue[tf], *newCandle)
				p.queueMu.Unlock()
			}
			continue
		}

		if current.T == bucketStart {
			// Intra-bucket update for currently forming candle
			current.H = maxPrice(current.H, candle.High)
			current.L = minPrice(current.L, candle.Low)
			current.C = candle.Close
			if candle.IsClosed {
				current.V = addVolume(current.V, candle.Volume)
			}
			p.updateRedis(tf, current)

			// When 1m candle is closed, finalize it immediately for candles_1m
			if tf == TF1m && candle.IsClosed {
				finalized := *current
				p.queueMu.Lock()
				p.finalizedQueue[tf] = append(p.finalizedQueue[tf], finalized)
				p.queueMu.Unlock()
			}
			continue
		}

		if current.T < bucketStart {
			// Boundary rollover: the previous candle has finalized!
			// Copy finalized candle to avoid race conditions
			finalized := *current

			p.queueMu.Lock()
			p.finalizedQueue[tf] = append(p.finalizedQueue[tf], finalized)
			p.queueMu.Unlock()

			// Reset current slot with the newly started bucket
			current.T = bucketStart
			current.O = candle.Open
			current.H = candle.High
			current.L = candle.Low
			current.C = candle.Close
			current.V = candle.Volume

			p.updateRedis(tf, current)
		}
	}
}

// RegisterListener attaches a callback invoked whenever any timeframe bar updates.
func (p *candleProcessor) RegisterListener(fn func(tf Timeframe, c *TimeframeCandle)) {
	p.listeners = append(p.listeners, fn)
}

func (p *candleProcessor) updateRedis(tf Timeframe, c *TimeframeCandle) {
	// Dispatch to registered in-process listeners (e.g. WebSocket Hub)
	for _, fn := range p.listeners {
		fn(tf, c)
	}

	if p.redisClient == nil {
		return
	}

	data, err := json.Marshal(c)
	if err != nil {
		return
	}

	key := fmt.Sprintf("binance:candle:%s:%s", tf, c.Symbol)
	// Fast async set without blocking the ingestion hot-path
	go func(k string, d []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.redisClient.Set(ctx, k, d, 0).Err()
	}(key, data)
}

func (p *candleProcessor) flushFinalized() {
	p.queueMu.Lock()
	// Swap queues atomically to minimize lock contention
	toFlush := make(map[Timeframe][]TimeframeCandle, len(AllTimeframes))
	hasItems := false
	for _, tf := range AllTimeframes {
		if len(p.finalizedQueue[tf]) > 0 {
			toFlush[tf] = p.finalizedQueue[tf]
			p.finalizedQueue[tf] = make([]TimeframeCandle, 0, 100)
			hasItems = true
		}
	}
	p.queueMu.Unlock()

	if !hasItems {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for tf, candles := range toFlush {
		if len(candles) == 0 {
			continue
		}
		if err := p.repo.UpsertBatch(ctx, tf, candles); err != nil {
			p.logger.Error("failed to batch upsert finalized candles",
				slog.String("timeframe", string(tf)),
				slog.Int("count", len(candles)),
				slog.Any("error", err),
			)
		} else {
			p.logger.Info("persisted finalized candles to database",
				slog.String("table", tf.TableName()),
				slog.Int("count", len(candles)),
			)
		}
	}
}
