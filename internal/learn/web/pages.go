package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/haqi/gophersocial/internal/learn"
)

type lessonView struct {
	Doc        *learn.Doc
	Progress   learn.Progress
	Bookmarked bool
}

type moduleView struct {
	Module  *learn.Module
	Lessons []lessonView
}

type courseProgress struct {
	Course    *learn.Course
	Total     int
	Started   int
	Completed int
	Percent   int
}

type courseData struct {
	Course    *learn.Course
	Modules   []moduleView
	Total     int
	Started   int
	Completed int
	Percent   int
}

type dashboardData struct {
	Summary    learn.ProgressSummary
	Recent     []learn.RecentLesson
	Next       *learn.Doc
	Bookmarks  []lessonView
	Courses    []courseProgress
	Review     []reviewItem
	Onboarding bool
}

type reviewItem struct {
	Doc      *learn.Doc
	Progress learn.Progress
	Reason   string
}

type exerciseItem struct {
	Exercise *learn.Exercise
	Lesson   *learn.Doc
	Latest   *learn.Attempt
}

type quizData struct {
	Quiz     *learn.Quiz
	Lesson   *learn.Doc
	Attempts []learn.Attempt
	Best     *learn.Attempt
}

type glossaryData struct {
	Terms   []learn.Term
	Query   string
	Letter  string
	Term    string
	Letters []string
}

type searchPageData struct {
	Result  learn.SearchResult
	Query   string
	Filters learn.SearchFilters
	Courses []*learn.Course
	Modules []*learn.Module
}

type progressData struct {
	Summary   learn.ProgressSummary
	Courses   []courseProgress
	Quizzes   []learn.Attempt
	Exercises []learn.Attempt
}

type bookmarksData struct {
	Items []lessonView
}

// lessonContentData carries the lesson state plus the request-scoped values the
// shared lesson fragment needs.
type lessonContentData struct {
	learn.LessonState
	CSRF    string
	Profile *learn.Profile
}

type exercisePageData struct {
	Exercise *learn.Exercise
	Lesson   *learn.Doc
	Attempts []learn.Attempt
	CSRF     string
	Profile  *learn.Profile
}

func (s *Server) summary(ctx context.Context, profile *learn.Profile) (learn.ProgressSummary, error) {
	if profile == nil {
		return learn.ProgressSummary{TotalLessons: s.curriculum.LessonCount()}, nil
	}
	id := uuid.MustParse(profile.ID)
	progress, err := s.store.Progress(ctx, id)
	if err != nil {
		return learn.ProgressSummary{}, fmt.Errorf("load progress: %w", err)
	}
	bookmarks, err := s.store.Bookmarks(ctx, id)
	if err != nil {
		return learn.ProgressSummary{}, fmt.Errorf("load bookmarks: %w", err)
	}
	quizzes, err := s.store.QuizAttempts(ctx, id)
	if err != nil {
		return learn.ProgressSummary{}, fmt.Errorf("load quiz attempts: %w", err)
	}
	exercises, err := s.store.ExerciseAttempts(ctx, id)
	if err != nil {
		return learn.ProgressSummary{}, fmt.Errorf("load exercise attempts: %w", err)
	}
	return s.summarize(progress, bookmarks, quizzes, exercises), nil
}

