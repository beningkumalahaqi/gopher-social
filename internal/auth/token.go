package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/apperr"
)

// AccessClaims is the payload of a short-lived access token. SessionID ties the
// token to a refresh session so a client can revoke it at logout.
type AccessClaims struct {
	jwt.RegisteredClaims
	SessionID uuid.UUID `json:"sid"`
}

// TokenManager creates and validates access tokens and opaque refresh tokens.
type TokenManager struct {
	secret     []byte
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(secret, issuer string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		issuer:     issuer,
		audience:   issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *TokenManager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *TokenManager) RefreshTTL() time.Duration { return m.refreshTTL }

// NewAccessToken signs an HS256 JWT. All temporal claims are derived from now.
func (m *TokenManager) NewAccessToken(userID, sessionID uuid.UUID, now time.Time) (string, time.Time, error) {
	expires := now.Add(m.accessTTL)
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
		SessionID: sessionID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expires, nil
}

// ParseAccessToken rejects tokens with the wrong algorithm, issuer, audience,
// missing expiry or an invalid signature.
func (m *TokenManager) ParseAccessToken(raw string) (AccessClaims, error) {
	var claims AccessClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	token, err := parser.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return AccessClaims{}, apperr.Wrap(apperr.ErrUnauthorized, "invalid access token")
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return AccessClaims{}, fmt.Errorf("invalid token subject: %w", err)
	}
	if claims.SessionID == uuid.Nil {
		return AccessClaims{}, errors.New("invalid token session")
	}
	return claims, nil
}

// NewRefreshToken returns an opaque URL-safe secret and the SHA-256 digest that
// is actually stored. A database leak therefore does not leak usable tokens.
func (m *TokenManager) NewRefreshToken() (raw string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	raw = "rt_" + base64.RawURLEncoding.EncodeToString(buf)
	return raw, m.HashRefreshToken(raw), nil
}

// HashRefreshToken is used again when a client presents a refresh token.
func (m *TokenManager) HashRefreshToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
