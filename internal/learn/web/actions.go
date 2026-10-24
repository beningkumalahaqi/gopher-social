package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/learn"
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	if state.profile != nil && !state.profile.Guest {
		s.redirect(w, r, "/learn", http.StatusSeeOther)
		return
	}
	s.renderLogin(w, r, http.StatusOK, "", "", safeNext(r.URL.Query().Get("next")))
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, status int, login, message, next string) {
	state := stateFrom(r)
	data := &loginData{
		Title:      "Sign in",
		CSRF:       state.csrf,
		Theme:      s.theme(r, state),
		FontScale:  s.fontScale(r, state),
		Next:       safeNext(next),
		Login:      login,
		Error:      message,
		HasAccount: true,
	}
	var out strings.Builder
	if err := s.renderer.authPage(&out, data); err != nil {
		s.logger.Error("render login page", "error", err)
		http.Error(w, "This page could not be rendered.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(out.String()))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	login := strings.TrimSpace(r.PostFormValue("login"))
	password := r.PostFormValue("password")
	next := safeNext(r.PostFormValue("next"))

	if allowed, retry := s.loginLimit.Allow(clientKey(r)); !allowed {
		s.renderLogin(w, r, http.StatusTooManyRequests, login, "Too many attempts. Try again in "+retry.Round(time.Second).String()+".", next)
		return
	}
	if login == "" || password == "" {
		s.renderLogin(w, r, http.StatusBadRequest, login, "Enter your username or email and your password.", next)
		return
	}

	user, err := s.credentials.VerifyCredentials(r.Context(), login, password)
	if err != nil {
		s.logger.Info("learn login rejected", "error", err)
		s.renderLogin(w, r, http.StatusUnauthorized, login, "That login and password do not match an account.", next)
		return
	}

	profile, err := s.store.EnsureUserProfile(r.Context(), user.ID, user.DisplayName)
	if err != nil {
		s.logger.Error("ensure learn profile", "error", err)
		s.renderLogin(w, r, http.StatusInternalServerError, login, "Your account could not be opened. Try again.", next)
		return
	}
	if state.profile != nil && state.profile.Guest {
		if err := s.store.MergeGuest(r.Context(), uuid.MustParse(state.profile.ID), uuid.MustParse(profile.ID)); err != nil && !errors.Is(err, learn.ErrNotFound) {
			s.logger.Error("merge guest progress", "error", err)
		}
	}

	token := newToken()
	displayName := profile.DisplayName
	if displayName == "" {
		displayName = user.Username
	}
	if err := s.store.CreateSession(r.Context(), uuid.MustParse(profile.ID), sessionHash(token), &user.ID, truncate(r.UserAgent(), 255), s.now().Add(s.settings.SessionTTL)); err != nil {
		s.logger.Error("create learn session", "error", err)
		s.renderLogin(w, r, http.StatusInternalServerError, login, "A session could not be created. Try again.", next)
		return
	}
	s.setCookie(w, r, sessionCookie, token, s.now().Add(s.settings.SessionTTL))
	s.redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) handleGuest(w http.ResponseWriter, r *http.Request) {
	token := newToken()
	profile, err := s.store.CreateGuest(r.Context(), guestHash(token))
	if err != nil {
		s.logger.Error("create guest profile", "error", err)
		s.fail(w, r, http.StatusInternalServerError, "Could not start", "A guest profile could not be created. Try again.")
		return
	}
	if err := s.store.CreateSession(r.Context(), uuid.MustParse(profile.ID), sessionHash(token), nil, truncate(r.UserAgent(), 255), s.now().Add(s.settings.SessionTTL)); err != nil {
		s.logger.Error("create guest session", "error", err)
		s.fail(w, r, http.StatusInternalServerError, "Could not start", "A guest session could not be created. Try again.")
		return
	}
	s.setCookie(w, r, sessionCookie, token, s.now().Add(s.settings.SessionTTL))
	s.redirect(w, r, safeNext(r.PostFormValue("next")), http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		if err := s.store.DeleteSession(r.Context(), sessionHash(cookie.Value)); err != nil {
			s.logger.Error("delete learn session", "error", err)
		}
	}
	s.clearCookie(w, r, sessionCookie)
	s.redirect(w, r, "/learn", http.StatusSeeOther)
}

type progressControlData struct {
	Doc          *learn.Doc
	Progress     learn.Progress
	NeedsProfile bool
	CSRF         string
}

func (s *Server) handleProgressUpdate(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	doc, ok := s.curriculum.Lesson(r.PostFormValue("lesson_slug"))
	if !ok {
		s.fail(w, r, http.StatusNotFound, "Lesson not found", "That lesson does not exist.")
		return
	}
	if state.profile == nil {
		data := progressControlData{Doc: doc, NeedsProfile: true, CSRF: state.csrf}
		if s.isHTMX(r) {
			s.renderFragment(w, r, "progress-control", data)
			return
		}
		s.redirect(w, r, "/learn/login?next="+doc.Slug, http.StatusSeeOther)
		return
	}
	id := uuid.MustParse(state.profile.ID)
	reading, _ := strconv.Atoi(r.PostFormValue("percent"))
	status := learn.ProgressStatus(r.PostFormValue("status"))

	var (
		row learn.Progress
		err error
	)
	switch status {
	case learn.StatusCompleted:
		row, err = s.store.SetProgress(r.Context(), id, doc.Slug, learn.StatusCompleted, 100)
	case learn.StatusInProgress:
		row, err = s.store.SetProgress(r.Context(), id, doc.Slug, learn.StatusInProgress, reading)
	default:
		row, err = s.store.SetReading(r.Context(), id, doc.Slug, reading)
	}
	if err != nil {
		s.logger.Error("save progress", "error", err)
		s.fail(w, r, http.StatusInternalServerError, "Progress not saved", "That update could not be stored. Try again.")
		return
	}
	data := progressControlData{Doc: doc, Progress: row, CSRF: state.csrf}
	if s.isHTMX(r) {
		s.renderFragment(w, r, "progress-control", data)
		return
	}
	s.redirect(w, r, "/learn/lessons/"+doc.Slug, http.StatusSeeOther)
}

type bookmarkControlData struct {
	Doc          *learn.Doc
	Bookmarked   bool
	NeedsProfile bool
	CSRF         string
}

func (s *Server) handleBookmarkAdd(w http.ResponseWriter, r *http.Request) {
	s.setBookmark(w, r, true)
}

func (s *Server) handleBookmarkRemove(w http.ResponseWriter, r *http.Request) {
	s.setBookmark(w, r, false)
}

func (s *Server) setBookmark(w http.ResponseWriter, r *http.Request, add bool) {
	state := stateFrom(r)
	doc, ok := s.curriculum.Lesson(r.PathValue("slug"))
	if !ok {
		s.fail(w, r, http.StatusNotFound, "Lesson not found", "That lesson does not exist.")
		return
	}
	if state.profile == nil {
		if s.isHTMX(r) {
			s.renderFragment(w, r, "bookmark-control", bookmarkControlData{Doc: doc, NeedsProfile: true, CSRF: state.csrf})
			return
		}
		s.redirect(w, r, "/learn/login", http.StatusSeeOther)
		return
	}
	id := uuid.MustParse(state.profile.ID)
	var err error
	if add {
		err = s.store.AddBookmark(r.Context(), id, doc.Slug)
	} else {
		err = s.store.RemoveBookmark(r.Context(), id, doc.Slug)
	}
	if err != nil {
		s.logger.Error("save bookmark", "error", err)
		s.fail(w, r, http.StatusInternalServerError, "Bookmark not saved", "That bookmark could not be stored. Try again.")
		return
	}
	data := bookmarkControlData{Doc: doc, Bookmarked: add, CSRF: state.csrf}
	if s.isHTMX(r) {
		s.renderFragment(w, r, "bookmark-control", data)
		return
	}
	s.redirect(w, r, "/learn/lessons/"+doc.Slug, http.StatusSeeOther)
}

func (s *Server) handleNoteSave(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	doc, ok := s.curriculum.Lesson(r.PathValue("slug"))
	if !ok {
		s.fail(w, r, http.StatusNotFound, "Lesson not found", "That lesson does not exist.")
		return
	}
	if state.profile == nil {
		s.renderFragment(w, r, "note-status", "Start a guest session or sign in to keep notes.")
		return
	}
	body := strings.TrimSpace(r.PostFormValue("body"))
	if len(body) > 20000 {
		s.renderFragment(w, r, "note-status", "Notes are limited to 20,000 characters.")
		return
	}
	if err := s.store.SaveNote(r.Context(), uuid.MustParse(state.profile.ID), doc.Slug, body); err != nil {
		s.logger.Error("save note", "error", err)
		s.renderFragment(w, r, "note-status", "That note could not be saved. Try again.")
		return
	}
	s.renderFragment(w, r, "note-status", "Saved")
}

func (s *Server) handlePreferences(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	theme := normalizeTheme(r.PostFormValue("theme"))
	font := normalizeFont(r.PostFormValue("font_scale"))
	motion := r.PostFormValue("reduced_motion") == "true" || r.PostFormValue("reduced_motion") == "on"

	expires := s.now().Add(365 * 24 * time.Hour)
	s.setCookie(w, r, themeCookie, theme, expires)
	s.setCookie(w, r, fontCookie, font, expires)
	motionValue := "allow"
	if motion {
		motionValue = "reduce"
	}
	s.setCookie(w, r, motionCookie, motionValue, expires)

	if state.profile != nil {
		prefs := learn.Preferences{Theme: theme, FontScale: font, ReducedMotion: motion}
		if err := s.store.SavePreferences(r.Context(), uuid.MustParse(state.profile.ID), prefs); err != nil {
			s.logger.Error("save preferences", "error", err)
			s.fail(w, r, http.StatusInternalServerError, "Preferences not saved", "That setting could not be stored. Try again.")
			return
		}
	}
	if s.isHTMX(r) {
		s.renderFragment(w, r, "prefs-status", "Saved")
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		s.redirect(w, r, safeNext(r.PostFormValue("next")), http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseAnswers reads only the fields belonging to the questions being graded,
// so a stray form key can never be mistaken for an answer.
func (s *Server) parseAnswers(r *http.Request, questions []learn.Question) map[string]learn.Answer {
	answers := make(map[string]learn.Answer, len(questions))
	for _, question := range questions {
		key := "q_" + question.ID
		if question.Type == learn.QuestionShort {
			if text := strings.TrimSpace(r.PostFormValue(key)); text != "" {
				answers[question.ID] = learn.Answer{Text: text}
			}
			continue
		}
		if values := r.PostForm[key]; len(values) > 0 {
			answers[question.ID] = learn.Answer{Options: values}
		}
	}
	return answers
}

func (s *Server) handleQuizSubmit(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	quiz, ok := s.activities.QuizBySlug[r.PathValue("slug")]
	if !ok {
		s.fail(w, r, http.StatusNotFound, "Quiz not found", "That quiz does not exist.")
		return
	}
	answers := s.parseAnswers(r, quiz.Questions)
	result := learn.Grade(quiz, answers)

	data := struct {
		Quiz         *learn.Quiz
		Result       learn.Result
		Saved        bool
		NeedsProfile bool
	}{Quiz: quiz, Result: result}

	if state.profile == nil {
		data.NeedsProfile = true
	} else {
		flat := make(map[string]string, len(answers))
		for id, answer := range answers {
			if len(answer.Options) > 0 {
				flat[id] = strings.Join(answer.Options, ",")
			} else {
				flat[id] = answer.Text
			}
		}
		if _, err := s.store.SaveQuizAttempt(r.Context(), uuid.MustParse(state.profile.ID), quiz.Slug, quiz.LessonSlug, result.Score, result.Total, flat); err != nil {
			s.logger.Error("save quiz attempt", "error", err)
		} else {
			data.Saved = true
		}
	}
	s.renderFragment(w, r, "quiz-result", data)
}

func (s *Server) handleExerciseSubmit(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	exercise, ok := s.activities.ExerciseBySlug[r.PathValue("slug")]
	if !ok {
		s.fail(w, r, http.StatusNotFound, "Exercise not found", "That exercise does not exist.")
		return
	}
	answers := s.parseAnswers(r, exercise.Questions)
	quiz := &learn.Quiz{Slug: exercise.Slug, Title: exercise.Title, LessonSlug: exercise.LessonSlug, Questions: exercise.Questions}
	result := learn.Grade(quiz, answers)

	status := "submitted"
	if result.Score == result.Total && result.Total > 0 {
		status = "completed"
	}
	data := struct {
		Exercise     *learn.Exercise
		Result       learn.Result
		Saved        bool
		NeedsProfile bool
	}{Exercise: exercise, Result: result}

	if state.profile != nil {
		if _, err := s.store.SaveExerciseAttempt(r.Context(), uuid.MustParse(state.profile.ID), exercise.Slug, status, result.Score, result.Total, ""); err != nil {
			s.logger.Error("save exercise attempt", "error", err)
		} else {
			data.Saved = true
		}
	} else {
		data.NeedsProfile = true
	}
	s.renderFragment(w, r, "exercise-result", data)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
