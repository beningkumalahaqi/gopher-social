package tests

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPLifecycle(t *testing.T) {
	e := newEnv(t)
	alice := e.register(t, "alice")
	bob := e.register(t, "bob")

	// Profile and patch
	status, body := e.do(t, http.MethodGet, "/api/v1/users/"+alice.username, "", nil)
	if status != http.StatusOK {
		t.Fatalf("profile status %d: %v", status, body)
	}
	status, body = e.do(t, http.MethodPatch, "/api/v1/users/me", alice.token, map[string]any{
		"display_name": "Alice Updated",
		"bio":          "Testing in Go",
	})
	if status != http.StatusOK || body["data"].(map[string]any)["display_name"] != "Alice Updated" {
		t.Fatalf("patch me: %d %v", status, body)
	}

	// Post lifecycle, including draft visibility
	status, body = e.do(t, http.MethodPost, "/api/v1/posts", alice.token, map[string]any{
		"content": "Hello GopherSocial integration test",
	})
	if status != http.StatusCreated {
		t.Fatalf("create post: %d %v", status, body)
	}
	post := body["data"].(map[string]any)
	postID := post["id"].(string)
	if post["author"].(map[string]any)["username"] != "alice" {
		t.Fatalf("unexpected author: %v", post["author"])
	}

	status, _ = e.do(t, http.MethodPatch, "/api/v1/posts/"+postID, alice.token, map[string]any{"status": "draft"})
	if status != http.StatusOK {
		t.Fatalf("draft status %d", status)
	}
	if status, _ = e.do(t, http.MethodGet, "/api/v1/posts/"+postID, "", nil); status != http.StatusNotFound {
		t.Fatalf("anonymous draft read status %d, want 404", status)
	}
	if status, _ = e.do(t, http.MethodGet, "/api/v1/posts/"+postID, bob.token, nil); status != http.StatusNotFound {
		t.Fatalf("other user draft read status %d, want 404", status)
	}
	if status, _ = e.do(t, http.MethodGet, "/api/v1/posts/"+postID, alice.token, nil); status != http.StatusOK {
		t.Fatalf("owner draft read status %d", status)
	}
	status, body = e.do(t, http.MethodPatch, "/api/v1/posts/"+postID, alice.token, map[string]any{
		"status": "published", "content": "Published again",
	})
	if status != http.StatusOK {
		t.Fatalf("republish: %d %v", status, body)
	}
	if status, _ = e.do(t, http.MethodPatch, "/api/v1/posts/"+postID, bob.token, map[string]any{"content": "hijack"}); status != http.StatusForbidden {
		t.Fatalf("non-owner edit status %d, want 403", status)
	}

	// Follow, comment, reaction
	if status, _ = e.do(t, http.MethodPost, "/api/v1/users/alice/follow", bob.token, nil); status != http.StatusNoContent {
		t.Fatalf("follow status %d", status)
	}
	if status, _ = e.do(t, http.MethodPost, "/api/v1/users/alice/follow", bob.token, nil); status != http.StatusNoContent {
		t.Fatalf("duplicate follow status %d", status)
	}
	status, body = e.do(t, http.MethodGet, "/api/v1/users/alice/followers?limit=10", "", nil)
	if status != http.StatusOK || len(dataList(t, body)) != 1 {
		t.Fatalf("followers: %d %v", status, body)
	}
	status, body = e.do(t, http.MethodPost, "/api/v1/posts/"+postID+"/comments", bob.token, map[string]any{"content": "Nice post"})
	if status != http.StatusCreated {
		t.Fatalf("comment: %d %v", status, body)
	}
	commentID := body["data"].(map[string]any)["id"].(string)

	status, body = e.do(t, http.MethodPut, "/api/v1/posts/"+postID+"/reaction", bob.token, map[string]any{"type": "love"})
	if status != http.StatusOK || body["data"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("reaction: %d %v", status, body)
	}
	status, body = e.do(t, http.MethodGet, "/api/v1/posts/"+postID+"/reaction-summary", bob.token, nil)
	summary := body["data"].(map[string]any)
	if status != http.StatusOK || summary["viewer_reaction"] != "love" {
		t.Fatalf("summary: %d %v", status, body)
	}

	// Notifications arrive asynchronously; poll briefly.
	unread := 0
	for i := 0; i < 40; i++ {
		status, body = e.do(t, http.MethodGet, "/api/v1/notifications/unread-count", alice.token, nil)
		if status != http.StatusOK {
			t.Fatalf("unread count: %d %v", status, body)
		}
		unread = int(body["data"].(map[string]any)["unread"].(float64))
		if unread >= 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if unread < 3 {
		t.Fatalf("expected 3 notifications (follow, comment, reaction), got %d", unread)
	}

	status, body = e.do(t, http.MethodGet, "/api/v1/notifications?limit=10", alice.token, nil)
	if status != http.StatusOK || len(dataList(t, body)) < 3 {
		t.Fatalf("notifications: %d %v", status, body)
	}
	firstNotification := dataList(t, body)[0].(map[string]any)["id"].(string)
	if status, _ = e.do(t, http.MethodPatch, "/api/v1/notifications/"+firstNotification+"/read", alice.token, nil); status != http.StatusNoContent {
		t.Fatalf("mark read status %d", status)
	}
	if status, _ = e.do(t, http.MethodPost, "/api/v1/notifications/read-all", alice.token, nil); status != http.StatusOK {
		t.Fatalf("mark all read status %d", status)
	}

	// Feed for bob contains alice's post; alice's feed contains her own.
	status, body = e.do(t, http.MethodGet, "/api/v1/feed?limit=20", bob.token, nil)
	if status != http.StatusOK {
		t.Fatalf("feed: %d %v", status, body)
	}
	if !containsPost(t, body, postID) {
		t.Fatalf("bob's feed is missing alice's post: %v", body)
	}
	if status, _ = e.do(t, http.MethodGet, "/api/v1/feed?limit=20", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous feed status %d, want 401", status)
	}

	// Search
	status, body = e.do(t, http.MethodGet, "/api/v1/search/users?q=ali", "", nil)
	if status != http.StatusOK || len(dataList(t, body)) == 0 {
		t.Fatalf("user search: %d %v", status, body)
	}
	status, body = e.do(t, http.MethodGet, "/api/v1/search/posts?q=Published", "", nil)
	if status != http.StatusOK || !containsPost(t, body, postID) {
		t.Fatalf("post search: %d %v", status, body)
	}

	// Comment permissions: post author may delete a comment by someone else.
	if status, _ = e.do(t, http.MethodDelete, "/api/v1/comments/"+commentID, alice.token, nil); status != http.StatusNoContent {
		t.Fatalf("post author deleting comment status %d", status)
	}

	// Refresh rotation and reuse detection.
	status, body = e.do(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refresh_token": bob.refresh})
	if status != http.StatusOK {
		t.Fatalf("refresh: %d %v", status, body)
	}
	rotated := body["data"].(map[string]any)["refresh_token"].(string)
	if rotated == bob.refresh {
		t.Fatal("refresh token was not rotated")
	}
	if status, _ = e.do(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refresh_token": bob.refresh}); status != http.StatusUnauthorized {
		t.Fatalf("reused refresh token status %d, want 401", status)
	}
	// Reuse revokes the whole family, including the rotated token.
	if status, _ = e.do(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refresh_token": rotated}); status != http.StatusUnauthorized {
		t.Fatalf("family after reuse status %d, want 401", status)
	}

	// Logout
	aliceRefresh := alice.refresh
	if status, _ = e.do(t, http.MethodPost, "/api/v1/auth/logout", "", map[string]any{"refresh_token": aliceRefresh}); status != http.StatusNoContent {
		t.Fatalf("logout status %d", status)
	}
	if status, _ = e.do(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refresh_token": aliceRefresh}); status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status %d", status)
	}

	// Password change revokes sessions and old credentials stop working.
	newLogin := e.register(t, "carol")
	if status, _ = e.do(t, http.MethodPatch, "/api/v1/users/me/password", newLogin.token, map[string]any{
		"current_password": "Password123!", "new_password": "NewPassword456!",
	}); status != http.StatusNoContent {
		t.Fatalf("change password status %d", status)
	}
	if status, _ = e.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"login": "carol", "password": "Password123!"}); status != http.StatusUnauthorized {
		t.Fatalf("old password status %d, want 401", status)
	}
	if status, _ = e.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"login": "carol", "password": "NewPassword456!"}); status != http.StatusOK {
		t.Fatalf("new password status %d", status)
	}

	// Duplicate registration is a conflict, and no partial user was created.
	if status, _ = e.do(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"username": "alice", "email": "other@example.com", "password": "Password123!",
	}); status != http.StatusConflict {
		t.Fatalf("duplicate username status %d, want 409", status)
	}
	var users int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE username = 'alice'`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("alice rows = %d, want 1", users)
	}
}

func containsPost(t *testing.T, body map[string]any, id string) bool {
	t.Helper()
	for _, item := range dataList(t, body) {
		if post, ok := item.(map[string]any); ok && post["id"] == id {
			return true
		}
	}
	return false
}
