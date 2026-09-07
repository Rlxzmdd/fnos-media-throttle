package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

// ReleaseIfUnbound avoids releasing a downloader still managed by another
// enabled chain (for example when deleting an unrelated disabled chain).
func (e *Engine) ReleaseIfUnbound(ctx context.Context, d Downloader) error {
	return e.withDownloader(ctx, d.ID, e.releaseIfUnbound)
}

func (e *Engine) releaseIfUnbound(ctx context.Context, d Downloader) error {
	_, err := e.Store.ActiveChainForDownloader(ctx, d.ID)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	return e.release(ctx, d)
}

// Release applies the configured policy only to directions captured by this app.
// The saved rule limits survive chain deletion and process restarts.
func (e *Engine) Release(ctx context.Context, d Downloader) error {
	return e.withDownloader(ctx, d.ID, e.release)
}

func (e *Engine) release(ctx context.Context, d Downloader) error {
	return e.releaseDirections(ctx, d, true, true)
}

// ReleaseDirections also handles a chain switching off only one direction.
func (e *Engine) ReleaseDirections(ctx context.Context, d Downloader, upload, download bool) error {
	return e.withDownloader(ctx, d.ID, func(ctx context.Context, current Downloader) error {
		return e.releaseDirections(ctx, current, upload, download)
	})
}

func (e *Engine) releaseDirections(ctx context.Context, d Downloader, upload, download bool) error {
	if !upload {
		d.OriginalUploadBytes = nil
	}
	if !download {
		d.OriginalDownloadBytes = nil
	}
	if d.OriginalUploadBytes == nil && d.OriginalDownloadBytes == nil {
		return nil
	}
	settings, err := e.Store.Settings(ctx)
	if err != nil {
		return err
	}
	if settings.ReleaseMode == domain.ReleaseOriginal {
		return e.restore(ctx, d)
	}
	var up, down int64
	if settings.ReleaseMode == domain.ReleaseMaximum {
		up, down, err = e.Store.ReleaseLimits(ctx, d.ID)
		if err != nil {
			return fmt.Errorf("读取退出限速快照: %w", err)
		}
	}
	patch := LimitPatch{}
	if d.OriginalUploadBytes != nil {
		patch.UploadBytes = &up
	}
	if d.OriginalDownloadBytes != nil {
		patch.DownloadBytes = &down
	}
	reason := "退出接管：解除限速"
	if settings.ReleaseMode == domain.ReleaseMaximum {
		reason = "退出接管：绑定链最高限速"
	}
	return e.releasePatch(ctx, d, patch, reason)
}
