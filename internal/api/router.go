// Package api wires HTTP routes to handlers.
package api

import (
	"log/slog"
	"net/http"

	"github.com/haqi/gophersocial/internal/api/handlers"
	"github.com/haqi/gophersocial/internal/api/middleware"
	"github.com/haqi/gophersocial/internal/config"
)

// Deps are the already-constructed handlers. The router does not build services;
// app.App owns wiring, which keeps constructors explicit.
type Deps struct {
	Config        config.Config
	Logger        *slog.Logger
	Auth          *handlers.Auth
	Users         *handlers.Users
	Posts         *handlers.Posts
	Follows       *handlers.Follows
	Comments      *handlers.Comments
	Reactions     *handlers.Reactions
	Feed          *handlers.Feed
	Notifications *handlers.Notifications
	Health        *handlers.Health
	TokenParser   middleware.TokenParser
	Learn         http.Handler
}

func NewRouter(d Deps) http.Handler {
	require := middleware.RequireAuth(d.TokenParser)
	optional := middleware.OptionalAuth(d.TokenParser)

	authLimiter := middleware.NewRateLimiter(d.Config.AuthRateLimit, d.Config.AuthRateWindow)
	limitAuth := func(h http.HandlerFunc) http.Handler {
		return authLimiter.Middleware(h)
	}
	auth := func(h http.HandlerFunc) http.Handler { return require(h) }
	opt := func(h http.HandlerFunc) http.Handler { return optional(h) }

	mux := http.NewServeMux()

	if d.Learn != nil {
		mux.Handle("/learn", d.Learn)
		mux.Handle("/learn/", d.Learn)
	}

	// Operations
	mux.Handle("GET /healthz", http.HandlerFunc(d.Health.Live))
	mux.Handle("GET /readyz", http.HandlerFunc(d.Health.Ready))

	// Interactive API docs. The spec is embedded, so this works offline.
	mux.Handle("GET /swagger/", swaggerUI())
	mux.HandleFunc("GET /swagger/openapi.yaml", openAPISpec)

	// Authentication
	mux.Handle("POST /api/v1/auth/register", limitAuth(d.Auth.Register))
	mux.Handle("POST /api/v1/auth/login", limitAuth(d.Auth.Login))
	mux.Handle("POST /api/v1/auth/refresh", limitAuth(d.Auth.Refresh))
	mux.Handle("POST /api/v1/auth/logout", http.HandlerFunc(d.Auth.Logout))

	// Users
	mux.Handle("GET /api/v1/users/me", auth(d.Users.Me))
	mux.Handle("PATCH /api/v1/users/me", auth(d.Users.UpdateMe))
	mux.Handle("PATCH /api/v1/users/me/password", auth(d.Auth.ChangePassword))
	mux.Handle("GET /api/v1/users/{username}", http.HandlerFunc(d.Users.Get))
	mux.Handle("GET /api/v1/users/{username}/posts", opt(d.Posts.UserPosts))
	mux.Handle("GET /api/v1/users/{username}/followers", http.HandlerFunc(d.Users.Followers))
	mux.Handle("GET /api/v1/users/{username}/following", http.HandlerFunc(d.Users.Following))
	mux.Handle("GET /api/v1/users/{username}/follow-status", auth(d.Users.FollowStatus))
	mux.Handle("GET /api/v1/users/{username}/suggestions", http.HandlerFunc(d.Users.Suggestions))
	mux.Handle("POST /api/v1/users/{username}/follow", auth(d.Follows.Follow))
	mux.Handle("DELETE /api/v1/users/{username}/follow", auth(d.Follows.Unfollow))

	// Posts
	mux.Handle("POST /api/v1/posts", auth(d.Posts.Create))
	mux.Handle("GET /api/v1/posts", opt(d.Posts.List))
	mux.Handle("GET /api/v1/posts/{postID}", opt(d.Posts.Get))
	mux.Handle("PATCH /api/v1/posts/{postID}", auth(d.Posts.Update))
	mux.Handle("DELETE /api/v1/posts/{postID}", auth(d.Posts.Delete))

	// Comments
	mux.Handle("POST /api/v1/posts/{postID}/comments", auth(d.Comments.Create))
	mux.Handle("GET /api/v1/posts/{postID}/comments", http.HandlerFunc(d.Comments.List))
	mux.Handle("PATCH /api/v1/comments/{commentID}", auth(d.Comments.Update))
	mux.Handle("DELETE /api/v1/comments/{commentID}", auth(d.Comments.Delete))

	// Reactions
	mux.Handle("PUT /api/v1/posts/{postID}/reaction", auth(d.Reactions.Set))
	mux.Handle("DELETE /api/v1/posts/{postID}/reaction", auth(d.Reactions.Delete))
	mux.Handle("GET /api/v1/posts/{postID}/reactions", http.HandlerFunc(d.Reactions.List))
	mux.Handle("GET /api/v1/posts/{postID}/reaction-summary", opt(d.Reactions.Summary))

	// Discovery
	mux.Handle("GET /api/v1/feed", auth(d.Feed.Feed))
	mux.Handle("GET /api/v1/explore", opt(d.Feed.Explore))
	mux.Handle("GET /api/v1/search/users", http.HandlerFunc(d.Users.Search))
	mux.Handle("GET /api/v1/search/posts", opt(d.Posts.Search))

	// Notifications
	mux.Handle("GET /api/v1/notifications", auth(d.Notifications.List))
	mux.Handle("GET /api/v1/notifications/unread-count", auth(d.Notifications.UnreadCount))
	mux.Handle("PATCH /api/v1/notifications/{notificationID}/read", auth(d.Notifications.MarkRead))
	mux.Handle("POST /api/v1/notifications/read-all", auth(d.Notifications.MarkAllRead))

	var handler http.Handler = mux
	handler = middleware.JSONErrors(handler)
	handler = middleware.Timeout(d.Config.RequestTimeout)(handler)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.Recover(d.Logger)(handler)
	handler = middleware.Logger(d.Logger)(handler)
	handler = middleware.RequestID(handler)
	return handler
}
