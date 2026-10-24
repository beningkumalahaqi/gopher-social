package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func testManager() *TokenManager {
	return NewTokenManager(
		"0123456789abcdef0123456789abcdef",
		"gophersocial-test",
		15*time.Minute,
		24*time.Hour,
	)
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := testManager()
	userID, sessionID := uuid.New(), uuid.New()
	now := time.Now()

	raw, expires, err := m.NewAccessToken(userID, sessionID, now)
	if err != nil {
		t.Fatal(err)
	}
	if !expires.After(now) {
		t.Fatal("expiry must be in the future")
	}

	claims, err := m.ParseAccessToken(raw)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.Subject != userID.String() || claims.SessionID != sessionID {
		t.Fatalf("claims = %+v, want subject %s session %s", claims, userID, sessionID)
	}
}

func TestAccessTokenRejectsExpired(t *testing.T) {
	m := testManager()
	raw, _, err := m.NewAccessToken(uuid.New(), uuid.New(), time.Now().Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ParseAccessToken(raw); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestAccessTokenRejectsWrongSecret(t *testing.T) {
	issuer := "gophersocial-test"
	m := testManager()
	raw, _, err := m.NewAccessToken(uuid.New(), uuid.New(), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	other := NewTokenManager("another-secret-another-secret-1234", issuer, time.Minute, time.Hour)
	if _, err := other.ParseAccessToken(raw); err == nil {
		t.Fatal("token signed with a different secret was accepted")
	}
}

func TestAccessTokenRejectsAlgNone(t *testing.T) {
	m := testManager()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "gophersocial-test",
			Audience:  jwt.ClaimStrings{"gophersocial-test"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		SessionID: uuid.New(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	raw, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ParseAccessToken(raw); err == nil {
		t.Fatal("alg=none token was accepted")
	}
}

func TestRefreshTokenHashing(t *testing.T) {
	m := testManager()
	raw, hash, err := m.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "rt_") {
		t.Fatalf("raw token %q lacks prefix", raw)
	}
	if string(hash) == raw {
		t.Fatal("stored hash must not equal the raw token")
	}
	if got := m.HashRefreshToken(raw); string(got) != string(hash) {
		t.Fatal("hash is not deterministic")
	}

	otherRaw, _, err := m.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if otherRaw == raw {
		t.Fatal("refresh tokens must be unique")
	}
}
