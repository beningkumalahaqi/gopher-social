package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/api/handlers"
	"github.com/haqi/gophersocial/internal/auth"
	"github.com/haqi/gophersocial/internal/users"
)

type fakeAuth struct {
	session auth.Session
	err     error
	called  bool
}

func (f *fakeAuth) Register(ctx context.Context, in auth.RegisterInput) (auth.Session, error) {
	f.called = true
	return f.session, f.err
}

func (f *fakeAuth) Login(ctx context.Context, in auth.LoginInput) (auth.Session, error) {
	return f.session, f.err
}

func (f *fakeAuth) Refresh(ctx context.Context, rawToken, userAgent string) (auth.Session, error) {
	return f.session, f.err
}

func (f *fakeAuth) Logout(ctx context.Context, rawToken string) error { return f.err }

func (f *fakeAuth) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	return f.err
}

func newAuthHandler(t *testing.T, fake *fakeAuth, maxBody int64) *handlers.Auth {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return handlers.NewAuth(logger, maxBody, fake)
}

func TestRegisterSuccess(t *testing.T) {
	userID := uuid.New()
	fake := &fakeAuth{session: auth.Session{
		User: users.User{
			ID: userID, Username: "alice", DisplayName: "Alice", Email: "alice@example.com",
			CreatedAt: time.Now(),
		},
		AccessToken: "access", RefreshToken: "refresh",
		AccessExpiresAt: time.Now().Add(15 * time.Minute), RefreshExpiresAt: time.Now().Add(24 * time.Hour),
	}}
	h := newAuthHandler(t, fake, 1<<20)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"username":"alice","email":"alice@example.com","password":"Password123!"}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !fake.called {
		t.Fatal("service was not called")
	}
	body := rec.Body.String()
	if strings.Contains(body, "password_hash") || strings.Contains(body, "$argon2id") {
		t.Fatal("response leaked password material")
	}
	if !strings.Contains(body, `"access_token":"access"`) {
		t.Fatalf("missing access token: %s", body)
	}
}

func TestRegisterValidation(t *testing.T) {
	h := newAuthHandler(t, &fakeAuth{}, 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"username":"x","email":"not-an-email","password":"short"}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"validation_error"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestRegisterRejectsUnknownFields(t *testing.T) {
	h := newAuthHandler(t, &fakeAuth{}, 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"username":"alice","email":"a@b.co","password":"Password123!","is_admin":true}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsOversizedBody(t *testing.T) {
	h := newAuthHandler(t, &fakeAuth{}, 16)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"username":"alice","email":"a@b.co","password":"Password123!"}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413: %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsMalformedJSON(t *testing.T) {
	h := newAuthHandler(t, &fakeAuth{}, 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
