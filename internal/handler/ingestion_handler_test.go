package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hossen/safar-trade/internal/handler"
	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
)

type mockIngester struct {
	ingestFn func(ctx context.Context) (*model.IngestionResult, error)
}

func (m *mockIngester) Ingest(ctx context.Context) (*model.IngestionResult, error) {
	if m.ingestFn != nil {
		return m.ingestFn(ctx)
	}
	return &model.IngestionResult{Status: "success"}, nil
}

type mockEconomicEventRepo struct {
	upsertFn      func(ctx context.Context, event *model.EconomicEvent) (*model.EconomicEvent, error)
	upsertBatchFn func(ctx context.Context, events []model.EconomicEvent) (int, error)
	getByIDFn     func(ctx context.Context, id int64) (*model.EconomicEvent, error)
	listFn        func(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error)
}

func (m *mockEconomicEventRepo) Upsert(ctx context.Context, event *model.EconomicEvent) (*model.EconomicEvent, error) {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, event)
	}
	return event, nil
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
	return nil, repository.ErrNotFound
}

func (m *mockEconomicEventRepo) List(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	return []model.EconomicEvent{}, nil
}

func setupIngestionTestRouter(ingester *mockIngester, repo *mockEconomicEventRepo, jsonPath string) *http.ServeMux {
	h := handler.NewIngestionHandler(ingester, repo, jsonPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func TestIngestionHandler_TriggerForexFactory(t *testing.T) {
	mockIng := &mockIngester{
		ingestFn: func(ctx context.Context) (*model.IngestionResult, error) {
			return &model.IngestionResult{
				Status:      "success",
				TotalDays:   7,
				TotalEvents: 42,
				Upserted:    42,
				DurationMs:  250,
			}, nil
		},
	}
	mockRepo := &mockEconomicEventRepo{}
	router := setupIngestionTestRouter(mockIng, mockRepo, "")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/forex-factory", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var res model.IngestionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Status != "success" || res.Upserted != 42 {
		t.Errorf("unexpected ingestion result: %+v", res)
	}
}

func TestIngestionHandler_ListEconomicEvents(t *testing.T) {
	now := time.Now()
	mockIng := &mockIngester{}

	t.Run("returns events from database when available", func(t *testing.T) {
		mockRepo := &mockEconomicEventRepo{
			listFn: func(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error) {
				return []model.EconomicEvent{
					{
						ID:         1,
						Name:       "CPI y/y",
						Country:    "USD",
						Currency:   "USD",
						ImpactName: "High",
						Actual:     "3.2%",
						Forecast:   "3.1%",
						Previous:   "3.0%",
						Date:       "2026-10-25",
						TimeLabel:  "16:00",
						Dateline:   1729830000000,
						CreatedAt:  now,
						UpdatedAt:  now,
					},
				}, nil
			},
		}
		router := setupIngestionTestRouter(mockIng, mockRepo, "")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/economic-events?limit=10", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if body["count"].(float64) != 1 || body["source"] != "database" {
			t.Errorf("unexpected response body: %+v", body)
		}
	})

	t.Run("falls back to local JSON file when database fails", func(t *testing.T) {
		tmpJSON := "test_fallback.json"
		fallbackContent := `[
			{
				"date": "2026-10-25",
				"events": [
					{
						"id": 99,
						"name": "Fallback GDP",
						"country": "USD",
						"currency": "USD",
						"impactName": "High",
						"dateline": 1729830000,
						"timeLabel": "14:30"
					}
				]
			}
		]`
		_ = os.WriteFile(tmpJSON, []byte(fallbackContent), 0644)
		defer os.Remove(tmpJSON)

		mockRepo := &mockEconomicEventRepo{
			listFn: func(ctx context.Context, limit, offset int32) ([]model.EconomicEvent, error) {
				return nil, errors.New("dial tcp connection refused")
			},
		}

		router := setupIngestionTestRouter(mockIng, mockRepo, tmpJSON)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/economic-events?limit=10", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected fallback status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)

		if body["count"].(float64) != 1 {
			t.Errorf("expected count 1, got %v", body["count"])
		}
	})
}

func TestIngestionHandler_GetEconomicEventByID(t *testing.T) {
	mockIng := &mockIngester{}
	mockRepo := &mockEconomicEventRepo{
		getByIDFn: func(ctx context.Context, id int64) (*model.EconomicEvent, error) {
			if id == 42 {
				return &model.EconomicEvent{
					ID:   42,
					Name: "Non-Farm Employment Change",
				}, nil
			}
			return nil, repository.ErrNotFound
		},
	}
	router := setupIngestionTestRouter(mockIng, mockRepo, "")

	t.Run("found event returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/economic-events/42", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("non-existent event returns 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/economic-events/999", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}
