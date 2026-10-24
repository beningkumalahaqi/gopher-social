package learn

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed activities/quizzes/*.yaml activities/exercises/*.yaml
var activityFS embed.FS

// QuestionType selects the interaction and the grading rule.
type QuestionType string

const (
	QuestionSingle    QuestionType = "single"
	QuestionMultiple  QuestionType = "multiple"
	QuestionTrueFalse QuestionType = "truefalse"
	QuestionCodeOut   QuestionType = "code-output"
	QuestionErrorID   QuestionType = "error-id"
	QuestionCompare   QuestionType = "comparison"
	QuestionShort     QuestionType = "short"
)

// Option is one selectable answer. Feedback explains why it is right or wrong
// and is shown only after submission.
type Option struct {
	ID       string `yaml:"id" json:"id"`
	Text     string `yaml:"text" json:"text"`
	Correct  bool   `yaml:"correct" json:"correct"`
	Feedback string `yaml:"feedback" json:"feedback"`
}

// Question is one quiz or exercise prompt. The correct option is never rendered
// before submission.
type Question struct {
	ID          string       `yaml:"id"`
	Type        QuestionType `yaml:"type"`
	Prompt      string       `yaml:"prompt"`
	Code        string       `yaml:"code"`
	Options     []Option     `yaml:"options"`
	Accepted    []string     `yaml:"accepted"`
	Explanation string       `yaml:"explanation"`
	Reference   string       `yaml:"reference"`
}

// Quiz is a set of questions attached to a lesson.
type Quiz struct {
	Slug       string     `yaml:"slug"`
	Title      string     `yaml:"title"`
	LessonSlug string     `yaml:"lesson"`
	Difficulty string     `yaml:"difficulty"`
	Summary    string     `yaml:"summary"`
	Questions  []Question `yaml:"questions"`

	Lesson *Doc `yaml:"-"`
}

// Exercise is a guided code-reading task with acceptance criteria.
type Exercise struct {
	Slug           string     `yaml:"slug"`
	Title          string     `yaml:"title"`
	LessonSlug     string     `yaml:"lesson"`
	Difficulty     string     `yaml:"difficulty"`
	Summary        string     `yaml:"summary"`
	Objectives     []string   `yaml:"objectives"`
	Prerequisites  []string   `yaml:"prerequisites"`
	Prompt         string     `yaml:"prompt"`
	Task           string     `yaml:"task"`
	Code           string     `yaml:"code"`
	Hints          []string   `yaml:"hints"`
	Acceptance     []string   `yaml:"acceptance"`
	Verification   string     `yaml:"verification"`
	ReferenceFiles []string   `yaml:"reference_files"`
	Questions      []Question `yaml:"questions"`

	LessonDoc *Doc `yaml:"-"`
}

// Activities is the loaded interactive content graph.
type Activities struct {
	Quizzes        []*Quiz
	QuizBySlug     map[string]*Quiz
	Exercises      []*Exercise
	ExerciseBySlug map[string]*Exercise
	Diagnostics    []Diagnostic
}

// LoadActivities parses structured quiz and exercise definitions and validates
// their references against the curriculum.
func LoadActivities(curriculum *Curriculum) (*Activities, error) {
	a := &Activities{
		QuizBySlug:     make(map[string]*Quiz),
		ExerciseBySlug: make(map[string]*Exercise),
	}

	quizFiles, _ := fs.Glob(activityFS, "activities/quizzes/*.yaml")
	sort.Strings(quizFiles)
	for _, name := range quizFiles {
		raw, err := activityFS.ReadFile(name)
		if err != nil {
			a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: err.Error()})
			continue
		}
		var quiz Quiz
		if err := yaml.Unmarshal(raw, &quiz); err != nil {
			a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: "invalid YAML: " + err.Error()})
			continue
		}
		if problems := validateQuiz(&quiz); len(problems) > 0 {
			a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: strings.Join(problems, "; ")})
			continue
		}
		if curriculum != nil {
			if doc, ok := curriculum.Lesson(quiz.LessonSlug); ok {
				quiz.Lesson = doc
			} else {
				a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: "unknown lesson slug " + quiz.LessonSlug})
			}
		}
		a.Quizzes = append(a.Quizzes, &quiz)
		a.QuizBySlug[quiz.Slug] = &quiz
	}

	exerciseFiles, _ := fs.Glob(activityFS, "activities/exercises/*.yaml")
	sort.Strings(exerciseFiles)
	for _, name := range exerciseFiles {
		raw, err := activityFS.ReadFile(name)
		if err != nil {
			a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: err.Error()})
			continue
		}
		var ex Exercise
		if err := yaml.Unmarshal(raw, &ex); err != nil {
			a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: "invalid YAML: " + err.Error()})
			continue
		}
		if problems := validateExercise(&ex); len(problems) > 0 {
			a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: strings.Join(problems, "; ")})
			continue
		}
		if curriculum != nil {
			if doc, ok := curriculum.Lesson(ex.LessonSlug); ok {
				ex.LessonDoc = doc
			} else {
				a.Diagnostics = append(a.Diagnostics, Diagnostic{Path: name, Message: "unknown lesson slug " + ex.LessonSlug})
			}
		}
		a.Exercises = append(a.Exercises, &ex)
		a.ExerciseBySlug[ex.Slug] = &ex
	}
	return a, nil
}

func validateQuiz(q *Quiz) []string {
	var problems []string
	if q.Slug == "" {
		problems = append(problems, "slug is required")
	}
	if q.Title == "" {
		problems = append(problems, "title is required")
	}
	if q.LessonSlug == "" {
		problems = append(problems, "lesson is required")
	}
	if len(q.Questions) == 0 {
		problems = append(problems, "at least one question is required")
	}
	for i := range q.Questions {
		problems = append(problems, validateQuestion(&q.Questions[i], i)...)
	}
	return problems
}

func validateExercise(e *Exercise) []string {
	var problems []string
	if e.Slug == "" || e.Title == "" || e.LessonSlug == "" {
		problems = append(problems, "slug, title and lesson are required")
	}
	if e.Prompt == "" || e.Task == "" {
		problems = append(problems, "prompt and task are required")
	}
	for i := range e.Questions {
		problems = append(problems, validateQuestion(&e.Questions[i], i)...)
	}
	return problems
}

func validateQuestion(q *Question, index int) []string {
	var problems []string
	label := fmt.Sprintf("question %d", index+1)
	if q.ID == "" {
		problems = append(problems, label+": id is required")
	}
	if strings.TrimSpace(q.Prompt) == "" {
		problems = append(problems, label+": prompt is required")
	}
	switch q.Type {
	case QuestionShort:
		if len(q.Accepted) == 0 {
			problems = append(problems, label+": short answers need accepted spellings")
		}
	case QuestionSingle, QuestionMultiple, QuestionTrueFalse, QuestionCodeOut, QuestionErrorID, QuestionCompare:
		if len(q.Options) < 2 {
			problems = append(problems, label+": at least two options are required")
		}
		correct := 0
		for _, o := range q.Options {
			if o.ID == "" || strings.TrimSpace(o.Text) == "" {
				problems = append(problems, label+": every option needs an id and text")
			}
			if o.Correct {
				correct++
			}
		}
		if q.Type == QuestionMultiple && correct < 2 {
			problems = append(problems, label+": multiple-choice needs at least two correct options")
		}
		if q.Type != QuestionMultiple && correct != 1 {
			problems = append(problems, label+": exactly one correct option is required")
		}
	default:
		problems = append(problems, label+": unknown type "+string(q.Type))
	}
	if strings.TrimSpace(q.Explanation) == "" {
		problems = append(problems, label+": explanation is required")
	}
	return problems
}

// QuizForLesson returns the first quiz attached to a lesson.
func (a *Activities) QuizForLesson(lessonSlug string) *Quiz {
	for _, quiz := range a.Quizzes {
		if quiz.LessonSlug == lessonSlug {
			return quiz
		}
	}
	return nil
}

// Answer is one submitted question.
type Answer struct {
	Options []string
	Text    string
}

// QuestionResult is safe to render after submission only.
type QuestionResult struct {
	Question    *Question
	Selected    []string
	Text        string
	Correct     bool
	NeedsReview bool
}

// SelectedOption reports whether an option was chosen.
func (r QuestionResult) SelectedOption(id string) bool {
	for _, s := range r.Selected {
		if s == id {
			return true
		}
	}
	return false
}

// Feedback returns the option's explanation, or an empty string.
func (q *Question) Feedback(id string) string {
	for _, o := range q.Options {
		if o.ID == id {
			return o.Feedback
		}
	}
	return ""
}

// Result is the graded outcome of a quiz attempt.
type Result struct {
	Quiz     *Quiz
	Results  []QuestionResult
	Score    int
	Total    int
	Percent  int
	Passed   bool
	Answered int
}

// Grade scores answers. Multiple choice is all-or-nothing; short answers accept
// a small set of normalized spellings and otherwise ask the learner to compare
// their wording with the explanation.
func Grade(quiz *Quiz, answers map[string]Answer) Result {
	res := Result{Quiz: quiz, Total: len(quiz.Questions)}
	for i := range quiz.Questions {
		q := &quiz.Questions[i]
		answer := answers[q.ID]
		item := QuestionResult{Question: q, Selected: answer.Options, Text: strings.TrimSpace(answer.Text)}
		if len(item.Selected) > 0 || item.Text != "" {
			res.Answered++
		}
		if q.Type == QuestionShort {
			item.Correct = matchesAccepted(item.Text, q.Accepted)
			item.NeedsReview = !item.Correct
		} else {
			item.Correct = sameSelection(q, item.Selected)
		}
		if item.Correct {
			res.Score++
		}
		res.Results = append(res.Results, item)
	}
	if res.Total > 0 {
		res.Percent = res.Score * 100 / res.Total
	}
	res.Passed = res.Total > 0 && res.Percent >= 70
	return res
}

func sameSelection(q *Question, selected []string) bool {
	var correct []string
	for _, o := range q.Options {
		if o.Correct {
			correct = append(correct, o.ID)
		}
	}
	if len(selected) != len(correct) {
		return false
	}
	seen := make(map[string]bool, len(selected))
	for _, s := range selected {
		seen[s] = true
	}
	for _, c := range correct {
		if !seen[c] {
			return false
		}
	}
	return true
}

func matchesAccepted(text string, accepted []string) bool {
	normalized := normalizeAnswer(text)
	if normalized == "" {
		return false
	}
	for _, a := range accepted {
		if normalizeAnswer(a) == normalized {
			return true
		}
	}
	return false
}

func normalizeAnswer(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	space := false
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		case r == ' ' || r == '\t' || r == '\n':
			space = true
		}
	}
	return b.String()
}
