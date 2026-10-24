package tests

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCursorPaginationWalksAllPosts(t *testing.T) {
	e := newEnv(t)
	alice := e.register(t, "alice")

	const total = 5
	ids := make(map[string]bool, total)
	for i := 0; i < total; i++ {
		ids[e.createPost(t, alice, fmt.Sprintf("Cursor post %d", i))] = true
	}

	seen := make(map[string]bool, total)
	cursor := ""
	for page := 0; page < 10; page++ {
		path := "/api/v1/explore?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		status, body := e.do(t, http.MethodGet, path, "", nil)
		if status != http.StatusOK {
			t.Fatalf("explore page %d: %d %v", page, status, body)
		}
		for _, item := range dataList(t, body) {
			id := item.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatalf("duplicate post %s across pages", id)
			}
			if !ids[id] {
				t.Fatalf("unexpected post %s", id)
			}
			seen[id] = true
		}
		meta := body["meta"].(map[string]any)
		next, _ := meta["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}

	if len(seen) != total {
		t.Fatalf("walked %d posts, want %d", len(seen), total)
	}
}

func TestPaginationRejectsBadInput(t *testing.T) {
	e := newEnv(t)
	alice := e.register(t, "alice")

	cases := []struct {
		path string
		want int
	}{
		{"/api/v1/posts?limit=0", http.StatusUnprocessableEntity},
		{"/api/v1/posts?limit=101", http.StatusUnprocessableEntity},
		{"/api/v1/posts?offset=-1", http.StatusUnprocessableEntity},
		{"/api/v1/posts?limit=abc", http.StatusUnprocessableEntity},
		{"/api/v1/explore?cursor=not-a-cursor", http.StatusBadRequest},
	}
	for _, tc := range cases {
		status, body := e.do(t, http.MethodGet, tc.path, alice.token, nil)
		if status != tc.want {
			t.Errorf("%s: status %d, want %d (%v)", tc.path, status, tc.want, body)
		}
	}
}
