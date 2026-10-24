package learn

import (
	"html"
	"html/template"
	"regexp"
	"sort"
	"strings"
)

// SearchFilters narrow a result set. Empty values mean "any".
type SearchFilters struct {
	Course string
	Module string
	Kind   string
	Status string
}

// SearchHit is one indexed document matched by a query.
type SearchHit struct {
	Slug        string
	Kind        string
	Title       string
	Summary     string
	Snippet     template.HTML
	CourseSlug  string
	ModuleSlug  string
	Number      int
	ReadMinutes int
	Status      ProgressStatus
	Score       int
}

// SearchGroup keeps result kinds readable instead of one mixed list.
type SearchGroup struct {
	Kind  string
	Title string
	Hits  []SearchHit
}

// SearchResult is what the search page renders.
type SearchResult struct {
	Query   string
	Filters SearchFilters
	Groups  []SearchGroup
	Hits    []SearchHit
	Total   int
}

type searchDoc struct {
	slug        string
	kind        string
	title       string
	titleLower  string
	summary     string
	text        string
	textLower   string
	codeLower   string
	headings    string
	headingsLow string
	course      string
	module      string
	number      int
	readMinutes int
	snippetText string
}

// SearchIndex is an immutable in-memory index built once from the curriculum.
// At this size a hand-written index is smaller and faster than a search service.
type SearchIndex struct {
	docs []searchDoc
}

// NewSearchIndex indexes lessons, exercise documents, interactive exercises,
// quizzes, and glossary terms. Solutions are deliberately excluded.
func NewSearchIndex(c *Curriculum, a *Activities) *SearchIndex {
	idx := &SearchIndex{}
	for _, doc := range c.Lessons {
		headings := make([]string, 0, len(doc.TOC))
		for _, h := range doc.TOC {
			headings = append(headings, h.Text)
		}
		idx.docs = append(idx.docs, searchDoc{
			slug: doc.Slug, kind: "lesson", title: doc.Title,
			titleLower: strings.ToLower(doc.Title), summary: doc.Summary,
			text: doc.Text, textLower: strings.ToLower(doc.Text),
			codeLower: strings.ToLower(strings.Join(doc.Code, "\n")),
			headings:  strings.Join(headings, " "), headingsLow: strings.ToLower(strings.Join(headings, " ")),
			course: doc.CourseSlug, module: doc.ModuleSlug, number: doc.Number,
			readMinutes: doc.ReadMinutes, snippetText: doc.Text,
		})
	}
	for _, doc := range c.Exercises {
		idx.docs = append(idx.docs, searchDoc{
			slug: doc.Slug, kind: "exercise", title: doc.Title,
			titleLower: strings.ToLower(doc.Title), summary: doc.Summary,
			text: doc.Text, textLower: strings.ToLower(doc.Text),
			codeLower:   strings.ToLower(strings.Join(doc.Code, "\n")),
			headingsLow: "", headings: "", number: doc.Number,
			readMinutes: doc.ReadMinutes, snippetText: doc.Text,
		})
	}
	if a != nil {
		for _, ex := range a.Exercises {
			body := strings.Join([]string{ex.Summary, ex.Prompt, ex.Task, strings.Join(ex.Objectives, " "), strings.Join(ex.Acceptance, " ")}, " ")
			idx.docs = append(idx.docs, searchDoc{
				slug: ex.Slug, kind: "lab", title: ex.Title,
				titleLower: strings.ToLower(ex.Title), summary: ex.Summary,
				text: body, textLower: strings.ToLower(body),
				course: "", module: "", number: 0, readMinutes: 10, snippetText: body,
			})
		}
		for _, quiz := range a.Quizzes {
			var prompts []string
			for _, q := range quiz.Questions {
				prompts = append(prompts, q.Prompt)
			}
			body := strings.Join(prompts, " ")
			idx.docs = append(idx.docs, searchDoc{
				slug: quiz.Slug, kind: "quiz", title: quiz.Title,
				titleLower: strings.ToLower(quiz.Title), summary: quiz.Summary,
				text: body, textLower: strings.ToLower(body),
				course: "", module: "", number: 0, readMinutes: 5, snippetText: body,
			})
		}
	}
	for _, term := range c.Glossary {
		idx.docs = append(idx.docs, searchDoc{
			slug: term.Slug, kind: "term", title: term.Term,
			titleLower: strings.ToLower(term.Term), summary: firstWords(term.Definition, 24),
			text: term.Definition, textLower: strings.ToLower(term.Definition),
			course: "", module: "", number: 0, readMinutes: 1, snippetText: term.Definition,
		})
	}
	return idx
}

