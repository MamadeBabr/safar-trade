package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hossen/safar-trade/internal/model"
)

// SessionRepository defines operations for managing refresh tokens / user sessions.
type SessionRepository interface {
	Create(ctx context.Context, session *model.SessionToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.SessionToken, error)
	Revoke(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID int64) error
}

type pgSessionRepository struct {
	pool *pgxpool.Pool
}

var _ SessionRepository = (*pgSessionRepository)(nil)

// NewSessionRepository constructs a PostgreSQL session token repository.
func NewSessionRepository(pool *pgxpool.Pool) SessionRepository {
	return &pgSessionRepository{pool: pool}
}

func (r *pgSessionRepository) Create(ctx context.Context, session *model.SessionToken) error {
	query := `
		INSERT INTO session_tokens (user_id, token_hash, user_agent, ip_address, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at;
	`

	err := r.pool.QueryRow(ctx, query,
		session.UserID,
		session.TokenHash,
		session.UserAgent,
		session.IPAddress,
		session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert session token: %w", err)
	}

	return nil
}

func (r *pgSessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*model.SessionToken, error) {
	query := `
		SELECT id, user_id, token_hash, user_agent, ip_address, expires_at, revoked_at, created_at
		FROM session_tokens
		WHERE token_hash = $1;
	`

	st := &model.SessionToken{}
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&st.ID,
		&st.UserID,
		&st.TokenHash,
		&st.UserAgent,
		&st.IPAddress,
		&st.ExpiresAt,
		&st.RevokedAt,
		&st.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get session token: %w", err)
	}

	return st, nil
}

func (r *pgSessionRepository) Revoke(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE session_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL;
	`

	_, err := r.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke session token: %w", err)
	}

	return nil
}

func (r *pgSessionRepository) RevokeAllForUser(ctx context.Context, userID int64) error {
	query := `
		UPDATE session_tokens
		SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL;
	`

	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}

	return nil
}
