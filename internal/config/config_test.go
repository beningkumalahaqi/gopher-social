package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when JWT_SECRET is missing")
	}
}

func TestLoadValid(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ACCESS_TOKEN_TTL", "5m")
	t.Setenv("DATABASE_MAX_CONNS", "4")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessTokenTTL != 5*time.Minute {
		t.Fatalf("AccessTokenTTL = %v, want 5m", cfg.AccessTokenTTL)
	}
	if cfg.DatabaseMaxConns != 4 {
		t.Fatalf("DatabaseMaxConns = %d, want 4", cfg.DatabaseMaxConns)
	}
	if cfg.Env != "development" {
		t.Fatalf("Env = %q, want development", cfg.Env)
	}
}

func TestLoadRejectsBadDuration(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("REQUEST_TIMEOUT", "soon")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for an invalid duration")
	}
}

func TestDotEnvDoesNotOverrideEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.env"
	if err := os.WriteFile(path, []byte("GOPHERSOCIAL_TEST_VALUE=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPHERSOCIAL_TEST_VALUE", "from-env")
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("GOPHERSOCIAL_TEST_VALUE"); got != "from-env" {
		t.Fatalf("value = %q, want from-env", got)
	}
}
