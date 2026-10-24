package posts

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/users"
)

const (
	maxContentRunes = 500
	maxOffset       = 10_000
)

// Service applies post business rules; stores never decide authorization.
type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store) *Service {
	return &Service{store: store, now: time.Now}
}

type CreateInput struct {
	Content  string
	ImageURL string
	Status   Status
}

func (s *Service) Create(ctx context.Context, authorID uuid.UUID, in CreateInput) (Post, error) {
	content, imageURL, status, err := validateFields(in.Content, in.ImageURL, in.Status, true)
	if err != nil {
		return Post{}, err
	}
	now := s.now()
	p := Post{
		AuthorID: authorID,
		Content:  content,
		ImageURL: imageURL,
		Status:   status,
	}
	if status == StatusPublished {
		p.PublishedAt = &now
	}
	return s.store.Create(ctx, p)
}

type PatchInput struct {
	Content  *string
	ImageURL *string
	Status   *Status
}

func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, in PatchInput) (Post, error) {
	post, err := s.store.AdminByID(ctx, id)
	if err != nil {
		return Post{}, err
	}
	if post.AuthorID != userID {
		return Post{}, apperr.Wrap(apperr.ErrForbidden, "you can only edit your own posts")
	}

	content := post.Content
	image := ""
	if post.ImageURL != nil {
		image = *post.ImageURL
	}
	status := post.Status
	if in.Content != nil {
		content = *in.Content
	}
	if in.ImageURL != nil {
		image = *in.ImageURL
	}
	if in.Status != nil {
		status = *in.Status
	}

	content, imageURL, status, err := validateFields(content, image, status, false)
	if err != nil {
		return Post{}, err
	}

	post.Content = content
	post.ImageURL = imageURL
	post.Status = status
	post.PublishedAt = PublishedAtIf(status, post.PublishedAt, s.now())
	return s.store.Update(ctx, post, userID, &userID)
}

// Get returns a visible post. Drafts are invisible to everyone but the author.
func (s *Service) Get(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (Post, error) {
	return s.store.ByID(ctx, id, viewerID)
}

func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	post, err := s.store.AdminByID(ctx, id)
	if err != nil {
		return err
	}
	if post.AuthorID != userID {
		return apperr.Wrap(apperr.ErrForbidden, "you can only delete your own posts")
	}
	deleted, err := s.store.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return apperr.ErrNotFound
	}
	return nil
}

type ListOptions struct {
	AuthorID *uuid.UUID
	Status   Status
	Limit    int
	Offset   int
}

func (s *Service) List(ctx context.Context, viewerID *uuid.UUID, opt ListOptions) ([]Post, bool, error) {
	if opt.Status != "" && !opt.Status.Valid() {
		return nil, false, apperr.Invalid(apperr.FieldError{Field: "status", Message: "must be draft or published"})
	}
	if opt.Offset > maxOffset {
		return nil, false, apperr.Invalid(apperr.FieldError{Field: "offset", Message: "must be at most 10000"})
	}
	// A viewer asking for drafts must be the author; the SQL additionally
	// filters drafts to the viewer's own posts.
	if opt.Status == StatusDraft && (viewerID == nil || opt.AuthorID == nil || *opt.AuthorID != *viewerID) {
		return nil, false, apperr.Wrap(apperr.ErrForbidden, "you can only list your own drafts")
	}
	return s.store.List(ctx, Filters{
		AuthorID: opt.AuthorID,
		Status:   opt.Status,
		ViewerID: viewerID,
		Limit:    opt.Limit,
		Offset:   opt.Offset,
	})
}

func (s *Service) Search(ctx context.Context, query string, viewerID *uuid.UUID, limit, offset int) ([]Post, bool, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, false, apperr.Invalid(apperr.FieldError{Field: "q", Message: "must be at least 2 characters"})
	}
	if offset > maxOffset {
		return nil, false, apperr.Invalid(apperr.FieldError{Field: "offset", Message: "must be at most 10000"})
	}
	return s.store.Search(ctx, query, viewerID, limit, offset)
}

// validateFields trims content and enforces the same invariants as the table.
func validateFields(content, imageURL string, status Status, defaultStatus bool) (string, *string, Status, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil, "", apperr.Invalid(apperr.FieldError{Field: "content", Message: "is required"})
	}
	if len([]rune(content)) > maxContentRunes {
		return "", nil, "", apperr.Invalid(apperr.FieldError{Field: "content", Message: "must be at most 500 characters"})
	}
	if status == "" && defaultStatus {
		status = StatusPublished
	}
	if !status.Valid() {
		return "", nil, "", apperr.Invalid(apperr.FieldError{Field: "status", Message: "must be draft or published"})
	}
	imageURL = strings.TrimSpace(imageURL)
	var image *string
	if imageURL != "" {
		if err := users.ValidateHTTPURL("image_url", imageURL); err != nil {
			return "", nil, "", err
		}
		image = &imageURL
	}
	return content, image, status, nil
}
