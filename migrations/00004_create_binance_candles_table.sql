-- +goose Up
-- SQL in this section is executed when the migration is applied.
CREATE TABLE IF NOT EXISTS binance_candles (
    symbol VARCHAR(50) NOT NULL,
    market VARCHAR(20) NOT NULL,
    open_time BIGINT NOT NULL,
    close_time BIGINT NOT NULL,
    open VARCHAR(50) NOT NULL,
    high VARCHAR(50) NOT NULL,
    low VARCHAR(50) NOT NULL,
    close VARCHAR(50) NOT NULL,
    volume VARCHAR(50) NOT NULL,
    is_closed BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (symbol, market, open_time)
);

CREATE INDEX IF NOT EXISTS idx_binance_candles_symbol_time ON binance_candles(symbol, open_time DESC);
CREATE INDEX IF NOT EXISTS idx_binance_candles_market ON binance_candles(market);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS binance_candles;
