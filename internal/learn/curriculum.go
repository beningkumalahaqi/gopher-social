package learn

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type courseDef struct {
	Slug    string
	Title   string
	Summary string
	New     bool
}

type moduleDef struct {
	Slug    string
	Title   string
	Summary string
	Course  string
	From    int
	To      int
	New     bool
}

// Courses and modules are metadata, not duplicated lesson content. Titles and
// bodies are always read from the Markdown files.
var courseDefs = []courseDef{
	{
		Slug:  "backend-engineering",
		Title: "Backend engineering",
		Summary: "Build the GopherSocial API from the first request to production: routing, " +
			"configuration, SQL, authentication, features, and shutdown.",
	},
	{
		Slug:    "server-rendered-ui",
		Title:   "Server-rendered interface",
		Summary: "Render HTML with Go templates, learn HTMX properly, and build the learning platform you are using.",
		New:     true,
	},
	{
		Slug:    "caching",
		Title:   "Caching",
		Summary: "Hit ratios, HTTP validators, in-process caches, invalidation, and when a distributed cache is worth the cost.",
		New:     true,
	},
	{
		Slug:    "performance",
		Title:   "Performance and deployment",
		Summary: "Measure before optimizing, then work through memory, SQL, HTTP, concurrency, production limits, and Vercel deployment.",
		New:     true,
	},
}

var moduleDefs = []moduleDef{
	{Slug: "foundations", Title: "Foundations", Course: "backend-engineering", From: 1, To: 5,
		Summary: "Run the system, trace a request, and understand packages, configuration, and logging."},
	{Slug: "http-engineering", Title: "HTTP engineering", Course: "backend-engineering", From: 6, To: 12,
		Summary: "Middleware, errors, pgx, migrations, transactions, DTOs, and validation."},
	{Slug: "identity", Title: "Identity", Course: "backend-engineering", From: 13, To: 16,
		Summary: "Passwords, JWTs, refresh rotation, and authentication middleware."},
	{Slug: "features", Title: "Features", Course: "backend-engineering", From: 17, To: 21,
		Summary: "Handlers, keyset pagination, search, the social graph, comments, and reactions."},
	{Slug: "reliability", Title: "Reliability", Course: "backend-engineering", From: 22, To: 25,
		Summary: "Bounded workers, ordered shutdown, tests, and the production checklist."},

	{Slug: "templates", Title: "Templates and architecture", Course: "server-rendered-ui", From: 26, To: 27,
		Summary: "html/template from first principles and the request lifecycle of a server-rendered app.", New: true},
	{Slug: "htmx", Title: "HTMX and interaction", Course: "server-rendered-ui", From: 28, To: 29,
		Summary: "Partial updates without a JavaScript framework, plus components that hold real state.", New: true},
	{Slug: "content-pipeline", Title: "Content pipeline", Course: "server-rendered-ui", From: 30, To: 31,
		Summary: "Turn Markdown into lessons safely, and test HTML with httptest.", New: true},

	{Slug: "caching-basics", Title: "Caching fundamentals", Course: "caching", From: 32, To: 33,
		Summary: "Hits, misses, TTL, invalidation, and HTTP validators.", New: true},
	{Slug: "caching-in-go", Title: "Caching in Go", Course: "caching", From: 34, To: 35,
		Summary: "Race-safe in-process caches, singleflight, and database cache-aside.", New: true},
	{Slug: "distributed-caching", Title: "Distributed caching", Course: "caching", From: 36, To: 36,
		Summary: "Redis, shared state, failure modes, and graceful fallback.", New: true},

	{Slug: "measurement", Title: "Measurement", Course: "performance", From: 37, To: 37,
		Summary: "Profiling, benchmarks, percentiles, and pprof.", New: true},
	{Slug: "memory", Title: "Memory", Course: "performance", From: 38, To: 38,
		Summary: "Allocations, escape analysis, and garbage collection.", New: true},
	{Slug: "database-performance", Title: "Database performance", Course: "performance", From: 39, To: 39,
		Summary: "Indexes, EXPLAIN, N+1 queries, and pagination.", New: true},
	{Slug: "http-performance", Title: "HTTP and rendering", Course: "performance", From: 40, To: 40,
		Summary: "Template execution, partial rendering, compression, and static assets.", New: true},
	{Slug: "concurrency-performance", Title: "Concurrency", Course: "performance", From: 41, To: 41,
		Summary: "Bounded work, contention, and goroutine hygiene.", New: true},
	{Slug: "production", Title: "Production", Course: "performance", From: 42, To: 43,
		Summary: "Failure handling, observability, capacity planning, and Vercel.", New: true},
}

