-- db/queries/economic_events.sql

-- name: UpsertEconomicEvent :one
INSERT INTO economic_events (
    id,
    name,
    country,
    currency,
    impact_name,
    actual,
    previous,
    forecast,
    revision,
    date,
    url,
    time_label,
    dateline,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW()
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    country = EXCLUDED.country,
    currency = EXCLUDED.currency,
    impact_name = EXCLUDED.impact_name,
    actual = EXCLUDED.actual,
    previous = EXCLUDED.previous,
    forecast = EXCLUDED.forecast,
    revision = EXCLUDED.revision,
    date = EXCLUDED.date,
    url = EXCLUDED.url,
    time_label = EXCLUDED.time_label,
    dateline = EXCLUDED.dateline,
    updated_at = NOW()
RETURNING id, name, country, currency, impact_name, actual, previous, forecast, revision, date, url, time_label, dateline, created_at, updated_at;

-- name: GetEconomicEventByID :one
SELECT id, name, country, currency, impact_name, actual, previous, forecast, revision, date, url, time_label, dateline, created_at, updated_at
FROM economic_events
WHERE id = $1;

-- name: ListEconomicEvents :many
SELECT id, name, country, currency, impact_name, actual, previous, forecast, revision, date, url, time_label, dateline, created_at, updated_at
FROM economic_events
ORDER BY dateline DESC
LIMIT $1 OFFSET $2;
