-- name: UpsertCandle :one
INSERT INTO binance_candles (
    symbol, market, open_time, close_time, open, high, low, close, volume, is_closed, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW()
)
ON CONFLICT (symbol, market, open_time)
DO UPDATE SET
    close_time = EXCLUDED.close_time,
    open = EXCLUDED.open,
    high = EXCLUDED.high,
    low = EXCLUDED.low,
    close = EXCLUDED.close,
    volume = EXCLUDED.volume,
    is_closed = EXCLUDED.is_closed,
    updated_at = NOW()
RETURNING *;

-- name: ListCandlesBySymbol :many
SELECT * FROM binance_candles
WHERE symbol = $1 AND market = $2
ORDER BY open_time DESC
LIMIT $3 OFFSET $4;

-- name: GetLatestCandle :one
SELECT * FROM binance_candles
WHERE symbol = $1 AND market = $2
ORDER BY open_time DESC
LIMIT 1;