// Load parses every Markdown document in fsys. The embedded filesystem is the
// canonical source: nothing is generated to disk, and no lesson body is copied
// into Go code.
func Load(fsys fs.FS) (*Curriculum, error) {
	c := &Curriculum{
		BySlug:      make(map[string]*Doc),
		moduleByKey: make(map[string]*Module),
		courseByKey: make(map[string]*Course),
	}
	renderer := NewRenderer()

	lessonFiles, err := fs.Glob(fsys, "[0-9][0-9]-*.md")
	if err != nil {
		return nil, fmt.Errorf("discover lessons: %w", err)
	}
	sort.Strings(lessonFiles)
	for _, name := range lessonFiles {
		if strings.HasPrefix(path.Base(name), "00-") {
			continue // the roadmap is a map, not a lesson
		}
		doc, diag := loadDoc(fsys, renderer, name, KindLesson)
		if diag != nil {
			c.Diagnostics = append(c.Diagnostics, *diag)
			continue
		}
		if doc == nil {
			continue
		}
		c.Lessons = append(c.Lessons, doc)
		c.Docs = append(c.Docs, doc)
		c.BySlug[doc.Slug] = doc
	}
	if len(c.Lessons) == 0 {
		return nil, fmt.Errorf("no lessons found in %s", fsys)
	}
	sort.Slice(c.Lessons, func(i, j int) bool { return c.Lessons[i].Number < c.Lessons[j].Number })

	exerciseFiles, err := fs.Glob(fsys, "exercises/*.md")
	if err != nil {
		return nil, fmt.Errorf("discover exercises: %w", err)
	}
	sort.Strings(exerciseFiles)
	for _, name := range exerciseFiles {
		if path.Base(name) == "README.md" {
			continue
		}
		doc, diag := loadDoc(fsys, renderer, name, KindExercise)
		if diag != nil {
			c.Diagnostics = append(c.Diagnostics, *diag)
			continue
		}
		if doc == nil {
			continue
		}
		c.Exercises = append(c.Exercises, doc)
		c.Docs = append(c.Docs, doc)
		c.BySlug[doc.Slug] = doc
	}

	solutionFiles, _ := fs.Glob(fsys, "solutions/*.md")
	for _, name := range solutionFiles {
		doc, diag := loadDoc(fsys, renderer, name, KindSolution)
		if diag != nil {
			c.Diagnostics = append(c.Diagnostics, *diag)
			continue
		}
		if doc == nil {
			continue
		}
		// Solutions reuse exercise filenames. Keep them out of BySlug so an
		// exercise slug cannot resolve to a solution document.
		c.Solutions = append(c.Solutions, doc)
		c.Docs = append(c.Docs, doc)
	}

	c.Glossary = loadGlossary(fsys, &c.Diagnostics)
	c.Modules = buildModules(c)
	c.Courses = buildCourses(c)
	linkLessons(c)
	return c, nil
}

func loadDoc(fsys fs.FS, renderer *Renderer, name string, kind DocKind) (*Doc, *Diagnostic) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, &Diagnostic{Path: name, Message: "cannot read file: " + err.Error()}
	}
	parsed, err := renderer.Render(raw)
	if err != nil {
		return nil, &Diagnostic{Path: name, Message: err.Error()}
	}
	base := strings.TrimSuffix(path.Base(name), ".md")
	doc := &Doc{
		Slug:        base,
		Kind:        kind,
		Number:      parseNumber(base),
		Title:       parsed.Title,
		Summary:     parsed.Summary,
		HTML:        parsed.HTML,
		TOC:         parsed.TOC,
		Text:        parsed.Text,
		Code:        parsed.Code,
		Words:       parsed.Words,
		ReadMinutes: readingMinutes(parsed.Words),
		SourcePath:  "docs-learn/" + name,
	}
	if doc.Title == "" {
		doc.Title = humanize(base)
	}
	return doc, nil
}

