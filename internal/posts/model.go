// Package posts owns posts and their visibility rules.
package posts

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
)

func (s Status) Valid() bool { return s == StatusDraft || s == StatusPublished }

// Author is the denormalized user data returned with a post.
type Author struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	AvatarURL   *string
}

// Post is a post plus the counters needed by list and detail responses.
type Post struct {
	ID             uuid.UUID
	AuthorID       uuid.UUID
	Content        string
	ImageURL       *string
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PublishedAt    *time.Time
	Author         Author
	CommentCount   int
	ReactionCount  int
	ViewerReaction *string
}
