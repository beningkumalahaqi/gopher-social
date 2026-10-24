package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/api/middleware"
	"github.com/haqi/gophersocial/internal/learn"
	"github.com/haqi/gophersocial/internal/learn/apispec"
	"github.com/haqi/gophersocial/internal/learn/cache"
	"github.com/haqi/gophersocial/internal/users"
)

const (
	sessionCookie = "gopher_learn_session"
	csrfCookie    = "gopher_learn_csrf"
	themeCookie   = "gopher_learn_theme"
	fontCookie    = "gopher_learn_font"
	motionCookie  = "gopher_learn_motion"

	sessionTTL = 30 * 24 * time.Hour
)

// Store is the persistence surface the learning interface needs.
type Store interface {
	CreateGuest(ctx context.Context, guestHash []byte) (learn.Profile, error)
	EnsureUserProfile(ctx context.Context, userID uuid.UUID, displayName string) (learn.Profile, error)
	CreateSession(ctx context.Context, profileID uuid.UUID, tokenHash []byte, userID *uuid.UUID, userAgent string, expires time.Time) error
	ProfileBySession(ctx context.Context, tokenHash []byte) (learn.Profile, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
	MergeGuest(ctx context.Context, guestID, targetProfileID uuid.UUID) error

	Progress(ctx context.Context, profileID uuid.UUID) ([]learn.Progress, error)
	TouchLesson(ctx context.Context, profileID uuid.UUID, slug string) error
	SetProgress(ctx context.Context, profileID uuid.UUID, slug string, status learn.ProgressStatus, percent int) (learn.Progress, error)
	SetReading(ctx context.Context, profileID uuid.UUID, slug string, percent int) (learn.Progress, error)
	Bookmarks(ctx context.Context, profileID uuid.UUID) ([]string, error)
	AddBookmark(ctx context.Context, profileID uuid.UUID, slug string) error
	RemoveBookmark(ctx context.Context, profileID uuid.UUID, slug string) error
	Note(ctx context.Context, profileID uuid.UUID, slug string) (string, error)
	SaveNote(ctx context.Context, profileID uuid.UUID, slug, body string) error
	SaveQuizAttempt(ctx context.Context, profileID uuid.UUID, quizSlug, lessonSlug string, score, total int, answers map[string]string) (learn.Attempt, error)
	QuizAttempts(ctx context.Context, profileID uuid.UUID) ([]learn.Attempt, error)
	SaveExerciseAttempt(ctx context.Context, profileID uuid.UUID, exerciseSlug, status string, score, total int, response string) (learn.Attempt, error)
	ExerciseAttempts(ctx context.Context, profileID uuid.UUID) ([]learn.Attempt, error)
	Preferences(ctx context.Context, profileID uuid.UUID) (learn.Preferences, error)
	SavePreferences(ctx context.Context, profileID uuid.UUID, prefs learn.Preferences) error
}

// Credentials verifies an existing account without issuing API tokens.
type Credentials interface {
	VerifyCredentials(ctx context.Context, login, password string) (users.User, error)
}

// Settings controls interface behaviour that differs between local and hosted
// deployments.
type Settings struct {
	SessionTTL        time.Duration
	SecureCookies     bool
	PlaygroundEnabled bool
	PlaygroundURL     string
	PlaygroundTimeout time.Duration
}

// Deps is everything the learning server needs. All fields are required except
// Settings, which has sane defaults.
type Deps struct {
	Curriculum  *learn.Curriculum
	Activities  *learn.Activities
	Spec        *apispec.Spec
	Store       Store
	Credentials Credentials
	Logger      *slog.Logger
	Settings    Settings
}

// Server renders the learning interface and handles its form actions.
type Server struct {
	curriculum  *learn.Curriculum
	activities  *learn.Activities
	spec        *apispec.Spec
	store       Store
	credentials Credentials
	logger      *slog.Logger
	renderer    *renderer
	index       *learn.SearchIndex
	settings    Settings

	searchCache *cache.Cache[string, learn.SearchResult]
	loginLimit  *middleware.RateLimiter
	runLimit    *middleware.RateLimiter
	playground  *playgroundClient
	now         func() time.Time
}

// New builds the learning server and compiles every template up front.
func New(deps Deps) (*Server, error) {
	if deps.Curriculum == nil || deps.Activities == nil || deps.Spec == nil {
		return nil, fmt.Errorf("learn web: curriculum, activities, and spec are required")
	}
	if deps.Store == nil || deps.Credentials == nil {
		return nil, fmt.Errorf("learn web: store and credentials are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Settings.SessionTTL <= 0 {
		deps.Settings.SessionTTL = sessionTTL
	}
	if deps.Settings.PlaygroundTimeout <= 0 {
		deps.Settings.PlaygroundTimeout = 10 * time.Second
	}

	render, err := newRenderer()
	if err != nil {
		return nil, err
	}

	server := &Server{
		curriculum:  deps.Curriculum,
		activities:  deps.Activities,
		spec:        deps.Spec,
		store:       deps.Store,
		credentials: deps.Credentials,
		logger:      deps.Logger,
		renderer:    render,
		index:       learn.NewSearchIndex(deps.Curriculum, deps.Activities),
		settings:    deps.Settings,
		searchCache: cache.New[string, learn.SearchResult](90*time.Second, 128),
		loginLimit:  middleware.NewRateLimiter(10, 5*time.Minute),
		runLimit:    middleware.NewRateLimiter(6, time.Minute),
		playground:  newPlaygroundClient(deps.Settings.PlaygroundURL, deps.Settings.PlaygroundTimeout),
		now:         time.Now,
	}
	return server, nil
}

// Handler mounts the learning interface. Every response is HTML except the
// playground proxy's JSON, which is returned verbatim inside a formatted block.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	staticFS, err := s.renderer.staticHandler()
	if err != nil {
		s.logger.Error("learn static assets unavailable", "error", err)
	}
	if staticFS != nil {
		files := http.FileServer(http.FS(staticFS))
		mux.Handle("GET /learn/static/", http.StripPrefix("/learn/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			files.ServeHTTP(w, r)
		})))
	}

	mux.HandleFunc("GET /learn", s.handleDashboard)
	mux.HandleFunc("GET /learn/courses", s.handleCourses)
	mux.HandleFunc("GET /learn/courses/{course}", s.handleCourse)
	mux.HandleFunc("GET /learn/lessons/{slug}", s.handleLesson)
	mux.HandleFunc("GET /learn/lessons/{slug}/content", s.handleLessonContent)

	mux.HandleFunc("GET /learn/exercises", s.handleExercises)
	mux.HandleFunc("POST /learn/exercises/{slug}", s.handleExerciseSubmit)
	mux.HandleFunc("GET /learn/exercises/{slug}", s.handleExercise)
	mux.HandleFunc("GET /learn/exercises/collections/{slug}", s.handleExerciseCollection)

	mux.HandleFunc("GET /learn/quizzes/{slug}", s.handleQuiz)
	mux.HandleFunc("POST /learn/quizzes/{slug}", s.handleQuizSubmit)

	mux.HandleFunc("GET /learn/glossary", s.handleGlossary)
	mux.HandleFunc("GET /learn/glossary/results", s.handleGlossaryResults)
	mux.HandleFunc("GET /learn/search", s.handleSearch)
	mux.HandleFunc("GET /learn/search/results", s.handleSearchResults)

	mux.HandleFunc("GET /learn/bookmarks", s.handleBookmarks)
	mux.HandleFunc("POST /learn/bookmarks/{slug}", s.handleBookmarkAdd)
	mux.HandleFunc("DELETE /learn/bookmarks/{slug}", s.handleBookmarkRemove)

	mux.HandleFunc("GET /learn/progress", s.handleProgress)
	mux.HandleFunc("GET /learn/review", s.handleReview)
	mux.HandleFunc("POST /learn/progress", s.handleProgressUpdate)
	mux.HandleFunc("POST /learn/notes/{slug}", s.handleNoteSave)
	mux.HandleFunc("POST /learn/preferences", s.handlePreferences)

	mux.HandleFunc("GET /learn/playground", s.handlePlayground)
	mux.HandleFunc("POST /learn/playground", s.handlePlaygroundRun)

	mux.HandleFunc("GET /learn/api", s.handleAPI)
	mux.HandleFunc("GET /learn/api/operations/{id}", s.handleAPIOperation)

	mux.HandleFunc("GET /learn/login", s.handleLoginPage)
	mux.HandleFunc("POST /learn/login", s.handleLogin)
	mux.HandleFunc("POST /learn/logout", s.handleLogout)
	mux.HandleFunc("POST /learn/session/guest", s.handleGuest)

	mux.HandleFunc("/learn/", s.handleNotFound)

	return s.withSession(mux)
}

