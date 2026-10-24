// Package reactions owns post reactions.
package reactions

import (
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeLike  Type = "like"
	TypeLove  Type = "love"
	TypeLaugh Type = "laugh"
)

func (t Type) Valid() bool {
	return t == TypeLike || t == TypeLove || t == TypeLaugh
}

type Reaction struct {
	PostID    uuid.UUID
	UserID    uuid.UUID
	Type      Type
	CreatedAt time.Time
	UpdatedAt time.Time
	User      User
}

type User struct {
	Username    string
	DisplayName string
	AvatarURL   *string
}

// Summary is the aggregated view of a post's reactions.
type Summary struct {
	Total  int
	Counts map[Type]int
	Viewer *Type
}
