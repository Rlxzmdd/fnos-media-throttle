// Package diagnostics carries optional debug logging through request contexts.
package diagnostics

import (
	"context"
	"fmt"
)

type loggerKey struct{}

// WithLogger attaches a debug logger without coupling adapters to persistence.
func WithLogger(ctx context.Context, logger func(string)) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// Logf emits a diagnostic message when a logger is attached to the context.
func Logf(ctx context.Context, format string, args ...any) {
	if logger, ok := ctx.Value(loggerKey{}).(func(string)); ok && logger != nil {
		logger(fmt.Sprintf(format, args...))
	}
}
