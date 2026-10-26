package candle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Repository defines database and cache operations for timeframe candle data.
type Repository interface {
	UpsertBatch(ctx context.Context, tf Timeframe, candles []TimeframeCandle) error
	GetHistoryByRange(ctx context.Context, tf Timeframe, symbol string, from, to int64) ([]TimeframeCandle, error)
	GetHistoryByCountback(ctx context.Context, tf Timeframe, symbol string, to int64, countback int) ([]TimeframeCandle, error)
	GetLiveCandle(ctx context.Context, tf Timeframe, symbol string) (*TimeframeCandle, error)
}

type pgRepository struct {
	pool        *pgxpool.Pool
	redisClient *redis.Client
}

var _ Repository = (*pgRepository)(nil)

// NewRepository creates a new timeframe candle repository with Postgres and Redis connectivity.
func NewRepository(pool *pgxpool.Pool, redisClient *redis.Client) Repository {
	return &pgRepository{
		pool:        pool,
		redisClient: redisClient,
	}
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

// GetHistoryByRange retrieves historical candles between `from` and `to` timestamps (inclusive), sorted chronologically.
func (r *pgRepository) GetHistoryByRange(ctx context.Context, tf Timeframe, symbol string, from, to int64) ([]TimeframeCandle, error) {
	query := fmt.Sprintf(`
		SELECT symbol, o, h, l, c, v, t
		FROM %s
		WHERE symbol = $1 AND t >= $2 AND t <= $3
		ORDER BY t ASC;
	`, tf.TableName())

	rows, err := r.pool.Query(ctx, query, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query candles for %s: %w", tf.TableName(), err)
	}
	defer rows.Close()

	candles := make([]TimeframeCandle, 0)
	for rows.Next() {
		var c TimeframeCandle
		if err := rows.Scan(&c.Symbol, &c.O, &c.H, &c.L, &c.C, &c.V, &c.T); err != nil {
			return nil, fmt.Errorf("failed to scan candle: %w", err)
		}
		candles = append(candles, c)
	}

	return candles, rows.Err()
}

// GetHistoryByCountback retrieves the last `countback` candles up to timestamp `to`, sorted chronologically.
func (r *pgRepository) GetHistoryByCountback(ctx context.Context, tf Timeframe, symbol string, to int64, countback int) ([]TimeframeCandle, error) {
	query := fmt.Sprintf(`
		SELECT symbol, o, h, l, c, v, t FROM (
			SELECT symbol, o, h, l, c, v, t
			FROM %s
			WHERE symbol = $1 AND t <= $2
			ORDER BY t DESC
			LIMIT $3
		) sub
		ORDER BY t ASC;
	`, tf.TableName())

	rows, err := r.pool.Query(ctx, query, symbol, to, countback)
	if err != nil {
		return nil, fmt.Errorf("failed to query countback candles for %s: %w", tf.TableName(), err)
	}
	defer rows.Close()

	candles := make([]TimeframeCandle, 0)
	for rows.Next() {
		var c TimeframeCandle
		if err := rows.Scan(&c.Symbol, &c.O, &c.H, &c.L, &c.C, &c.V, &c.T); err != nil {
			return nil, fmt.Errorf("failed to scan candle: %w", err)
		}
		candles = append(candles, c)
	}

	return candles, rows.Err()
}

// GetLiveCandle fetches the active unfinalized candle from Redis.
func (r *pgRepository) GetLiveCandle(ctx context.Context, tf Timeframe, symbol string) (*TimeframeCandle, error) {
	if r.redisClient == nil {
		return nil, nil
	}

	key := fmt.Sprintf("binance:candle:%s:%s", tf, symbol)
	val, err := r.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var live TimeframeCandle
	if err := json.Unmarshal(val, &live); err != nil {
		return nil, fmt.Errorf("failed to decode live candle: %w", err)
	}

	return &live, nil
}
