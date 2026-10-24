package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/auth"
)

func TestRateLimiterAllowsBurstThenRejects(t *testing.T) {
	l := NewRateLimiter(2, time.Minute)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("first request rejected")
	}
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("second request rejected")
	}
	ok, retry := l.Allow("1.2.3.4")
	if ok {
		t.Fatal("third request allowed over the limit")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("retryAfter = %v, want within (0, 1m]", retry)
	}
	if ok, _ := l.Allow("5.6.7.8"); !ok {
		t.Fatal("different key should have its own window")
	}

	now = now.Add(time.Minute)
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("window did not reset")
	}
}

func TestRateLimitMiddlewareWrites429(t *testing.T) {
	l := NewRateLimiter(1, time.Minute)
	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
}

type fakeParser struct {
	claims auth.AccessClaims
	err    error
}

func (f fakeParser) ParseAccessToken(string) (auth.AccessClaims, error) {
	return f.claims, f.err
}

func TestRequireAuth(t *testing.T) {
	userID, sessionID := uuid.New(), uuid.New()
	parser := fakeParser{claims: auth.AccessClaims{SessionID: sessionID}}
	parser.claims.Subject = userID.String()

	var got auth.Identity
	handler := RequireAuth(parser)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = auth.IdentityFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got.UserID != userID || got.SessionID != sessionID {
		t.Fatalf("identity = %+v", got)
	}
}

func TestRequireAuthRejectsMissingAndMalformed(t *testing.T) {
	parser := fakeParser{err: context.Canceled}
	handler := RequireAuth(parser)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run")
	}))

	cases := []struct {
		name   string
		header string
	}{
		{"missing", ""},
		{"wrong scheme", "Basic abc"},
		{"empty token", "Bearer "},
		{"parser error", "Bearer bad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestOptionalAuthAllowsAnonymous(t *testing.T) {
	handler := OptionalAuth(fakeParser{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.IdentityFrom(r.Context()); ok {
			t.Fatal("anonymous request should have no identity")
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
