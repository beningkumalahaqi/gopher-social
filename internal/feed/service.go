// Package feed builds chronological feeds with cursor pagination.
package feed

import (
	"context"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/pagination"
	"github.com/haqi/gophersocial/internal/posts"
)

type Service struct {
	posts *posts.Store
}

func NewService(postStore *posts.Store) *Service { return &Service{posts: postStore} }

// Feed returns the authenticated user's feed. nextCursor is empty on the last
// page.
func (s *Service) Feed(ctx context.Context, viewerID uuid.UUID, limit int, rawCursor string) ([]posts.Post, string, error) {
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, "", err
	}
	items, err := s.posts.Feed(ctx, viewerID, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	return items, nextCursor(items, limit), nil
}

// Explore is public; a viewer token only enriches each post with the viewer's
// own reaction.
func (s *Service) Explore(ctx context.Context, viewerID *uuid.UUID, limit int, rawCursor string) ([]posts.Post, string, error) {
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return nil, "", err
	}
	items, err := s.posts.Explore(ctx, viewerID, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	return items, nextCursor(items, limit), nil
}

func decodeCursor(raw string) (*pagination.Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	c, err := pagination.Decode(raw)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func nextCursor(items []posts.Post, limit int) string {
	if len(items) < limit {
		return ""
	}
	last := items[len(items)-1]
	return pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
}
