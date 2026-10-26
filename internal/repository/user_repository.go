package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/internal/model"
)

// UserRepository defines data access operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
}

type pgUserRepository struct {
	pool *pgxpool.Pool
}

var _ UserRepository = (*pgUserRepository)(nil)

// NewUserRepository creates a new PostgreSQL user repository.
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &pgUserRepository{pool: pool}
}

func (r *pgUserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	query := `
		INSERT INTO users (email, name, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, email, name, password_hash, role, is_active, created_at, updated_at;
	`

	created := &model.User{}
	err := r.pool.QueryRow(ctx, query,
		user.Email,
		user.Name,
		user.PasswordHash,
		user.Role,
		user.IsActive,
	).Scan(
		&created.ID,
		&created.Email,
		&created.Name,
		&created.PasswordHash,
		&created.Role,
		&created.IsActive,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return created, nil
}

func (r *pgUserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	query := `
		SELECT id, email, name, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE id = $1;
	`

	u := &model.User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.PasswordHash,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return u, nil
}

func (r *pgUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, name, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1);
	`

	u := &model.User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&u.PasswordHash,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return u, nil
}

func (r *pgUserRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	query := `
		UPDATE users
		SET name = $2, role = $3, is_active = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, email, name, password_hash, role, is_active, created_at, updated_at;
	`

	updated := &model.User{}
	err := r.pool.QueryRow(ctx, query,
		user.ID,
		user.Name,
		user.Role,
		user.IsActive,
	).Scan(
		&updated.ID,
		&updated.Email,
		&updated.Name,
		&updated.PasswordHash,
		&updated.Role,
		&updated.IsActive,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return updated, nil
}
