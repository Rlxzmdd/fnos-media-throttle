package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fnos-media-throttle/fnos-media-throttle/downloader"
	"github.com/fnos-media-throttle/fnos-media-throttle/media"
)

type Engine struct {
	commands        *coordinator
	Store           *Store
	Media           LibraryAdapter
	Downloaders     DownloaderAdapter
	mu              sync.Mutex
	lastLibraries   map[int64]time.Time
	lastDownloaders map[int64]time.Time
}

func NewEngine(s *Store) *Engine {
	httpMedia := HTTPMediaAdapter{}
	libraries := media.NewRegistry(
		media.Registration{Kind: LibraryFNOS, Impl: FNOSMediaAdapter{}},
		media.Registration{Kind: LibraryEmby, Impl: httpMedia},
		media.Registration{Kind: LibraryJellyfin, Impl: httpMedia},
		media.Registration{Kind: LibraryHTTP, Impl: httpMedia},
	)
	downloaders := downloader.NewRegistry(
		downloader.Registration{Kind: DownloaderQB, Impl: downloader.QBittorrentAdapter{}},
		downloader.Registration{Kind: DownloaderTransmission, Impl: downloader.TransmissionAdapter{}},
		downloader.Registration{Kind: DownloaderFNOS, Impl: downloader.FNOSDownloaderAdapter{}},
	)
	return &Engine{
		commands: newCoordinator(),
		Store:    s, Media: libraries, Downloaders: downloaders,
		lastLibraries: map[int64]time.Time{}, lastDownloaders: map[int64]time.Time{},
	}
}

func (e *Engine) downloaderAdapter() DownloaderAdapter {
	return e.Downloaders
}

func (e *Engine) DebugContext(ctx context.Context) context.Context {
	if settings, err := e.Store.Settings(ctx); err == nil && settings.Debug {
		return WithDebug(ctx, func(message string) {
			log.Printf("[debug] %s", message)
			e.Store.Event(context.Background(), nil, "debug", message)
		})
	}
	return ctx
}
