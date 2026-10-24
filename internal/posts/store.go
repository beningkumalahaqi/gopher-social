package posts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/database"
	"github.com/haqi/gophersocial/internal/pagination"
)

// Store contains post SQL. The viewer parameter drives both draft visibility
// and the viewer_reaction field.
type Store struct {
	db database.Querier
}

func NewStore(db database.Querier) *Store { return &Store{db: db} }

// postColumns expects $1 to be the viewer UUID (nullable).
const postColumns = `
	p.id, p.author_id, p.content, p.image_url, p.status,
	p.created_at, p.updated_at, p.published_at,
	u.username, u.display_name, u.avatar_url,
	(SELECT count(*) FROM comments c WHERE c.post_id = p.id),
	(SELECT count(*) FROM reactions r WHERE r.post_id = p.id),
	(SELECT r.type FROM reactions r WHERE r.post_id = p.id AND r.user_id = $1)`

const postFrom = ` FROM posts p JOIN users u ON u.id = p.author_id `

func scanPost(row pgx.Row) (Post, error) {
	var p Post
	err := row.Scan(
		&p.ID, &p.AuthorID, &p.Content, &p.ImageURL, &p.Status,
		&p.CreatedAt, &p.UpdatedAt, &p.PublishedAt,
		&p.Author.Username, &p.Author.DisplayName, &p.Author.AvatarURL,
		&p.CommentCount, &p.ReactionCount, &p.ViewerReaction,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, apperr.ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("scan post: %w", err)
	}
	p.Author.ID = p.AuthorID
	return p, nil
}

