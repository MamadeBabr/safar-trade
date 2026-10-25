package binance

import (
	"encoding/json"
	"testing"
)

func TestChunkSlice(t *testing.T) {
	items := []string{"s1", "s2", "s3", "s4", "s5"}
	chunks := chunkSlice(items, 2)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || chunks[0][0] != "s1" || chunks[0][1] != "s2" {
		t.Errorf("unexpected chunk 0: %v", chunks[0])
	}
	if len(chunks[2]) != 1 || chunks[2][0] != "s5" {
		t.Errorf("unexpected chunk 2: %v", chunks[2])
	}
}

func TestKlineMessageParsing(t *testing.T) {
	rawJSON := []byte(`{
		"stream": "btcusdt@kline_1m",
		"data": {
			"e": "kline",
			"E": 1729830005000,
			"s": "BTCUSDT",
			"k": {
				"t": 1729830000000,
				"T": 1729830059999,
				"s": "BTCUSDT",
				"i": "1m",
				"o": "67000.50",
				"c": "67120.00",
				"h": "67150.00",
				"l": "66980.00",
				"v": "15.423",
				"x": true
			}
		}
	}`)

	var msg RawKlineStreamMessage
	if err := json.Unmarshal(rawJSON, &msg); err != nil {
		t.Fatalf("failed to unmarshal kline stream message: %v", err)
	}

	if msg.Stream != "btcusdt@kline_1m" {
		t.Errorf("expected stream btcusdt@kline_1m, got %s", msg.Stream)
	}
	if msg.Data == nil || msg.Data.Symbol != "BTCUSDT" {
		t.Fatalf("expected symbol BTCUSDT, got %v", msg.Data)
	}
	if msg.Data.Kline.Close != "67120.00" {
		t.Errorf("expected close 67120.00, got %s", msg.Data.Kline.Close)
	}
	if !msg.Data.Kline.IsClosed {
		t.Errorf("expected isClosed true, got false")
	}
}
