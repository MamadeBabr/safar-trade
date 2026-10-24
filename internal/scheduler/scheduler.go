package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hossen/safar-trade/internal/ingestion/forexfactory"
)

// Scheduler manages scheduled periodic data ingestion jobs in the background.
type Scheduler struct {
	ingester forexfactory.Ingester
	logger   *slog.Logger
	interval time.Duration
	wg       sync.WaitGroup
}

// NewScheduler creates a Scheduler configured with a given execution interval.
func NewScheduler(ingester forexfactory.Ingester, logger *slog.Logger, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	return &Scheduler{
		ingester: ingester,
		logger:   logger,
		interval: interval,
	}
}

// Start spawns a background goroutine that triggers ingestion immediately on boot,
// and then on every interval tick until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.logger.Info("starting ingestion background scheduler", slog.Duration("interval", s.interval))

		// Execute an initial ingestion pass on startup
		s.runIngestion(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("ingestion scheduler stopped gracefully")
				return
			case <-ticker.C:
				s.runIngestion(ctx)
			}
		}
	}()
}

// Wait blocks until the scheduler's background worker completes (called during graceful shutdown).
func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) runIngestion(ctx context.Context) {
	s.logger.Info("triggering scheduled forex factory ingestion")

	// Create a dedicated job context with a 3-minute timeout
	jobCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	res, err := s.ingester.Ingest(jobCtx)
	if err != nil {
		s.logger.Error("scheduled forex factory ingestion failed", slog.Any("error", err))
		return
	}

	s.logger.Info("scheduled ingestion finished successfully",
		slog.Int("events_saved", res.Upserted),
		slog.Int64("duration_ms", res.DurationMs),
	)
}
