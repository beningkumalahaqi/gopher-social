package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/api/dto"
	"github.com/haqi/gophersocial/internal/api/params"
	"github.com/haqi/gophersocial/internal/api/response"
	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/auth"
	"github.com/haqi/gophersocial/internal/posts"
)

type FeedService interface {
	Feed(ctx context.Context, viewerID uuid.UUID, limit int, rawCursor string) ([]posts.Post, string, error)
	Explore(ctx context.Context, viewerID *uuid.UUID, limit int, rawCursor string) ([]posts.Post, string, error)
}

type Feed struct {
	base
	svc FeedService
}

func NewFeed(logger *slog.Logger, maxBody int64, svc FeedService) *Feed {
	return &Feed{base: newBase(logger, maxBody), svc: svc}
}

func (h *Feed) Feed(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	limit, err := params.Limit(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, next, err := h.svc.Feed(r.Context(), userID, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewPosts(items), cursorMeta(limit, next))
}

func (h *Feed) Explore(w http.ResponseWriter, r *http.Request) {
	limit, err := params.Limit(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, next, err := h.svc.Explore(r.Context(), viewer(r), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewPosts(items), cursorMeta(limit, next))
}

func cursorMeta(limit int, next string) map[string]any {
	if next == "" {
		return map[string]any{"limit": limit, "next_cursor": nil}
	}
	return map[string]any{"limit": limit, "next_cursor": next}
}
