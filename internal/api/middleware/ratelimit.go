package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/haqi/gophersocial/internal/api/response"
	"github.com/haqi/gophersocial/internal/apperr"
)

// RateLimiter is a fixed-window limiter keyed by an arbitrary string. It is
// intentionally in-process: one API instance, no Redis, no distributed state.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	clients map[string]*window
}

type window struct {
	start time.Time
	count int
}

func NewRateLimiter(limit int, windowDuration time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  windowDuration,
		now:     time.Now,
		clients: make(map[string]*window),
	}
}

// Allow records a request and reports whether it fits in the current window.
func (l *RateLimiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	w, ok := l.clients[key]
	if !ok || now.Sub(w.start) >= l.window {
		l.clients[key] = &window{start: now, count: 1}
		return true, 0
	}
	w.count++
	if w.count > l.limit {
		return false, l.window - now.Sub(w.start)
	}
	return true, 0
}

// cleanup removes expired windows; called opportunistically to bound memory.
func (l *RateLimiter) cleanup(now time.Time) {
	if len(l.clients) < 1024 {
		return
	}
	for key, w := range l.clients {
		if now.Sub(w.start) >= l.window {
			delete(l.clients, key)
		}
	}
}

// Middleware limits by client IP. Authentication attempts are the main use.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := l.Allow(clientIP(r))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			response.Error(w, r, apperr.ErrRateLimited)
			return
		}
		l.mu.Lock()
		l.cleanup(l.now())
		l.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
