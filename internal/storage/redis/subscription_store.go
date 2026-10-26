package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hossen/safar-trade/internal/model"
)

// SubscriptionStore defines operations for caching user subscription state in Redis.
type SubscriptionStore interface {
	Get(ctx context.Context, userID int64) (*model.CachedSubscription, error)
	Set(ctx context.Context, sub *model.CachedSubscription, ttl time.Duration) error
	Invalidate(ctx context.Context, userID int64) error
}

type subscriptionStore struct {
	client *redis.Client
}

// Compile-time check
var _ SubscriptionStore = (*subscriptionStore)(nil)

// NewSubscriptionStore creates a new Redis subscription cache store.
func NewSubscriptionStore(client *redis.Client) SubscriptionStore {
	return &subscriptionStore{client: client}
}

func cacheKey(userID int64) string {
	return fmt.Sprintf("sub:user:%d", userID)
}

// Get retrieves cached subscription info. Returns nil, nil on cache miss.
func (s *subscriptionStore) Get(ctx context.Context, userID int64) (*model.CachedSubscription, error) {
	if s.client == nil {
		return nil, nil // Graceful degradation if redis is disabled/nil
	}

	val, err := s.client.Get(ctx, cacheKey(userID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get cached subscription: %w", err)
	}

	var cached model.CachedSubscription
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached subscription: %w", err)
	}

	return &cached, nil
}

// Set saves subscription info in Redis with the given TTL.
func (s *subscriptionStore) Set(ctx context.Context, sub *model.CachedSubscription, ttl time.Duration) error {
	if s.client == nil || sub == nil {
		return nil
	}

	data, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription for cache: %w", err)
	}

	if err := s.client.Set(ctx, cacheKey(sub.UserID), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to cache subscription: %w", err)
	}

	return nil
}

// Invalidate removes the user's subscription from Redis immediately.
func (s *subscriptionStore) Invalidate(ctx context.Context, userID int64) error {
	if s.client == nil {
		return nil
	}

	if err := s.client.Del(ctx, cacheKey(userID)).Err(); err != nil {
		return fmt.Errorf("failed to invalidate cached subscription: %w", err)
	}

	return nil
}
