package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/api/dto"
	"github.com/haqi/gophersocial/internal/api/response"
	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/auth"
)

// AuthService is the subset of auth.Service used by HTTP. The interface is
// declared where it is consumed, which makes handler tests independent of
// PostgreSQL.
type AuthService interface {
	Register(ctx context.Context, in auth.RegisterInput) (auth.Session, error)
	Login(ctx context.Context, in auth.LoginInput) (auth.Session, error)
	Refresh(ctx context.Context, rawToken, userAgent string) (auth.Session, error)
	Logout(ctx context.Context, rawToken string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error
}

type Auth struct {
	base
	svc AuthService
}

func NewAuth(logger *slog.Logger, maxBody int64, svc AuthService) *Auth {
	return &Auth{base: newBase(logger, maxBody), svc: svc}
}

type registerRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=30"`
	Email       string `json:"email" validate:"required,email,max=254"`
	Password    string `json:"password" validate:"required,min=8,max=128"`
	DisplayName string `json:"display_name" validate:"omitempty,max=60"`
}

func (h *Auth) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}

	session, err := h.svc.Register(r.Context(), auth.RegisterInput{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		UserAgent:   r.UserAgent(),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusCreated, dto.NewSession(session))
}

type loginRequest struct {
	Login    string `json:"login" validate:"required,max=254"`
	Password string `json:"password" validate:"required,max=128"`
}

func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	session, err := h.svc.Login(r.Context(), auth.LoginInput{
		Login:     req.Login,
		Password:  req.Password,
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewSession(session))
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (h *Auth) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	session, err := h.svc.Refresh(r.Context(), req.RefreshToken, r.UserAgent())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response.Data(w, http.StatusOK, dto.NewSession(session))
}

func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,max=128"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=128"`
}

func (h *Auth) ChangePassword(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFrom(r.Context())
	if !ok {
		h.fail(w, r, apperr.ErrUnauthorized)
		return
	}
	var req changePasswordRequest
	if err := h.decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.validate(&req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.ChangePassword(r.Context(), identity.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		h.fail(w, r, err)
		return
	}
	response.NoContent(w)
}
