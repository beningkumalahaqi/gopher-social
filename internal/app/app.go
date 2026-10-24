// Package app wires the application and owns its lifecycle.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/haqi/gophersocial/docs"
	learnfs "github.com/haqi/gophersocial/docs-learn"
	"github.com/haqi/gophersocial/internal/api"
	"github.com/haqi/gophersocial/internal/api/handlers"
	"github.com/haqi/gophersocial/internal/auth"
	"github.com/haqi/gophersocial/internal/comments"
	"github.com/haqi/gophersocial/internal/config"
	"github.com/haqi/gophersocial/internal/database"
	"github.com/haqi/gophersocial/internal/feed"
	"github.com/haqi/gophersocial/internal/follows"
	"github.com/haqi/gophersocial/internal/learn"
	"github.com/haqi/gophersocial/internal/learn/apispec"
	learnpg "github.com/haqi/gophersocial/internal/learn/pgstore"
	learnweb "github.com/haqi/gophersocial/internal/learn/web"
	"github.com/haqi/gophersocial/internal/notifications"
	"github.com/haqi/gophersocial/internal/posts"
	"github.com/haqi/gophersocial/internal/reactions"
	"github.com/haqi/gophersocial/internal/users"
)

type App struct {
	cfg      config.Config
	logger   *slog.Logger
	pool     *pgxpool.Pool
	notifier *notifications.Service
	server   *http.Server
}

// New constructs every dependency explicitly. No globals, no container.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return nil, err
	}

	// Stores: SQL only.
	userStore := users.NewStore(pool)
	refreshStore := auth.NewRefreshStore(pool)
	postStore := posts.NewStore(pool)
	followStore := follows.NewStore(pool)
	commentStore := comments.NewStore(pool)
	reactionStore := reactions.NewStore(pool)
	notificationStore := notifications.NewStore(pool)

	// Services: business rules.
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authService := auth.NewService(pool, userStore, refreshStore, tokens)
	userService := users.NewService(userStore)
	postService := posts.NewService(postStore)
	feedService := feed.NewService(postStore)
	notificationService := notifications.NewService(
		notificationStore, logger, cfg.NotificationQueue, cfg.NotificationWorkers,
	)
	followService := follows.NewService(followStore, userStore, notificationService)
	commentService := comments.NewService(commentStore, postStore, notificationService)
	reactionService := reactions.NewService(reactionStore, postStore, notificationService)

	// HTTP handlers.
	authHandler := handlers.NewAuth(logger, cfg.MaxBodyBytes, authService)
	usersHandler := handlers.NewUsers(logger, cfg.MaxBodyBytes, userService, followService, authService)
	postsHandler := handlers.NewPosts(logger, cfg.MaxBodyBytes, postService, userService)
	followsHandler := handlers.NewFollows(logger, cfg.MaxBodyBytes, followService)
	commentsHandler := handlers.NewComments(logger, cfg.MaxBodyBytes, commentService)
	reactionsHandler := handlers.NewReactions(logger, cfg.MaxBodyBytes, reactionService)
	feedHandler := handlers.NewFeed(logger, cfg.MaxBodyBytes, feedService)
	notificationsHandler := handlers.NewNotifications(logger, cfg.MaxBodyBytes, notificationService)
	healthHandler := handlers.NewHealth(pool, cfg.ReadinessTimeout, logger)

	// Learning interface: the curriculum is loaded once and shared read-only.
	curriculum, err := learn.Load(learnfs.FS)
	if err != nil {
		return nil, err
	}
	for _, diagnostic := range curriculum.Diagnostics {
		logger.Warn("curriculum diagnostic", "path", diagnostic.Path, "message", diagnostic.Message)
	}
	activities, err := learn.LoadActivities(curriculum)
	if err != nil {
		return nil, err
	}
	for _, diagnostic := range activities.Diagnostics {
		logger.Warn("activity diagnostic", "path", diagnostic.Path, "message", diagnostic.Message)
	}
	spec, err := apispec.Parse(docs.OpenAPI)
	if err != nil {
		return nil, err
	}
	learnServer, err := learnweb.New(learnweb.Deps{
		Curriculum:  curriculum,
		Activities:  activities,
		Spec:        spec,
		Store:       learnpg.New(pool),
		Credentials: authService,
		Logger:      logger,
		Settings: learnweb.Settings{
			SecureCookies:     cfg.IsProduction(),
			PlaygroundEnabled: cfg.LearnPlayground,
			PlaygroundURL:     cfg.LearnPlaygroundURL,
			PlaygroundTimeout: 12 * time.Second,
		},
	})
	if err != nil {
		return nil, err
	}

	router := api.NewRouter(api.Deps{
		Config:        cfg,
		Logger:        logger,
		Auth:          authHandler,
		Users:         usersHandler,
		Posts:         postsHandler,
		Follows:       followsHandler,
		Comments:      commentsHandler,
		Reactions:     reactionsHandler,
		Feed:          feedHandler,
		Notifications: notificationsHandler,
		Health:        healthHandler,
		TokenParser:   tokens,
		Learn:         learnServer.Handler(),
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	return &App{
		cfg:      cfg,
		logger:   logger,
		pool:     pool,
		notifier: notificationService,
		server:   server,
	}, nil
}

// Run starts the background workers and HTTP server, then blocks until ctx is
// cancelled or the listener fails. Shutdown order: stop accepting requests,
// drain notifications, close the pool.
func (a *App) Run(ctx context.Context) error {
	a.notifier.Start()

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("http server listening", "addr", a.cfg.HTTPAddr, "env", a.cfg.Env)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		a.cleanup()
		return err
	case <-ctx.Done():
		a.logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("http shutdown failed", "error", err)
	} else {
		a.logger.Info("http server stopped")
	}

	if err := a.notifier.Stop(shutdownCtx); err != nil {
		a.logger.Warn("notification shutdown incomplete", "error", err)
	}
	a.pool.Close()
	a.logger.Info("shutdown complete")
	return nil
}

// cleanup releases resources when the listener failed before shutdown.
func (a *App) cleanup() {
	stopCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()
	_ = a.notifier.Stop(stopCtx)
	a.pool.Close()
}

// Handler exposes the fully wired router. Tests use it with httptest; production
// code goes through Run.
func (a *App) Handler() http.Handler { return a.server.Handler }

// StartWorkers is used by tests that drive the handler without Run.
func (a *App) StartWorkers() { a.notifier.Start() }

// Close is available for tests that construct an App and do not call Run.
func (a *App) Close() {
	a.cleanup()
}