func parseNumber(base string) int {
	if len(base) < 2 {
		return 0
	}
	n, err := strconv.Atoi(base[:2])
	if err != nil {
		return 0
	}
	return n
}

func readingMinutes(words int) int {
	minutes := words / 220
	if minutes < 1 {
		return 1
	}
	return minutes
}

func humanize(slug string) string {
	parts := strings.Split(strings.TrimPrefix(slug, fmt.Sprintf("%02d-", parseNumber(slug))), "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func buildModules(c *Curriculum) []*Module {
	modules := make([]*Module, 0, len(moduleDefs))
	for _, def := range moduleDefs {
		m := &Module{
			Slug: def.Slug, Title: def.Title, Summary: def.Summary,
			CourseSlug: def.Course, From: def.From, To: def.To, New: def.New,
		}
		for _, lesson := range c.Lessons {
			if lesson.Number >= def.From && lesson.Number <= def.To {
				lesson.ModuleSlug = m.Slug
				lesson.CourseSlug = def.Course
				m.Docs = append(m.Docs, lesson)
			}
		}
		if len(m.Docs) == 0 {
			c.Diagnostics = append(c.Diagnostics, Diagnostic{Path: "module " + def.Slug, Message: "no lessons matched the declared range"})
		}
		c.moduleByKey[m.Slug] = m
		modules = append(modules, m)
	}
	return modules
}

func buildCourses(c *Curriculum) []*Course {
	courses := make([]*Course, 0, len(courseDefs))
	for _, def := range courseDefs {
		course := &Course{Slug: def.Slug, Title: def.Title, Summary: def.Summary, New: def.New}
		for _, m := range c.Modules {
			if m.CourseSlug != def.Slug {
				continue
			}
			course.Modules = append(course.Modules, m)
			course.Lessons += len(m.Docs)
			for _, doc := range m.Docs {
				course.Minutes += doc.ReadMinutes
				if course.FirstDoc == nil {
					course.FirstDoc = doc
				}
			}
		}
		c.courseByKey[def.Slug] = course
		courses = append(courses, course)
	}
	return courses
}

func linkLessons(c *Curriculum) {
	for i, doc := range c.Lessons {
		if i > 0 {
			doc.Prev = c.Lessons[i-1]
		}
		if i < len(c.Lessons)-1 {
			doc.Next = c.Lessons[i+1]
		}
	}
}

var glossaryEntry = regexp.MustCompile(`^\*\*(.+?)\*\*\s*(?:\x{2014}|-|:)\s*(.+)$`)

func loadGlossary(fsys fs.FS, diagnostics *[]Diagnostic) []Term {
	raw, err := fs.ReadFile(fsys, "glossary.md")
	if err != nil {
		*diagnostics = append(*diagnostics, Diagnostic{Path: "glossary.md", Message: "missing glossary"})
		return nil
	}
	var terms []Term
	letter := ""
	var current *Term
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			letter = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			if letter != "" {
				letter = string([]rune(letter)[0])
			}
			continue
		}
		if m := glossaryEntry.FindStringSubmatch(trimmed); m != nil {
			term := strings.TrimSpace(m[1])
			definition := strings.TrimSpace(m[2])
			terms = append(terms, Term{
				Term:       term,
				Definition: definition,
				Letter:     letter,
				Slug:       termSlug(term),
			})
			current = &terms[len(terms)-1]
			continue
		}
		if current != nil && trimmed == "" {
			current = nil
			continue
		}
		if current != nil && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			current.Definition += " " + trimmed
		}
	}
	if len(terms) == 0 {
		*diagnostics = append(*diagnostics, Diagnostic{Path: "glossary.md", Message: "no entries parsed; expected **Term** followed by a dash"})
	}
	return terms
}

func termSlug(term string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(term) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_', r == '/':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// FindTerm returns glossary entries whose term or definition contains q.
func (c *Curriculum) FindTerm(q string) []Term {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return c.Glossary
	}
	var out []Term
	for _, t := range c.Glossary {
		if strings.Contains(strings.ToLower(t.Term), q) || strings.Contains(strings.ToLower(t.Definition), q) {
			out = append(out, t)
		}
	}
	return out
}
