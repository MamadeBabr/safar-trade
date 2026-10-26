-- +goose Up
-- Drop the legacy binance_candles table if it still exists
DROP TABLE IF EXISTS binance_candles CASCADE;

-- +goose Down
-- Table is permanently superseded by timeframe tables (candles_1m, candles_5m, etc.)
