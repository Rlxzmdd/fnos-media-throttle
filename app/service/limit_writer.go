package service

import (
	"context"
	"fmt"
	"log"

	"github.com/fnos-media-throttle/fnos-media-throttle/diagnostics"
)

type limitWriteError struct{ cause error }

func (err *limitWriteError) Error() string { return err.cause.Error() }
func (err *limitWriteError) Unwrap() error { return err.cause }

func (e *Engine) applyDownloaderLimits(ctx context.Context, downloader Downloader, patch LimitPatch, reason string) error {
	if err := e.downloaderAdapter().ApplyLimits(ctx, downloader, patch); err != nil {
		failure := formatLimitResult("限速设置失败", downloader, patch, reason) + fmt.Sprintf("，错误=%v", err)
		log.Print(failure)
		e.Store.Event(ctx, &downloader.ID, "error", failure)
		return &limitWriteError{cause: err}
	}

	success := formatLimitResult("限速设置成功", downloader, patch, reason)
	log.Print(success)
	e.Store.Event(ctx, &downloader.ID, "info", success)
	return nil
}

func formatLimitResult(result string, downloader Downloader, patch LimitPatch, reason string) string {
	return fmt.Sprintf(
		"%s：下载器=%s(id=%d)，场景=%s，上传=%s，下载=%s",
		result,
		downloader.Name,
		downloader.ID,
		reason,
		formatLimitDirection(patch.UploadBytes),
		formatLimitDirection(patch.DownloadBytes),
	)
}

func formatLimitDirection(value *int64) string {
	if value == nil {
		return "未修改"
	}
	return diagnostics.Speed(*value)
}