// Search scores documents with a deliberately simple, explainable model:
// title matches outweigh headings, headings outweigh body text, and body text
// outweighs code. It is fast enough to run on every keystroke at this corpus
// size, and it never guesses relevance from unmeasured weights.
func (s *SearchIndex) Search(q string, filters SearchFilters, status map[string]ProgressStatus) SearchResult {
	result := SearchResult{Query: strings.TrimSpace(q), Filters: filters}
	tokens := tokenize(q)
	if len(tokens) == 0 {
		return result
	}

	for i := range s.docs {
		doc := &s.docs[i]
		if filters.Course != "" && doc.course != filters.Course {
			continue
		}
		if filters.Module != "" && doc.module != filters.Module {
			continue
		}
		if filters.Kind != "" && doc.kind != filters.Kind {
			continue
		}
		state := status[doc.slug]
		if state == "" {
			state = StatusNotStarted
		}
		if filters.Status != "" && string(state) != filters.Status {
			continue
		}
		score := doc.score(tokens)
		if score == 0 {
			continue
		}
		result.Hits = append(result.Hits, SearchHit{
			Slug: doc.slug, Kind: doc.kind, Title: doc.title, Summary: doc.summary,
			Snippet:    highlightSnippet(doc.snippetText, tokens),
			CourseSlug: doc.course, ModuleSlug: doc.module, Number: doc.number,
			ReadMinutes: doc.readMinutes, Status: state, Score: score,
		})
	}
	sort.SliceStable(result.Hits, func(i, j int) bool {
		if result.Hits[i].Score != result.Hits[j].Score {
			return result.Hits[i].Score > result.Hits[j].Score
		}
		return result.Hits[i].Title < result.Hits[j].Title
	})
	result.Total = len(result.Hits)

	titles := map[string]string{
		"lesson": "Lessons", "exercise": "Exercise collections", "lab": "Interactive exercises",
		"quiz": "Quizzes", "term": "Glossary",
	}
	for _, kind := range []string{"lesson", "exercise", "lab", "quiz", "term"} {
		group := SearchGroup{Kind: kind, Title: titles[kind]}
		for _, hit := range result.Hits {
			if hit.Kind == kind {
				group.Hits = append(group.Hits, hit)
			}
		}
		if len(group.Hits) > 0 {
			result.Groups = append(result.Groups, group)
		}
	}
	if len(result.Hits) > 40 {
		result.Hits = result.Hits[:40]
	}
	return result
}

func (d *searchDoc) score(tokens []string) int {
	score := 0
	for _, token := range tokens {
		matched := false
		if d.titleLower == token {
			score += 25
			matched = true
		} else if strings.Contains(d.titleLower, token) {
			score += 15
			matched = true
		}
		if strings.Contains(d.headingsLow, token) {
			score += 7
			matched = true
		}
		if count := strings.Count(d.textLower, token); count > 0 {
			if count > 5 {
				count = 5
			}
			score += count * 2
			matched = true
		}
		if strings.Contains(d.codeLower, token) {
			score++
			matched = true
		}
		if !matched {
			return 0 // every token must match something
		}
	}
	return score
}

func tokenize(q string) []string {
	fields := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r > 127)
	})
	seen := make(map[string]bool, len(fields))
	var out []string
	for _, f := range fields {
		f = strings.Trim(f, "-_")
		if len([]rune(f)) < 2 || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

var markToken = regexp.MustCompile(`[a-zA-Z0-9_-]+`)

func highlightSnippet(text string, tokens []string) template.HTML {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	lower := strings.ToLower(text)
	start := 0
	for _, token := range tokens {
		if idx := strings.Index(lower, token); idx >= 0 {
			start = idx
			break
		}
	}
	// Convert byte index to rune index using the already-normalized ASCII-ish
	// prefix. Search text can contain unicode, so count runes explicitly.
	runeStart := len([]rune(lower[:start]))
	if runeStart > 60 {
		runeStart -= 60
	} else {
		runeStart = 0
	}
	end := runeStart + 220
	if end > len(runes) {
		end = len(runes)
	}
	window := strings.TrimSpace(string(runes[runeStart:end]))
	if runeStart > 0 {
		window = "..." + window
	}
	if end < len(runes) {
		window += "..."
	}

	escaped := html.EscapeString(window)
	if len(tokens) == 0 {
		return template.HTML(escaped) // #nosec G203 -- escaped above.
	}
	lowerEscaped := strings.ToLower(escaped)
	var b strings.Builder
	last := 0
	for _, loc := range markToken.FindAllStringIndex(lowerEscaped, -1) {
		word := escaped[loc[0]:loc[1]]
		low := strings.ToLower(word)
		hit := false
		for _, token := range tokens {
			if strings.HasPrefix(low, token) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		b.WriteString(escaped[last:loc[0]])
		b.WriteString("<mark>")
		b.WriteString(word)
		b.WriteString("</mark>")
		last = loc[1]
	}
	b.WriteString(escaped[last:])
	return template.HTML(b.String()) // #nosec G203 -- escaped, only <mark> added.
}

func firstWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) <= n {
		return s
	}
	return strings.Join(fields[:n], " ") + "..."
}
