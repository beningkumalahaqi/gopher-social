package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Service is both the notification API and the bounded background writer.
// Requests enqueue small jobs; workers write them outside the request path.
type Service struct {
	store   *Store
	logger  *slog.Logger
	jobs    chan job
	workers int

	mu      sync.RWMutex
	closed  bool
	started bool
	wg      sync.WaitGroup

	jobTimeout time.Duration
}

type job struct {
	recipientID uuid.UUID
	actorID     uuid.UUID
	kind        Type
	postID      *uuid.UUID
	commentID   *uuid.UUID
}

func NewService(store *Store, logger *slog.Logger, queueSize, workers int) *Service {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 1
	}
	return &Service{
		store:      store,
		logger:     logger,
		jobs:       make(chan job, queueSize),
		workers:    workers,
		jobTimeout: 5 * time.Second,
	}
}

// Start launches the worker goroutines. Calling it twice is a no-op.
func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.closed {
		return
	}
	s.started = true
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.run(i)
	}
	s.logger.Info("notification workers started", "workers", s.workers, "queue_size", cap(s.jobs))
}

func (s *Service) run(id int) {
	defer s.wg.Done()
	for j := range s.jobs {
		// Each job gets its own context: the request context is already gone by
		// the time a queued notification is written.
		ctx, cancel := context.WithTimeout(context.Background(), s.jobTimeout)
		err := s.store.Insert(ctx, Notification{
			UserID:    j.recipientID,
			ActorID:   j.actorID,
			Type:      j.kind,
			PostID:    j.postID,
			CommentID: j.commentID,
		})
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Warn("notification job failed", "worker", id, "type", j.kind, "error", err)
		}
	}
}

// Stop closes the queue, lets workers drain it and waits for them. The supplied
// context bounds the wait so shutdown cannot hang forever.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.jobs)
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("notification workers stopped")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for notification workers: %w", ctx.Err())
	}
}

// enqueue never blocks on a full queue: notifications are best-effort and a
// slow database must not add latency to user requests.
func (s *Service) enqueue(j job) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		s.logger.Debug("notification queue closed; dropping job", "type", j.kind)
		return
	}
	select {
	case s.jobs <- j:
	default:
		s.logger.Warn("notification queue full; dropping job", "type", j.kind, "capacity", cap(s.jobs))
	}
}

func (s *Service) EnqueueFollow(_ context.Context, recipientID, actorID uuid.UUID) {
	if recipientID == actorID {
		return
	}
	s.enqueue(job{recipientID: recipientID, actorID: actorID, kind: TypeFollow})
}

func (s *Service) EnqueueComment(_ context.Context, recipientID, actorID, postID, commentID uuid.UUID) {
	if recipientID == actorID {
		return
	}
	s.enqueue(job{
		recipientID: recipientID,
		actorID:     actorID,
		kind:        TypeComment,
		postID:      &postID,
		commentID:   &commentID,
	})
}

func (s *Service) EnqueueReaction(_ context.Context, recipientID, actorID, postID uuid.UUID) {
	if recipientID == actorID {
		return
	}
	s.enqueue(job{recipientID: recipientID, actorID: actorID, kind: TypeReaction, postID: &postID})
}

// --- read-side API used by HTTP handlers ---

func (s *Service) List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int) ([]Notification, bool, error) {
	return s.store.List(ctx, userID, unreadOnly, limit, offset)
}

func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.store.UnreadCount(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.store.MarkRead(ctx, userID, id)
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.store.MarkAllRead(ctx, userID)
}
