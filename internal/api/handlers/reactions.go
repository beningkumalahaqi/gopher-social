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
	"github.com/haqi/gophersocial/internal/reactions"
)

type ReactionService interface {
	Set(ctx context.Context, postID, userID uuid.UUID, t reactions.Type) (reactions.Summary, error)
	Remove(ctx context.Context, postID, userID uuid.UUID) error
	List(ctx context.Context, postID uuid.UUID, filter reactions.Type, limit, offset int) ([]reactions.Reaction, bool, error)
	Summary(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (reactions.Summary, error)
}

type Reactions struct {
	base
	svc ReactionService
}

func NewReactions(logger *slog.Logger, maxBody int64, svc ReactionService) *Reactions {
	return &Reactions{base: newBase(logger, maxBody), svc: svc}
}

type reactionRequest struct {
	Type string `json:"type" validate:"required,oneof=like love laugh"`
}

func (h *Reactions) Set(w http.ResponseWriter, r *http.Request) {
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
	var req reactionRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	summary, err := h.svc.Set(r.Context(), postID, userID, reactions.Type(req.Type))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewReactionSummary(summary))
}

func (h *Reactions) Delete(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.Remove(r.Context(), postID, userID); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *Reactions) List(w http.ResponseWriter, r *http.Request) {
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
	filter, err := params.Enum(r, "type", string(reactions.TypeLike), string(reactions.TypeLove), string(reactions.TypeLaugh))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, hasMore, err := h.svc.List(r.Context(), postID, reactions.Type(filter), page.Limit, page.Offset)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewReactions(items), offsetMeta(page.Limit, page.Offset, hasMore))
}

func (h *Reactions) Summary(w http.ResponseWriter, r *http.Request) {
	postID, err := params.UUID(r, "postID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	summary, err := h.svc.Summary(r.Context(), postID, viewer(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewReactionSummary(summary))
}
