package model

import "time"

// Candle represents a single OHLCV candlestick in our domain.
type Candle struct {
	Symbol    string    `json:"symbol"`
	Market    string    `json:"market"` // "spot" or "futures"
	OpenTime  int64     `json:"open_time"`
	CloseTime int64     `json:"close_time"`
	Open      string    `json:"open"`
	High      string    `json:"high"`
	Low       string    `json:"low"`
	Close     string    `json:"close"`
	Volume    string    `json:"volume"`
	IsClosed  bool      `json:"is_closed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
