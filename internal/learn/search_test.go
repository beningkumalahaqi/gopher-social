package learn

import (
	"testing"

	curriculum "github.com/haqi/gophersocial/docs-learn"
)

func testIndex(t *testing.T) *SearchIndex {
	t.Helper()
	c, err := Load(curriculum.FS)
	if err != nil {
		t.Fatalf("load curriculum: %v", err)
	}
	return NewSearchIndex(c, nil)
}

func TestSearchRanksTitleFirst(t *testing.T) {
	idx := testIndex(t)
	result := idx.Search("caching fundamentals", SearchFilters{}, nil)
	if result.Total == 0 {
		t.Fatal("no results")
	}
	if got := result.Hits[0].Slug; got != "32-caching-fundamentals" {
		t.Fatalf("top result = %q; want 32-caching-fundamentals", got)
	}
}

func TestSearchRequiresEveryToken(t *testing.T) {
	idx := testIndex(t)
	result := idx.Search("caching zzzzznotaword", SearchFilters{}, nil)
	if result.Total != 0 {
		t.Fatalf("total = %d; want 0", result.Total)
	}
}

func TestSearchFilters(t *testing.T) {
	idx := testIndex(t)
	result := idx.Search("cache", SearchFilters{Kind: "lesson", Course: "caching"}, nil)
	if result.Total == 0 {
		t.Fatal("no filtered results")
	}
	for _, hit := range result.Hits {
		if hit.Kind != "lesson" {
			t.Fatalf("kind = %q; want lesson", hit.Kind)
		}
	}
}

func TestSearchStatusFilter(t *testing.T) {
	idx := testIndex(t)
	status := map[string]ProgressStatus{
		"32-caching-fundamentals": StatusCompleted,
	}
	result := idx.Search("caching", SearchFilters{Status: string(StatusCompleted)}, status)
	if result.Total != 1 || result.Hits[0].Slug != "32-caching-fundamentals" {
		t.Fatalf("hits = %+v; want only the completed lesson", result.Hits)
	}
}

func BenchmarkSearch(b *testing.B) {
	c, err := Load(curriculum.FS)
	if err != nil {
		b.Fatalf("load curriculum: %v", err)
	}
	idx := NewSearchIndex(c, nil)
	status := map[string]ProgressStatus{
		"32-caching-fundamentals": StatusInProgress,
	}
	filters := SearchFilters{Kind: "lesson"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search("caching", filters, status)
	}
}

func TestExerciseSlugDoesNotResolveToSolution(t *testing.T) {
	c, err := Load(curriculum.FS)
	if err != nil {
		t.Fatalf("load curriculum: %v", err)
	}
	doc, ok := c.Exercise("06-testing")
	if !ok {
		t.Fatal("06-testing is not available as an exercise document")
	}
	if doc.Kind != KindExercise {
		t.Fatalf("kind = %q; want %q", doc.Kind, KindExercise)
	}
}
