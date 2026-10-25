package candle

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines database operations for timeframe candle tables.
type Repository interface {
	UpsertBatch(ctx context.Context, tf Timeframe, candles []TimeframeCandle) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

var _ Repository = (*pgRepository)(nil)

// NewRepository creates a new timeframe candle repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// UpsertBatch inserts or updates a slice of candles into the respective table `candles_<tf>`.
func (r *pgRepository) UpsertBatch(ctx context.Context, tf Timeframe, candles []TimeframeCandle) error {
	if len(candles) == 0 {
		return nil
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (symbol, o, h, l, c, v, t)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (symbol, t) DO UPDATE SET
			o = EXCLUDED.o,
			h = EXCLUDED.h,
			l = EXCLUDED.l,
			c = EXCLUDED.c,
			v = EXCLUDED.v;
	`, tf.TableName())

	batch := &pgx.Batch{}
	for _, c := range candles {
		batch.Queue(query, c.Symbol, c.O, c.H, c.L, c.C, c.V, c.T)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range candles {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to execute batch upsert for %s: %w", tf.TableName(), err)
		}
	}

	return nil
}
