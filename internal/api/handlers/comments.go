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
	"github.com/haqi/gophersocial/internal/comments"
)

type CommentService interface {
	Create(ctx context.Context, postID, authorID uuid.UUID, content string) (comments.Comment, error)
	List(ctx context.Context, postID uuid.UUID, limit, offset int) ([]comments.Comment, bool, error)
	Update(ctx context.Context, commentID, userID uuid.UUID, content string) (comments.Comment, error)
	Delete(ctx context.Context, commentID, userID uuid.UUID) error
}

type Comments struct {
	base
	svc CommentService
}

func NewComments(logger *slog.Logger, maxBody int64, svc CommentService) *Comments {
	return &Comments{base: newBase(logger, maxBody), svc: svc}
}

type commentRequest struct {
	Content string `json:"content" validate:"required,max=1000"`
}

func (h *Comments) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	postID, err := params.UUID(r, "postID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req commentRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	comment, err := h.svc.Create(r.Context(), postID, userID, req.Content)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusCreated, dto.NewComment(comment))
}

func (h *Comments) List(w http.ResponseWriter, r *http.Request) {
	postID, err := params.UUID(r, "postID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, hasMore, err := h.svc.List(r.Context(), postID, page.Limit, page.Offset)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewComments(items), offsetMeta(page.Limit, page.Offset, hasMore))
}

func (h *Comments) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	commentID, err := params.UUID(r, "commentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req commentRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	comment, err := h.svc.Update(r.Context(), commentID, userID, req.Content)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewComment(comment))
}

func (h *Comments) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	commentID, err := params.UUID(r, "commentID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), commentID, userID); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}
