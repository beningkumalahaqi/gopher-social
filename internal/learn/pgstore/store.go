// Package pgstore is the PostgreSQL implementation of the learning platform
// store. Every query is parameterized; content stays in Markdown, only activity
// is persisted.
package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/haqi/gophersocial/internal/learn"
)

// Store owns SQL for learner profiles, sessions, progress, bookmarks, notes,
// quiz attempts, exercise attempts, and preferences.
type Store struct {
	pool *pgxpool.Pool
}

// New builds a store over the application pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for health checks and tests.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// CreateGuest inserts a guest profile bound to a hashed cookie token.
func (s *Store) CreateGuest(ctx context.Context, guestHash []byte) (learn.Profile, error) {
	const q = `
		INSERT INTO learn_profiles (guest_hash, display_name)
		VALUES ($1, 'Guest learner')
		RETURNING id, created_at`
	var id uuid.UUID
	var createdAt time.Time
	if err := s.pool.QueryRow(ctx, q, guestHash).Scan(&id, &createdAt); err != nil {
		return learn.Profile{}, fmt.Errorf("create guest profile: %w", err)
	}
	return learn.Profile{ID: id.String(), DisplayName: "Guest learner", Guest: true, CreatedAt: createdAt}, nil
}

// EnsureUserProfile returns the profile for an authenticated user, creating it
// on first visit.
func (s *Store) EnsureUserProfile(ctx context.Context, userID uuid.UUID, displayName string) (learn.Profile, error) {
	const q = `
		INSERT INTO learn_profiles (user_id, display_name)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET last_seen_at = now()
		RETURNING id, user_id, display_name, created_at`
	var p learn.Profile
	var id, uid uuid.UUID
	var name *string
	var createdAt time.Time
	if err := s.pool.QueryRow(ctx, q, userID, displayName).Scan(&id, &uid, &name, &createdAt); err != nil {
		return learn.Profile{}, fmt.Errorf("ensure user profile: %w", err)
	}
	p.ID = id.String()
	uidText := uid.String()
	p.UserID = &uidText
	p.DisplayName = displayName
	if name != nil {
		p.DisplayName = *name
	}
	p.CreatedAt = createdAt
	return p, nil
}