type stateKey struct{}

// requestState is loaded once per request by withSession.
type requestState struct {
	profile *learn.Profile
	prefs   learn.Preferences
	csrf    string
}

func stateFrom(r *http.Request) *requestState {
	if state, ok := r.Context().Value(stateKey{}).(*requestState); ok {
		return state
	}
	return &requestState{}
}

func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		state := &requestState{}

		if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
			profile, err := s.store.ProfileBySession(ctx, sessionHash(cookie.Value))
			switch {
			case err == nil:
				state.profile = &profile
				if prefs, err := s.store.Preferences(ctx, uuid.MustParse(profile.ID)); err == nil {
					state.prefs = prefs
				} else if !errors.Is(err, learn.ErrNotFound) {
					s.logger.Error("load learn preferences", "error", err)
				}
			case errors.Is(err, learn.ErrNotFound):
				s.clearCookie(w, r, sessionCookie)
			default:
				s.logger.Error("load learn session", "error", err)
			}
		}

		if cookie, err := r.Cookie(csrfCookie); err == nil && len(cookie.Value) >= 32 {
			state.csrf = cookie.Value
		} else {
			state.csrf = newToken()
			s.setCookie(w, r, csrfCookie, state.csrf, time.Time{})
		}

		if unsafeMethod(r.Method) {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := r.ParseForm(); err != nil {
				s.fail(w, r, http.StatusBadRequest, "Request too large", "That form was larger than this page accepts.")
				return
			}
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				token = r.PostFormValue("csrf_token")
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(state.csrf)) != 1 {
				s.fail(w, r, http.StatusForbidden, "Request rejected", "That form token is missing or stale. Reload the page and submit again.")
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, stateKey{}, state)))
	})
}

