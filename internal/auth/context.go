package auth

import (
	"context"

	"github.com/google/uuid"
)

type identityKey struct{}

// Identity is the authenticated caller derived from a validated access token.
type Identity struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFrom returns the caller when authentication middleware ran.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}

// UserIDFrom is the common case.
func UserIDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return uuid.Nil, false
	}
	return id.UserID, true
}
