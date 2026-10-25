package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hossen/safar-trade/internal/model"
)

// CandleStore abstracts operations for persisting live candlestick snapshots in Redis.
type CandleStore interface {
	SetLatestCandle(ctx context.Context, candle *model.Candle) error
	GetLatestCandle(ctx context.Context, symbol string) (*model.Candle, error)
}

type redisCandleStore struct {
	client *redis.Client
	ttl    time.Duration
}

var _ CandleStore = (*redisCandleStore)(nil)

// NewCandleStore creates a new Redis candle store.
func NewCandleStore(client *redis.Client, ttl time.Duration) CandleStore {
	return &redisCandleStore{
		client: client,
		ttl:    ttl,
	}
}

// RedisCandlePayload represents the serialized JSON format stored in Redis for fast client consumption.
type RedisCandlePayload struct {
	Symbol    string `json:"symbol"`
	Market    string `json:"market"`
	Open      string `json:"o"`
	High      string `json:"h"`
	Low       string `json:"l"`
	Close     string `json:"c"`
	Volume    string `json:"v"`
	OpenTime  int64  `json:"openTime"`
	CloseTime int64  `json:"closeTime"`
	IsClosed  bool   `json:"isClosed"`
}

// SetLatestCandle persists the latest candle to Redis under `binance:candle:<symbol>`.
func (s *redisCandleStore) SetLatestCandle(ctx context.Context, candle *model.Candle) error {
	payload := RedisCandlePayload{
		Symbol:    candle.Symbol,
		Market:    candle.Market,
		Open:      candle.Open,
		High:      candle.High,
		Low:       candle.Low,
		Close:     candle.Close,
		Volume:    candle.Volume,
		OpenTime:  candle.OpenTime,
		CloseTime: candle.CloseTime,
		IsClosed:  candle.IsClosed,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal candle payload: %w", err)
	}

	key := fmt.Sprintf("binance:candle:%s", candle.Symbol)
	return s.client.Set(ctx, key, data, s.ttl).Err()
}

// GetLatestCandle fetches the latest candle from Redis by symbol.
func (s *redisCandleStore) GetLatestCandle(ctx context.Context, symbol string) (*model.Candle, error) {
	key := fmt.Sprintf("binance:candle:%s", symbol)
	val, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var payload RedisCandlePayload
	if err := json.Unmarshal(val, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal candle payload: %w", err)
	}

	return &model.Candle{
		Symbol:    payload.Symbol,
		Market:    payload.Market,
		Open:      payload.Open,
		High:      payload.High,
		Low:       payload.Low,
		Close:     payload.Close,
		Volume:    payload.Volume,
		OpenTime:  payload.OpenTime,
		CloseTime: payload.CloseTime,
		IsClosed:  payload.IsClosed,
	}, nil
}
