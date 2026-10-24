package web

import (
	"testing"

	"github.com/haqi/gophersocial/internal/learn"
)

// Every page and fragment template must compile before the server starts. A
// template typo should fail the build, not a learner's request.
func TestTemplatesParse(t *testing.T) {
	if _, err := newRenderer(); err != nil {
		t.Fatalf("parse templates: %v", err)
	}
}

func TestHitURLByKind(t *testing.T) {
	cases := []struct {
		kind, slug, want string
	}{
		{"lesson", "01-orientation", "/learn/lessons/01-orientation"},
		{"exercise", "05-concurrency", "/learn/exercises/collections/05-concurrency"},
		{"lab", "context-cancellation", "/learn/exercises/context-cancellation"},
		{"quiz", "slice-aliasing", "/learn/quizzes/slice-aliasing"},
		{"term", "context", "/learn/glossary?term=context"},
	}
	for _, tc := range cases {
		if got := hitURL(tc.kind, tc.slug); got != tc.want {
			t.Errorf("hitURL(%q, %q) = %q; want %q", tc.kind, tc.slug, got, tc.want)
		}
	}
}

func TestSearchCacheKeyIncludesProgress(t *testing.T) {
	base := map[string]learn.ProgressStatus{"01-orientation": learn.StatusInProgress}
	changed := map[string]learn.ProgressStatus{"01-orientation": learn.StatusCompleted}
	filters := learn.SearchFilters{Kind: "lesson"}
	a := searchCacheKey("cache", filters, base)
	b := searchCacheKey("cache", filters, changed)
	if a == b {
		t.Fatal("cache key did not change when progress changed")
	}
	if a != searchCacheKey("cache", filters, map[string]learn.ProgressStatus{"01-orientation": learn.StatusInProgress}) {
		t.Fatal("cache key is not stable for identical input")
	}
}
