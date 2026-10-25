package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient parses the Redis URL, configures the connection pool, and pings the server.
func NewClient(ctx context.Context, redisURL string, logger *slog.Logger) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}

	// Sane defaults for connection pooling
	opts.PoolSize = 30
	opts.MinIdleConns = 5
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("redis server unreachable during initialization", slog.Any("error", err), slog.String("addr", opts.Addr))
		return client, fmt.Errorf("redis ping failed: %w", err)
	}

	logger.Info("connected to redis", slog.String("addr", opts.Addr))
	return client, nil
}
