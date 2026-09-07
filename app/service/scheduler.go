package service

import (
	"context"
	"time"
)

func (e *Engine) Run(ctx context.Context) {
	e.Tick(ctx)
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.Tick(ctx)
		}
	}
}

func (e *Engine) Tick(ctx context.Context) {
	libs, err := e.Store.Libraries(ctx)
	if err == nil {
		for _, library := range libs {
			if !library.Enabled || !e.dueLibrary(library) {
				continue
			}
			e.PollLibrary(ctx, library)
		}
	}
	downloaders, err := e.Store.Downloaders(ctx)
	if err != nil {
		return
	}
	for _, downloader := range downloaders {
		if !e.dueDownloader(downloader) {
			continue
		}
		e.Apply(ctx, downloader)
	}
}

func (e *Engine) dueLibrary(library Library) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	last := e.lastLibraries[library.ID]
	if time.Since(last) < time.Duration(library.PollSeconds)*time.Second {
		return false
	}
	e.lastLibraries[library.ID] = time.Now()
	return true
}

func (e *Engine) dueDownloader(downloader Downloader) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	last := e.lastDownloaders[downloader.ID]
	if time.Since(last) < time.Duration(downloader.PollSeconds)*time.Second {
		return false
	}
	e.lastDownloaders[downloader.ID] = time.Now()
	return true
}

// ResetSchedule makes imported resources eligible for an immediate poll even
// when their IDs overlap resources from the previous configuration.
func (e *Engine) ResetSchedule() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastLibraries = map[int64]time.Time{}
	e.lastDownloaders = map[int64]time.Time{}
}
