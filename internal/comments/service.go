package comments

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/posts"
)

type Notifier interface {
	EnqueueComment(ctx context.Context, recipientID, actorID, postID, commentID uuid.UUID)
}

type Service struct {
	store    *Store
	posts    *posts.Store
	notifier Notifier
}

func NewService(store *Store, postStore *posts.Store, notifier Notifier) *Service {
	return &Service{store: store, posts: postStore, notifier: notifier}
}

func (s *Service) Create(ctx context.Context, postID, authorID uuid.UUID, content string) (Comment, error) {
	content, err := validateContent(content)
	if err != nil {
		return Comment{}, err
	}
	// ByID with a nil viewer only returns published posts, so comments on
	// drafts or missing posts both produce 404.
	post, err := s.posts.ByID(ctx, postID, nil)
	if err != nil {
		return Comment{}, err
	}

	comment, err := s.store.Create(ctx, Comment{PostID: postID, AuthorID: authorID, Content: content})
	if err != nil {
		return Comment{}, err
	}
	if post.AuthorID != authorID && s.notifier != nil {
		s.notifier.EnqueueComment(ctx, post.AuthorID, authorID, postID, comment.ID)
	}
	return comment, nil
}

func (s *Service) List(ctx context.Context, postID uuid.UUID, limit, offset int) ([]Comment, bool, error) {
	if _, err := s.posts.ByID(ctx, postID, nil); err != nil {
		return nil, false, err
	}
	return s.store.List(ctx, postID, limit, offset)
}

func (s *Service) Update(ctx context.Context, commentID, userID uuid.UUID, content string) (Comment, error) {
	content, err := validateContent(content)
	if err != nil {
		return Comment{}, err
	}
	comment, err := s.store.ByID(ctx, commentID)
	if err != nil {
		return Comment{}, err
	}
	if comment.AuthorID != userID {
		return Comment{}, apperr.Wrap(apperr.ErrForbidden, "you can only edit your own comments")
	}
	return s.store.Update(ctx, commentID, userID, content)
}

// Delete lets the comment author or the post author remove a comment.
func (s *Service) Delete(ctx context.Context, commentID, userID uuid.UUID) error {
	comment, err := s.store.ByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment.AuthorID != userID && comment.PostAuthorID != userID {
		return apperr.Wrap(apperr.ErrForbidden, "you cannot delete this comment")
	}
	return s.store.Delete(ctx, commentID)
}

func validateContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", apperr.Invalid(apperr.FieldError{Field: "content", Message: "is required"})
	}
	if len([]rune(content)) > 1000 {
		return "", apperr.Invalid(apperr.FieldError{Field: "content", Message: "must be at most 1000 characters"})
	}
	return content, nil
}
