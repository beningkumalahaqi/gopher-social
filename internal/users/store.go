package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/database"
)

// Store contains all SQL for users. Every query is parameterized; no value is
// ever concatenated into a statement.
type Store struct {
	db database.Querier
}

func NewStore(db database.Querier) *Store { return &Store{db: db} }

// WithTx returns a store that runs on the given transaction.
func (s *Store) WithTx(tx pgx.Tx) *Store { return &Store{db: tx} }

const userColumns = `id, username, display_name, email, password_hash, bio, avatar_url, created_at, updated_at`

// Create inserts a user. Unique violations bubble up so the service can turn
// them into one generic conflict message.
func (s *Store) Create(ctx context.Context, u User) (User, error) {
	const q = `
		INSERT INTO users (username, display_name, email, password_hash, bio, avatar_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + userColumns

	var out User
	err := s.db.QueryRow(ctx, q, u.Username, u.DisplayName, u.Email, u.PasswordHash, u.Bio, u.AvatarURL).
		Scan(&out.ID, &out.Username, &out.DisplayName, &out.Email, &out.PasswordHash, &out.Bio, &out.AvatarURL, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return User{}, apperr.Wrap(apperr.ErrConflict, "username or email already registered")
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return out, nil
}

// ByID loads a user or returns apperr.ErrNotFound.
func (s *Store) ByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.scanOne(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
}

// ByUsername loads by the normalized username.
func (s *Store) ByUsername(ctx context.Context, username string) (User, error) {
	return s.scanOne(ctx, `SELECT `+userColumns+` FROM users WHERE username = $1`, username)
}

// ByLogin accepts either a username or an email address. Both are stored
// normalized (lowercase), so one parameter is enough.
func (s *Store) ByLogin(ctx context.Context, login string) (User, error) {
	return s.scanOne(ctx, `SELECT `+userColumns+` FROM users WHERE username = $1 OR email = $1`, login)
}

func (s *Store) scanOne(ctx context.Context, query string, args ...any) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, query, args...).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.PasswordHash, &u.Bio, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return u, nil
}

// UpdateProfile writes the already-merged profile fields.
func (s *Store) UpdateProfile(ctx context.Context, u User) (User, error) {
	const q = `
		UPDATE users
		SET display_name = $2, bio = $3, avatar_url = $4, updated_at = now()
		WHERE id = $1
		RETURNING ` + userColumns

	var out User
	err := s.db.QueryRow(ctx, q, u.ID, u.DisplayName, u.Bio, u.AvatarURL).
		Scan(&out.ID, &out.Username, &out.DisplayName, &out.Email, &out.PasswordHash, &out.Bio, &out.AvatarURL, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("update profile: %w", err)
	}
	return out, nil
}

// UpdatePassword replaces the password hash.
func (s *Store) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

// Stats returns profile counters in one round trip instead of three.
func (s *Store) Stats(ctx context.Context, id uuid.UUID) (ProfileStats, error) {
	const q = `
		SELECT
			(SELECT count(*) FROM posts p WHERE p.author_id = $1),
			(SELECT count(*) FROM follows f WHERE f.following_id = $1),
			(SELECT count(*) FROM follows f WHERE f.follower_id = $1)`

	var st ProfileStats
	err := s.db.QueryRow(ctx, q, id).Scan(&st.Posts, &st.Followers, &st.Following)
	if err != nil {
		return ProfileStats{}, fmt.Errorf("profile stats: %w", err)
	}
	return st, nil
}

// Search combines prefix matching (index friendly) with full-text matching for
// display names, returning one extra row so callers can compute has_more.
func (s *Store) Search(ctx context.Context, query string, limit, offset int) ([]Public, bool, error) {
	const q = `
		SELECT id, username, display_name, bio, avatar_url, created_at
		FROM users
		WHERE username LIKE $1
		   OR to_tsvector('simple', username || ' ' || display_name) @@ websearch_to_tsquery('simple', $2)
		ORDER BY username ASC
		LIMIT $3 OFFSET $4`

	rows, err := s.db.Query(ctx, q, query+"%", query, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	out := make([]Public, 0, limit)
	for rows.Next() {
		var u Public
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Bio, &u.AvatarURL, &u.CreatedAt); err != nil {
			return nil, false, fmt.Errorf("scan searched user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate users: %w", err)
	}

	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

// Suggestions ranks accounts the user does not already follow by follower
// count. NULLIF guards the division when the target user has no followers.
func (s *Store) Suggestions(ctx context.Context, userID uuid.UUID, limit int) ([]Public, error) {
	const q = `
		SELECT u.id, u.username, u.display_name, u.bio, u.avatar_url, u.created_at
		FROM users u
		LEFT JOIN follows f
			ON f.following_id = u.id AND f.follower_id = $1
		WHERE u.id <> $1 AND f.follower_id IS NULL
		ORDER BY
			(SELECT count(*) FROM follows x WHERE x.following_id = u.id) DESC,
			u.username ASC
		LIMIT $2`

	rows, err := s.db.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("suggest users: %w", err)
	}
	defer rows.Close()

	out := make([]Public, 0, limit)
	for rows.Next() {
		var u Public
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Bio, &u.AvatarURL, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan suggestion: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
