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

CREATE TABLE IF NOT EXISTS candles_5m (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_5m_lookup ON candles_5m(symbol, t DESC);

CREATE TABLE IF NOT EXISTS candles_15m (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_15m_lookup ON candles_15m(symbol, t DESC);

CREATE TABLE IF NOT EXISTS candles_30m (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_30m_lookup ON candles_30m(symbol, t DESC);

CREATE TABLE IF NOT EXISTS candles_60m (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_60m_lookup ON candles_60m(symbol, t DESC);

CREATE TABLE IF NOT EXISTS candles_240m (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_240m_lookup ON candles_240m(symbol, t DESC);

CREATE TABLE IF NOT EXISTS candles_1d (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_1d_lookup ON candles_1d(symbol, t DESC);

CREATE TABLE IF NOT EXISTS candles_1w (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_1w_lookup ON candles_1w(symbol, t DESC);

CREATE TABLE IF NOT EXISTS candles_1M (
    symbol VARCHAR(50) NOT NULL,
    o VARCHAR(50) NOT NULL,
    h VARCHAR(50) NOT NULL,
    l VARCHAR(50) NOT NULL,
    c VARCHAR(50) NOT NULL,
    v VARCHAR(50) NOT NULL,
    t BIGINT NOT NULL,
    PRIMARY KEY (symbol, t)
);
CREATE INDEX IF NOT EXISTS idx_candles_1M_lookup ON candles_1M(symbol, t DESC);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS candles_1M;
DROP TABLE IF EXISTS candles_1w;
DROP TABLE IF EXISTS candles_1d;
DROP TABLE IF EXISTS candles_240m;
DROP TABLE IF EXISTS candles_60m;
DROP TABLE IF EXISTS candles_30m;
DROP TABLE IF EXISTS candles_15m;
DROP TABLE IF EXISTS candles_5m;
DROP TABLE IF EXISTS candles_1m;
