package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"strings"
	"time"

	"github.com/haqi/gophersocial/internal/learn"
)

//go:embed templates/*.html templates/pages/*.html static/*
var contentFS embed.FS

// pageData is the stable outer shape every full page receives.
type pageData struct {
	Title         string
	Description   string
	Path          string
	Theme         string
	FontScale     string
	ReducedMotion bool
	CSRF          string
	Profile       *learn.Profile
	SignedIn      bool
	Guest         bool
	Nav           []*learn.Course
	Summary       learn.ProgressSummary
	ActiveCourse  string
	ActiveModule  string
	ActiveSlug    string
	CrumbCourse   string
	CrumbModule   string
	Diagnostics   []learn.Diagnostic
	Data          any
}

type renderer struct {
	base    *template.Template
	pages   map[string]*template.Template
	frag    *template.Template
	assets  map[string]string
	version string
}

var pageNames = []string{
	"dashboard", "courses", "course", "lesson", "exercises", "exercise",
	"glossary", "search", "bookmarks", "progress", "review", "quiz",
	"playground", "api", "error", "exercise-collection",
}

func newRenderer() (*renderer, error) {
	r := &renderer{pages: make(map[string]*template.Template), assets: make(map[string]string)}

	entries, err := fs.ReadDir(contentFS, "static")
	if err != nil {
		return nil, fmt.Errorf("read static assets: %w", err)
	}
	var builder strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := fs.ReadFile(contentFS, "static/"+entry.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		version := hex.EncodeToString(sum[:])[:10]
		r.assets[entry.Name()] = "/learn/static/" + entry.Name() + "?v=" + version
		builder.WriteString(version)
	}
	r.version = hex.EncodeToString(sha256Bytes([]byte(builder.String())))[:8]

	funcs := template.FuncMap{
		"asset":       func(name string) string { return r.assets[name] },
		"lessonURL":   func(slug string) string { return "/learn/lessons/" + slug },
		"exerciseURL": func(slug string) string { return "/learn/exercises/" + slug },
		"quizURL":     func(slug string) string { return "/learn/quizzes/" + slug },
		"termURL":     func(slug string) string { return "/learn/glossary?term=" + slug },
		"hitURL":      hitURL,
		"statusClass": statusClass,
		"statusLabel": statusLabel,
		"pct":         percent,
		"add":         func(a, b int) int { return a + b },
		"dict":        dict,
		"plural":      plural,
		"initials":    initials,
		"shortDate":   func(t time.Time) string { return t.Format("2 Jan") },
		"timeAgo":     timeAgo,
		"trim":        strings.TrimSpace,
		"lower":       strings.ToLower,
		"hasPrefix":   strings.HasPrefix,
		"slice": func(items []string, n int) []string {
			if n > len(items) {
				n = len(items)
			}
			return items[:n]
		},
		"contentVersion": func() string { return r.version },
	}

	baseFiles := []string{"templates/layout.html", "templates/partials.html"}
	base, err := template.New("layout").Funcs(funcs).ParseFS(contentFS, baseFiles...)
	if err != nil {
		return nil, fmt.Errorf("parse layout: %w", err)
	}
	r.base = base

	for _, name := range pageNames {
		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := clone.ParseFS(contentFS, "templates/pages/"+name+".html"); err != nil {
			return nil, fmt.Errorf("parse page %s: %w", name, err)
		}
		r.pages[name] = clone
	}

	auth, err := base.Clone()
	if err != nil {
		return nil, err
	}
	if _, err := auth.ParseFS(contentFS, "templates/auth.html"); err != nil {
		return nil, fmt.Errorf("parse auth layout: %w", err)
	}
	r.frag = auth
	return r, nil
}

func sha256Bytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// page executes a full page into a buffer first, so a template error becomes a
// clean 500 instead of a half-written HTML document.
func (r *renderer) page(w *strings.Builder, name string, data *pageData) error {
	t, ok := r.pages[name]
	if !ok {
		return fmt.Errorf("unknown page %q", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

func (r *renderer) fragment(w *strings.Builder, name string, data any) error {
	return r.frag.ExecuteTemplate(w, name, data)
}

func (r *renderer) authPage(w *strings.Builder, data *loginData) error {
	return r.frag.ExecuteTemplate(w, "auth", data)
}

// staticHandler serves embedded assets with an immutable cache policy. URLs are
// content-versioned by the asset helper, so a deploy cannot serve stale CSS.
func (r *renderer) staticHandler() (fs.FS, error) {
	return fs.Sub(contentFS, "static")
}

func statusClass(status learn.ProgressStatus) string {
	switch status {
	case learn.StatusCompleted:
		return "is-done"
	case learn.StatusInProgress:
		return "is-active"
	default:
		return "is-todo"
	}
}

func statusLabel(status learn.ProgressStatus) string {
	switch status {
	case learn.StatusCompleted:
		return "Completed"
	case learn.StatusInProgress:
		return "In progress"
	default:
		return "Not started"
	}
}

func percent(part, total int) int {
	if total <= 0 {
		return 0
	}
	return part * 100 / total
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict requires key/value pairs")
	}
	out := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		out[key] = values[i+1]
	}
	return out, nil
}

func plural(n int, singular, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:1])
	}
	return strings.ToUpper(parts[0][:1] + parts[len(parts)-1][:1])
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d d ago", int(d.Hours()/24))
	default:
		return t.Format("2 Jan 2006")
	}
}

var _ = bytes.MinRead

// hitURL routes a search hit to the page that can render it. Search indexes
// documents of different kinds, and only lessons share a URL shape.
func hitURL(kind, slug string) string {
	switch kind {
	case "lab":
		return "/learn/exercises/" + slug
	case "exercise":
		return "/learn/exercises/collections/" + slug
	case "quiz":
		return "/learn/quizzes/" + slug
	case "term":
		return "/learn/glossary?term=" + slug
	default:
		return "/learn/lessons/" + slug
	}
}