// CreateSession stores a hashed session token for a profile.
func (s *Store) CreateSession(ctx context.Context, profileID uuid.UUID, tokenHash []byte, userID *uuid.UUID, userAgent string, expires time.Time) error {
	const q = `
		INSERT INTO learn_sessions (token_hash, profile_id, user_id, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := s.pool.Exec(ctx, q, tokenHash, profileID, userID, truncate(userAgent, 200), expires); err != nil {
		return fmt.Errorf("create learn session: %w", err)
	}
	return nil
}

// ProfileBySession resolves a session token hash and touches last_seen.
func (s *Store) ProfileBySession(ctx context.Context, tokenHash []byte) (learn.Profile, error) {
	const q = `
		WITH touched AS (
			UPDATE learn_sessions SET last_seen_at = now()
			WHERE token_hash = $1 AND expires_at > now()
			RETURNING profile_id
		)
		SELECT p.id, p.user_id, COALESCE(u.username, ''), COALESCE(u.display_name, p.display_name, 'Learner'),
		       p.guest_hash IS NOT NULL, p.created_at
		FROM touched t
		JOIN learn_profiles p ON p.id = t.profile_id
		LEFT JOIN users u ON u.id = p.user_id`
	var p learn.Profile
	var id uuid.UUID
	var userID *uuid.UUID
	var username, displayName string
	var guest bool
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, q, tokenHash).Scan(&id, &userID, &username, &displayName, &guest, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return learn.Profile{}, learn.ErrNotFound
	}
	if err != nil {
		return learn.Profile{}, fmt.Errorf("resolve session: %w", err)
	}
	p.ID = id.String()
	if userID != nil {
		text := userID.String()
		p.UserID = &text
	}
	p.Username = username
	p.DisplayName = displayName
	p.Guest = guest
	p.CreatedAt = createdAt
	return p, nil
}

// DeleteSession removes one session. Logout stays idempotent.
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM learn_sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// MergeGuest copies guest activity into the target profile, keeping any existing
// target rows, then deletes the guest profile. It runs in one transaction.
func (s *Store) MergeGuest(ctx context.Context, guestID, targetProfileID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin merge: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	statements := []string{
		`INSERT INTO learn_lesson_progress (profile_id, lesson_slug, status, reading_percent, first_opened_at, last_opened_at, completed_at)
		 SELECT $2, lesson_slug, status, reading_percent, first_opened_at, last_opened_at, completed_at
		 FROM learn_lesson_progress WHERE profile_id = $1
		 ON CONFLICT (profile_id, lesson_slug) DO NOTHING`,
		`INSERT INTO learn_bookmarks (profile_id, lesson_slug, created_at)
		 SELECT $2, lesson_slug, created_at FROM learn_bookmarks WHERE profile_id = $1
		 ON CONFLICT DO NOTHING`,
		`INSERT INTO learn_notes (profile_id, lesson_slug, body, created_at, updated_at)
		 SELECT $2, lesson_slug, body, created_at, updated_at FROM learn_notes WHERE profile_id = $1
		 ON CONFLICT DO NOTHING`,
		`INSERT INTO learn_quiz_attempts (profile_id, quiz_slug, lesson_slug, score, total, answers, created_at)
		 SELECT $2, quiz_slug, lesson_slug, score, total, answers, created_at
		 FROM learn_quiz_attempts WHERE profile_id = $1`,
		`INSERT INTO learn_exercise_attempts (profile_id, exercise_slug, status, score, total, response, created_at)
		 SELECT $2, exercise_slug, status, score, total, response, created_at
		 FROM learn_exercise_attempts WHERE profile_id = $1`,
		`INSERT INTO learn_preferences (profile_id, theme, font_scale, reduced_motion, updated_at)
		 SELECT $2, theme, font_scale, reduced_motion, updated_at FROM learn_preferences WHERE profile_id = $1
		 ON CONFLICT DO NOTHING`,
	}
	for _, q := range statements {
		if _, err := tx.Exec(ctx, q, guestID, targetProfileID); err != nil {
			return fmt.Errorf("merge guest rows: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM learn_profiles WHERE id = $1`, guestID); err != nil {
		return fmt.Errorf("delete guest profile: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit merge: %w", err)
	}
	return nil
}

// Progress returns every stored lesson row.
func (s *Store) Progress(ctx context.Context, profileID uuid.UUID) ([]learn.Progress, error) {
	const q = `
		SELECT lesson_slug, status, reading_percent, first_opened_at, last_opened_at, completed_at
		FROM learn_lesson_progress WHERE profile_id = $1
		ORDER BY last_opened_at DESC`
	rows, err := s.pool.Query(ctx, q, profileID)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}
	defer rows.Close()

	var out []learn.Progress
	for rows.Next() {
		var p learn.Progress
		if err := rows.Scan(&p.LessonSlug, &p.Status, &p.ReadingPercent, &p.FirstOpenedAt, &p.LastOpenedAt, &p.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TouchLesson records an open without changing completion.
func (s *Store) TouchLesson(ctx context.Context, profileID uuid.UUID, slug string) error {
	const q = `
		INSERT INTO learn_lesson_progress (profile_id, lesson_slug, status)
		VALUES ($1, $2, 'in_progress')
		ON CONFLICT (profile_id, lesson_slug) DO UPDATE SET last_opened_at = now()`
	if _, err := s.pool.Exec(ctx, q, profileID, slug); err != nil {
		return fmt.Errorf("touch lesson: %w", err)
	}
	return nil
}

// SetProgress writes an explicit status and reading percentage. Opening a lesson
// must use TouchLesson instead: comprehension is not inferred from a page view.
func (s *Store) SetProgress(ctx context.Context, profileID uuid.UUID, slug string, status learn.ProgressStatus, percent int) (learn.Progress, error) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	const q = `
		INSERT INTO learn_lesson_progress
			(profile_id, lesson_slug, status, reading_percent, completed_at)
		VALUES ($1, $2, $3, $4, CASE WHEN $3 = 'completed' THEN now() ELSE NULL END)
		ON CONFLICT (profile_id, lesson_slug) DO UPDATE SET
			status = $3,
			reading_percent = GREATEST(learn_lesson_progress.reading_percent, $4),
			completed_at = CASE
				WHEN $3 = 'completed' THEN COALESCE(learn_lesson_progress.completed_at, now())
				ELSE NULL
			END,
			last_opened_at = now()
		RETURNING lesson_slug, status, reading_percent, first_opened_at, last_opened_at, completed_at`
	var p learn.Progress
	err := s.pool.QueryRow(ctx, q, profileID, slug, status, percent).
		Scan(&p.LessonSlug, &p.Status, &p.ReadingPercent, &p.FirstOpenedAt, &p.LastOpenedAt, &p.CompletedAt)
	if err != nil {
		return learn.Progress{}, fmt.Errorf("set progress: %w", err)
	}
	return p, nil
}

// SetReading records scroll progress without changing completion.
func (s *Store) SetReading(ctx context.Context, profileID uuid.UUID, slug string, percent int) (learn.Progress, error) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	const q = `
		INSERT INTO learn_lesson_progress (profile_id, lesson_slug, status, reading_percent)
		VALUES ($1, $2, 'in_progress', $3)
		ON CONFLICT (profile_id, lesson_slug) DO UPDATE SET
			reading_percent = GREATEST(learn_lesson_progress.reading_percent, $3),
			last_opened_at = now()
		RETURNING lesson_slug, status, reading_percent, first_opened_at, last_opened_at, completed_at`
	var p learn.Progress
	err := s.pool.QueryRow(ctx, q, profileID, slug, percent).
		Scan(&p.LessonSlug, &p.Status, &p.ReadingPercent, &p.FirstOpenedAt, &p.LastOpenedAt, &p.CompletedAt)
	if err != nil {
		return learn.Progress{}, fmt.Errorf("set reading progress: %w", err)
	}
	return p, nil
}

