package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/internal/model"
)

// PackageRepository defines operations for subscription packages.
type PackageRepository interface {
	ListActive(ctx context.Context) ([]model.Package, error)
	GetByID(ctx context.Context, id int64) (*model.Package, error)
}

type pgPackageRepository struct {
	pool *pgxpool.Pool
}

var _ PackageRepository = (*pgPackageRepository)(nil)

// NewPackageRepository constructs a PostgreSQL package repository.
func NewPackageRepository(pool *pgxpool.Pool) PackageRepository {
	return &pgPackageRepository{pool: pool}
}

func (r *pgPackageRepository) ListActive(ctx context.Context) ([]model.Package, error) {
	query := `
		SELECT id, name, description, price_cents, duration_days, is_active, created_at, updated_at
		FROM packages
		WHERE is_active = true
		ORDER BY price_cents ASC;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}
	defer rows.Close()

	var packages []model.Package
	for rows.Next() {
		var p model.Package
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.PriceCents,
			&p.DurationDays,
			&p.IsActive,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan package row: %w", err)
		}
		packages = append(packages, p)
	}

	return packages, rows.Err()
}

func (r *pgPackageRepository) GetByID(ctx context.Context, id int64) (*model.Package, error) {
	query := `
		SELECT id, name, description, price_cents, duration_days, is_active, created_at, updated_at
		FROM packages
		WHERE id = $1;
	`

	p := &model.Package{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.Name,
		&p.Description,
		&p.PriceCents,
		&p.DurationDays,
		&p.IsActive,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get package by id: %w", err)
	}

	return p, nil
}
