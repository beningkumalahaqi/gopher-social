package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/haqi/gophersocial/internal/app"
	"github.com/haqi/gophersocial/internal/config"
	"github.com/haqi/gophersocial/internal/database"
)

var (
	testDBURL     string
	dbUnavailable bool
)

func TestMain(m *testing.M) {
	testDBURL = os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgresql://admin:password@localhost:5432/gopherdb_test"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, testDBURL, 4)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration tests skipped: database unavailable: %v\n", err)
		dbUnavailable = true
		os.Exit(m.Run())
	}
	pool.Close()

	// Start from a clean schema every run. Drop is a no-op on a fresh database.
	if err := database.Migrate(testDBURL, "drop"); err != nil {
		fmt.Fprintf(os.Stderr, "drop test schema: %v\n", err)
		os.Exit(1)
	}
	if err := database.Migrate(testDBURL, "up"); err != nil {
		fmt.Fprintf(os.Stderr, "migrate test schema: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

type env struct {
	app    *app.App
	server *httptest.Server
	pool   *pgxpool.Pool
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if dbUnavailable {
		t.Skip("PostgreSQL is unavailable; integration test skipped")
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, testDBURL, 4)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE users, refresh_tokens, posts, follows, comments, reactions, notifications CASCADE`); err != nil {
		pool.Close()
		t.Fatalf("truncate: %v", err)
	}

	cfg := config.Config{
		Env:                 "test",
		HTTPAddr:            "127.0.0.1:0",
		DatabaseURL:         testDBURL,
		DatabaseMaxConns:    4,
		JWTIssuer:           "gophersocial-test",
		JWTSecret:           "integration-test-secret-0123456789abcdef",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     24 * time.Hour,
		LogLevel:            slog.LevelError,
		RequestTimeout:      5 * time.Second,
		ShutdownTimeout:     5 * time.Second,
		MaxBodyBytes:        1 << 20,
		AuthRateLimit:       1000,
		AuthRateWindow:      time.Minute,
		NotificationQueue:   64,
		NotificationWorkers: 2,
		ReadinessTimeout:    time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		pool.Close()
		t.Fatalf("app.New: %v", err)
	}
	application.StartWorkers()
	server := httptest.NewServer(application.Handler())

	t.Cleanup(func() {
		server.Close()
		application.Close()
		pool.Close()
	})
	return &env{app: application, server: server, pool: pool}
}

// do performs a JSON request. It returns status and a decoded object.
func (e *env) do(t *testing.T, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("%s %s: invalid JSON %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode, parsed
}

type session struct {
	userID   string
	username string
	token    string
	refresh  string
}

func (e *env) register(t *testing.T, username string) session {
	t.Helper()
	status, body := e.do(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"username": username,
		"email":    username + "@example.com",
		"password": "Password123!",
	})
	if status != http.StatusCreated {
		t.Fatalf("register %s: status %d body %v", username, status, body)
	}
	data := body["data"].(map[string]any)
	user := data["user"].(map[string]any)
	return session{
		userID:   user["id"].(string),
		username: username,
		token:    data["access_token"].(string),
		refresh:  data["refresh_token"].(string),
	}
}

func (e *env) createPost(t *testing.T, s session, content string) string {
	t.Helper()
	status, body := e.do(t, http.MethodPost, "/api/v1/posts", s.token, map[string]any{"content": content})
	if status != http.StatusCreated {
		t.Fatalf("create post: status %d body %v", status, body)
	}
	return body["data"].(map[string]any)["id"].(string)
}

func dataList(t *testing.T, body map[string]any) []any {
	t.Helper()
	list, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("response has no data array: %v", body)
	}
	return list
}
