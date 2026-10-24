package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/haqi/gophersocial/internal/api/response"
)

// Pinger is the readiness dependency: a *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Health struct {
	pool    Pinger
	timeout time.Duration
	logger  *slog.Logger
}

func NewHealth(pool Pinger, timeout time.Duration, logger *slog.Logger) *Health {
	return &Health{pool: pool, timeout: timeout, logger: logger}
}

// Live reports process liveness; it must not touch dependencies.
func (h *Health) Live(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready verifies the database with a bounded context.
func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		h.logger.WarnContext(r.Context(), "readiness check failed", "error", err)
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
