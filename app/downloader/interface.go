// Package downloader contains downloader contracts, routing, and implementations.
package downloader

import (
	"context"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

// Adapter is the stable extension point implemented by every downloader.
type Adapter interface {
	Test(context.Context, domain.Downloader) error
	Limits(context.Context, domain.Downloader) (int64, int64, error)
	Stats(context.Context, domain.Downloader) (domain.TransferStats, error)
	ApplyLimits(context.Context, domain.Downloader, domain.LimitPatch) error
}

// CycleAdapter optionally shares a connection across one sequential polling
// cycle. Providers own the lifecycle; callers must always invoke the cleanup.
type CycleAdapter interface {
	BeginCycle(context.Context, domain.Downloader) (context.Context, func())
}
