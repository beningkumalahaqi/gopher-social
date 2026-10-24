package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword correct = %v, %v; want true, nil", ok, err)
	}
	ok, err = VerifyPassword("wrong", hash)
	if err != nil || ok {
		t.Fatalf("VerifyPassword wrong = %v, %v; want false, nil", ok, err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	first, err := HashPassword("same password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("same password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	for _, encoded := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=65536,t=3,p=2$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=19$m=0,t=3,p=2$c2FsdHNhbHQ$aGFzaGhhc2g",
	} {
		if _, err := VerifyPassword("x", encoded); err == nil {
			t.Errorf("expected an error for %q", encoded)
		}
	}
}

func TestDummyHashIsValid(t *testing.T) {
	// The dummy hash is only useful if parsing succeeds; otherwise a missing
	// user would return quickly and leak account existence through timing.
	if _, err := VerifyPassword("anything", dummyHash); err != nil {
		t.Fatalf("dummyHash is not a valid Argon2id hash: %v", err)
	}
}