// summarize is pure so the dashboard numbers are testable without a database.
func (s *Server) summarize(progress []learn.Progress, bookmarks []string, quizzes, exercises []learn.Attempt) learn.ProgressSummary {
	summary := learn.ProgressSummary{
		TotalLessons:     s.curriculum.LessonCount(),
		Bookmarks:        bookmarks,
		QuizAttempts:     len(quizzes),
		ExerciseAttempts: len(exercises),
	}
	bySlug := make(map[string]learn.Progress, len(progress))
	for _, row := range progress {
		bySlug[row.LessonSlug] = row
	}
	summary.NotStarted = summary.TotalLessons

	var recent []learn.RecentLesson
	var lastActivity time.Time
	for _, doc := range s.curriculum.Lessons {
		row, ok := bySlug[doc.Slug]
		if !ok {
			continue
		}
		started := row.Status == learn.StatusCompleted || row.ReadingPercent > 0 || row.Status == learn.StatusInProgress
		if !started {
			continue
		}
		summary.Started++
		summary.NotStarted--
		if row.Status == learn.StatusCompleted {
			summary.Completed++
			summary.MinutesRead += doc.ReadMinutes
		} else {
			summary.MinutesRead += doc.ReadMinutes * row.ReadingPercent / 100
		}
		if row.LastOpenedAt.After(lastActivity) {
			lastActivity = row.LastOpenedAt
			summary.LastLesson = doc
		}
		recent = append(recent, learn.RecentLesson{Doc: doc, Status: row.Status, At: row.LastOpenedAt})
	}
	summary.Percent = percent(summary.Started, summary.TotalLessons)
	summary.CompletedPercent = percent(summary.Completed, summary.TotalLessons)
	sort.Slice(recent, func(i, j int) bool { return recent[i].At.After(recent[j].At) })
	if len(recent) > 5 {
		recent = recent[:5]
	}
	summary.Recent = recent

	bestQuiz := make(map[string]int)
	bestQuizTotal := make(map[string]int)
	exercisesDone := make(map[string]bool)
	for _, attempt := range quizzes {
		if attempt.Total == 0 {
			continue
		}
		score := attempt.Score * 100 / attempt.Total
		if score > bestQuiz[attempt.QuizSlug] {
			bestQuiz[attempt.QuizSlug] = score
			bestQuizTotal[attempt.QuizSlug] = attempt.Total
		}
	}
	for slug, score := range bestQuiz {
		summary.QuizScore += score * bestQuizTotal[slug] / 100
		summary.QuizTotal += bestQuizTotal[slug]
	}
	for _, attempt := range exercises {
		if attempt.Status == "completed" {
			exercisesDone[attempt.ExerciseSlug] = true
		}
	}
	summary.ExercisesDone = len(exercisesDone)

	summary.NextLesson = s.nextLesson(bySlug, summary.LastLesson)

	stale := s.now().Add(-7 * 24 * time.Hour)
	for _, doc := range s.curriculum.Lessons {
		row, ok := bySlug[doc.Slug]
		if !ok || row.Status == learn.StatusCompleted {
			continue
		}
		quiz := s.activities.QuizForLesson(doc.Slug)
		if quiz != nil && bestQuiz[quiz.Slug] > 0 && bestQuiz[quiz.Slug] < 70 {
			summary.ReviewLessons = append(summary.ReviewLessons, doc)
			continue
		}
		if row.Status == learn.StatusInProgress && row.LastOpenedAt.Before(stale) {
			summary.ReviewLessons = append(summary.ReviewLessons, doc)
		}
	}
	if len(summary.ReviewLessons) > 8 {
		summary.ReviewLessons = summary.ReviewLessons[:8]
	}
	return summary
}

// nextLesson prefers the lesson after the most recent one, then the first
// unfinished lesson in curriculum order.
func (s *Server) nextLesson(bySlug map[string]learn.Progress, last *learn.Doc) *learn.Doc {
	start := 0
	if last != nil {
		for i, doc := range s.curriculum.Lessons {
			if doc.Slug == last.Slug {
				start = i + 1
				break
			}
		}
	}
	for _, doc := range s.curriculum.Lessons[start:] {
		if bySlug[doc.Slug].Status != learn.StatusCompleted {
			return doc
		}
	}
	for _, doc := range s.curriculum.Lessons {
		if bySlug[doc.Slug].Status != learn.StatusCompleted {
			return doc
		}
	}
	return nil
}

func (s *Server) bookmarkSet(ctx context.Context, profile *learn.Profile) map[string]bool {
	set := make(map[string]bool)
	if profile == nil {
		return set
	}
	slugs, err := s.store.Bookmarks(ctx, uuid.MustParse(profile.ID))
	if err != nil {
		s.logger.Error("load bookmarks", "error", err)
		return set
	}
	for _, slug := range slugs {
		set[slug] = true
	}
	return set
}

func (s *Server) progressMap(ctx context.Context, profile *learn.Profile) map[string]learn.Progress {
	out := make(map[string]learn.Progress)
	if profile == nil {
		return out
	}
	rows, err := s.store.Progress(ctx, uuid.MustParse(profile.ID))
	if err != nil {
		s.logger.Error("load progress", "error", err)
		return out
	}
	for _, row := range rows {
		out[row.LessonSlug] = row
	}
	return out
}

