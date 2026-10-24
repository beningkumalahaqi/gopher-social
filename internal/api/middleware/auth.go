package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/api/response"
	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/auth"
)

// TokenParser is the one capability authentication middleware needs. Keeping it
// an interface lets tests use a fake without a database.
type TokenParser interface {
	ParseAccessToken(raw string) (auth.AccessClaims, error)
}

// RequireAuth rejects requests without a valid Bearer access token.
func RequireAuth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, err := authenticate(r, parser)
			if err != nil {
				response.Error(w, r, err)
				return
			}
			ctx := auth.WithIdentity(r.Context(), identity)
			recordIdentity(ctx, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth attaches an identity when a valid token is present. A malformed
// or expired token is still rejected rather than silently ignored.
func OptionalAuth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
			}
			identity, err := authenticate(r, parser)
			if err != nil {
				response.Error(w, r, err)
				return
			}
			ctx := auth.WithIdentity(r.Context(), identity)
			recordIdentity(ctx, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authenticate(r *http.Request, parser TokenParser) (auth.Identity, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	scheme, raw, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(raw) == "" {
		return auth.Identity{}, apperr.Wrap(apperr.ErrUnauthorized, "missing or malformed bearer token")
	}

	claims, err := parser.ParseAccessToken(strings.TrimSpace(raw))
	if err != nil {
		return auth.Identity{}, apperr.Wrap(apperr.ErrUnauthorized, "invalid or expired access token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return auth.Identity{}, apperr.Wrap(apperr.ErrUnauthorized, "invalid access token")
	}
	if claims.SessionID == uuid.Nil {
		return auth.Identity{}, apperr.Wrap(apperr.ErrUnauthorized, "invalid access token")
	}
	return auth.Identity{UserID: userID, SessionID: claims.SessionID}, nil
}

// identitySlot lets the outer logger observe an identity that is attached by
// auth middleware running deeper in the chain. Context values are immutable, so
// a pointer created before the handler runs is the simplest way to share the
// result without putting a logger in the context.
type identitySlot struct {
	identity auth.Identity
	ok       bool
}

type identitySlotKey struct{}

func withIdentitySlot(ctx context.Context) (context.Context, *identitySlot) {
	slot := &identitySlot{}
	return context.WithValue(ctx, identitySlotKey{}, slot), slot
}

func recordIdentity(ctx context.Context, identity auth.Identity) {
	if slot, ok := ctx.Value(identitySlotKey{}).(*identitySlot); ok {
		slot.identity = identity
		slot.ok = true
	}
}
