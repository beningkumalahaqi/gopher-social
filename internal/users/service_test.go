package users

import (
	"testing"
)

func TestNormalizeUsername(t *testing.T) {
	if got := NormalizeUsername("  Alice_99 "); got != "alice_99" {
		t.Fatalf("NormalizeUsername = %q", got)
	}
}

func TestValidateUsername(t *testing.T) {
	valid := []string{"abc", "alice_99", "a23456789012345678901234567890"}
	for _, username := range valid {
		if err := ValidateUsername(username); err != nil {
			t.Errorf("ValidateUsername(%q) = %v", username, err)
		}
	}
	invalid := []string{"", "ab", "Alice", "has space", "has-dash", "a234567890123456789012345678901"}
	for _, username := range invalid {
		if err := ValidateUsername(username); err == nil {
			t.Errorf("ValidateUsername(%q) accepted invalid input", username)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	if err := ValidateEmail("alice@example.com"); err != nil {
		t.Fatalf("valid email rejected: %v", err)
	}
	for _, email := range []string{"", "not-an-email", "a@b", "Alice <alice@example.com>"} {
		if err := ValidateEmail(email); err == nil {
			t.Errorf("ValidateEmail(%q) accepted invalid input", email)
		}
	}
}

func TestValidateHTTPURL(t *testing.T) {
	if err := ValidateHTTPURL("image_url", "https://example.com/a.png"); err != nil {
		t.Fatalf("valid URL rejected: %v", err)
	}
	for _, raw := range []string{"ftp://example.com/a.png", "javascript:alert(1)", "https://exa mple.com"} {
		if err := ValidateHTTPURL("image_url", raw); err == nil {
			t.Errorf("ValidateHTTPURL(%q) accepted invalid input", raw)
		}
	}
}
