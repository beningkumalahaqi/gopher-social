// Package users owns user profiles and the user table.
package users

import (
	"time"

	"github.com/google/uuid"
)

// User is the complete row used inside the application. It deliberately has no
// JSON tags: HTTP shapes live in internal/api/dto, so a database column can never
// leak into a response by accident.
type User struct {
	ID           uuid.UUID
	Username     string
	DisplayName  string
	Email        string
	PasswordHash string
	Bio          string
	AvatarURL    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Public hides credentials. Handlers still map this to a DTO.
type Public struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	Bio         string
	AvatarURL   *string
	CreatedAt   time.Time
}

// ProfileStats are computed on demand for the public profile endpoint.
type ProfileStats struct {
	Posts     int
	Followers int
	Following int
}
