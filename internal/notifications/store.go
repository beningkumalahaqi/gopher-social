package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/database"
)

type Store struct {
	db database.Querier
}

func NewStore(db database.Querier) *Store { return &Store{db: db} }

// Insert writes one notification. The database constraints enforce recipient,
// actor and target consistency.
func (s *Store) Insert(ctx context.Context, n Notification) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO notifications (user_id, actor_id, type, post_id, comment_id)
		VALUES ($1, $2, $3, $4, $5)`,
		n.UserID, n.ActorID, n.Type, n.PostID, n.CommentID,
	)
	if err != nil {
		if database.IsForeignKeyViolation(err) {
			// The post/comment/actor disappeared while the job was queued.
			return apperr.ErrNotFound
		}
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int) ([]Notification, bool, error) {
	const q = `
		SELECT n.id, n.user_id, n.actor_id, n.type, n.post_id, n.comment_id, n.read_at, n.created_at,
		       u.username, u.display_name, u.avatar_url
		FROM notifications n
		JOIN users u ON u.id = n.actor_id
		WHERE n.user_id = $1 AND (NOT $2::boolean OR n.read_at IS NULL)
		ORDER BY n.created_at DESC, n.id DESC
		LIMIT $3 OFFSET $4`

	rows, err := s.db.Query(ctx, q, userID, unreadOnly, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	out := make([]Notification, 0, limit+1)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.ActorID, &n.Type, &n.PostID, &n.CommentID,
			&n.ReadAt, &n.CreatedAt, &n.Actor.Username, &n.Actor.DisplayName, &n.Actor.AvatarURL,
		); err != nil {
			return nil, false, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate notifications: %w", err)
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

func (s *Store) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// MarkRead scopes by user_id so one user cannot read another's notification.
func (s *Store) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2 AND read_at IS NULL`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either it does not exist, belongs to someone else, or was already
		// read. The endpoint is idempotent, so verify ownership once.
		var exists bool
		err := s.db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM notifications WHERE id = $1 AND user_id = $2)`,
			id, userID,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check notification: %w", err)
		}
		if !exists {
			return apperr.ErrNotFound
		}
	}
	return nil
}

func (s *Store) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ErrQueueFull is used by tests and callers that care about dropped jobs.
var ErrQueueFull = errors.New("notification queue full")

var _ = pgx.ErrNoRows
