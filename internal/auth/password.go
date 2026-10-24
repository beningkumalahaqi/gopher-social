// Package auth owns credentials, tokens and the authentication service.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. Memory is measured in KiB: 64 MiB per hash. Tune these
// against your hardware; the encoded hash stores them, so old hashes keep
// verifying after a parameter change.
const (
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// ErrInvalidHash means the stored value is not in the expected PHC format.
var ErrInvalidHash = errors.New("invalid password hash format")

// HashPassword returns a PHC-formatted Argon2id hash, including its salt and
// parameters. Hashing is deliberately slow.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword checks a password against an encoded hash in constant time
// with respect to the hash contents.
func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

type hashParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

// decodeHash parses $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>.
func decodeHash(encoded string) (hashParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return hashParams{}, nil, nil, fmt.Errorf("%w: unsupported version %q", ErrInvalidHash, parts[2])
	}

	var p hashParams
	values := strings.Split(parts[3], ",")
	if len(values) != 3 {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		if !ok {
			return hashParams{}, nil, nil, ErrInvalidHash
		}
		n, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return hashParams{}, nil, nil, ErrInvalidHash
		}
		switch key {
		case "m":
			p.memory = uint32(n)
		case "t":
			p.time = uint32(n)
		case "p":
			if n > 255 {
				return hashParams{}, nil, nil, ErrInvalidHash
			}
			p.threads = uint8(n)
		default:
			return hashParams{}, nil, nil, ErrInvalidHash
		}
	}
	if p.memory == 0 || p.time == 0 || p.threads == 0 {
		return hashParams{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) < 16 {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	return p, salt, hash, nil
}

// dummyHash is verified when a login names a user that does not exist, so a
// missing account costs roughly the same time as a wrong password. It is a
// valid Argon2id hash of an unguessable random value.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4Zm9vYmE"

// verifyDummy burns the same work as a real verification.
func verifyDummy(password string) {
	_, _ = VerifyPassword(password, dummyHash)
}