// Create inserts a post and returns its stored representation with counters.
func (s *Store) Create(ctx context.Context, p Post) (Post, error) {
	const q = `
		INSERT INTO posts (author_id, content, image_url, status, published_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	err := s.db.QueryRow(ctx, q, p.AuthorID, p.Content, p.ImageURL, p.Status, p.PublishedAt).Scan(&p.ID)
	if err != nil {
		if database.IsForeignKeyViolation(err) {
			return Post{}, apperr.ErrNotFound
		}
		return Post{}, fmt.Errorf("insert post: %w", err)
	}
	return s.ByID(ctx, p.ID, &p.AuthorID)
}

// ByID loads one post for a viewer. Drafts are filtered here as well as in the
// service, so a direct store call can never leak someone else's draft.
func (s *Store) ByID(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (Post, error) {
	const q = `SELECT ` + postColumns + postFrom + `
		WHERE p.id = $2 AND (p.status = 'published' OR p.author_id = $1)`
	p, err := scanPost(s.db.QueryRow(ctx, q, viewerID, id))
	if err != nil {
		return Post{}, err
	}
	return p, nil
}

// AdminByID bypasses visibility for internal checks such as ownership and
// comment creation. It must never be used to build a response directly.
func (s *Store) AdminByID(ctx context.Context, id uuid.UUID) (Post, error) {
	const q = `SELECT ` + postColumns + postFrom + ` WHERE p.id = $2`
	return scanPost(s.db.QueryRow(ctx, q, nil, id))
}

// Filters controls List. A nil Status means "everything visible".
type Filters struct {
	AuthorID *uuid.UUID
	Status   Status
	ViewerID *uuid.UUID
	Limit    int
	Offset   int
}

// List returns at most limit+1 rows so the caller can compute has_more.
func (s *Store) List(ctx context.Context, f Filters) ([]Post, bool, error) {
	const q = `SELECT ` + postColumns + postFrom + `
		WHERE ($1::uuid IS NULL OR p.author_id = $1)
		  AND ($2::text IS NULL OR p.status = $2)
		  AND (p.status = 'published' OR p.author_id = $3)
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $4 OFFSET $5`

	var status *string
	if f.Status != "" {
		v := string(f.Status)
		status = &v
	}
	rows, err := s.db.Query(ctx, q, f.AuthorID, status, f.ViewerID, f.Limit+1, f.Offset)
	if err != nil {
		return nil, false, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	out, err := collectPosts(rows, f.Limit)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(out) > f.Limit
	if hasMore {
		out = out[:f.Limit]
	}
	return out, hasMore, nil
}

// Update writes merged fields. Only the owner is allowed; the WHERE clause
// enforces it in SQL in addition to the service check.
func (s *Store) Update(ctx context.Context, p Post, ownerID uuid.UUID, viewerID *uuid.UUID) (Post, error) {
	const q = `
		UPDATE posts
		SET content = $3, image_url = $4, status = $5, published_at = $6, updated_at = now()
		WHERE id = $1 AND author_id = $2
		RETURNING id`

	var id uuid.UUID
	err := s.db.QueryRow(ctx, q, p.ID, ownerID, p.Content, p.ImageURL, p.Status, p.PublishedAt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, apperr.ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("update post: %w", err)
	}
	return s.ByID(ctx, p.ID, viewerID)
}

// Delete removes a post owned by authorID, reporting whether a row matched.
func (s *Store) Delete(ctx context.Context, id, authorID uuid.UUID) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM posts WHERE id = $1 AND author_id = $2`, id, authorID)
	if err != nil {
		return false, fmt.Errorf("delete post: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Search uses full-text search over published posts.
func (s *Store) Search(ctx context.Context, query string, viewerID *uuid.UUID, limit, offset int) ([]Post, bool, error) {
	const q = `SELECT ` + postColumns + postFrom + `
		WHERE p.status = 'published'
		  AND to_tsvector('english', p.content) @@ websearch_to_tsquery('english', $2)
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $3 OFFSET $4`

	rows, err := s.db.Query(ctx, q, viewerID, query, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("search posts: %w", err)
	}
	defer rows.Close()

	out, err := collectPosts(rows, limit)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

func collectPosts(rows pgx.Rows, limit int) ([]Post, error) {
	out := make([]Post, 0, limit+1)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return out, nil
}

// PublishedAtIf returns the timestamp a status transition should store.
func PublishedAtIf(status Status, previous *time.Time, now time.Time) *time.Time {
	if status != StatusPublished {
		return nil
	}
	if previous != nil {
		return previous
	}
	return &now
}

// Feed returns published posts from the viewer and the accounts they follow,
// ordered by (created_at, id) descending. The keyset comparison avoids the
// offset drift that plagues "load more" feeds when new posts arrive.
func (s *Store) Feed(ctx context.Context, viewerID uuid.UUID, cursor *pagination.Cursor, limit int) ([]Post, error) {
	const q = `SELECT ` + postColumns + postFrom + `
		WHERE p.status = 'published'
		  AND (
			p.author_id = $1
			OR EXISTS (
				SELECT 1 FROM follows f
				WHERE f.follower_id = $1 AND f.following_id = p.author_id
			)
		  )
		  AND ($2::timestamptz IS NULL OR (p.created_at, p.id) < ($2::timestamptz, $3::uuid))
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $4`

	cursorTime, cursorID := cursorArgs(cursor)
	rows, err := s.db.Query(ctx, q, viewerID, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, fmt.Errorf("feed query: %w", err)
	}
	defer rows.Close()

	out, err := collectPosts(rows, limit)
	if err != nil {
		return nil, err
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Explore returns every published post, newest first.
func (s *Store) Explore(ctx context.Context, viewerID *uuid.UUID, cursor *pagination.Cursor, limit int) ([]Post, error) {
	const q = `SELECT ` + postColumns + postFrom + `
		WHERE p.status = 'published'
		  AND ($2::timestamptz IS NULL OR (p.created_at, p.id) < ($2::timestamptz, $3::uuid))
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $4`

	cursorTime, cursorID := cursorArgs(cursor)
	rows, err := s.db.Query(ctx, q, viewerID, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, fmt.Errorf("explore query: %w", err)
	}
	defer rows.Close()

	out, err := collectPosts(rows, limit)
	if err != nil {
		return nil, err
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func cursorArgs(cursor *pagination.Cursor) (any, any) {
	if cursor == nil {
		return nil, nil
	}
	return cursor.CreatedAt, cursor.ID
}
