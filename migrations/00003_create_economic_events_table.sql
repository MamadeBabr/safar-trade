-- +goose Up
-- SQL in this section is executed when the migration is applied.
CREATE TABLE IF NOT EXISTS economic_events (
    id BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    country VARCHAR(10) NOT NULL DEFAULT '',
    currency VARCHAR(10) NOT NULL DEFAULT '',
    impact_name VARCHAR(50) NOT NULL DEFAULT '',
    actual VARCHAR(50) NOT NULL DEFAULT '',
    previous VARCHAR(50) NOT NULL DEFAULT '',
    forecast VARCHAR(50) NOT NULL DEFAULT '',
    revision VARCHAR(50) NOT NULL DEFAULT '',
    date VARCHAR(50) NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    time_label VARCHAR(50) NOT NULL DEFAULT '',
    dateline BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_economic_events_currency ON economic_events(currency);
CREATE INDEX IF NOT EXISTS idx_economic_events_dateline ON economic_events(dateline);
CREATE INDEX IF NOT EXISTS idx_economic_events_impact ON economic_events(impact_name);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS economic_events;
