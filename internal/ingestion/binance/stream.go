package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/hossen/safar-trade/internal/model"
)

const (
	MaxStreamsPerConnection = 200
	SubscribeBatchSize      = 50
	SubscribeInterval       = 500 * time.Millisecond
	RestartInterval         = 12 * time.Hour
	MaxReconnectDelay       = 30 * time.Second
)

// StreamOptions defines parameters for a Binance market stream.
type StreamOptions struct {
	URL          string
	Market       string // "spot" or "futures"
	Symbols      []string
	SymbolSuffix string // e.g. "_FUTURES" or ""
	OnCandle     func(candle *model.Candle)
}

// Stream manages sharded WebSocket connections to Binance Kline streams.
type Stream struct {
	opts   StreamOptions
	logger *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	reqIDCounter int64
}

// NewStream creates a new Binance WebSocket stream manager.
func NewStream(opts StreamOptions, logger *slog.Logger) *Stream {
	return &Stream{
		opts:   opts,
		logger: logger.With(slog.String("market", opts.Market)),
	}
}

// Start begins sharded WebSocket connections and streaming.
func (s *Stream) Start(parentCtx context.Context) {
	s.ctx, s.cancel = context.WithCancel(parentCtx)

	streams := make([]string, 0, len(s.opts.Symbols))
	for _, sym := range s.opts.Symbols {
		streams = append(streams, fmt.Sprintf("%s@kline_1m", strings.ToLower(sym)))
	}

	chunks := chunkSlice(streams, MaxStreamsPerConnection)
	s.logger.Info("distributing streams across connections",
		slog.Int("total_streams", len(streams)),
		slog.Int("connections", len(chunks)),
	)

	for i, chunk := range chunks {
		s.wg.Add(1)
		go s.runConnection(chunk, i)
	}
}

// Stop closes all active connections and waits for goroutines to terminate.
func (s *Stream) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.logger.Info("all stream connections stopped")
}

func (s *Stream) runConnection(streams []string, connIdx int) {
	defer s.wg.Done()

	log := s.logger.With(slog.Int("conn", connIdx))
	reconnectAttempts := 0

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		err := s.connectAndServe(log, streams)
		if s.ctx.Err() != nil {
			return // Context cancelled, exit cleanly
		}

		reconnectAttempts++
		delaySec := math.Min(30, math.Pow(2, math.Min(float64(reconnectAttempts), 5)))
		delay := time.Duration(delaySec) * time.Second

		log.Warn("connection dropped, reconnecting",
			slog.Int("attempt", reconnectAttempts),
			slog.Duration("delay", delay),
			slog.Any("error", err),
		)

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (s *Stream) connectAndServe(log *slog.Logger, streams []string) error {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 15 * time.Second,
	}

	conn, _, err := dialer.DialContext(s.ctx, s.opts.URL, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	log.Info("websocket connected, subscribing to streams", slog.Int("count", len(streams)))

	// Channel for serializing WebSocket writes
	writeChan := make(chan []byte, 100)
	writeDone := make(chan struct{})

	go func() {
		defer close(writeDone)
		for {
			select {
			case <-s.ctx.Done():
				return
			case msg, ok := <-writeChan:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Warn("failed to write to websocket", slog.Any("error", err))
					return
				}
			}
		}
	}()

	// Subscribe to streams in rate-limited batches
	go func() {
		batchChunks := chunkSlice(streams, SubscribeBatchSize)
		for _, batch := range batchChunks {
			select {
			case <-s.ctx.Done():
				return
			default:
				id := atomic.AddInt64(&s.reqIDCounter, 1)
				cmd := WSCommand{
					Method: "SUBSCRIBE",
					Params: batch,
					ID:     int(id),
				}
				bytes, err := json.Marshal(cmd)
				if err == nil {
					select {
					case writeChan <- bytes:
					case <-s.ctx.Done():
						return
					}
				}
				time.Sleep(SubscribeInterval)
			}
		}
		log.Info("all subscribe batches sent for connection")
	}()

	// Scheduled connection renewal to avoid 24h Binance disconnects
	restartTicker := time.NewTicker(RestartInterval)
	defer restartTicker.Stop()

	// Read loop
	readErrChan := make(chan error, 1)
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				readErrChan <- err
				return
			}
			s.handleMessage(message)
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second))
			close(writeChan)
			<-writeDone
			return nil

		case <-restartTicker.C:
			log.Info("scheduled connection restart triggered")
			_ = conn.Close()
			close(writeChan)
			<-writeDone
			return nil

		case err := <-readErrChan:
			close(writeChan)
			<-writeDone
			return err
		}
	}
}

func (s *Stream) handleMessage(raw []byte) {
	// First attempt parsing as stream envelope: {"stream": "...", "data": {...}}
	var env RawKlineStreamMessage
	if err := json.Unmarshal(raw, &env); err == nil && env.Data != nil && env.Data.EventType == "kline" {
		s.dispatchCandle(env.Data)
		return
	}

	// Fallback to direct event message: {"e": "kline", ...}
	var directEvent RawKlineEvent
	if err := json.Unmarshal(raw, &directEvent); err == nil && directEvent.EventType == "kline" {
		s.dispatchCandle(&directEvent)
		return
	}
}

func (s *Stream) dispatchCandle(event *RawKlineEvent) {
	if event == nil || event.Kline.Symbol == "" {
		return
	}

	k := event.Kline
	candle := &model.Candle{
		Symbol:    fmt.Sprintf("%s%s", event.Symbol, s.opts.SymbolSuffix),
		Market:    s.opts.Market,
		OpenTime:  k.StartTime,
		CloseTime: k.EndTime,
		Open:      k.Open,
		High:      k.High,
		Low:       k.Low,
		Close:     k.Close,
		Volume:    k.Volume,
		IsClosed:  k.IsClosed,
	}

	if s.opts.OnCandle != nil {
		s.opts.OnCandle(candle)
	}
}

func chunkSlice(items []string, chunkSize int) [][]string {
	if chunkSize <= 0 || len(items) == 0 {
		return nil
	}
	var chunks [][]string
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}
