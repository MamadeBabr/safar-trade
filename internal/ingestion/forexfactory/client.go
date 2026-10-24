package forexfactory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"github.com/hossen/safar-trade/internal/model"
)

const (
	forexFactoryAPIURL = "https://www.forexfactory.com/calendar/apply-settings/1?navigation=0"
	refererHeader      = "https://www.forexfactory.com/calendar?week=this"
	firefoxUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0"
)

// Client defines the contract for fetching raw Forex Factory calendar data.
type Client interface {
	FetchDays(ctx context.Context, beginDate, endDate string) ([]model.RawDay, error)
}

type tlsForexFactoryClient struct {
	httpClient tls_client.HttpClient
	logger     *slog.Logger
	maxRetries int
	retryDelay time.Duration
}

// Compile-time interface assertion
var _ Client = (*tlsForexFactoryClient)(nil)

// NewClient creates a new ForexFactory client using TLS fingerprint impersonation.
func NewClient(logger *slog.Logger) (Client, error) {
	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Firefox_120),
		tls_client.WithInsecureSkipVerify(), // Corresponds to ignoreTlsErrors: true in TypeScript
		tls_client.WithCookieJar(jar),
	}

	httpClient, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize TLS client: %w", err)
	}

	return &tlsForexFactoryClient{
		httpClient: httpClient,
		logger:     logger,
		maxRetries: 3,
		retryDelay: 5 * time.Second, // Graceful retry backoff
	}, nil
}

func (c *tlsForexFactoryClient) FetchDays(ctx context.Context, beginDate, endDate string) ([]model.RawDay, error) {
	payload := map[string]any{
		"default_view": "this_week",
		"begin_date":   beginDate,
		"end_date":     endDate,
		"impacts":      []int{3, 2, 1, 0},
		"currencies":   []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
		"event_types":  []int{1, 2, 3, 4, 5, 7, 8, 9, 10, 11},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var lastErr error

	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		c.logger.InfoContext(ctx, "fetching forex factory data",
			slog.Int("attempt", attempt),
			slog.String("begin_date", beginDate),
			slog.String("end_date", endDate),
		)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			forexFactoryAPIURL,
			bytes.NewReader(bodyBytes),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Referer", refererHeader)
		req.Header.Set("User-Agent", firefoxUserAgent)
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request attempt %d failed: %w", attempt, err)
			c.logger.WarnContext(ctx, "forex factory request error",
				slog.Int("attempt", attempt),
				slog.Any("error", err),
			)
			if attempt < c.maxRetries {
				time.Sleep(c.retryDelay)
			}
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", readErr)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("forex factory returned non-200 status: %d, body: %s", resp.StatusCode, string(respBody))
			c.logger.WarnContext(ctx, "forex factory non-200 status",
				slog.Int("status", resp.StatusCode),
				slog.Int("attempt", attempt),
			)
			if attempt < c.maxRetries {
				time.Sleep(c.retryDelay)
			}
			continue
		}

		var apiResp model.ForexFactoryResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal forex factory response: %w", err)
		}

		c.logger.InfoContext(ctx, "forex factory data successfully fetched",
			slog.Int("days_count", len(apiResp.Days)),
		)

		return apiResp.Days, nil
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", c.maxRetries, lastErr)
}
