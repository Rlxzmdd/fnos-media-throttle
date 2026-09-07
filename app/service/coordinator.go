package service

import (
	"context"
	"errors"
	"sync"
)

var ErrStopping = errors.New("应用正在停止，不再接受修改")

type commandContextKey struct{}

// coordinator serializes configuration changes and device writes. The context
// marker supports nested synchronous engine calls; never share it with a new
// goroutine. Read-only API requests do not enter this coordinator.
type coordinator struct {
	permit   chan struct{}
	lifetime context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
}

func newCoordinator() *coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	c := &coordinator{permit: make(chan struct{}, 1), lifetime: ctx, cancel: cancel}
	c.permit <- struct{}{}
	return c
}

func (c *coordinator) stop() { c.stopOnce.Do(c.cancel) }

func (c *coordinator) run(ctx context.Context, action func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ctx.Value(commandContextKey{}) == c {
		return action(ctx)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.lifetime.Done():
		return ErrStopping
	case <-c.permit:
	}
	defer func() { c.permit <- struct{}{} }()
	if c.lifetime.Err() != nil {
		return ErrStopping
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	commandCtx, cancel := context.WithCancel(ctx)
	stopCancel := context.AfterFunc(c.lifetime, cancel)
	defer cancel()
	defer stopCancel()
	return action(context.WithValue(commandCtx, commandContextKey{}, c))
}

func (c *coordinator) shutdown(ctx context.Context, release func(context.Context) error) error {
	c.stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.permit:
	}
	defer func() { c.permit <- struct{}{} }()
	if err := ctx.Err(); err != nil {
		return err
	}
	return release(context.WithValue(ctx, commandContextKey{}, c))
}

// Coordinate encloses the entire read-modify-write command, not only SQL or
// the last device write. Nested engine calls use the provided context.
func (e *Engine) Coordinate(ctx context.Context, action func(context.Context) error) error {
	return e.commands.run(ctx, action)
}

func (e *Engine) StopCommands() { e.commands.stop() }

func (e *Engine) withDownloader(ctx context.Context, id int64, action func(context.Context, Downloader) error) error {
	return e.Coordinate(ctx, func(ctx context.Context) error {
		current, err := e.Store.Downloader(ctx, id)
		if err != nil {
			return err
		}
		return action(ctx, current)
	})
}