// Bookmarks lists saved lesson slugs, newest first.
func (s *Store) Bookmarks(ctx context.Context, profileID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT lesson_slug FROM learn_bookmarks WHERE profile_id = $1 ORDER BY created_at DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		out = append(out, slug)
	}
	return out, rows.Err()
}

// AddBookmark is idempotent.
func (s *Store) AddBookmark(ctx context.Context, profileID uuid.UUID, slug string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO learn_bookmarks (profile_id, lesson_slug) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		profileID, slug)
	if err != nil {
		return fmt.Errorf("add bookmark: %w", err)
	}
	return nil
}

// RemoveBookmark is idempotent.
func (s *Store) RemoveBookmark(ctx context.Context, profileID uuid.UUID, slug string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM learn_bookmarks WHERE profile_id = $1 AND lesson_slug = $2`, profileID, slug)
	if err != nil {
		return fmt.Errorf("remove bookmark: %w", err)
	}
	return nil
}

// Note returns one lesson note or an empty string.
func (s *Store) Note(ctx context.Context, profileID uuid.UUID, slug string) (string, error) {
	var body string
	err := s.pool.QueryRow(ctx,
		`SELECT body FROM learn_notes WHERE profile_id = $1 AND lesson_slug = $2`, profileID, slug).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load note: %w", err)
	}
	return body, nil
}

// SaveNote upserts a note. An empty body deletes it.
func (s *Store) SaveNote(ctx context.Context, profileID uuid.UUID, slug, body string) error {
	if body == "" {
		_, err := s.pool.Exec(ctx,
			`DELETE FROM learn_notes WHERE profile_id = $1 AND lesson_slug = $2`, profileID, slug)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO learn_notes (profile_id, lesson_slug, body)
		VALUES ($1, $2, $3)
		ON CONFLICT (profile_id, lesson_slug) DO UPDATE SET body = EXCLUDED.body, updated_at = now()`,
		profileID, slug, body)
	if err != nil {
		return fmt.Errorf("save note: %w", err)
	}
	return nil
}

