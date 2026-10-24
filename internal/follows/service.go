// Package follows owns the social graph.
package follows

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/database"
	"github.com/haqi/gophersocial/internal/users"
)

// Store contains follow SQL.
type Store struct {
	db database.Querier
}

func NewStore(db database.Querier) *Store { return &Store{db: db} }

// Follow inserts the edge. The composite primary key plus ON CONFLICT makes
// concurrent duplicate requests harmless; inserted reports whether this request
// was the one that created the row.
func (s *Store) Follow(ctx context.Context, followerID, followingID uuid.UUID) (inserted bool, err error) {
	tag, err := s.db.Exec(ctx,
		`INSERT INTO follows (follower_id, following_id) VALUES ($1, $2)
		 ON CONFLICT (follower_id, following_id) DO NOTHING`,
		followerID, followingID,
	)
	if err != nil {
		return false, fmt.Errorf("insert follow: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) Unfollow(ctx context.Context, followerID, followingID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`,
		followerID, followingID,
	)
	if err != nil {
		return fmt.Errorf("delete follow: %w", err)
	}
	return nil
}

// Status returns both directions in a single round trip.
func (s *Store) Status(ctx context.Context, followerID, followingID uuid.UUID) (following, followedBy bool, err error) {
	const q = `
		SELECT
			EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = $2),
			EXISTS (SELECT 1 FROM follows WHERE follower_id = $2 AND following_id = $1)`
	if err := s.db.QueryRow(ctx, q, followerID, followingID).Scan(&following, &followedBy); err != nil {
		return false, false, fmt.Errorf("follow status: %w", err)
	}
	return following, followedBy, nil
}

// Followers lists users who follow userID, newest first.
func (s *Store) Followers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]users.Public, bool, error) {
	return s.list(ctx, `
		SELECT u.id, u.username, u.display_name, u.bio, u.avatar_url, u.created_at
		FROM follows f
		JOIN users u ON u.id = f.follower_id
		WHERE f.following_id = $1
		ORDER BY f.created_at DESC, u.id
		LIMIT $2 OFFSET $3`, userID, limit, offset)
}

// Following lists users followed by userID, newest first.
func (s *Store) Following(ctx context.Context, userID uuid.UUID, limit, offset int) ([]users.Public, bool, error) {
	return s.list(ctx, `
		SELECT u.id, u.username, u.display_name, u.bio, u.avatar_url, u.created_at
		FROM follows f
		JOIN users u ON u.id = f.following_id
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC, u.id
		LIMIT $2 OFFSET $3`, userID, limit, offset)
}

func (s *Store) list(ctx context.Context, query string, userID uuid.UUID, limit, offset int) ([]users.Public, bool, error) {
	rows, err := s.db.Query(ctx, query, userID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("list follow edges: %w", err)
	}
	defer rows.Close()

	out := make([]users.Public, 0, limit+1)
	for rows.Next() {
		var u users.Public
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Bio, &u.AvatarURL, &u.CreatedAt); err != nil {
			return nil, false, fmt.Errorf("scan followed user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate follow edges: %w", err)
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

// Notifier is the capability the follow service needs. The notification
// package implements it; tests can substitute a recorder.
type Notifier interface {
	EnqueueFollow(ctx context.Context, recipientID, actorID uuid.UUID)
}

// Service applies follow rules and emits notifications for new edges only.
type Service struct {
	store    *Store
	users    *users.Store
	notifier Notifier
}

func NewService(store *Store, userStore *users.Store, notifier Notifier) *Service {
	return &Service{store: store, users: userStore, notifier: notifier}
}

func (s *Service) Follow(ctx context.Context, followerID uuid.UUID, username string) error {
	target, err := s.users.ByUsername(ctx, users.NormalizeUsername(username))
	if err != nil {
		return err
	}
	if target.ID == followerID {
		return apperr.Invalid(apperr.FieldError{Field: "username", Message: "you cannot follow yourself"})
	}

	inserted, err := s.store.Follow(ctx, followerID, target.ID)
	if err != nil {
		return err
	}
	if inserted && s.notifier != nil {
		s.notifier.EnqueueFollow(ctx, target.ID, followerID)
	}
	return nil
}

func (s *Service) Unfollow(ctx context.Context, followerID uuid.UUID, username string) error {
	target, err := s.users.ByUsername(ctx, users.NormalizeUsername(username))
	if err != nil {
		return err
	}
	return s.store.Unfollow(ctx, followerID, target.ID)
}

// Status is relative to the authenticated viewer.
func (s *Service) Status(ctx context.Context, viewerID uuid.UUID, username string) (following, followedBy bool, err error) {
	target, err := s.users.ByUsername(ctx, users.NormalizeUsername(username))
	if err != nil {
		return false, false, err
	}
	return s.store.Status(ctx, viewerID, target.ID)
}

func (s *Service) Followers(ctx context.Context, username string, limit, offset int) ([]users.Public, bool, error) {
	target, err := s.users.ByUsername(ctx, users.NormalizeUsername(username))
	if err != nil {
		return nil, false, err
	}
	return s.store.Followers(ctx, target.ID, limit, offset)
}

func (s *Service) Following(ctx context.Context, username string, limit, offset int) ([]users.Public, bool, error) {
	target, err := s.users.ByUsername(ctx, users.NormalizeUsername(username))
	if err != nil {
		return nil, false, err
	}
	return s.store.Following(ctx, target.ID, limit, offset)
}