func (s *Server) courseViews(ctx context.Context, profile *learn.Profile) []courseProgress {
	progress := s.progressMap(ctx, profile)
	var out []courseProgress
	for _, course := range s.curriculum.Courses {
		item := courseProgress{Course: course, Total: course.Lessons}
		for _, module := range course.Modules {
			for _, doc := range module.Docs {
				switch progress[doc.Slug].Status {
				case learn.StatusCompleted:
					item.Completed++
					item.Started++
				case learn.StatusInProgress:
					item.Started++
				}
			}
		}
		item.Percent = percent(item.Completed, item.Total)
		out = append(out, item)
	}
	return out
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Learn Go by reading the code you already wrote", "A reading-first Go curriculum built on a real social API: lessons, quizzes, exercises, and a reference you can run.")

	summary := page.Summary
	progress := s.progressMap(r.Context(), state.profile)

	var bookmarked []lessonView
	for _, slug := range summary.Bookmarks {
		if doc, ok := s.curriculum.Lesson(slug); ok {
			bookmarked = append(bookmarked, lessonView{Doc: doc, Progress: progress[slug], Bookmarked: true})
		}
	}
	if len(bookmarked) > 4 {
		bookmarked = bookmarked[:4]
	}

	data := dashboardData{
		Summary:    summary,
		Recent:     summary.Recent,
		Next:       summary.NextLesson,
		Bookmarks:  bookmarked,
		Courses:    s.courseViews(r.Context(), state.profile),
		Review:     s.reviewItems(summary.ReviewLessons, progress),
		Onboarding: summary.Started == 0 && summary.QuizAttempts == 0 && summary.ExerciseAttempts == 0,
	}
	page.Data = data
	if state.profile == nil {
		page.Description = "Start reading as a guest, or sign in to keep progress across devices."
	}
	s.fullPage(w, r, "dashboard", page)
}

func (s *Server) reviewItems(docs []*learn.Doc, progress map[string]learn.Progress) []reviewItem {
	items := make([]reviewItem, 0, len(docs))
	for _, doc := range docs {
		reason := "Opened a while ago and not finished"
		if quiz := s.activities.QuizForLesson(doc.Slug); quiz != nil {
			reason = "Quiz score below 70 percent"
		}
		items = append(items, reviewItem{Doc: doc, Progress: progress[doc.Slug], Reason: reason})
	}
	return items
}

func (s *Server) handleCourses(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Courses", "Four paths through the same codebase, ordered so each lesson depends only on the ones before it.")
	page.Data = s.courseViews(r.Context(), state.profile)
	s.fullPage(w, r, "courses", page)
}

func (s *Server) handleCourse(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	course, ok := s.curriculum.Course(r.PathValue("course"))
	if !ok {
		s.notFound(w, r, "That course does not exist.")
		return
	}
	page := s.newPage(r, state, course.Title, course.Summary)
	page.ActiveCourse = course.Slug
	page.CrumbCourse = course.Title

	progress := s.progressMap(r.Context(), state.profile)
	data := courseData{Course: course, Total: course.Lessons}
	for _, module := range course.Modules {
		view := moduleView{Module: module}
		for _, doc := range module.Docs {
			row := progress[doc.Slug]
			view.Lessons = append(view.Lessons, lessonView{Doc: doc, Progress: row})
			if row.Status == learn.StatusCompleted {
				data.Completed++
				data.Started++
			} else if row.Status == learn.StatusInProgress {
				data.Started++
			}
		}
		data.Modules = append(data.Modules, view)
	}
	data.Percent = percent(data.Completed, data.Total)
	page.Data = data
	s.fullPage(w, r, "course", page)
}

func (s *Server) handleLesson(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	doc, ok := s.curriculum.Lesson(r.PathValue("slug"))
	if !ok {
		s.notFound(w, r, "That lesson does not exist.")
		return
	}
	page := s.lessonPage(r, state, doc)
	data, err := s.lessonState(r.Context(), state, doc)
	if err != nil {
		s.logger.Error("load lesson state", "error", err)
	}
	page.Data = lessonContentData{LessonState: data, CSRF: state.csrf, Profile: state.profile}
	s.fullPage(w, r, "lesson", page)
}

