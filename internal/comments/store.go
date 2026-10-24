package comments

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

const commentColumns = `
	c.id, c.post_id, c.author_id, p.author_id, c.content, c.created_at, c.updated_at,
	u.username, u.display_name, u.avatar_url`

const commentFrom = `
	FROM comments c
	JOIN users u ON u.id = c.author_id
	JOIN posts p ON p.id = c.post_id`

func scan(row pgx.Row) (Comment, error) {
	var c Comment
	err := row.Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.PostAuthorID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.Username, &c.Author.DisplayName, &c.Author.AvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, apperr.ErrNotFound
	}
	if err != nil {
		return Comment{}, fmt.Errorf("scan comment: %w", err)
	}
	c.Author.ID = c.AuthorID
	return c, nil
}

func (s *Store) Create(ctx context.Context, c Comment) (Comment, error) {
	const q = `INSERT INTO comments (post_id, author_id, content) VALUES ($1, $2, $3) RETURNING id`
	err := s.db.QueryRow(ctx, q, c.PostID, c.AuthorID, c.Content).Scan(&c.ID)
	if err != nil {
		if database.IsForeignKeyViolation(err) {
			return Comment{}, apperr.ErrNotFound
		}
		return Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	return s.ByID(ctx, c.ID)
}

func (s *Store) ByID(ctx context.Context, id uuid.UUID) (Comment, error) {
	return scan(s.db.QueryRow(ctx, `SELECT `+commentColumns+commentFrom+` WHERE c.id = $1`, id))
}

// List returns oldest-first comments, limit+1 rows for has_more.
func (s *Store) List(ctx context.Context, postID uuid.UUID, limit, offset int) ([]Comment, bool, error) {
	const q = `SELECT ` + commentColumns + commentFrom + `
		WHERE c.post_id = $1
		ORDER BY c.created_at ASC, c.id ASC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.Query(ctx, q, postID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	out := make([]Comment, 0, limit+1)
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate comments: %w", err)
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

// Update only matches the comment author.
func (s *Store) Update(ctx context.Context, id, authorID uuid.UUID, content string) (Comment, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE comments SET content = $3, updated_at = now() WHERE id = $1 AND author_id = $2`,
		id, authorID, content,
	)
	if err != nil {
		return Comment{}, fmt.Errorf("update comment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Comment{}, apperr.ErrNotFound
	}
	return s.ByID(ctx, id)
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}
