package downloader

import (
	"context"

	"github.com/fnos-media-throttle/fnos-media-throttle/diagnostics"
	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

type Downloader = domain.Downloader
type DownloaderKind = domain.DownloaderKind
type DownloaderAdapter = Adapter
type TransferStats = domain.TransferStats
type LimitPatch = domain.LimitPatch

const (
	DownloaderQB           = domain.DownloaderQB
	DownloaderTransmission = domain.DownloaderTransmission
	DownloaderFNOS         = domain.DownloaderFNOS
)

func debugf(ctx context.Context, format string, args ...any) {
	diagnostics.Logf(ctx, format, args...)
}

func ptr(value int64) *int64 { return &value }