// handleLessonContent returns the interactive column for in-place navigation.
func (s *Server) handleLessonContent(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	doc, ok := s.curriculum.Lesson(r.PathValue("slug"))
	if !ok {
		s.fail(w, r, http.StatusNotFound, "Lesson not found", "That lesson does not exist.")
		return
	}
	data, err := s.lessonState(r.Context(), state, doc)
	if err != nil {
		s.logger.Error("load lesson state", "error", err)
	}
	s.renderFragment(w, r, "lesson-content", lessonContentData{LessonState: data, CSRF: state.csrf, Profile: state.profile})
}

func (s *Server) lessonPage(r *http.Request, state *requestState, doc *learn.Doc) *pageData {
	page := s.newPage(r, state, doc.Title, doc.Summary)
	page.ActiveSlug = doc.Slug
	if module := s.curriculum.ModuleForLesson(doc); module != nil {
		page.ActiveModule = module.Slug
		page.ActiveCourse = module.CourseSlug
		page.CrumbCourse = module.Title
		if course := s.curriculum.CourseForLesson(doc); course != nil {
			page.CrumbCourse = course.Title
			page.CrumbModule = module.Title
		}
	}
	return page
}

func (s *Server) lessonState(ctx context.Context, state *requestState, doc *learn.Doc) (learn.LessonState, error) {
	out := learn.LessonState{Doc: doc}
	if state.profile == nil {
		return out, nil
	}
	id := uuid.MustParse(state.profile.ID)
	if err := s.store.TouchLesson(ctx, id, doc.Slug); err != nil {
		return out, fmt.Errorf("touch lesson: %w", err)
	}
	rows, err := s.store.Progress(ctx, id)
	if err != nil {
		return out, fmt.Errorf("load progress: %w", err)
	}
	for _, row := range rows {
		if row.LessonSlug == doc.Slug {
			out.Progress = row
			break
		}
	}
	bookmarks, err := s.store.Bookmarks(ctx, id)
	if err != nil {
		return out, fmt.Errorf("load bookmarks: %w", err)
	}
	for _, slug := range bookmarks {
		if slug == doc.Slug {
			out.Bookmarked = true
		}
	}
	note, err := s.store.Note(ctx, id, doc.Slug)
	if err != nil && err != learn.ErrNotFound {
		return out, fmt.Errorf("load note: %w", err)
	}
	out.Note = note
	out.Quiz = s.activities.QuizForLesson(doc.Slug)
	return out, nil
}

func (s *Server) handleExercises(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Exercises", "Code-reading tasks with explicit acceptance criteria. Each one points at a file in this repository.")
	var items []exerciseItem
	attempts := map[string]*learn.Attempt{}
	if state.profile != nil {
		rows, err := s.store.ExerciseAttempts(r.Context(), uuid.MustParse(state.profile.ID))
		if err != nil {
			s.logger.Error("load exercise attempts", "error", err)
		}
		for i := range rows {
			row := &rows[i]
			if current, ok := attempts[row.ExerciseSlug]; !ok || row.CreatedAt.After(current.CreatedAt) {
				attempts[row.ExerciseSlug] = row
			}
		}
	}
	for _, exercise := range s.activities.Exercises {
		items = append(items, exerciseItem{
			Exercise: exercise,
			Lesson:   exercise.LessonDoc,
			Latest:   attempts[exercise.Slug],
		})
	}
	page.Data = items
	s.fullPage(w, r, "exercises", page)
}

