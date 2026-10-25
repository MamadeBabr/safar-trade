package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/db"
	"github.com/hossen/safar-trade/internal/model"
)

// CandleRepository defines persistence operations for candlestick market data.
type CandleRepository interface {
	Upsert(ctx context.Context, candle *model.Candle) (*model.Candle, error)
	UpsertBatch(ctx context.Context, candles []model.Candle) (int, error)
	GetLatest(ctx context.Context, symbol, market string) (*model.Candle, error)
	ListBySymbol(ctx context.Context, symbol, market string, limit, offset int32) ([]model.Candle, error)
}

type pgCandleRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

var _ CandleRepository = (*pgCandleRepository)(nil)

func NewCandleRepository(pool *pgxpool.Pool) CandleRepository {
	return &pgCandleRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *pgCandleRepository) Upsert(ctx context.Context, candle *model.Candle) (*model.Candle, error) {
	res, err := r.queries.UpsertCandle(ctx, db.UpsertCandleParams{
		Symbol:    candle.Symbol,
		Market:    candle.Market,
		OpenTime:  candle.OpenTime,
		CloseTime: candle.CloseTime,
		Open:      candle.Open,
		High:      candle.High,
		Low:       candle.Low,
		Close:     candle.Close,
		Volume:    candle.Volume,
		IsClosed:  candle.IsClosed,
	})
	if err != nil {
		return nil, err
	}

	return toDomainCandle(res), nil
}

func (r *pgCandleRepository) UpsertBatch(ctx context.Context, candles []model.Candle) (int, error) {
	if len(candles) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)
	count := 0

	for _, candle := range candles {
		_, err := qtx.UpsertCandle(ctx, db.UpsertCandleParams{
			Symbol:    candle.Symbol,
			Market:    candle.Market,
			OpenTime:  candle.OpenTime,
			CloseTime: candle.CloseTime,
			Open:      candle.Open,
			High:      candle.High,
			Low:       candle.Low,
			Close:     candle.Close,
			Volume:    candle.Volume,
			IsClosed:  candle.IsClosed,
		})
		if err != nil {
			return count, err
		}
		count++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return count, nil
}

func (r *pgCandleRepository) GetLatest(ctx context.Context, symbol, market string) (*model.Candle, error) {
	res, err := r.queries.GetLatestCandle(ctx, db.GetLatestCandleParams{
		Symbol: symbol,
		Market: market,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return toDomainCandle(res), nil
}

func (r *pgCandleRepository) ListBySymbol(ctx context.Context, symbol, market string, limit, offset int32) ([]model.Candle, error) {
	rows, err := r.queries.ListCandlesBySymbol(ctx, db.ListCandlesBySymbolParams{
		Symbol: symbol,
		Market: market,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	candles := make([]model.Candle, len(rows))
	for i, row := range rows {
		candles[i] = *toDomainCandle(row)
	}
	return candles, nil
}

func toDomainCandle(c db.BinanceCandle) *model.Candle {
	return &model.Candle{
		Symbol:    c.Symbol,
		Market:    c.Market,
		OpenTime:  c.OpenTime,
		CloseTime: c.CloseTime,
		Open:      c.Open,
		High:      c.High,
		Low:       c.Low,
		Close:     c.Close,
		Volume:    c.Volume,
		IsClosed:  c.IsClosed,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
