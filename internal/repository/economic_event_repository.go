package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/db"
	"github.com/hossen/safar-trade/internal/model"
)

// EconomicEventRepository defines persistence operations for economic events.
type EconomicEventRepository interface {
	Upsert(ctx context.Context, event *model.EconomicEvent) (*model.EconomicEvent, error)
	UpsertBatch(ctx context.Context, events []model.EconomicEvent) (int, error)
	GetByID(ctx context.Context, id int64) (*model.EconomicEvent, error)
	List(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error)
}

type pgEconomicEventRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

var _ EconomicEventRepository = (*pgEconomicEventRepository)(nil)

func NewEconomicEventRepository(pool *pgxpool.Pool) EconomicEventRepository {
	return &pgEconomicEventRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *pgEconomicEventRepository) Upsert(ctx context.Context, event *model.EconomicEvent) (*model.EconomicEvent, error) {
	res, err := r.queries.UpsertEconomicEvent(ctx, db.UpsertEconomicEventParams{
		ID:         event.ID,
		Name:       event.Name,
		Country:    event.Country,
		Currency:   event.Currency,
		ImpactName: event.ImpactName,
		Actual:     event.Actual,
		Previous:   event.Previous,
		Forecast:   event.Forecast,
		Revision:   event.Revision,
		Date:       event.Date,
		Url:        event.URL,
		TimeLabel:  event.TimeLabel,
		Dateline:   event.Dateline,
	})
	if err != nil {
		return nil, err
	}

	return toDomainEconomicEvent(res), nil
}

func (r *pgEconomicEventRepository) UpsertBatch(ctx context.Context, events []model.EconomicEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	// Execute within an atomic transaction for database consistency and speed
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)
	count := 0

	for _, event := range events {
		_, err := qtx.UpsertEconomicEvent(ctx, db.UpsertEconomicEventParams{
			ID:         event.ID,
			Name:       event.Name,
			Country:    event.Country,
			Currency:   event.Currency,
			ImpactName: event.ImpactName,
			Actual:     event.Actual,
			Previous:   event.Previous,
			Forecast:   event.Forecast,
			Revision:   event.Revision,
			Date:       event.Date,
			Url:        event.URL,
			TimeLabel:  event.TimeLabel,
			Dateline:   event.Dateline,
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

func (r *pgEconomicEventRepository) GetByID(ctx context.Context, id int64) (*model.EconomicEvent, error) {
	res, err := r.queries.GetEconomicEventByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return toDomainEconomicEvent(res), nil
}

func (r *pgEconomicEventRepository) List(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error) {
	rows, err := r.queries.ListEconomicEvents(ctx, db.ListEconomicEventsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	events := make([]model.EconomicEvent, len(rows))
	for i, row := range rows {
		events[i] = *toDomainEconomicEvent(row)
	}
	return events, nil
}

func toDomainEconomicEvent(e db.EconomicEvent) *model.EconomicEvent {
	return &model.EconomicEvent{
		ID:         e.ID,
		Name:       e.Name,
		Country:    e.Country,
		Currency:   e.Currency,
		ImpactName: e.ImpactName,
		Actual:     e.Actual,
		Previous:   e.Previous,
		Forecast:   e.Forecast,
		Revision:   e.Revision,
		Date:       e.Date,
		URL:        e.Url,
		TimeLabel:  e.TimeLabel,
		Dateline:   e.Dateline,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}
