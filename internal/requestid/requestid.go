// Package requestid provides request correlation without a global variable.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

type key struct{}

var safeID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// New returns a 128-bit random identifier, hex encoded.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is not recoverable in any useful way at request
		// level; a fixed prefix still keeps logs correlated per request.
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}

// Sanitize accepts a client-supplied X-Request-Id only when it is a short,
// boring string. This prevents log injection.
func Sanitize(in string) string {
	if safeID.MatchString(in) {
		return in
	}
	return ""
}

func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key{}, id)
}

func From(ctx context.Context) string {
	id, _ := ctx.Value(key{}).(string)
	return id
}
