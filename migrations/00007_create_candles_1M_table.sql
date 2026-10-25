-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE IF NOT EXISTS "candles_1M" (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_1M_lookup ON "candles_1M"(symbol, t DESC);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS "candles_1M";
