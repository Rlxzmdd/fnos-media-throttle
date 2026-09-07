package service

import (
	"context"

	"github.com/fnos-media-throttle/fnos-media-throttle/chain"
	"github.com/fnos-media-throttle/fnos-media-throttle/diagnostics"
	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
	"github.com/fnos-media-throttle/fnos-media-throttle/downloader"
	"github.com/fnos-media-throttle/fnos-media-throttle/media"
	"github.com/fnos-media-throttle/fnos-media-throttle/storage"
)

type Store = storage.Store
type Library = domain.Library
type LibraryKind = domain.LibraryKind
type Downloader = domain.Downloader
type DownloaderKind = domain.DownloaderKind
type BindingChain = domain.BindingChain
type DirectionRule = domain.DirectionRule
type ChainRule = domain.ChainRule
type LimitPatch = domain.LimitPatch
type LibraryAdapter = media.Adapter
type DownloaderAdapter = downloader.Adapter
type HTTPMediaAdapter = media.HTTPMediaAdapter
type FNOSMediaAdapter = media.FNOSMediaAdapter

const (
	LibraryFNOS            = domain.LibraryFNOS
	LibraryEmby            = domain.LibraryEmby
	LibraryJellyfin        = domain.LibraryJellyfin
	LibraryHTTP            = domain.LibraryHTTP
	DownloaderQB           = domain.DownloaderQB
	DownloaderTransmission = domain.DownloaderTransmission
	DownloaderFNOS         = domain.DownloaderFNOS
)

var Open = storage.Open
var patchForRule = chain.PatchForRule

func debugf(ctx context.Context, format string, args ...any) {
	diagnostics.Logf(ctx, format, args...)
}

func WithDebug(ctx context.Context, logger func(string)) context.Context {
	return diagnostics.WithLogger(ctx, logger)
}
