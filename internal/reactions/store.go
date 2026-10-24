package reactions

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

// Get returns one reaction or (nil, nil) when it does not exist.
func (s *Store) Get(ctx context.Context, postID, userID uuid.UUID) (*Type, error) {
	var t Type
	err := s.db.QueryRow(ctx,
		`SELECT type FROM reactions WHERE post_id = $1 AND user_id = $2`,
		postID, userID,
	).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reaction: %w", err)
	}
	return &t, nil
}

// Upsert relies on the composite primary key. The database guarantees exactly
// one row per (post, user) no matter how many requests race.
func (s *Store) Upsert(ctx context.Context, postID, userID uuid.UUID, t Type) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO reactions (post_id, user_id, type)
		VALUES ($1, $2, $3)
		ON CONFLICT (post_id, user_id)
		DO UPDATE SET type = excluded.type, updated_at = now()`,
		postID, userID, t,
	)
	if err != nil {
		if database.IsForeignKeyViolation(err) {
			return apperr.ErrNotFound
		}
		return fmt.Errorf("upsert reaction: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, postID, userID uuid.UUID) (bool, error) {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM reactions WHERE post_id = $1 AND user_id = $2`,
		postID, userID,
	)
	if err != nil {
		return false, fmt.Errorf("delete reaction: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) List(ctx context.Context, postID uuid.UUID, filter Type, limit, offset int) ([]Reaction, bool, error) {
	const q = `
		SELECT r.post_id, r.user_id, r.type, r.created_at, r.updated_at,
		       u.username, u.display_name, u.avatar_url
		FROM reactions r
		JOIN users u ON u.id = r.user_id
		WHERE r.post_id = $1 AND ($2::text IS NULL OR r.type = $2)
		ORDER BY r.created_at DESC, r.user_id
		LIMIT $3 OFFSET $4`

	var typeFilter *string
	if filter != "" {
		v := string(filter)
		typeFilter = &v
	}
	rows, err := s.db.Query(ctx, q, postID, typeFilter, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("list reactions: %w", err)
	}
	defer rows.Close()

	out := make([]Reaction, 0, limit+1)
	for rows.Next() {
		var r Reaction
		if err := rows.Scan(
			&r.PostID, &r.UserID, &r.Type, &r.CreatedAt, &r.UpdatedAt,
			&r.User.Username, &r.User.DisplayName, &r.User.AvatarURL,
		); err != nil {
			return nil, false, fmt.Errorf("scan reaction: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate reactions: %w", err)
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

// Summary computes counts and the viewer's own reaction in one query.
func (s *Store) Summary(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (Summary, error) {
	const q = `
		SELECT
			count(*),
			count(*) FILTER (WHERE type = 'like'),
			count(*) FILTER (WHERE type = 'love'),
			count(*) FILTER (WHERE type = 'laugh'),
			(SELECT type FROM reactions WHERE post_id = $1 AND user_id = $2)
		FROM reactions
		WHERE post_id = $1`

	sum := Summary{Counts: map[Type]int{}}
	var like, love, laugh int
	var viewer *Type
	err := s.db.QueryRow(ctx, q, postID, viewerID).
		Scan(&sum.Total, &like, &love, &laugh, &viewer)
	if err != nil {
		return Summary{}, fmt.Errorf("reaction summary: %w", err)
	}
	sum.Counts[TypeLike] = like
	sum.Counts[TypeLove] = love
	sum.Counts[TypeLaugh] = laugh
	sum.Viewer = viewer
	return sum, nil
}
