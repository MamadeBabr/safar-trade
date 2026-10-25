package binance

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
)

// LiveBuffer collects candles in memory and flushes them to PostgreSQL in batches.
type LiveBuffer struct {
	repo          repository.CandleRepository
	logger        *slog.Logger
	flushInterval time.Duration
	maxBufferSize int

	mu     sync.Mutex
	buffer []model.Candle

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewLiveBuffer initializes a new batch buffer for candles.
func NewLiveBuffer(repo repository.CandleRepository, logger *slog.Logger, flushInterval time.Duration, maxBufferSize int) *LiveBuffer {
	if flushInterval <= 0 {
		flushInterval = 3 * time.Second
	}
	if maxBufferSize <= 0 {
		maxBufferSize = 500
	}
	return &LiveBuffer{
		repo:          repo,
		logger:        logger,
		flushInterval: flushInterval,
		maxBufferSize: maxBufferSize,
		buffer:        make([]model.Candle, 0, maxBufferSize),
	}
}

// Push adds a candle to the buffer. If the buffer is full, it triggers a synchronous or asynchronous flush.
func (b *LiveBuffer) Push(candle model.Candle) {
	b.mu.Lock()
	b.buffer = append(b.buffer, candle)
	shouldFlush := len(b.buffer) >= b.maxBufferSize
	b.mu.Unlock()

	if shouldFlush {
		b.flush()
	}
}

// StartFlushing starts the periodic background flush worker.
func (b *LiveBuffer) StartFlushing(parentCtx context.Context) {
	b.ctx, b.cancel = context.WithCancel(parentCtx)
	b.wg.Add(1)

	go func() {
		defer b.wg.Done()
		ticker := time.NewTicker(b.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-b.ctx.Done():
				b.flush() // final flush on stop
				return
			case <-ticker.C:
				b.flush()
			}
		}
	}()
}

// StopFlushing gracefully stops the background flusher and drains remaining buffered items.
func (b *LiveBuffer) StopFlushing() {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
}

func (b *LiveBuffer) flush() {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return
	}
	toFlush := b.buffer
	b.buffer = make([]model.Candle, 0, b.maxBufferSize)
	b.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	saved, err := b.repo.UpsertBatch(ctx, toFlush)
	if err != nil {
		b.logger.Error("failed to batch insert candles to postgres",
			slog.Int("attempted", len(toFlush)),
			slog.Any("error", err),
		)
		return
	}

	b.logger.Debug("persisted candle batch to database",
		slog.Int("count", saved),
	)
}
