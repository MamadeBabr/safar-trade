package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
	redisStore "github.com/hossen/safar-trade/internal/storage/redis"
)

var (
	ErrSubscriptionNotFound = errors.New("active subscription not found")
	ErrPackageNotFound      = errors.New("subscription package not found")
)

// SubscriptionService manages packages, subscriptions, and Redis caching.
type SubscriptionService interface {
	ListPackages(ctx context.Context) ([]model.PackageResponse, error)
	GetActiveSubscription(ctx context.Context, userID int64) (*model.SubscriptionResponse, error)
	Subscribe(ctx context.Context, userID int64, req model.SubscribeRequest) (*model.SubscriptionResponse, error)
	Cancel(ctx context.Context, userID int64) error
	HasActiveSubscription(ctx context.Context, userID int64) (bool, error)
}

type subscriptionService struct {
	subRepo     repository.SubscriptionRepository
	pkgRepo     repository.PackageRepository
	cacheStore  redisStore.SubscriptionStore
	cacheTTL    time.Duration
}

var _ SubscriptionService = (*subscriptionService)(nil)

// NewSubscriptionService constructs a SubscriptionService.
func NewSubscriptionService(
	subRepo repository.SubscriptionRepository,
	pkgRepo repository.PackageRepository,
	cacheStore redisStore.SubscriptionStore,
	cacheTTL time.Duration,
) SubscriptionService {
	return &subscriptionService{
		subRepo:    subRepo,
		pkgRepo:    pkgRepo,
		cacheStore: cacheStore,
		cacheTTL:   cacheTTL,
	}
}

func (s *subscriptionService) ListPackages(ctx context.Context) ([]model.PackageResponse, error) {
	pkgs, err := s.pkgRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	resps := make([]model.PackageResponse, len(pkgs))
	for i, p := range pkgs {
		resps[i] = p.ToResponse()
	}
	return resps, nil
}

func (s *subscriptionService) GetActiveSubscription(ctx context.Context, userID int64) (*model.SubscriptionResponse, error) {
	// 1. Try Redis cache first
	if s.cacheStore != nil {
		cached, err := s.cacheStore.Get(ctx, userID)
		if err == nil && cached != nil {
			if cached.IsActive && time.Now().Before(cached.ExpiresAt) {
				return &model.SubscriptionResponse{
					UserID:      cached.UserID,
					PackageID:   cached.PackageID,
					PackageName: cached.PackageName,
					Status:      model.SubscriptionStatusActive,
					IsActive:    true,
					ExpiresAt:   cached.ExpiresAt,
				}, nil
			}
		}
	}

	// 2. Cache miss: Fetch from PostgreSQL
	sub, err := s.subRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}

	// 3. Cache in Redis
	s.cacheSubscription(ctx, sub)

	resp := sub.ToResponse()
	return &resp, nil
}

func (s *subscriptionService) Subscribe(ctx context.Context, userID int64, req model.SubscribeRequest) (*model.SubscriptionResponse, error) {
	// Verify package exists and is active
	pkg, err := s.pkgRepo.GetByID(ctx, req.PackageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, pkg.DurationDays)

	sub := &model.Subscription{
		UserID:      userID,
		PackageID:   pkg.ID,
		Status:      model.SubscriptionStatusActive,
		StartsAt:    now,
		ExpiresAt:   expiresAt,
		PackageName: pkg.Name,
	}

	created, err := s.subRepo.Create(ctx, sub)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}
	created.PackageName = pkg.Name

	// Invalidate previous cache and populate with fresh data
	if s.cacheStore != nil {
		_ = s.cacheStore.Invalidate(ctx, userID)
		s.cacheSubscription(ctx, created)
	}

	resp := created.ToResponse()
	return &resp, nil
}

func (s *subscriptionService) Cancel(ctx context.Context, userID int64) error {
	err := s.subRepo.CancelActiveByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return err
	}

	// Invalidate cache immediately on change
	if s.cacheStore != nil {
		_ = s.cacheStore.Invalidate(ctx, userID)
	}

	return nil
}

func (s *subscriptionService) HasActiveSubscription(ctx context.Context, userID int64) (bool, error) {
	// 1. Check Redis cache
	if s.cacheStore != nil {
		cached, err := s.cacheStore.Get(ctx, userID)
		if err == nil && cached != nil {
			return cached.IsActive && time.Now().Before(cached.ExpiresAt), nil
		}
	}

	// 2. Query DB on cache miss
	sub, err := s.subRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	// Cache result in Redis
	s.cacheSubscription(ctx, sub)

	return sub.IsActive(), nil
}

func (s *subscriptionService) cacheSubscription(ctx context.Context, sub *model.Subscription) {
	if s.cacheStore == nil || sub == nil {
		return
	}

	ttl := s.cacheTTL
	timeRemaining := time.Until(sub.ExpiresAt)
	if timeRemaining < ttl {
		ttl = timeRemaining
	}
	if ttl <= 0 {
		return
	}

	cached := &model.CachedSubscription{
		UserID:      sub.UserID,
		IsActive:    sub.IsActive(),
		PackageID:   sub.PackageID,
		PackageName: sub.PackageName,
		ExpiresAt:   sub.ExpiresAt,
	}

	_ = s.cacheStore.Set(ctx, cached, ttl)
}
