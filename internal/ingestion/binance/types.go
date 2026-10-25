package binance

// RawKlineStreamMessage handles stream multiplexed messages from Binance.
// Format: {"stream":"btcusdt@kline_1m","data":{"e":"kline",...}}
type RawKlineStreamMessage struct {
	Stream string          `json:"stream"`
	Data   *RawKlineEvent  `json:"data"`
}

// RawKlineEvent represents the event payload from Binance WebSocket.
type RawKlineEvent struct {
	EventType string         `json:"e"` // "kline"
	EventTime int64          `json:"E"`
	Symbol    string         `json:"s"`
	Kline     RawKlineDetail `json:"k"`
}

// RawKlineDetail contains the detailed candlestick data.
type RawKlineDetail struct {
	StartTime    int64  `json:"t"` // Kline start time
	EndTime      int64  `json:"T"` // Kline close time
	Symbol       string `json:"s"` // Symbol
	Interval     string `json:"i"` // Interval (e.g. "1m")
	FirstTradeID int64  `json:"f"` // First trade ID
	LastTradeID  int64  `json:"L"` // Last trade ID (must be explicit to avoid collision with "l")
	Open         string `json:"o"` // Open price
	Close        string `json:"c"` // Close price
	High         string `json:"h"` // High price
	Low          string `json:"l"` // Low price
	Volume       string `json:"v"` // Base asset volume
	NumTrades    int    `json:"n"` // Number of trades
	IsClosed     bool   `json:"x"` // Is this kline closed?
	QuoteVolume  string `json:"q"` // Quote asset volume
	TakerBuyVol  string `json:"V"` // Taker buy base asset volume (must be explicit to avoid collision with "v")
	TakerBuyQVol string `json:"Q"` // Taker buy quote asset volume (must be explicit to avoid collision with "q")
}

// WSCommand represents the JSON-RPC command sent to Binance WebSocket.
type WSCommand struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int      `json:"id"`
}
