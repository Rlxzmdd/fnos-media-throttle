package service

import (
	"context"
)

func (e *Engine) Restore(ctx context.Context, downloader Downloader) error {
	return e.withDownloader(ctx, downloader.ID, e.restore)
}

func (e *Engine) restore(ctx context.Context, downloader Downloader) error {
	patch := LimitPatch{UploadBytes: downloader.OriginalUploadBytes, DownloadBytes: downloader.OriginalDownloadBytes}
	return e.releasePatch(ctx, downloader, patch, "恢复接管前原值")
}

// releasePatch clears takeover state only after the selected values are applied.
func (e *Engine) releasePatch(ctx context.Context, downloader Downloader, patch LimitPatch, reason string) error {
	if patch.UploadBytes == nil && patch.DownloadBytes == nil {
		return nil
	}
	if !e.limitPatchIsCurrent(ctx, downloader, patch) {
		if err := e.applyDownloaderLimits(ctx, downloader, patch, reason); err != nil {
			return err
		}
	}
	if err := e.Store.ClearOriginalDirections(ctx, downloader.ID, patch.UploadBytes != nil, patch.DownloadBytes != nil); err != nil {
		return err
	}
	return nil
}

func (e *Engine) limitPatchIsCurrent(ctx context.Context, downloader Downloader, patch LimitPatch) bool {
	upload, download, err := e.downloaderAdapter().Limits(ctx, downloader)
	if err != nil {
		return false
	}
	return (patch.UploadBytes == nil || *patch.UploadBytes == upload) &&
		(patch.DownloadBytes == nil || *patch.DownloadBytes == download)
}
