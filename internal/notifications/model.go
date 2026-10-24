// Package notifications owns notification storage and the background writer.
package notifications

import (
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeFollow   Type = "follow"
	TypeComment  Type = "comment"
	TypeReaction Type = "reaction"
)

type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ActorID   uuid.UUID
	Type      Type
	PostID    *uuid.UUID
	CommentID *uuid.UUID
	ReadAt    *time.Time
	CreatedAt time.Time
	Actor     Actor
}

type Actor struct {
	Username    string
	DisplayName string
	AvatarURL   *string
}
