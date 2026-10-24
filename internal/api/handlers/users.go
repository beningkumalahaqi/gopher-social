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
	"github.com/haqi/gophersocial/internal/users"
)

type UserService interface {
	Profile(ctx context.Context, username string) (users.Profile, error)
	Me(ctx context.Context, id uuid.UUID) (users.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, patch users.PatchProfile) (users.User, error)
	Search(ctx context.Context, query string, limit, offset int) ([]users.Public, bool, error)
	Suggestions(ctx context.Context, username string, limit int) ([]users.Public, error)
}

type FollowService interface {
	Follow(ctx context.Context, followerID uuid.UUID, username string) error
	Unfollow(ctx context.Context, followerID uuid.UUID, username string) error
	Status(ctx context.Context, viewerID uuid.UUID, username string) (following, followedBy bool, err error)
	Followers(ctx context.Context, username string, limit, offset int) ([]users.Public, bool, error)
	Following(ctx context.Context, username string, limit, offset int) ([]users.Public, bool, error)
}

type Users struct {
	base
	users   UserService
	follows FollowService
	auth    AuthService
}

func NewUsers(logger *slog.Logger, maxBody int64, userSvc UserService, followSvc FollowService, authSvc AuthService) *Users {
	return &Users{base: newBase(logger, maxBody), users: userSvc, follows: followSvc, auth: authSvc}
}

func (h *Users) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	user, err := h.users.Me(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewPrivateUser(user))
}

type updateMeRequest struct {
	DisplayName *string `json:"display_name" validate:"omitempty,max=60"`
	Bio         *string `json:"bio" validate:"omitempty,max=160"`
	AvatarURL   *string `json:"avatar_url" validate:"omitempty,max=500"`
}

func (h *Users) UpdateMe(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	var req updateMeRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	user, err := h.users.UpdateProfile(r.Context(), id, users.PatchProfile{
		DisplayName: req.DisplayName,
		Bio:         req.Bio,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewPrivateUser(user))
}

func (h *Users) Get(w http.ResponseWriter, r *http.Request) {
	profile, err := h.users.Profile(r.Context(), r.PathValue("username"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewProfile(profile))
}

func (h *Users) Followers(w http.ResponseWriter, r *http.Request) {
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, hasMore, err := h.follows.Followers(r.Context(), r.PathValue("username"), page.Limit, page.Offset)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.userList(w, items, page.Limit, page.Offset, hasMore)
}

func (h *Users) Following(w http.ResponseWriter, r *http.Request) {
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, hasMore, err := h.follows.Following(r.Context(), r.PathValue("username"), page.Limit, page.Offset)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.userList(w, items, page.Limit, page.Offset, hasMore)
}

func (h *Users) userList(w http.ResponseWriter, items []users.Public, limit, offset int, hasMore bool) {
	out := make([]dto.User, 0, len(items))
	for _, u := range items {
		out = append(out, dto.NewUser(u))
	}
	response.List(w, http.StatusOK, out, offsetMeta(limit, offset, hasMore))
}

func (h *Users) FollowStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	following, followedBy, err := h.follows.Status(r.Context(), id, r.PathValue("username"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, map[string]bool{
		"following":   following,
		"followed_by": followedBy,
	})
}

func (h *Users) Suggestions(w http.ResponseWriter, r *http.Request) {
	limit, err := params.Limit(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.users.Suggestions(r.Context(), r.PathValue("username"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]dto.User, 0, len(items))
	for _, u := range items {
		out = append(out, dto.NewUser(u))
	}
	response.List(w, http.StatusOK, out, map[string]any{"limit": limit})
}

func (h *Users) Search(w http.ResponseWriter, r *http.Request) {
	page, err := params.Pagination(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, hasMore, err := h.users.Search(r.Context(), r.URL.Query().Get("q"), page.Limit, page.Offset)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.userList(w, items, page.Limit, page.Offset, hasMore)
}

// Follow and Unfollow live on the users resource but mutate the graph.
type Follows struct {
	base
	svc FollowService
}

func NewFollows(logger *slog.Logger, maxBody int64, svc FollowService) *Follows {
	return &Follows{base: newBase(logger, maxBody), svc: svc}
}

func (h *Follows) Follow(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	if err := h.svc.Follow(r.Context(), id, r.PathValue("username")); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *Follows) Unfollow(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.UserIDFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	if err := h.svc.Unfollow(r.Context(), id, r.PathValue("username")); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}

// offsetMeta is shared by offset-paginated handlers.
func offsetMeta(limit, offset int, hasMore bool) map[string]any {
	meta := map[string]any{
		"limit":    limit,
		"offset":   offset,
		"has_more": hasMore,
	}
	if hasMore {
		meta["next_offset"] = offset + limit
	} else {
		meta["next_offset"] = nil
	}
	return meta
}
