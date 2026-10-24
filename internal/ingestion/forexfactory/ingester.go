package forexfactory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
)

// Ingester coordinates fetching, parsing, and persisting Forex Factory economic events.
type Ingester interface {
	Ingest(ctx context.Context) (*model.IngestionResult, error)
}

type ingester struct {
	client   Client
	repo     repository.EconomicEventRepository
	logger   *slog.Logger
	jsonPath string
}

var _ Ingester = (*ingester)(nil)

// NewIngester initializes an Ingester instance.
func NewIngester(client Client, repo repository.EconomicEventRepository, logger *slog.Logger, jsonPath string) Ingester {
	if jsonPath == "" {
		jsonPath = "forexfactory.json"
	}
	return &ingester{
		client:   client,
		repo:     repo,
		logger:   logger,
		jsonPath: jsonPath,
	}
}

func (ing *ingester) Ingest(ctx context.Context) (*model.IngestionResult, error) {
	start := time.Now()

	today := time.Now().Format("2006-01-02")
	nextWeek := time.Now().AddDate(0, 0, 7).Format("2006-01-02")

	// 1. Fetch live economic calendar data from Forex Factory using TLS fingerprint client
	days, err := ing.client.FetchDays(ctx, today, nextWeek)
	if err != nil {
		return nil, fmt.Errorf("ingestion fetch failed: %w", err)
	}

	// 2. Parse and map raw events into domain EconomicEvent structures
	var eventsToSave []model.EconomicEvent

	for _, day := range days {
		if len(day.Events) == 0 {
			continue
		}

		for _, raw := range day.Events {
			id, err := raw.ID.Int64()
			if err != nil {
				ing.logger.WarnContext(ctx, "skipping event with invalid id",
					slog.String("raw_id", raw.ID.String()),
					slog.String("name", raw.Name),
				)
				continue
			}

			rawDateline, _ := raw.Dateline.Int64()
			datelineMs := rawDateline * 1000
			timeLabel := FormatTehranTime(rawDateline, raw.TimeLabel)

			event := model.EconomicEvent{
				ID:         id,
				Name:       raw.Name,
				Country:    raw.Country,
				Currency:   raw.Currency,
				ImpactName: raw.ImpactName,
				Actual:     raw.Actual,
				Previous:   raw.Previous,
				Forecast:   raw.Forecast,
				Revision:   raw.Revision,
				Date:       raw.Date,
				URL:        raw.URL,
				TimeLabel:  timeLabel,
				Dateline:   datelineMs,
			}

			eventsToSave = append(eventsToSave, event)
		}
	}

	// 3. ALWAYS write to JSON fallback FIRST so data is preserved even if PostgreSQL is offline
	var jsonSaved bool
	if ing.jsonPath != "" {
		if err := writeJSONFile(ing.jsonPath, days); err != nil {
			ing.logger.WarnContext(ctx, "failed to write local forexfactory.json fallback", slog.Any("error", err))
		} else {
			jsonSaved = true
			ing.logger.InfoContext(ctx, "saved local fallback to JSON", slog.String("path", ing.jsonPath))
		}
	}

	// 4. Persist to PostgreSQL via batch upsert
	upsertedCount, dbErr := ing.repo.UpsertBatch(ctx, eventsToSave)
	duration := time.Since(start)

	if dbErr != nil {
		ing.logger.WarnContext(ctx, "database unavailable, relying on json fallback",
			slog.Any("db_error", dbErr),
			slog.Bool("json_saved", jsonSaved),
		)

		if jsonSaved {
			// Gracefully succeed with degraded status notice
			return &model.IngestionResult{
				Status:      "degraded (saved to json fallback, database unavailable)",
				TotalDays:   len(days),
				TotalEvents: len(eventsToSave),
				Upserted:    0,
				DurationMs:  duration.Milliseconds(),
			}, nil
		}

		return nil, fmt.Errorf("failed to save to database (%v) and json fallback failed", dbErr)
	}

	result := &model.IngestionResult{
		Status:      "success",
		TotalDays:   len(days),
		TotalEvents: len(eventsToSave),
		Upserted:    upsertedCount,
		DurationMs:  duration.Milliseconds(),
	}

	ing.logger.InfoContext(ctx, "forex factory ingestion completed",
		slog.Int("days", result.TotalDays),
		slog.Int("events", result.TotalEvents),
		slog.Int("upserted", result.Upserted),
		slog.Duration("duration", duration),
	)

	return result, nil
}

func writeJSONFile(filePath string, data any) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, bytes, 0644)
}

// ReadEventsFromJSON reads raw days from the fallback JSON file and converts them to domain EconomicEvents.
func ReadEventsFromJSON(filePath string, limit, offset int32) ([]model.EconomicEvent, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var days []model.RawDay
	if err := json.Unmarshal(bytes, &days); err != nil {
		return nil, err
	}

	var events []model.EconomicEvent
	for _, day := range days {
		for _, raw := range day.Events {
			id, _ := raw.ID.Int64()
			rawDateline, _ := raw.Dateline.Int64()
			events = append(events, model.EconomicEvent{
				ID:         id,
				Name:       raw.Name,
				Country:    raw.Country,
				Currency:   raw.Currency,
				ImpactName: raw.ImpactName,
				Actual:     raw.Actual,
				Previous:   raw.Previous,
				Forecast:   raw.Forecast,
				Revision:   raw.Revision,
				Date:       raw.Date,
				URL:        raw.URL,
				TimeLabel:  FormatTehranTime(rawDateline, raw.TimeLabel),
				Dateline:   rawDateline * 1000,
			})
		}
	}

	// Apply offset & limit
	total := int32(len(events))
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []model.EconomicEvent{}, nil
	}

	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}

	return events[offset:end], nil
}
