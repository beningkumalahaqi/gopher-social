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
	"github.com/haqi/gophersocial/internal/notifications"
)

type NotificationService interface {
	List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int) ([]notifications.Notification, bool, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error)
}

type Notifications struct {
	base
	svc NotificationService
}

func NewNotifications(logger *slog.Logger, maxBody int64, svc NotificationService) *Notifications {
	return &Notifications{base: newBase(logger, maxBody), svc: svc}
}

func (h *Notifications) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	unread := r.URL.Query().Get("unread") == "true"
	items, hasMore, err := h.svc.List(r.Context(), userID, unread, page.Limit, page.Offset)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewNotifications(items), offsetMeta(page.Limit, page.Offset, hasMore))
}

func (h *Notifications) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	count, err := h.svc.UnreadCount(r.Context(), userID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, map[string]int{"unread": count})
}

func (h *Notifications) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	id, err := params.UUID(r, "notificationID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.MarkRead(r.Context(), userID, id); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *Notifications) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	updated, err := h.svc.MarkAllRead(r.Context(), userID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, map[string]int64{"updated": updated})
}
