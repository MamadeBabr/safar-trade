package forexfactory_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/hossen/safar-trade/internal/ingestion/forexfactory"
	"github.com/hossen/safar-trade/internal/model"
)

type mockClient struct {
	fetchDaysFn func(ctx context.Context, beginDate, endDate string) ([]model.RawDay, error)
}

func (m *mockClient) FetchDays(ctx context.Context, beginDate, endDate string) ([]model.RawDay, error) {
	if m.fetchDaysFn != nil {
		return m.fetchDaysFn(ctx, beginDate, endDate)
	}
	return nil, errors.New("fetchDaysFn not implemented")
}

type mockEconomicEventRepo struct {
	upsertBatchFn func(ctx context.Context, events []model.EconomicEvent) (int, error)
	upsertFn      func(ctx context.Context, event *model.EconomicEvent) (*model.EconomicEvent, error)
	getByIDFn     func(ctx context.Context, id int64) (*model.EconomicEvent, error)
	listFn        func(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error)
}

func (m *mockEconomicEventRepo) Upsert(ctx context.Context, event *model.EconomicEvent) (*model.EconomicEvent, error) {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, event)
	}
	return nil, errors.New("upsertFn not implemented")
}

func (m *mockEconomicEventRepo) UpsertBatch(ctx context.Context, events []model.EconomicEvent) (int, error) {
	if m.upsertBatchFn != nil {
		return m.upsertBatchFn(ctx, events)
	}
	return len(events), nil
}

func (m *mockEconomicEventRepo) GetByID(ctx context.Context, id int64) (*model.EconomicEvent, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("getByIDFn not implemented")
}

func (m *mockEconomicEventRepo) List(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	return nil, errors.New("listFn not implemented")
}

func TestIngester_Ingest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tmpFile := "test_forexfactory.json"
	defer os.Remove(tmpFile)

	sampleDays := []model.RawDay{
		{
			Date: "2026-10-25",
			Events: []model.RawEvent{
				{
					ID:         json.Number("101"),
					Name:       "Fed Chair Powell Speaks",
					Country:    "USD",
					Dateline:   json.Number("1729830000"),
					Currency:   "USD",
					ImpactName: "High",
					Actual:     "5.25%",
					Previous:   "5.50%",
					Forecast:   "5.25%",
					Revision:   "",
					Date:       "2026-10-25",
					URL:        "/calendar/101",
					TimeLabel:  "12:30pm",
				},
			},
		},
	}

	t.Run("successfully fetches, transforms, and stores economic events", func(t *testing.T) {
		clientMock := &mockClient{
			fetchDaysFn: func(ctx context.Context, beginDate, endDate string) ([]model.RawDay, error) {
				return sampleDays, nil
			},
		}

		var savedEvents []model.EconomicEvent
		repoMock := &mockEconomicEventRepo{
			upsertBatchFn: func(ctx context.Context, events []model.EconomicEvent) (int, error) {
				savedEvents = events
				return len(events), nil
			},
		}

		ingester := forexfactory.NewIngester(clientMock, repoMock, logger, tmpFile)
		res, err := ingester.Ingest(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.TotalEvents != 1 || res.Upserted != 1 {
			t.Errorf("expected 1 event upserted, got %d", res.Upserted)
		}

		if len(savedEvents) != 1 {
			t.Fatalf("expected 1 saved event, got %d", len(savedEvents))
		}

		event := savedEvents[0]
		if event.ID != 101 {
			t.Errorf("expected ID 101, got %d", event.ID)
		}
		if event.Dateline != 1729830000000 {
			t.Errorf("expected dateline in milliseconds 1729830000000, got %d", event.Dateline)
		}
		if event.Currency != "USD" || event.ImpactName != "High" {
			t.Errorf("unexpected event data: %+v", event)
		}

		// Verify local JSON copy was written
		if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
			t.Errorf("expected JSON file %s to be created", tmpFile)
		}
	})

	t.Run("gracefully falls back to JSON when database is offline", func(t *testing.T) {
		defer os.Remove(tmpFile)

		clientMock := &mockClient{
			fetchDaysFn: func(ctx context.Context, beginDate, endDate string) ([]model.RawDay, error) {
				return sampleDays, nil
			},
		}
		repoMock := &mockEconomicEventRepo{
			upsertBatchFn: func(ctx context.Context, events []model.EconomicEvent) (int, error) {
				return 0, errors.New("dial tcp connection refused")
			},
		}

		ingester := forexfactory.NewIngester(clientMock, repoMock, logger, tmpFile)
		res, err := ingester.Ingest(context.Background())

		if err != nil {
			t.Fatalf("expected ingestion to succeed gracefully with JSON fallback, got: %v", err)
		}

		if res.Upserted != 0 {
			t.Errorf("expected 0 upserted to DB, got %d", res.Upserted)
		}
		if res.TotalEvents != 1 {
			t.Errorf("expected 1 total event, got %d", res.TotalEvents)
		}

		// Verify JSON file exists despite DB failure
		if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
			t.Fatalf("expected fallback JSON file %s to exist", tmpFile)
		}
	})

	t.Run("handles client fetch errors gracefully", func(t *testing.T) {
		clientMock := &mockClient{
			fetchDaysFn: func(ctx context.Context, beginDate, endDate string) ([]model.RawDay, error) {
				return nil, errors.New("network timeout")
			},
		}
		repoMock := &mockEconomicEventRepo{}

		ingester := forexfactory.NewIngester(clientMock, repoMock, logger, "")
		_, err := ingester.Ingest(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
