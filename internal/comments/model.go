// Package comments owns comments on posts.
package comments

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	ID           uuid.UUID
	PostID       uuid.UUID
	AuthorID     uuid.UUID
	PostAuthorID uuid.UUID
	Content      string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Author       Author
}

type Author struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	AvatarURL   *string
}
