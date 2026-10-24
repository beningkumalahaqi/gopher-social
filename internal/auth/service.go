package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/haqi/gophersocial/internal/apperr"
	"github.com/haqi/gophersocial/internal/users"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 128
)

// Service implements registration, login and token lifecycle.
type Service struct {
	pool    *pgxpool.Pool
	users   *users.Store
	refresh *RefreshStore
	tokens  *TokenManager
	now     func() time.Time
}

func NewService(pool *pgxpool.Pool, userStore *users.Store, refreshStore *RefreshStore, tokens *TokenManager) *Service {
	return &Service{
		pool:    pool,
		users:   userStore,
		refresh: refreshStore,
		tokens:  tokens,
		now:     time.Now,
	}
}

// Session is returned by register, login and refresh.
type Session struct {
	User             users.User
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time

	refreshID uuid.UUID
}

type RegisterInput struct {
	Username    string
	Email       string
	Password    string
	DisplayName string
	UserAgent   string
}

// Register creates the user and the first refresh session atomically.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Session, error) {
	username := users.NormalizeUsername(in.Username)
	email := users.NormalizeEmail(in.Email)
	if err := users.ValidateUsername(username); err != nil {
		return Session{}, err
	}
	if err := users.ValidateEmail(email); err != nil {
		return Session{}, err
	}
	if err := validatePassword(in.Password); err != nil {
		return Session{}, err
	}
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if len([]rune(displayName)) > 60 {
		return Session{}, apperr.Invalid(apperr.FieldError{Field: "display_name", Message: "must be at most 60 characters"})
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return Session{}, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin register transaction: %w", err)
	}
	defer rollback(ctx, tx)

	user, err := s.users.WithTx(tx).Create(ctx, users.User{
		Username:     username,
		DisplayName:  displayName,
		Email:        email,
		PasswordHash: hash,
	})
	if err != nil {
		return Session{}, err
	}

	session, err := s.issueSession(ctx, tx, user, in.UserAgent)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit register transaction: %w", err)
	}
	return session, nil
}

type LoginInput struct {
	Login     string
	Password  string
	UserAgent string
}

// Login deliberately returns the same error for unknown users and wrong
// passwords. The dummy verification keeps timing roughly constant.
func (s *Service) Login(ctx context.Context, in LoginInput) (Session, error) {
	user, err := s.VerifyCredentials(ctx, in.Login, in.Password)
	if err != nil {
		return Session{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin login transaction: %w", err)
	}
	defer rollback(ctx, tx)

	session, err := s.issueSession(ctx, tx, user, in.UserAgent)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit login transaction: %w", err)
	}
	return session, nil
}

// VerifyCredentials checks a login and password without issuing tokens. The
// learning interface uses it to create its own cookie session, so browser auth
// does not depend on long-lived bearer tokens in JavaScript.
func (s *Service) VerifyCredentials(ctx context.Context, login, password string) (users.User, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || password == "" {
		return users.User{}, apperr.Invalid(
			apperr.FieldError{Field: "login", Message: "login and password are required"},
		)
	}

	user, err := s.users.ByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			verifyDummy(password)
			return users.User{}, errInvalidCredentials()
		}
		return users.User{}, err
	}

	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return users.User{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return users.User{}, errInvalidCredentials()
	}
	return user, nil
}

// Refresh rotates a refresh token. Reusing an already-rotated token is treated
// as theft and revokes every active session for that user.
func (s *Service) Refresh(ctx context.Context, rawToken, userAgent string) (Session, error) {
	if rawToken == "" {
		return Session{}, apperr.Invalid(apperr.FieldError{Field: "refresh_token", Message: "is required"})
	}
	hash := s.tokens.HashRefreshToken(rawToken)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer rollback(ctx, tx)

	stored, err := s.refresh.WithTx(tx).ByHashForUpdate(ctx, hash)
	if err != nil {
		return Session{}, errInvalidRefresh()
	}

	if stored.RevokedAt != nil {
		// Rotation already happened: this token is being replayed.
		if _, err := s.refresh.WithTx(tx).RevokeAllForUser(ctx, stored.UserID); err != nil {
			return Session{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Session{}, fmt.Errorf("commit reuse revocation: %w", err)
		}
		return Session{}, errInvalidRefresh()
	}
	if s.now().After(stored.ExpiresAt) {
		return Session{}, errInvalidRefresh()
	}

	user, err := s.users.WithTx(tx).ByID(ctx, stored.UserID)
	if err != nil {
		return Session{}, err
	}

	session, err := s.issueSession(ctx, tx, user, userAgent)
	if err != nil {
		return Session{}, err
	}
	newID := session.refreshID
	if err := s.refresh.WithTx(tx).Revoke(ctx, stored.ID, &newID); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit refresh transaction: %w", err)
	}
	return session, nil
}

// Logout revokes the presented refresh token. It is idempotent and never leaks
// whether the token existed.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return apperr.Invalid(apperr.FieldError{Field: "refresh_token", Message: "is required"})
	}
	return s.refresh.RevokeByHash(ctx, s.tokens.HashRefreshToken(rawToken))
}

// ChangePassword verifies the current password, stores the new hash and revokes
// every refresh session. Access tokens expire naturally within their TTL.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(current, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify current password: %w", err)
	}
	if !ok {
		return apperr.Invalid(apperr.FieldError{Field: "current_password", Message: "is incorrect"})
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	if current == next {
		return apperr.Invalid(apperr.FieldError{Field: "new_password", Message: "must differ from the current password"})
	}

	hash, err := HashPassword(next)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	if _, err := s.refresh.RevokeAllForUser(ctx, userID); err != nil {
		return err
	}
	return nil
}

// issueSession creates the refresh row and signs the access token. It returns a
// Session with an unexported refresh id used for rotation bookkeeping.
func (s *Service) issueSession(ctx context.Context, tx pgx.Tx, user users.User, userAgent string) (Session, error) {
	rawRefresh, hash, err := s.tokens.NewRefreshToken()
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	expires := now.Add(s.tokens.RefreshTTL())

	stored, err := s.refresh.WithTx(tx).Create(ctx, RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		UserAgent: truncate(userAgent, 200),
		ExpiresAt: expires,
	})
	if err != nil {
		return Session{}, err
	}

	access, accessExpires, err := s.tokens.NewAccessToken(user.ID, stored.ID, now)
	if err != nil {
		return Session{}, err
	}

	return Session{
		User:             user,
		AccessToken:      access,
		RefreshToken:     rawRefresh,
		AccessExpiresAt:  accessExpires,
		RefreshExpiresAt: expires,
		refreshID:        stored.ID,
	}, nil
}

func validatePassword(password string) error {
	n := len([]rune(password))
	if n < minPasswordLen {
		return apperr.Invalid(apperr.FieldError{Field: "password", Message: "must be at least 8 characters"})
	}
	if n > maxPasswordLen {
		return apperr.Invalid(apperr.FieldError{Field: "password", Message: "must be at most 128 characters"})
	}
	return nil
}

func errInvalidCredentials() error {
	return apperr.Wrap(apperr.ErrUnauthorized, "invalid credentials")
}

func errInvalidRefresh() error {
	return apperr.Wrap(apperr.ErrUnauthorized, "invalid refresh token")
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx) // a committed transaction returns ErrTxClosed; ignore it
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
