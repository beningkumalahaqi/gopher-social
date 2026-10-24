// Package learn turns the Markdown curriculum into a renderable, searchable
// learning platform. It owns no HTTP concerns and no SQL: the web package and
// pgstore package depend on the types declared here.
package learn

import (
	"errors"
	"html/template"
	"time"
)

// DocKind separates the canonical content kinds discovered on disk.
type DocKind string

const (
	KindLesson   DocKind = "lesson"
	KindExercise DocKind = "exercise"
	KindSolution DocKind = "solution"
)

// Heading is one entry in a document table of contents.
type Heading struct {
	Level int
	ID    string
	Text  string
}

// Doc is a parsed Markdown document. HTML is sanitized and safe to inject.
type Doc struct {
	Slug        string
	Kind        DocKind
	Number      int
	Title       string
	Summary     string
	HTML        template.HTML
	TOC         []Heading
	Text        string
	Code        []string
	Words       int
	ReadMinutes int
	SourcePath  string

	ModuleSlug string
	CourseSlug string

	Prev *Doc
	Next *Doc
}

// Module groups lessons inside a course.
type Module struct {
	Slug       string
	Title      string
	Summary    string
	CourseSlug string
	From       int
	To         int
	New        bool
	Docs       []*Doc
}

// Course is a learning path over one or more modules.
type Course struct {
	Slug     string
	Title    string
	Summary  string
	New      bool
	Modules  []*Module
	Lessons  int
	Minutes  int
	FirstDoc *Doc
}

// Term is one glossary entry parsed from glossary.md.
type Term struct {
	Term       string
	Definition string
	Letter     string
	Slug       string
}

// Diagnostic describes content that could not be parsed. Boot continues; the
// dashboard surfaces warnings so a broken lesson is visible, not silent.
type Diagnostic struct {
	Path    string
	Message string
}

// Curriculum is the fully loaded, immutable content graph. It is built once at
// startup and shared read-only between requests.
type Curriculum struct {
	Docs        []*Doc
	Lessons     []*Doc
	Exercises   []*Doc
	Solutions   []*Doc
	BySlug      map[string]*Doc
	Modules     []*Module
	moduleByKey map[string]*Module
	Courses     []*Course
	courseByKey map[string]*Course
	Glossary    []Term
	Diagnostics []Diagnostic
}

// LessonCount returns the number of canonical lessons.
func (c *Curriculum) LessonCount() int { return len(c.Lessons) }

// Lesson returns a lesson by slug.
func (c *Curriculum) Lesson(slug string) (*Doc, bool) {
	d, ok := c.BySlug[slug]
	return d, ok && d.Kind == KindLesson
}

// Exercise returns an exercise document by slug.
func (c *Curriculum) Exercise(slug string) (*Doc, bool) {
	d, ok := c.BySlug[slug]
	return d, ok && d.Kind == KindExercise
}

// Module returns a module by slug.
func (c *Curriculum) Module(slug string) (*Module, bool) {
	m, ok := c.moduleByKey[slug]
	return m, ok
}

// Course returns a course by slug.
func (c *Curriculum) Course(slug string) (*Course, bool) {
	co, ok := c.courseByKey[slug]
	return co, ok
}

// ModuleForLesson returns the module that owns a lesson.
func (c *Curriculum) ModuleForLesson(d *Doc) *Module {
	if d == nil {
		return nil
	}
	return c.moduleByKey[d.ModuleSlug]
}

// CourseForLesson returns the course that owns a lesson.
func (c *Curriculum) CourseForLesson(d *Doc) *Course {
	if d == nil {
		return nil
	}
	return c.courseByKey[d.CourseSlug]
}

// ProgressStatus is the stored learning state of a lesson.
type ProgressStatus string

const (
	StatusNotStarted ProgressStatus = "not_started"
	StatusInProgress ProgressStatus = "in_progress"
	StatusCompleted  ProgressStatus = "completed"
)

// Progress is one lesson's persisted activity.
type Progress struct {
	LessonSlug     string
	Status         ProgressStatus
	ReadingPercent int
	FirstOpenedAt  time.Time
	LastOpenedAt   time.Time
	CompletedAt    *time.Time
}

// Profile is the learner identity behind a request. UserID is nil for guests.
type Profile struct {
	ID          string
	UserID      *string
	Username    string
	DisplayName string
	Guest       bool
	CreatedAt   time.Time
}

// Preferences are the learner's UI settings.
type Preferences struct {
	Theme         string
	FontScale     string
	ReducedMotion bool
}

// Attempt is a scored quiz or exercise submission.
type Attempt struct {
	ID           string
	ProfileID    string
	QuizSlug     string
	LessonSlug   string
	ExerciseSlug string
	Score        int
	Total        int
	Answers      map[string]string
	Status       string
	CreatedAt    time.Time
}

// ProgressSummary is the dashboard aggregate. It is computed from stored rows,
// never invented.
type ProgressSummary struct {
	TotalLessons     int
	Started          int
	Completed        int
	NotStarted       int
	Percent          int
	CompletedPercent int
	MinutesRead      int
	LastLesson       *Doc
	NextLesson       *Doc
	Recent           []RecentLesson
	Bookmarks        []string
	QuizAttempts     int
	QuizScore        int
	QuizTotal        int
	ExerciseAttempts int
	ExercisesDone    int
	ReviewLessons    []*Doc
}

// RecentLesson pairs a lesson with its last activity.
type RecentLesson struct {
	Doc    *Doc
	Status ProgressStatus
	At     time.Time
}

// BookmarkedLesson is a saved lesson plus metadata.
type BookmarkedLesson struct {
	Doc       *Doc
	CreatedAt time.Time
}

// LessonState is the per-request state of the currently open lesson.
type LessonState struct {
	Doc        *Doc
	Progress   Progress
	Bookmarked bool
	Note       string
	Quiz       *Quiz
	QuizState  *Result
}

// ErrNotFound reports a missing learner record.
var ErrNotFound = errors.New("learn: not found")
