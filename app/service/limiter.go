package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	downloaders "github.com/fnos-media-throttle/fnos-media-throttle/downloader"
)

func (e *Engine) Apply(ctx context.Context, downloader Downloader) {
	_ = e.withDownloader(ctx, downloader.ID, func(ctx context.Context, current Downloader) error {
		if !current.Enabled {
			if err := e.release(ctx, current); err != nil {
				e.recordApplyError(ctx, current, err)
			}
			return nil
		}
		e.apply(ctx, current)
		return nil
	})
}

func (e *Engine) apply(ctx context.Context, downloader Downloader) {
	ctx = e.DebugContext(ctx)
	adapter := e.downloaderAdapter()
	if adapter == nil {
		return
	}
	if cycle, ok := adapter.(downloaders.CycleAdapter); ok {
		cycleContext, closeCycle := cycle.BeginCycle(ctx, downloader)
		ctx = cycleContext
		defer closeCycle()
	}
	if stats, err := adapter.Stats(ctx, downloader); err == nil {
		_ = e.Store.UpdateTransferSpeeds(ctx, downloader.ID, stats.UploadSpeedBytes/1000, stats.DownloadSpeedBytes/1000)
	}
	chain, err := e.Store.ActiveChainForDownloader(ctx, downloader.ID)
	if err == sql.ErrNoRows {
		if err := e.Release(ctx, downloader); err != nil {
			e.recordApplyError(ctx, downloader, err)
		}
		return
	}
	if err != nil {
		e.recordApplyError(ctx, downloader, fmt.Errorf("读取绑定链: %w", err))
		return
	}
	// Retry a direction release that failed during a previous chain edit.
	if err := e.ReleaseDirections(ctx, downloader, !chain.Rule.Upload.Enabled, !chain.Rule.Download.Enabled); err != nil {
		e.recordApplyError(ctx, downloader, err)
		return
	}
	libraries, err := e.Store.LibrariesForChain(ctx, chain.ID)
	if err != nil {
		e.failToChainBase(ctx, downloader, chain, fmt.Errorf("读取绑定链影视库: %w", err))
		return
	}
	viewers := 0
	for _, library := range libraries {
		if !library.Enabled {
			continue
		}
		if library.LastError != "" {
			e.failToChainBase(ctx, downloader, chain, fmt.Errorf("影视库 %s 不可用: %s", library.Name, library.LastError))
			return
		}
		viewers += library.LastCount
	}
	if err = e.applyChain(ctx, downloader, chain, viewers); err != nil {
		e.recordApplyError(ctx, downloader, err)
		_ = e.Store.SetChainStatus(ctx, chain.ID, viewers, err.Error())
		return
	}
	_ = e.Store.SetChainStatus(ctx, chain.ID, viewers, "")
}

func (e *Engine) applyChain(ctx context.Context, downloader Downloader, chain BindingChain, viewers int) error {
	patch, uploadKB, downloadKB := patchForRule(chain.Rule, viewers)
	if err := e.captureOriginalLimits(ctx, downloader, patch); err != nil {
		return err
	}
	if !limitPatchMatchesRecorded(downloader, patch) {
		if err := e.applyDownloaderLimits(ctx, downloader, patch, fmt.Sprintf("绑定链 %s（%d 人观看）", chain.Name, viewers)); err != nil {
			return fmt.Errorf("设置下载器限速: %w", err)
		}
	}
	currentUp, currentDown := downloader.CurrentUploadKB, downloader.CurrentDownloadKB
	if chain.Rule.Upload.Enabled {
		currentUp = uploadKB
	}
	if chain.Rule.Download.Enabled {
		currentDown = downloadKB
	}
	if err := e.Store.UpdateApplied(ctx, downloader.ID, viewers, currentUp, currentDown, nil, nil, ""); err != nil {
		return err
	}
	return nil
}

func (e *Engine) failToChainBase(ctx context.Context, downloader Downloader, chain BindingChain, cause error) {
	patch, uploadKB, downloadKB := patchForRule(chain.Rule, 0)
	applied := false
	if err := e.captureOriginalLimits(ctx, downloader, patch); err != nil {
		cause = fmt.Errorf("%v；保存接管前限速失败: %w", cause, err)
	} else if limitPatchMatchesRecorded(downloader, patch) {
		applied = true
	} else if err := e.applyDownloaderLimits(ctx, downloader, patch, "绑定链 "+chain.Name+" 故障回退"); err != nil {
		cause = fmt.Errorf("%v；恢复链路基础速度失败: %w", cause, err)
	} else {
		applied = true
	}
	currentUp, currentDown := downloader.CurrentUploadKB, downloader.CurrentDownloadKB
	if applied {
		if chain.Rule.Upload.Enabled {
			currentUp = uploadKB
		}
		if chain.Rule.Download.Enabled {
			currentDown = downloadKB
		}
	}
	_ = e.Store.UpdateApplied(ctx, downloader.ID, 0, currentUp, currentDown, nil, nil, cause.Error())
	_ = e.Store.SetChainStatus(ctx, chain.ID, 0, cause.Error())
	e.Store.Event(ctx, &downloader.ID, "warning", cause.Error())
}

func limitPatchMatchesRecorded(downloader Downloader, patch LimitPatch) bool {
	if downloader.LastAppliedAt == nil {
		return false
	}
	// Cleared originals mean a new takeover, not a continuation of the last
	// recorded write. Never reuse its stale cache after restoring/releasing.
	if (patch.UploadBytes != nil && downloader.OriginalUploadBytes == nil) ||
		(patch.DownloadBytes != nil && downloader.OriginalDownloadBytes == nil) {
		return false
	}
	if patch.UploadBytes != nil && *patch.UploadBytes/1000 != downloader.CurrentUploadKB {
		return false
	}
	if patch.DownloadBytes != nil && *patch.DownloadBytes/1000 != downloader.CurrentDownloadKB {
		return false
	}
	return true
}

func (e *Engine) captureOriginalLimits(ctx context.Context, downloader Downloader, patch LimitPatch) error {
	needUpload := patch.UploadBytes != nil && downloader.OriginalUploadBytes == nil
	needDownload := patch.DownloadBytes != nil && downloader.OriginalDownloadBytes == nil
	if !needUpload && !needDownload {
		return nil
	}

	upload, download, err := e.downloaderAdapter().Limits(ctx, downloader)
	if err != nil {
		return fmt.Errorf("读取下载器现有限速: %w", err)
	}
	var originalUpload, originalDownload *int64
	if needUpload {
		originalUpload = &upload
	}
	if needDownload {
		originalDownload = &download
	}
	if err := e.Store.CaptureOriginalLimits(ctx, downloader.ID, originalUpload, originalDownload); err != nil {
		return fmt.Errorf("持久化接管前限速: %w", err)
	}
	return nil
}

func (e *Engine) recordApplyError(ctx context.Context, downloader Downloader, err error) {
	_ = e.Store.UpdateApplied(ctx, downloader.ID, downloader.ViewerCount, downloader.CurrentUploadKB, downloader.CurrentDownloadKB, nil, nil, err.Error())
	var writeError *limitWriteError
	if !errors.As(err, &writeError) {
		e.Store.Event(ctx, &downloader.ID, "error", err.Error())
	}
}
