// Package pagination contains offset and keyset pagination primitives.
package pagination

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
)

// Page describes an offset page. HasMore is computed by fetching limit+1 rows,
// which avoids a separate COUNT query.
type Page struct {
	Limit   int
	Offset  int
	HasMore bool
}

func (p Page) NextOffset() int {
	if !p.HasMore {
		return 0
	}
	return p.Offset + p.Limit
}

// Cursor is a position in a (created_at DESC, id DESC) ordering.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Encode produces an opaque, URL-safe cursor. Clients must treat it as a token.
func (c Cursor) Encode() string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// Decode validates both components; a malformed cursor is a client error.
func Decode(raw string) (Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return Cursor{}, apperr.Wrap(apperr.ErrBadRequest, "invalid cursor")
	}
	ts, id, ok := strings.Cut(string(decoded), "|")
	if !ok {
		return Cursor{}, apperr.Wrap(apperr.ErrBadRequest, "invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Cursor{}, apperr.Wrap(apperr.ErrBadRequest, "invalid cursor")
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return Cursor{}, apperr.Wrap(apperr.ErrBadRequest, "invalid cursor")
	}
	return Cursor{CreatedAt: createdAt, ID: parsedID}, nil
}

// OffsetPage returns the metadata object used by list endpoints.
func OffsetMeta(limit, offset int, hasMore bool) map[string]any {
	meta := map[string]any{
		"limit":    limit,
		"offset":   offset,
		"has_more": hasMore,
	}
	if hasMore {
		meta["next_offset"] = offset + limit
	} else {
		meta["next_offset"] = nil
	}
	return meta
}

// CursorMeta returns metadata for keyset pages.
func CursorMeta(limit int, next string) map[string]any {
	if next == "" {
		return map[string]any{"limit": limit, "next_cursor": nil}
	}
	return map[string]any{"limit": limit, "next_cursor": next}
}

// Validate checks bounds and returns a field-specific error.
func Validate(limit, offset, _, maxLimit int) error {
	if limit < 1 || limit > maxLimit {
		return apperr.Invalid(apperr.FieldError{
			Field:   "limit",
			Message: fmt.Sprintf("must be between 1 and %d", maxLimit),
		})
	}
	if offset < 0 {
		return apperr.Invalid(apperr.FieldError{Field: "offset", Message: "must be zero or positive"})
	}
	return nil
}
