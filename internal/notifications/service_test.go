package notifications

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestStopIsIdempotent(t *testing.T) {
	svc := NewService(nil, testLogger(), 4, 2)
	svc.Start()
	svc.Start() // no-op

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestStopWithoutStart(t *testing.T) {
	svc := NewService(nil, testLogger(), 1, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestEnqueueAfterStopIsSafe(t *testing.T) {
	svc := NewService(nil, testLogger(), 1, 1)
	svc.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	// Must not panic on a closed channel.
	svc.EnqueueFollow(context.Background(), uuid.New(), uuid.New())
}