func (s *Server) handleExercise(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	exercise, ok := s.activities.ExerciseBySlug[r.PathValue("slug")]
	if !ok {
		s.notFound(w, r, "That exercise does not exist.")
		return
	}
	page := s.newPage(r, state, exercise.Title, exercise.Summary)
	page.ActiveSlug = exercise.LessonSlug
	if exercise.LessonDoc != nil {
		if course := s.curriculum.CourseForLesson(exercise.LessonDoc); course != nil {
			page.ActiveCourse = course.Slug
			page.CrumbCourse = course.Title
		}
		if module := s.curriculum.ModuleForLesson(exercise.LessonDoc); module != nil {
			page.CrumbModule = module.Title
		}
	}
	data := exercisePageData{Exercise: exercise, Lesson: exercise.LessonDoc, CSRF: state.csrf, Profile: state.profile}
	if state.profile != nil {
		rows, err := s.store.ExerciseAttempts(r.Context(), uuid.MustParse(state.profile.ID))
		if err != nil {
			s.logger.Error("load exercise attempts", "error", err)
		}
		for _, row := range rows {
			if row.ExerciseSlug == exercise.Slug {
				data.Attempts = append(data.Attempts, row)
			}
		}
	}
	page.Data = data
	s.fullPage(w, r, "exercise", page)
}

func (s *Server) handleQuiz(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	quiz, ok := s.activities.QuizBySlug[r.PathValue("slug")]
	if !ok {
		s.notFound(w, r, "That quiz does not exist.")
		return
	}
	page := s.newPage(r, state, quiz.Title, quiz.Summary)
	page.ActiveSlug = quiz.LessonSlug
	if quiz.Lesson != nil {
		if course := s.curriculum.CourseForLesson(quiz.Lesson); course != nil {
			page.ActiveCourse = course.Slug
			page.CrumbCourse = course.Title
		}
		if module := s.curriculum.ModuleForLesson(quiz.Lesson); module != nil {
			page.CrumbModule = module.Title
		}
	}
	data := quizData{Quiz: quiz, Lesson: quiz.Lesson}
	if state.profile != nil {
		rows, err := s.store.QuizAttempts(r.Context(), uuid.MustParse(state.profile.ID))
		if err != nil {
			s.logger.Error("load quiz attempts", "error", err)
		}
		for _, row := range rows {
			if row.QuizSlug != quiz.Slug {
				continue
			}
			data.Attempts = append(data.Attempts, row)
			if data.Best == nil || row.Score*data.Best.Total > data.Best.Score*row.Total {
				best := row
				data.Best = &best
			}
		}
	}
	page.Data = data
	s.fullPage(w, r, "quiz", page)
}

func (s *Server) handleGlossary(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Glossary", "The terms this codebase uses, defined once, linked from every lesson.")
	page.Data = s.glossary(r)
	s.fullPage(w, r, "glossary", page)
}

func (s *Server) handleGlossaryResults(w http.ResponseWriter, r *http.Request) {
	s.renderFragment(w, r, "glossary-results", s.glossary(r))
}

func (s *Server) glossary(r *http.Request) glossaryData {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	letter := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("letter")))
	termSlug := strings.TrimSpace(r.URL.Query().Get("term"))
	data := glossaryData{Query: query, Letter: letter, Term: termSlug}
	seen := map[string]bool{}
	for _, term := range s.curriculum.Glossary {
		if !seen[term.Letter] {
			seen[term.Letter] = true
			data.Letters = append(data.Letters, term.Letter)
		}
		if letter != "" && term.Letter != letter {
			continue
		}
		if termSlug != "" && term.Slug != termSlug {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(term.Term), query) && !strings.Contains(strings.ToLower(term.Definition), query) {
			continue
		}
		data.Terms = append(data.Terms, term)
	}
	return data
}

func (s *Server) handleExerciseCollection(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	doc, ok := s.curriculum.Exercise(slug)
	if !ok {
		s.notFound(w, r, "That exercise collection does not exist.")
		return
	}
	state := stateFrom(r)
	page := s.newPage(r, state, doc.Title, doc.Summary)
	page.ActiveSlug = doc.Slug
	page.Data = doc
	s.fullPage(w, r, "exercise-collection", page)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Search", "Search lessons, exercises, and definitions without leaving the curriculum.")
	page.Data = s.searchData(r, state)
	s.fullPage(w, r, "search", page)
}

func (s *Server) handleSearchResults(w http.ResponseWriter, r *http.Request) {
	s.renderFragment(w, r, "search-results", s.searchData(r, stateFrom(r)))
}

