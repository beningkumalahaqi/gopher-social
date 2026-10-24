package tests

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestConcurrentReactionsAreIdempotent(t *testing.T) {
	e := newEnv(t)
	alice := e.register(t, "alice")
	bob := e.register(t, "bob")
	postID := e.createPost(t, alice, "Race me")

	types := []string{"like", "love", "laugh"}
	const workers = 24
	var wg sync.WaitGroup
	statuses := make([]int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status, _ := e.do(t, http.MethodPut, "/api/v1/posts/"+postID+"/reaction", bob.token,
				map[string]any{"type": types[i%len(types)]})
			statuses[i] = status
		}(i)
	}
	wg.Wait()

	for i, status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("worker %d status %d", i, status)
		}
	}

	var rows int
	if err := e.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM reactions WHERE post_id = $1 AND user_id = $2`, postID, bob.userID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("reaction rows = %d, want 1", rows)
	}

	status, body := e.do(t, http.MethodGet, "/api/v1/posts/"+postID+"/reaction-summary", bob.token, nil)
	if status != http.StatusOK {
		t.Fatalf("summary status %d", status)
	}
	summary := body["data"].(map[string]any)
	if summary["total"].(float64) != 1 {
		t.Fatalf("summary total = %v, want 1", summary["total"])
	}
	if summary["viewer_reaction"] == nil {
		t.Fatal("viewer_reaction missing")
	}
}

func TestConcurrentFollowsCreateOneEdgeAndNotification(t *testing.T) {
	e := newEnv(t)
	alice := e.register(t, "alice")
	bob := e.register(t, "bob")

	const workers = 16
	var wg sync.WaitGroup
	statuses := make([]int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status, _ := e.do(t, http.MethodPost, "/api/v1/users/alice/follow", bob.token, nil)
			statuses[i] = status
		}(i)
	}
	wg.Wait()
	for i, status := range statuses {
		if status != http.StatusNoContent {
			t.Fatalf("worker %d status %d", i, status)
		}
	}

	var edges int
	if err := e.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM follows WHERE follower_id = $1 AND following_id = $2`, bob.userID, alice.userID).Scan(&edges); err != nil {
		t.Fatal(err)
	}
	if edges != 1 {
		t.Fatalf("follow rows = %d, want 1", edges)
	}

	// Notifications are asynchronous; wait for the follow notification, then
	// assert exactly one exists.
	deadline := time.Now().Add(5 * time.Second)
	var notifications int
	for time.Now().Before(deadline) {
		if err := e.pool.QueryRow(t.Context(),
			`SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'follow'`, alice.userID).Scan(&notifications); err != nil {
			t.Fatal(err)
		}
		if notifications > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if notifications != 1 {
		t.Fatalf("follow notifications = %d, want 1", notifications)
	}

	// Unfollow is idempotent and removes the edge.
	if status, _ := e.do(t, http.MethodDelete, "/api/v1/users/alice/follow", bob.token, nil); status != http.StatusNoContent {
		t.Fatalf("unfollow status %d", status)
	}
	if status, _ := e.do(t, http.MethodDelete, "/api/v1/users/alice/follow", bob.token, nil); status != http.StatusNoContent {
		t.Fatalf("second unfollow status %d", status)
	}
	if err := e.pool.QueryRow(t.Context(),
		`SELECT count(*) FROM follows WHERE follower_id = $1 AND following_id = $2`, bob.userID, alice.userID).Scan(&edges); err != nil {
		t.Fatal(err)
	}
	if edges != 0 {
		t.Fatalf("follow rows after unfollow = %d", edges)
	}
}
