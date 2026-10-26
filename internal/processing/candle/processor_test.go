package candle

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/hossen/safar-trade/internal/model"
)

type mockRepository struct {
	mu      sync.Mutex
	upserts map[Timeframe][]TimeframeCandle
}

func (m *mockRepository) UpsertBatch(ctx context.Context, tf Timeframe, candles []TimeframeCandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upserts[tf] = append(m.upserts[tf], candles...)
	return nil
}

func (m *mockRepository) GetHistoryByRange(ctx context.Context, tf Timeframe, symbol string, from, to int64) ([]TimeframeCandle, error) {
	return nil, nil
}

func (m *mockRepository) GetHistoryByCountback(ctx context.Context, tf Timeframe, symbol string, to int64, countback int) ([]TimeframeCandle, error) {
	return nil, nil
}

func (m *mockRepository) GetLiveCandle(ctx context.Context, tf Timeframe, symbol string) (*TimeframeCandle, error) {
	return nil, nil
}

func TestProcessorAggregationAndRollover(t *testing.T) {
	mockRepo := &mockRepository{
		upserts: make(map[Timeframe][]TimeframeCandle),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	proc := NewProcessor(mockRepo, nil, logger, 50*time.Millisecond).(*candleProcessor)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proc.Start(ctx)

	// Base timestamp: 2026-10-25 12:00:00 (Bucket 12:00:00)
	t0 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC).UnixMilli()

	// 1. First 1m candle (12:00)
	proc.Process(&model.Candle{
		Symbol:   "BTCUSDT",
		OpenTime: t0,
		Open:     "60000",
		High:     "60200",
		Low:      "59900",
		Close:    "60100",
		Volume:   "10",
		IsClosed: true,
	})

	// Check 5m state
	c5m := proc.activeCandles[TF5m]["BTCUSDT"]
	if c5m.O != "60000" || c5m.H != "60200" || c5m.L != "59900" || c5m.C != "60100" {
		t.Fatalf("unexpected 5m candle: %+v", c5m)
	}

	// 2. Second 1m candle in the same 5m bucket (12:01)
	t1 := t0 + 60*1000
	proc.Process(&model.Candle{
		Symbol:   "BTCUSDT",
		OpenTime: t1,
		Open:     "60100",
		High:     "60500", // New high
		Low:      "60050",
		Close:    "60400", // New close
		Volume:   "5",
		IsClosed: true,
	})

	if c5m.H != "60500" {
		t.Errorf("expected 5m high 60500, got %s", c5m.H)
	}
	if c5m.C != "60400" {
		t.Errorf("expected 5m close 60400, got %s", c5m.C)
	}
	if c5m.V != "15" {
		t.Errorf("expected 5m volume 15, got %s", c5m.V)
	}

	// 3. Rollover: Candle in next 5m bucket (12:05)
	t5 := t0 + 5*60*1000
	proc.Process(&model.Candle{
		Symbol:   "BTCUSDT",
		OpenTime: t5,
		Open:     "60400",
		High:     "60600",
		Low:      "60300",
		Close:    "60550",
		Volume:   "8",
		IsClosed: true,
	})

	// Previous 5m candle should now be in the finalized queue
	proc.queueMu.Lock()
	qLen := len(proc.finalizedQueue[TF5m])
	proc.queueMu.Unlock()

	if qLen != 1 {
		t.Fatalf("expected 1 finalized 5m candle, got %d", qLen)
	}

	// Wait for background worker to flush to mockRepo
	time.Sleep(100 * time.Millisecond)

	mockRepo.mu.Lock()
	persisted := len(mockRepo.upserts[TF5m])
	mockRepo.mu.Unlock()

	if persisted != 1 {
		t.Fatalf("expected 1 persisted 5m candle in database, got %d", persisted)
	}

	finalized := mockRepo.upserts[TF5m][0]
	if finalized.T != t0 || finalized.O != "60000" || finalized.H != "60500" || finalized.L != "59900" || finalized.C != "60400" || finalized.V != "15" {
		t.Errorf("incorrect finalized 5m candle: %+v", finalized)
	}

	// Verify 1m candles were also persisted to candles_1m
	if len(mockRepo.upserts[TF1m]) != 3 {
		t.Errorf("expected 3 persisted 1m candles, got %d", len(mockRepo.upserts[TF1m]))
	}

	proc.Stop()
}