func (s *Server) searchData(r *http.Request, state *requestState) searchPageData {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	filters := learn.SearchFilters{
		Course: r.URL.Query().Get("course"),
		Module: r.URL.Query().Get("module"),
		Kind:   r.URL.Query().Get("kind"),
		Status: r.URL.Query().Get("status"),
	}
	status := make(map[string]learn.ProgressStatus)
	for slug, row := range s.progressMap(r.Context(), state.profile) {
		status[slug] = row.Status
	}

	var result learn.SearchResult
	if query == "" {
		result = s.index.Search(query, filters, status)
	} else {
		key := searchCacheKey(query, filters, status)
		cached, err := s.searchCache.GetOrLoad(r.Context(), key, func(context.Context) (learn.SearchResult, error) {
			return s.index.Search(query, filters, status), nil
		})
		if err != nil {
			result = s.index.Search(query, filters, status)
		} else {
			result = cached
		}
	}
	data := searchPageData{
		Result:  result,
		Query:   query,
		Filters: filters,
		Courses: s.curriculum.Courses,
		Modules: s.curriculum.Modules,
	}
	if query == "" {
		data.Result.Query = ""
	}
	return data
}

func (s *Server) handleBookmarks(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Bookmarks", "Lessons you saved for later, with the same progress state as everywhere else.")
	var items []lessonView
	if state.profile != nil {
		slugs, err := s.store.Bookmarks(r.Context(), uuid.MustParse(state.profile.ID))
		if err != nil {
			s.logger.Error("load bookmarks", "error", err)
		}
		progress := s.progressMap(r.Context(), state.profile)
		for _, slug := range slugs {
			if doc, ok := s.curriculum.Lesson(slug); ok {
				items = append(items, lessonView{Doc: doc, Progress: progress[slug], Bookmarked: true})
			}
		}
	}
	page.Data = bookmarksData{Items: items}
	s.fullPage(w, r, "bookmarks", page)
}

func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Progress", "Every number here comes from your stored activity: reading, quizzes, and exercises.")
	data := progressData{
		Summary: page.Summary,
		Courses: s.courseViews(r.Context(), state.profile),
	}
	if state.profile != nil {
		id := uuid.MustParse(state.profile.ID)
		if rows, err := s.store.QuizAttempts(r.Context(), id); err == nil {
			data.Quizzes = rows
			sort.Slice(data.Quizzes, func(i, j int) bool { return data.Quizzes[i].CreatedAt.After(data.Quizzes[j].CreatedAt) })
			if len(data.Quizzes) > 12 {
				data.Quizzes = data.Quizzes[:12]
			}
		}
		if rows, err := s.store.ExerciseAttempts(r.Context(), id); err == nil {
			data.Exercises = rows
			sort.Slice(data.Exercises, func(i, j int) bool { return data.Exercises[i].CreatedAt.After(data.Exercises[j].CreatedAt) })
			if len(data.Exercises) > 12 {
				data.Exercises = data.Exercises[:12]
			}
		}
	}
	page.Data = data
	s.fullPage(w, r, "progress", page)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Review", "Low quiz scores and stale reading sessions, ordered by what needs attention next.")
	progress := s.progressMap(r.Context(), state.profile)
	page.Data = struct {
		Summary learn.ProgressSummary
		Items   []reviewItem
	}{Summary: page.Summary, Items: s.reviewItems(page.Summary.ReviewLessons, progress)}
	s.fullPage(w, r, "review", page)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request, message string) {
	s.errorPageStatus(w, r, http.StatusNotFound, "Not found", message)
}

// searchCacheKey includes every input that changes the result, including the
// profile's progress, which is part of each hit's status.
func searchCacheKey(query string, filters learn.SearchFilters, status map[string]learn.ProgressStatus) string {
	slugs := make([]string, 0, len(status))
	for slug := range status {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	var b strings.Builder
	b.WriteString(query)
	b.WriteByte('|')
	b.WriteString(filters.Course)
	b.WriteByte('|')
	b.WriteString(filters.Module)
	b.WriteByte('|')
	b.WriteString(filters.Kind)
	b.WriteByte('|')
	b.WriteString(filters.Status)
	for _, slug := range slugs {
		b.WriteByte('|')
		b.WriteString(slug)
		b.WriteByte('=')
		b.WriteString(string(status[slug]))
	}
	return b.String()
}