func clientKey(r *http.Request) string {
	return middleware.ClientIP(r)
}

func unsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func sessionHash(token string) []byte {
	sum := sha256.Sum256([]byte("session:" + token))
	return sum[:]
}

func guestHash(token string) []byte {
	sum := sha256.Sum256([]byte("guest:" + token))
	return sum[:]
}

func newToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand failure is unrecoverable for session security.
		panic("learn web: entropy source unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, name, value string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/learn",
		HttpOnly: name != themeCookie && name != fontCookie && name != motionCookie,
		Secure:   s.secure(r),
		SameSite: http.SameSiteLaxMode,
	}
	if !expires.IsZero() {
		cookie.Expires = expires
		cookie.MaxAge = int(time.Until(expires).Seconds())
	}
	http.SetCookie(w, cookie)
}

func (s *Server) clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/learn",
		HttpOnly: name != themeCookie && name != fontCookie && name != motionCookie,
		Secure:   s.secure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (s *Server) secure(r *http.Request) bool {
	return s.settings.SecureCookies || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.notFound(w, r, "That page does not exist. Use the navigation to find a lesson or course.")
}

func (s *Server) isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (s *Server) fullPage(w http.ResponseWriter, r *http.Request, name string, page *pageData) {
	if page.Path == "" {
		page.Path = r.URL.Path
	}
	var out strings.Builder
	if err := s.renderer.page(&out, name, page); err != nil {
		s.logger.Error("render page", "page", name, "error", err)
		http.Error(w, "This page could not be rendered.", http.StatusInternalServerError)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(out.String()))
}

