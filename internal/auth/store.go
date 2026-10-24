package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/database"
)

// RefreshToken is one row of refresh_tokens. Only the hash is stored.
type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  []byte
	UserAgent  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
}

// RefreshStore owns SQL for refresh sessions.
type RefreshStore struct {
	db database.Querier
}

func NewRefreshStore(db database.Querier) *RefreshStore { return &RefreshStore{db: db} }

func (s *RefreshStore) WithTx(tx pgx.Tx) *RefreshStore { return &RefreshStore{db: tx} }

// Create inserts a session and returns its generated id.
func (s *RefreshStore) Create(ctx context.Context, token RefreshToken) (RefreshToken, error) {
	const q = `
		INSERT INTO refresh_tokens (user_id, token_hash, user_agent, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	err := s.db.QueryRow(ctx, q, token.UserID, token.TokenHash, token.UserAgent, token.ExpiresAt).
		Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		return RefreshToken{}, fmt.Errorf("insert refresh token: %w", err)
	}
	return token, nil
}

// ByHashForUpdate locks the row for the duration of the refresh transaction,
// preventing two concurrent requests from rotating the same token.
func (s *RefreshStore) ByHashForUpdate(ctx context.Context, hash []byte) (RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, user_agent, expires_at, created_at, revoked_at, replaced_by
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE`

	var t RefreshToken
	err := s.db.QueryRow(ctx, q, hash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.UserAgent, &t.ExpiresAt,
		&t.CreatedAt, &t.RevokedAt, &t.ReplacedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, apperr.ErrUnauthorized
	}
	if err != nil {
		return RefreshToken{}, fmt.Errorf("select refresh token: %w", err)
	}
	return t, nil
}

// Revoke marks one session as rotated or logged out.
func (s *RefreshStore) Revoke(ctx context.Context, id uuid.UUID, replacedBy *uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2 WHERE id = $1`,
		id, replacedBy,
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeByHash is idempotent: unknown or already-revoked tokens are fine.
func (s *RefreshStore) RevokeByHash(ctx context.Context, hash []byte) error {
	_, err := s.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hash,
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token by hash: %w", err)
	}
	return nil
}

// RevokeAllForUser is used on password change and on refresh-token reuse.
func (s *RefreshStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("revoke user sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
