package candle

import "time"

// Timeframe represents a candlestick interval.
type Timeframe string

const (
	TF1m   Timeframe = "1m"
	TF5m   Timeframe = "5m"
	TF15m  Timeframe = "15m"
	TF30m  Timeframe = "30m"
	TF60m  Timeframe = "60m"
	TF240m Timeframe = "240m"
	TF1d   Timeframe = "1d"
	TF1w   Timeframe = "1w"
	TF1M   Timeframe = "1M"
)

// AllTimeframes contains all supported higher timeframes in processing order.
var AllTimeframes = []Timeframe{
	TF1m,
	TF5m,
	TF15m,
	TF30m,
	TF60m,
	TF240m,
	TF1d,
	TF1w,
	TF1M,
}

const (
	ms1m       = 60 * 1000
	ms5m       = 5 * 60 * 1000
	ms15m      = 15 * 60 * 1000
	ms30m      = 30 * 60 * 1000
	ms60m      = 60 * 60 * 1000
	ms240m     = 240 * 60 * 1000
	ms1d       = 24 * 60 * 60 * 1000
	ms1w       = 7 * 24 * 60 * 60 * 1000
	weekOffset = 3 * 24 * 60 * 60 * 1000 // 1970-01-01 was Thursday (3 days after Monday)
)

// TableName returns the exact database table name for a timeframe.
func (tf Timeframe) TableName() string {
	if tf == TF1M {
		return `"candles_1M"`
	}
	return "candles_" + string(tf)
}

// CalculateBucketTime calculates the open timestamp of the timeframe bucket in UTC milliseconds.
// Designed for ultra-high performance: pure integer arithmetic without heap allocations for intraday, daily, and weekly.
func CalculateBucketTime(t int64, tf Timeframe) int64 {
	switch tf {
	case TF1m:
		return t - (t % ms1m)
	case TF5m:
		return t - (t % ms5m)
	case TF15m:
		return t - (t % ms15m)
	case TF30m:
		return t - (t % ms30m)
	case TF60m:
		return t - (t % ms60m)
	case TF240m:
		return t - (t % ms240m)
	case TF1d:
		return t - (t % ms1d)
	case TF1w:
		return t - ((t + weekOffset) % ms1w)
	case TF1M:
		tm := time.UnixMilli(t).UTC()
		return time.Date(tm.Year(), tm.Month(), 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	default:
		return t
	}
}

// ParseTimeframe parses and validates a string into a supported Timeframe.
// Also supports common aliases such as 1h, 4h, 1D, 1W.
func ParseTimeframe(s string) (Timeframe, bool) {
	switch s {
	case "1m", "5m", "15m", "30m", "60m", "240m", "1d", "1w", "1M":
		return Timeframe(s), true
	case "1h", "60":
		return TF60m, true
	case "4h", "240":
		return TF240m, true
	case "1D", "D":
		return TF1d, true
	case "1W", "W":
		return TF1w, true
	default:
		return "", false
	}
}
