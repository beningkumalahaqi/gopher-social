package users

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

// NormalizeUsername trims and lowercases a handle. Callers must normalize
// before validating; "Alice" and "alice" are the same account.
func NormalizeUsername(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ValidateUsername enforces the same rule as the database CHECK constraint, so
// users get a field error instead of a driver error.
func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return apperr.Invalid(apperr.FieldError{
			Field:   "username",
			Message: "must be 3-30 characters using lowercase letters, digits or underscores",
		})
	}
	return nil
}

// ValidateEmail uses net/mail rather than a hand-written regex.
func ValidateEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	_, domain, hasDomain := strings.Cut(email, "@")
	if err != nil || addr.Address != email || !hasDomain ||
		!strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return apperr.Invalid(apperr.FieldError{Field: "email", Message: "must be a valid email address"})
	}
	return nil
}

// Service implements user-facing profile operations.
type Service struct {
	store *Store
}

func NewService(store *Store) *Service { return &Service{store: store} }

// Profile is the data needed by GET /users/{username}.
type Profile struct {
	User  Public
	Stats ProfileStats
}

func (s *Service) Profile(ctx context.Context, username string) (Profile, error) {
	u, err := s.store.ByUsername(ctx, NormalizeUsername(username))
	if err != nil {
		return Profile{}, err
	}
	stats, err := s.store.Stats(ctx, u.ID)
	if err != nil {
		return Profile{}, err
	}
	return Profile{User: toPublic(u), Stats: stats}, nil
}

// PatchProfile applies only the fields present in the request.
type PatchProfile struct {
	DisplayName *string
	Bio         *string
	// AvatarURL is optional; an empty string explicitly clears the avatar.
	AvatarURL *string
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, patch PatchProfile) (User, error) {
	u, err := s.store.ByID(ctx, userID)
	if err != nil {
		return User{}, err
	}

	if patch.DisplayName != nil {
		name := strings.TrimSpace(*patch.DisplayName)
		if name == "" {
			name = u.Username
		}
		if len([]rune(name)) > 60 {
			return User{}, apperr.Invalid(apperr.FieldError{Field: "display_name", Message: "must be at most 60 characters"})
		}
		u.DisplayName = name
	}
	if patch.Bio != nil {
		bio := strings.TrimSpace(*patch.Bio)
		if len([]rune(bio)) > 160 {
			return User{}, apperr.Invalid(apperr.FieldError{Field: "bio", Message: "must be at most 160 characters"})
		}
		u.Bio = bio
	}
	if patch.AvatarURL != nil {
		url := strings.TrimSpace(*patch.AvatarURL)
		if url == "" {
			u.AvatarURL = nil
		} else {
			if err := ValidateHTTPURL("avatar_url", url); err != nil {
				return User{}, err
			}
			u.AvatarURL = &url
		}
	}

	updated, err := s.store.UpdateProfile(ctx, u)
	if err != nil {
		return User{}, err
	}
	return updated, nil
}

// Me returns the complete private user record for GET /users/me.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (User, error) {
	return s.store.ByID(ctx, userID)
}

// Search validates the term before hitting PostgreSQL.
func (s *Service) Search(ctx context.Context, query string, limit, offset int) ([]Public, bool, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, false, apperr.Invalid(apperr.FieldError{Field: "q", Message: "must be at least 2 characters"})
	}
	return s.store.Search(ctx, query, limit, offset)
}

func (s *Service) Suggestions(ctx context.Context, username string, limit int) ([]Public, error) {
	u, err := s.store.ByUsername(ctx, NormalizeUsername(username))
	if err != nil {
		return nil, err
	}
	return s.store.Suggestions(ctx, u.ID, limit)
}

// GetPublic is used by other packages that only need a public user.
func (s *Service) GetPublic(ctx context.Context, username string) (Public, error) {
	u, err := s.store.ByUsername(ctx, NormalizeUsername(username))
	if err != nil {
		return Public{}, err
	}
	return toPublic(u), nil
}

func toPublic(u User) Public {
	return Public{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Bio:         u.Bio,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt,
	}
}

// ValidateHTTPURL is shared by profiles and posts. Image "uploads" are URLs in
// this project, so the scheme is restricted deliberately.
func ValidateHTTPURL(field, raw string) error {
	if raw == "" {
		return nil
	}
	u, err := parseHTTPURL(raw)
	if err != nil {
		return apperr.Invalid(apperr.FieldError{Field: field, Message: "must be an absolute http(s) URL"})
	}
	_ = u
	return nil
}

func parseHTTPURL(raw string) (string, error) {
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if strings.ContainsAny(raw, " \n\t") {
		return "", fmt.Errorf("url contains whitespace")
	}
	return raw, nil
}
