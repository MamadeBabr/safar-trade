package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/internal/model"
)

// SubscriptionRepository defines data access methods for subscriptions.
type SubscriptionRepository interface {
	Create(ctx context.Context, sub *model.Subscription) (*model.Subscription, error)
	GetActiveByUserID(ctx context.Context, userID int64) (*model.Subscription, error)
	CancelActiveByUserID(ctx context.Context, userID int64) error
}

type pgSubscriptionRepository struct {
	pool *pgxpool.Pool
}

var _ SubscriptionRepository = (*pgSubscriptionRepository)(nil)

// NewSubscriptionRepository creates a new PostgreSQL subscription repository.
func NewSubscriptionRepository(pool *pgxpool.Pool) SubscriptionRepository {
	return &pgSubscriptionRepository{pool: pool}
}

func (r *pgSubscriptionRepository) Create(ctx context.Context, sub *model.Subscription) (*model.Subscription, error) {
	// First deactivate any previous active subscriptions for this user
	_, err := r.pool.Exec(ctx, `
		UPDATE subscriptions 
		SET status = 'canceled', updated_at = NOW() 
		WHERE user_id = $1 AND status = 'active';
	`, sub.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel existing subscriptions: %w", err)
	}

	query := `
		INSERT INTO subscriptions (user_id, package_id, status, starts_at, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, user_id, package_id, status, starts_at, expires_at, created_at, updated_at;
	`

	created := &model.Subscription{}
	err = r.pool.QueryRow(ctx, query,
		sub.UserID,
		sub.PackageID,
		sub.Status,
		sub.StartsAt,
		sub.ExpiresAt,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.PackageID,
		&created.Status,
		&created.StartsAt,
		&created.ExpiresAt,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	return created, nil
}

func (r *pgSubscriptionRepository) GetActiveByUserID(ctx context.Context, userID int64) (*model.Subscription, error) {
	query := `
		SELECT s.id, s.user_id, s.package_id, s.status, s.starts_at, s.expires_at, s.created_at, s.updated_at,
		       p.name AS package_name
		FROM subscriptions s
		JOIN packages p ON p.id = s.package_id
		WHERE s.user_id = $1 
		  AND s.status = 'active' 
		  AND s.expires_at > NOW()
		ORDER BY s.expires_at DESC
		LIMIT 1;
	`

	sub := &model.Subscription{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&sub.ID,
		&sub.UserID,
		&sub.PackageID,
		&sub.Status,
		&sub.StartsAt,
		&sub.ExpiresAt,
		&sub.CreatedAt,
		&sub.UpdatedAt,
		&sub.PackageName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get active subscription: %w", err)
	}

	return sub, nil
}

func (r *pgSubscriptionRepository) CancelActiveByUserID(ctx context.Context, userID int64) error {
	query := `
		UPDATE subscriptions
		SET status = 'canceled', updated_at = NOW()
		WHERE user_id = $1 AND status = 'active';
	`

	res, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}

	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
