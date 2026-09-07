package media

import (
	"context"

	"github.com/fnos-media-throttle/fnos-media-throttle/diagnostics"
	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

type Library = domain.Library
type LibraryKind = domain.LibraryKind
type LibraryAdapter = Adapter

const (
	LibraryFNOS     = domain.LibraryFNOS
	LibraryEmby     = domain.LibraryEmby
	LibraryJellyfin = domain.LibraryJellyfin
	LibraryHTTP     = domain.LibraryHTTP
)

func debugf(ctx context.Context, format string, args ...any) {
	diagnostics.Logf(ctx, format, args...)
}
