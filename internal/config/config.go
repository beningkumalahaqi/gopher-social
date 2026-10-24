package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully resolved configuration of the API. It is built once at
// startup and passed explicitly to the components that need it.
type Config struct {
	Env                 string
	HTTPAddr            string
	DatabaseURL         string
	DatabaseMaxConns    int32
	JWTIssuer           string
	JWTSecret           string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	LogLevel            slog.Level
	RequestTimeout      time.Duration
	ShutdownTimeout     time.Duration
	MaxBodyBytes        int64
	AuthRateLimit       int
	AuthRateWindow      time.Duration
	NotificationQueue   int
	NotificationWorkers int
	ReadinessTimeout    time.Duration
	LearnPlayground     bool
	LearnPlaygroundURL  string
}

// Load reads configuration from the process environment, optionally preceded
// by values in a local .env file. Real environment variables always win.
func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	addr := env("HTTP_ADDR", "")
	if addr == "" {
		if port := os.Getenv("PORT"); port != "" {
			addr = ":" + port
		} else {
			addr = ":8080"
		}
	}

	cfg := Config{
		Env:                 env("APP_ENV", "development"),
		HTTPAddr:            addr,
		DatabaseURL:         env("DATABASE_URL", "postgresql://admin:password@localhost:5432/gopherdb"),
		DatabaseMaxConns:    int32(envInt("DATABASE_MAX_CONNS", 10)),
		JWTIssuer:           env("JWT_ISSUER", "gophersocial"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		NotificationQueue:   envInt("NOTIFICATION_QUEUE_SIZE", 256),
		NotificationWorkers: envInt("NOTIFICATION_WORKERS", 2),
		AuthRateLimit:       envInt("AUTH_RATE_LIMIT", 10),
		LearnPlayground:     envBool("LEARN_PLAYGROUND", true),
		LearnPlaygroundURL:  env("LEARN_PLAYGROUND_URL", "https://go.dev/_/compile"),
	}

	var err error
	if cfg.AccessTokenTTL, err = envDuration("ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.RefreshTokenTTL, err = envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RequestTimeout, err = envDuration("REQUEST_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = envDuration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.AuthRateWindow, err = envDuration("AUTH_RATE_WINDOW", time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.ReadinessTimeout, err = envDuration("READINESS_TIMEOUT", 2*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.MaxBodyBytes, err = envInt64("MAX_BODY_BYTES", 1<<20); err != nil {
		return Config{}, err
	}
	if cfg.LogLevel, err = parseLevel(env("LOG_LEVEL", "info")); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate fails fast instead of letting a broken configuration surface as a
// confusing error minutes later.
func (c Config) Validate() error {
	var problems []string
	if c.JWTSecret == "" {
		problems = append(problems, "JWT_SECRET is required")
	} else if len(c.JWTSecret) < 32 {
		problems = append(problems, "JWT_SECRET must be at least 32 characters")
	}
	if c.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	}
	if c.DatabaseMaxConns < 1 {
		problems = append(problems, "DATABASE_MAX_CONNS must be positive")
	}
	if c.NotificationQueue < 1 || c.NotificationWorkers < 1 {
		problems = append(problems, "notification queue size and worker count must be positive")
	}
	if c.RequestTimeout <= 0 || c.ShutdownTimeout <= 0 {
		problems = append(problems, "REQUEST_TIMEOUT and SHUTDOWN_TIMEOUT must be positive")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

// IsProduction reports whether development-only behaviour must be disabled.
func (c Config) IsProduction() bool { return c.Env == "production" }

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := envInt64(key, int64(fallback))
	if err != nil {
		return fallback
	}
	return int(v)
}

func envInt64(key string, fallback int64) (int64, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return n, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 15m: %w", key, err)
	}
	return d, nil
}

func parseLevel(raw string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error: %w", err)
	}
	return level, nil
}

// loadDotEnv implements the tiny subset of dotenv syntax the project uses:
// KEY=VALUE lines, optional surrounding quotes, and # comments. It never
// overrides variables that already exist in the real environment.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