// SaveQuizAttempt persists a graded attempt.
func (s *Store) SaveQuizAttempt(ctx context.Context, profileID uuid.UUID, quizSlug, lessonSlug string, score, total int, answers map[string]string) (learn.Attempt, error) {
	encoded, err := json.Marshal(answers)
	if err != nil {
		return learn.Attempt{}, fmt.Errorf("encode answers: %w", err)
	}
	const q = `
		INSERT INTO learn_quiz_attempts (profile_id, quiz_slug, lesson_slug, score, total, answers)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	var id uuid.UUID
	var createdAt time.Time
	if err := s.pool.QueryRow(ctx, q, profileID, quizSlug, lessonSlug, score, total, encoded).Scan(&id, &createdAt); err != nil {
		return learn.Attempt{}, fmt.Errorf("save quiz attempt: %w", err)
	}
	return learn.Attempt{
		ID: id.String(), ProfileID: profileID.String(), QuizSlug: quizSlug,
		LessonSlug: lessonSlug, Score: score, Total: total, Answers: answers, CreatedAt: createdAt,
	}, nil
}

// QuizAttempts returns attempts, newest first.
func (s *Store) QuizAttempts(ctx context.Context, profileID uuid.UUID) ([]learn.Attempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT quiz_slug, lesson_slug, score, total, created_at
		FROM learn_quiz_attempts WHERE profile_id = $1 ORDER BY created_at DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list quiz attempts: %w", err)
	}
	defer rows.Close()
	var out []learn.Attempt
	for rows.Next() {
		var a learn.Attempt
		if err := rows.Scan(&a.QuizSlug, &a.LessonSlug, &a.Score, &a.Total, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SaveExerciseAttempt records a submission. Completion is stored only when the
// learner explicitly marks it.
func (s *Store) SaveExerciseAttempt(ctx context.Context, profileID uuid.UUID, exerciseSlug, status string, score, total int, response string) (learn.Attempt, error) {
	const q = `
		INSERT INTO learn_exercise_attempts (profile_id, exercise_slug, status, score, total, response)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	var id uuid.UUID
	var createdAt time.Time
	if err := s.pool.QueryRow(ctx, q, profileID, exerciseSlug, status, score, total, response).Scan(&id, &createdAt); err != nil {
		return learn.Attempt{}, fmt.Errorf("save exercise attempt: %w", err)
	}
	return learn.Attempt{
		ID: id.String(), ProfileID: profileID.String(), ExerciseSlug: exerciseSlug,
		Status: status, Score: score, Total: total, CreatedAt: createdAt,
	}, nil
}

// ExerciseAttempts returns attempts, newest first.
func (s *Store) ExerciseAttempts(ctx context.Context, profileID uuid.UUID) ([]learn.Attempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT exercise_slug, status, score, total, created_at
		FROM learn_exercise_attempts WHERE profile_id = $1 ORDER BY created_at DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list exercise attempts: %w", err)
	}
	defer rows.Close()
	var out []learn.Attempt
	for rows.Next() {
		var a learn.Attempt
		if err := rows.Scan(&a.ExerciseSlug, &a.Status, &a.Score, &a.Total, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Preferences returns stored settings or defaults.
func (s *Store) Preferences(ctx context.Context, profileID uuid.UUID) (learn.Preferences, error) {
	prefs := learn.Preferences{Theme: "auto", FontScale: "default"}
	err := s.pool.QueryRow(ctx,
		`SELECT theme, font_scale, reduced_motion FROM learn_preferences WHERE profile_id = $1`, profileID).
		Scan(&prefs.Theme, &prefs.FontScale, &prefs.ReducedMotion)
	if errors.Is(err, pgx.ErrNoRows) {
		return prefs, nil
	}
	if err != nil {
		return prefs, fmt.Errorf("load preferences: %w", err)
	}
	return prefs, nil
}

// SavePreferences upserts settings.
func (s *Store) SavePreferences(ctx context.Context, profileID uuid.UUID, prefs learn.Preferences) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO learn_preferences (profile_id, theme, font_scale, reduced_motion)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (profile_id) DO UPDATE SET
			theme = EXCLUDED.theme, font_scale = EXCLUDED.font_scale,
			reduced_motion = EXCLUDED.reduced_motion, updated_at = now()`,
		profileID, prefs.Theme, prefs.FontScale, prefs.ReducedMotion)
	if err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	return nil
}

// UserID parses the optional profile user id for callers that need it.
func UserID(p learn.Profile) (uuid.UUID, bool) {
	if p.UserID == nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(*p.UserID)
	return id, err == nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
