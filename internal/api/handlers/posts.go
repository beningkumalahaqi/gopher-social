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
	"github.com/haqi/gophersocial/internal/users"
)

type PostService interface {
	Create(ctx context.Context, authorID uuid.UUID, in posts.CreateInput) (posts.Post, error)
	Get(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (posts.Post, error)
	Update(ctx context.Context, id, userID uuid.UUID, in posts.PatchInput) (posts.Post, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	List(ctx context.Context, viewerID *uuid.UUID, opt posts.ListOptions) ([]posts.Post, bool, error)
	Search(ctx context.Context, query string, viewerID *uuid.UUID, limit, offset int) ([]posts.Post, bool, error)
}

// PostAuthorResolver is the one users capability post handlers need.
type PostAuthorResolver interface {
	GetPublic(ctx context.Context, username string) (users.Public, error)
}

type Posts struct {
	base
	svc     PostService
	authors PostAuthorResolver
}

func NewPosts(logger *slog.Logger, maxBody int64, svc PostService, authors PostAuthorResolver) *Posts {
	return &Posts{base: newBase(logger, maxBody), svc: svc, authors: authors}
}

type createPostRequest struct {
	Content  string `json:"content" validate:"required,max=500"`
	ImageURL string `json:"image_url" validate:"omitempty,max=500"`
	Status   string `json:"status" validate:"omitempty,oneof=draft published"`
}

func (h *Posts) Create(w http.ResponseWriter, r *http.Request) {
	authorID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	var req createPostRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	post, err := h.svc.Create(r.Context(), authorID, posts.CreateInput{
		Content:  req.Content,
		ImageURL: req.ImageURL,
		Status:   posts.Status(req.Status),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusCreated, dto.NewPost(post))
}

func (h *Posts) List(w http.ResponseWriter, r *http.Request) {
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status, err := params.Enum(r, "status", string(posts.StatusDraft), string(posts.StatusPublished))
	if err != nil {
		h.fail(w, r, err)
		return
	}

	var authorID *uuid.UUID
	if username := r.URL.Query().Get("author"); username != "" {
		author, err := h.authors.GetPublic(r.Context(), username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		authorID = &author.ID
	}

	items, hasMore, err := h.svc.List(r.Context(), viewer(r), posts.ListOptions{
		AuthorID: authorID,
		Status:   posts.Status(status),
		Limit:    page.Limit,
		Offset:   page.Offset,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewPosts(items), offsetMeta(page.Limit, page.Offset, hasMore))
}

func (h *Posts) Get(w http.ResponseWriter, r *http.Request) {
	id, err := params.UUID(r, "postID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	post, err := h.svc.Get(r.Context(), id, viewer(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewPost(post))
}

type patchPostRequest struct {
	Content  *string `json:"content" validate:"omitempty,max=500"`
	ImageURL *string `json:"image_url" validate:"omitempty,max=500"`
	Status   *string `json:"status" validate:"omitempty,oneof=draft published"`
}

func (h *Posts) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	id, err := params.UUID(r, "postID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req patchPostRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}

	var status *posts.Status
	if req.Status != nil {
		s := posts.Status(*req.Status)
		status = &s
	}
	post, err := h.svc.Update(r.Context(), id, userID, posts.PatchInput{
		Content:  req.Content,
		ImageURL: req.ImageURL,
		Status:   status,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewPost(post))
}

func (h *Posts) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	id, err := params.UUID(r, "postID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id, userID); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}

// UserPosts serves GET /users/{username}/posts with the same filters as the
// global list, but the author is fixed by the path.
func (h *Posts) UserPosts(w http.ResponseWriter, r *http.Request) {
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status, err := params.Enum(r, "status", string(posts.StatusDraft), string(posts.StatusPublished))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	author, err := h.authors.GetPublic(r.Context(), r.PathValue("username"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, hasMore, err := h.svc.List(r.Context(), viewer(r), posts.ListOptions{
		AuthorID: &author.ID,
		Status:   posts.Status(status),
		Limit:    page.Limit,
		Offset:   page.Offset,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewPosts(items), offsetMeta(page.Limit, page.Offset, hasMore))
}

func (h *Posts) Search(w http.ResponseWriter, r *http.Request) {
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, hasMore, err := h.svc.Search(r.Context(), r.URL.Query().Get("q"), viewer(r), page.Limit, page.Offset)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.List(w, http.StatusOK, dto.NewPosts(items), offsetMeta(page.Limit, page.Offset, hasMore))
}
