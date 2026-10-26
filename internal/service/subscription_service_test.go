package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/service"
)

type mockSubscriptionRepository struct {
	createFn               func(ctx context.Context, sub *model.Subscription) (*model.Subscription, error)
	getActiveByUserIDFn    func(ctx context.Context, userID int64) (*model.Subscription, error)
	cancelActiveByUserIDFn func(ctx context.Context, userID int64) error
}

func (m *mockSubscriptionRepository) Create(ctx context.Context, sub *model.Subscription) (*model.Subscription, error) {
	if m.createFn != nil {
		return m.createFn(ctx, sub)
	}
	return nil, errors.New("createFn not implemented")
}

func (m *mockSubscriptionRepository) GetActiveByUserID(ctx context.Context, userID int64) (*model.Subscription, error) {
	if m.getActiveByUserIDFn != nil {
		return m.getActiveByUserIDFn(ctx, userID)
	}
	return nil, errors.New("getActiveByUserIDFn not implemented")
}

func (m *mockSubscriptionRepository) CancelActiveByUserID(ctx context.Context, userID int64) error {
	if m.cancelActiveByUserIDFn != nil {
		return m.cancelActiveByUserIDFn(ctx, userID)
	}
	return nil
}

type mockPackageRepository struct {
	listActiveFn func(ctx context.Context) ([]model.Package, error)
	getByIDFn    func(ctx context.Context, id int64) (*model.Package, error)
}

func (m *mockPackageRepository) ListActive(ctx context.Context) ([]model.Package, error) {
	if m.listActiveFn != nil {
		return m.listActiveFn(ctx)
	}
	return nil, nil
}

func (m *mockPackageRepository) GetByID(ctx context.Context, id int64) (*model.Package, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("getByIDFn not implemented")
}

type mockSubscriptionStore struct {
	getFn        func(ctx context.Context, userID int64) (*model.CachedSubscription, error)
	setFn        func(ctx context.Context, sub *model.CachedSubscription, ttl time.Duration) error
	invalidateFn func(ctx context.Context, userID int64) error
}

func (m *mockSubscriptionStore) Get(ctx context.Context, userID int64) (*model.CachedSubscription, error) {
	if m.getFn != nil {
		return m.getFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockSubscriptionStore) Set(ctx context.Context, sub *model.CachedSubscription, ttl time.Duration) error {
	if m.setFn != nil {
		return m.setFn(ctx, sub, ttl)
	}
	return nil
}

func (m *mockSubscriptionStore) Invalidate(ctx context.Context, userID int64) error {
	if m.invalidateFn != nil {
		return m.invalidateFn(ctx, userID)
	}
	return nil
}

func TestSubscriptionService_GetActive_CacheHit(t *testing.T) {
	cacheStore := &mockSubscriptionStore{
		getFn: func(ctx context.Context, userID int64) (*model.CachedSubscription, error) {
			return &model.CachedSubscription{
				UserID:      1,
				IsActive:    true,
				PackageID:   2,
				PackageName: "Pro",
				ExpiresAt:   time.Now().Add(24 * time.Hour),
			}, nil
		},
	}
	subRepo := &mockSubscriptionRepository{
		getActiveByUserIDFn: func(ctx context.Context, userID int64) (*model.Subscription, error) {
			t.Fatal("DB should not be queried on cache hit")
			return nil, nil
		},
	}
	pkgRepo := &mockPackageRepository{}

	svc := service.NewSubscriptionService(subRepo, pkgRepo, cacheStore, 15*time.Minute)

	sub, err := svc.GetActiveSubscription(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected active subscription, got %v", err)
	}

	if !sub.IsActive {
		t.Errorf("expected subscription to be active")
	}
	if sub.PackageName != "Pro" {
		t.Errorf("expected package name 'Pro', got '%s'", sub.PackageName)
	}
}

func TestSubscriptionService_Subscribe_InvalidatesAndCaches(t *testing.T) {
	var invalidated bool
	var cachedSet bool

	cacheStore := &mockSubscriptionStore{
		invalidateFn: func(ctx context.Context, userID int64) error {
			invalidated = true
			return nil
		},
		setFn: func(ctx context.Context, sub *model.CachedSubscription, ttl time.Duration) error {
			cachedSet = true
			return nil
		},
	}

	pkgRepo := &mockPackageRepository{
		getByIDFn: func(ctx context.Context, id int64) (*model.Package, error) {
			return &model.Package{
				ID:           2,
				Name:         "Pro",
				PriceCents:   2999,
				DurationDays: 30,
				IsActive:     true,
			}, nil
		},
	}

	subRepo := &mockSubscriptionRepository{
		createFn: func(ctx context.Context, sub *model.Subscription) (*model.Subscription, error) {
			sub.ID = 100
			return sub, nil
		},
	}

	svc := service.NewSubscriptionService(subRepo, pkgRepo, cacheStore, 15*time.Minute)

	resp, err := svc.Subscribe(context.Background(), 1, model.SubscribeRequest{PackageID: 2})
	if err != nil {
		t.Fatalf("expected subscribe to succeed, got %v", err)
	}

	if resp.ID != 100 {
		t.Errorf("expected subscription id 100, got %d", resp.ID)
	}
	if !invalidated {
		t.Errorf("expected Redis cache to be invalidated on subscribe")
	}
	if !cachedSet {
		t.Errorf("expected fresh subscription to be cached in Redis")
	}
}

func TestSubscriptionService_Cancel_InvalidatesCache(t *testing.T) {
	var invalidated bool

	cacheStore := &mockSubscriptionStore{
		invalidateFn: func(ctx context.Context, userID int64) error {
			invalidated = true
			return nil
		},
	}

	subRepo := &mockSubscriptionRepository{
		cancelActiveByUserIDFn: func(ctx context.Context, userID int64) error {
			return nil
		},
	}
	pkgRepo := &mockPackageRepository{}

	svc := service.NewSubscriptionService(subRepo, pkgRepo, cacheStore, 15*time.Minute)

	err := svc.Cancel(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected cancel to succeed, got %v", err)
	}

	if !invalidated {
		t.Errorf("expected Redis cache to be invalidated on cancel")
	}
}