func (s *Server) renderFragment(w http.ResponseWriter, r *http.Request, name string, data any) {
	var out strings.Builder
	if err := s.renderer.fragment(&out, name, data); err != nil {
		s.logger.Error("render fragment", "fragment", name, "error", err)
		http.Error(w, "This section could not be rendered.", http.StatusInternalServerError)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(out.String()))
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	s.errorPageStatus(w, r, status, title, message)
}

func (s *Server) errorPageStatus(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	state := stateFrom(r)
	page := s.newPage(r, state, title, message)
	page.Data = errorData{Status: status, Title: title, Message: message}
	var out strings.Builder
	if err := s.renderer.page(&out, "error", page); err != nil {
		s.logger.Error("render error page", "error", err)
		http.Error(w, message, status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(out.String()))
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, target string, status int) {
	if s.isHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, status)
}

// safeNext only allows returns inside the learning interface.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/learn") || strings.HasPrefix(next, "//") {
		return "/learn"
	}
	return next
}

func (s *Server) newPage(r *http.Request, state *requestState, title, description string) *pageData {
	page := &pageData{
		Title:         title,
		Description:   description,
		Path:          r.URL.Path,
		Theme:         s.theme(r, state),
		FontScale:     s.fontScale(r, state),
		ReducedMotion: s.reducedMotion(r, state),
		CSRF:          state.csrf,
		Nav:           s.curriculum.Courses,
		Diagnostics:   s.curriculum.Diagnostics,
	}
	if state.profile != nil {
		page.Profile = state.profile
		page.SignedIn = !state.profile.Guest
		page.Guest = state.profile.Guest
		summary, err := s.summary(r.Context(), state.profile)
		if err != nil {
			s.logger.Error("load progress summary", "error", err)
		} else {
			page.Summary = summary
		}
	}
	return page
}

func (s *Server) theme(r *http.Request, state *requestState) string {
	if state.profile != nil && state.prefs.Theme != "" {
		return normalizeTheme(state.prefs.Theme)
	}
	if cookie, err := r.Cookie(themeCookie); err == nil {
		return normalizeTheme(cookie.Value)
	}
	return "auto"
}

func (s *Server) fontScale(r *http.Request, state *requestState) string {
	if state.profile != nil && state.prefs.FontScale != "" {
		return normalizeFont(state.prefs.FontScale)
	}
	if cookie, err := r.Cookie(fontCookie); err == nil {
		return normalizeFont(cookie.Value)
	}
	return "normal"
}

func (s *Server) reducedMotion(r *http.Request, state *requestState) bool {
	if state.profile != nil && state.prefs.ReducedMotion {
		return true
	}
	if cookie, err := r.Cookie(motionCookie); err == nil && cookie.Value == "reduce" {
		return true
	}
	return false
}

func normalizeTheme(value string) string {
	switch value {
	case "light", "dark":
		return value
	default:
		return "auto"
	}
}

func normalizeFont(value string) string {
	switch value {
	case "compact", "large":
		return value
	default:
		return "normal"
	}
}

// loginData is the shape of the dedicated auth screen.
type loginData struct {
	Title      string
	CSRF       string
	Theme      string
	FontScale  string
	Next       string
	Login      string
	Error      string
	HasAccount bool
}

type errorData struct {
	Status  int
	Title   string
	Message string
}
