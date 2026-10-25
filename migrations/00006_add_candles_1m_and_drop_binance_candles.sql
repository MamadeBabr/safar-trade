-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE IF NOT EXISTS candles_1m (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_1m_lookup ON candles_1m(symbol, t DESC);

DROP TABLE IF EXISTS binance_candles;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS candles_1m;
