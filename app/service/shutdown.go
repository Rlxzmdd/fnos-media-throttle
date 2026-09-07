package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// Shutdown stops admission, cancels in-flight commands, drains the coordinator,
// then releases all captured limits. No normal command can run afterwards.
func (e *Engine) Shutdown(ctx context.Context, perDownloaderTimeout time.Duration) error {
	return e.commands.shutdown(ctx, func(ctx context.Context) error {
		return e.releaseAll(ctx, perDownloaderTimeout)
	})
}

func (e *Engine) releaseAll(ctx context.Context, perDownloaderTimeout time.Duration) error {
	if perDownloaderTimeout <= 0 {
		perDownloaderTimeout = 3 * time.Second
	}

	downloaders, err := e.Store.Downloaders(ctx)
	if err != nil {
		return fmt.Errorf("load downloaders for recovery: %w", err)
	}

	var restoreErrors []error
	for _, downloader := range downloaders {
		if downloader.OriginalUploadBytes == nil && downloader.OriginalDownloadBytes == nil {
			continue
		}

		restoreContext, cancel := context.WithTimeout(ctx, perDownloaderTimeout)
		err := e.release(restoreContext, downloader)
		cancel()
		if err != nil {
			wrapped := fmt.Errorf("restore downloader %q (id=%d): %w", downloader.Name, downloader.ID, err)
			log.Printf("shutdown recovery failed: %v", wrapped)
			restoreErrors = append(restoreErrors, wrapped)
			continue
		}
		log.Printf("shutdown recovery succeeded for downloader %q (id=%d)", downloader.Name, downloader.ID)
	}
	return errors.Join(restoreErrors...)
}

// PrepareConfigImport serializes import with API writes and releases any
// active takeover before the database is replaced.
func (e *Engine) PrepareConfigImport(ctx context.Context, perDownloaderTimeout time.Duration) error {
	return e.Coordinate(ctx, func(ctx context.Context) error {
		return e.releaseAll(ctx, perDownloaderTimeout)
	})
}
