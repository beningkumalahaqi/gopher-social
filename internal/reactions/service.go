package reactions

import (
	"context"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/posts"
)

type Notifier interface {
	EnqueueReaction(ctx context.Context, recipientID, actorID, postID uuid.UUID)
}

type Service struct {
	store    *Store
	posts    *posts.Store
	notifier Notifier
}

func NewService(store *Store, postStore *posts.Store, notifier Notifier) *Service {
	return &Service{store: store, posts: postStore, notifier: notifier}
}

// Set creates or replaces the viewer's reaction. A notification is emitted only
// when the reaction is new or its type actually changes.
func (s *Service) Set(ctx context.Context, postID, userID uuid.UUID, t Type) (Summary, error) {
	if !t.Valid() {
		return Summary{}, apperr.Invalid(apperr.FieldError{Field: "type", Message: "must be like, love or laugh"})
	}
	post, err := s.posts.ByID(ctx, postID, nil)
	if err != nil {
		return Summary{}, err
	}

	previous, err := s.store.Get(ctx, postID, userID)
	if err != nil {
		return Summary{}, err
	}
	if err := s.store.Upsert(ctx, postID, userID, t); err != nil {
		return Summary{}, err
	}

	changed := previous == nil || *previous != t
	if changed && post.AuthorID != userID && s.notifier != nil {
		s.notifier.EnqueueReaction(ctx, post.AuthorID, userID, postID)
	}
	return s.store.Summary(ctx, postID, &userID)
}

func (s *Service) Remove(ctx context.Context, postID, userID uuid.UUID) error {
	// Deleting a reaction on a missing post is still a no-op rather than 404:
	// DELETE is idempotent, and the post may have been deleted concurrently.
	_, err := s.store.Delete(ctx, postID, userID)
	return err
}

func (s *Service) List(ctx context.Context, postID uuid.UUID, filter Type, limit, offset int) ([]Reaction, bool, error) {
	if filter != "" && !filter.Valid() {
		return nil, false, apperr.Invalid(apperr.FieldError{Field: "type", Message: "must be like, love or laugh"})
	}
	if _, err := s.posts.ByID(ctx, postID, nil); err != nil {
		return nil, false, err
	}
	return s.store.List(ctx, postID, filter, limit, offset)
}

func (s *Service) Summary(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (Summary, error) {
	if _, err := s.posts.ByID(ctx, postID, nil); err != nil {
		return Summary{}, err
	}
	return s.store.Summary(ctx, postID, viewerID)
}
