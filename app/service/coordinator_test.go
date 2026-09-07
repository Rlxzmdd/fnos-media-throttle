package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCoordinatorNestedCancellationAndShutdown(t *testing.T) {
	c := newCoordinator()
	entered, done := make(chan struct{}), make(chan error, 1)
	go func() {
		done <- c.run(context.Background(), func(ctx context.Context) error {
			if err := c.run(ctx, func(context.Context) error { return nil }); err != nil {
				return err
			}
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := c.run(ctx, func(context.Context) error { t.Error("cancelled waiter entered"); return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter error=%v", err)
	}
	released := false
	shutdownCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := c.shutdown(shutdownCtx, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		released = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !released {
		t.Fatal("shutdown did not release")
	}
	if err := c.run(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, ErrStopping) {
		t.Fatal("commands accepted after shutdown")
	}
}
