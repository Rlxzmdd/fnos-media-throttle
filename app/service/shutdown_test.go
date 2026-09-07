package service

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

type restoreAdapter struct {
	calls      []int64
	failID     int64
	blockID    int64
	upload     int64
	download   int64
	limitReads int
	applyError error
	limitsByID map[int64][2]int64
}

func (adapter *restoreAdapter) Test(context.Context, Downloader) error { return nil }
func (adapter *restoreAdapter) Limits(_ context.Context, downloader Downloader) (int64, int64, error) {
	adapter.limitReads++
	if limits, ok := adapter.limitsByID[downloader.ID]; ok {
		return limits[0], limits[1], nil
	}
	return adapter.upload, adapter.download, nil
}
func (adapter *restoreAdapter) Stats(context.Context, Downloader) (domain.TransferStats, error) {
	return domain.TransferStats{}, nil
}
func (adapter *restoreAdapter) ApplyLimits(ctx context.Context, downloader Downloader, patch LimitPatch) error {
	adapter.calls = append(adapter.calls, downloader.ID)
	if downloader.ID == adapter.blockID {
		<-ctx.Done()
		return ctx.Err()
	}
	if downloader.ID == adapter.failID {
		return errors.New("downloader unavailable")
	}
	if adapter.applyError != nil {
		return adapter.applyError
	}
	if adapter.limitsByID != nil {
		limits := adapter.limitsByID[downloader.ID]
		if patch.UploadBytes != nil {
			limits[0] = *patch.UploadBytes
		}
		if patch.DownloadBytes != nil {
			limits[1] = *patch.DownloadBytes
		}
		adapter.limitsByID[downloader.ID] = limits
		return nil
	}
	if patch.UploadBytes != nil {
		adapter.upload = *patch.UploadBytes
	}
	if patch.DownloadBytes != nil {
		adapter.download = *patch.DownloadBytes
	}
	return nil
}

func TestShutdownContinuesAfterDownloaderFailure(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	first := Downloader{Name: "first", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	second := Downloader{Name: "second", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	for _, downloader := range []*Downloader{&first, &second} {
		if err := store.SaveDownloader(ctx, downloader); err != nil {
			t.Fatal(err)
		}
		originalUpload, originalDownload := int64(1024), int64(2048)
		if err := store.UpdateApplied(ctx, downloader.ID, 0, 0, 0, &originalUpload, &originalDownload, ""); err != nil {
			t.Fatal(err)
		}
	}

	adapter := &restoreAdapter{failID: first.ID, limitsByID: map[int64][2]int64{}}
	engine := NewEngine(store)
	engine.Downloaders = adapter
	if err := engine.Shutdown(ctx, 100*time.Millisecond); err == nil {
		t.Fatal("Shutdown returned nil after a downloader failed")
	}
	if !slices.Contains(adapter.calls, first.ID) || !slices.Contains(adapter.calls, second.ID) {
		t.Fatalf("restore calls = %v, want both downloader IDs", adapter.calls)
	}

	failed, err := store.Downloader(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	succeeded, err := store.Downloader(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.OriginalUploadBytes == nil || failed.OriginalDownloadBytes == nil {
		t.Fatal("failed recovery cleared the persisted original limits")
	}
	if succeeded.OriginalUploadBytes != nil || succeeded.OriginalDownloadBytes != nil {
		t.Fatal("successful recovery did not clear the persisted original limits")
	}
}

func TestApplyPersistsOriginalLimitsBeforeExternalWriteAndReusesThemOnRetry(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	downloader := Downloader{Name: "retry", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	if err := store.SaveDownloader(ctx, &downloader); err != nil {
		t.Fatal(err)
	}
	chain := BindingChain{
		Name: "test chain",
		Rule: ChainRule{
			Upload:   DirectionRule{Enabled: true, BaseKB: 1000, MinKB: 100},
			Download: DirectionRule{Enabled: true, BaseKB: 2000, MinKB: 100},
		},
	}
	adapter := &restoreAdapter{upload: 777000, download: 888000, applyError: errors.New("write failed")}
	engine := NewEngine(store)
	engine.Downloaders = adapter

	if err := engine.applyChain(ctx, downloader, chain, 1); err == nil {
		t.Fatal("first apply returned nil after the external write failed")
	}
	persisted, err := store.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.OriginalUploadBytes == nil || *persisted.OriginalUploadBytes != adapter.upload ||
		persisted.OriginalDownloadBytes == nil || *persisted.OriginalDownloadBytes != adapter.download {
		t.Fatalf("original limits were not persisted before apply: %#v", persisted)
	}

	adapter.applyError = nil
	if err := engine.applyChain(ctx, persisted, chain, 1); err != nil {
		t.Fatal(err)
	}
	if adapter.limitReads != 1 {
		t.Fatalf("limit reads = %d, want 1; retry must reuse persisted originals", adapter.limitReads)
	}
	if len(adapter.calls) != 2 {
		t.Fatalf("apply calls = %d, want 2", len(adapter.calls))
	}

	events, err := store.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundSettingLog := false
	for _, event := range events {
		if strings.Contains(event.Message, "限速设置成功") {
			foundSettingLog = true
			break
		}
	}
	if !foundSettingLog {
		t.Fatal("successful limit write was not recorded in the event log")
	}
}

func TestApplySkipsWriteAndLogWhenRecordedLimitIsUnchanged(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	downloader := Downloader{Name: "unchanged", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	if err := store.SaveDownloader(ctx, &downloader); err != nil {
		t.Fatal(err)
	}
	originalUpload := int64(900000)
	if err := store.UpdateApplied(ctx, downloader.ID, 1, 1000, 0, &originalUpload, nil, ""); err != nil {
		t.Fatal(err)
	}
	downloader, err = store.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	chain := BindingChain{
		Name: "unchanged chain",
		Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 1000, MinKB: 100}},
	}
	adapter := &restoreAdapter{upload: 1000000}
	engine := NewEngine(store)
	engine.Downloaders = adapter
	if err := engine.applyChain(ctx, downloader, chain, 1); err != nil {
		t.Fatal(err)
	}
	if len(adapter.calls) != 0 {
		t.Fatalf("unchanged limit performed %d external writes", len(adapter.calls))
	}
	events, err := store.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("unchanged limit produced events: %#v", events)
	}
}

func TestRestoreTreatsMatchingCurrentLimitsAsNoop(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	downloader := Downloader{Name: "noop", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	if err := store.SaveDownloader(ctx, &downloader); err != nil {
		t.Fatal(err)
	}
	originalUpload, originalDownload := int64(123000), int64(456000)
	if err := store.CaptureOriginalLimits(ctx, downloader.ID, &originalUpload, &originalDownload); err != nil {
		t.Fatal(err)
	}
	downloader, err = store.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &restoreAdapter{upload: originalUpload, download: originalDownload}
	engine := NewEngine(store)
	engine.Downloaders = adapter

	if err := engine.Restore(ctx, downloader); err != nil {
		t.Fatal(err)
	}
	if len(adapter.calls) != 0 {
		t.Fatalf("restore performed %d external writes, want no-op", len(adapter.calls))
	}
	restored, err := store.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.OriginalUploadBytes != nil || restored.OriginalDownloadBytes != nil {
		t.Fatal("no-op restore did not clear persisted takeover state")
	}
}

func TestCrashRestartKeepsFirstOriginalAndRestoresItOnShutdown(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "throttle.db")
	store, err := Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	downloader := Downloader{Name: "restart", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	if err := store.SaveDownloader(ctx, &downloader); err != nil {
		t.Fatal(err)
	}
	chain := BindingChain{
		Name: "restart chain",
		Rule: ChainRule{
			Upload:   DirectionRule{Enabled: true, BaseKB: 1000, MinKB: 100},
			Download: DirectionRule{Enabled: true, BaseKB: 2000, MinKB: 100},
		},
	}
	beforeCrash := &restoreAdapter{upload: 700000, download: 800000}
	engine := NewEngine(store)
	engine.Downloaders = beforeCrash
	if err := engine.applyChain(ctx, downloader, chain, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen the database without running Restore, as if the process terminated
	// abruptly. The first captured values must survive the restart.
	restartedStore, err := Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer restartedStore.Close()
	restartedDownloader, err := restartedStore.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterRestart := &restoreAdapter{upload: beforeCrash.upload, download: beforeCrash.download}
	restartedEngine := NewEngine(restartedStore)
	restartedEngine.Downloaders = afterRestart
	if err := restartedEngine.applyChain(ctx, restartedDownloader, chain, 1); err != nil {
		t.Fatal(err)
	}
	if afterRestart.limitReads != 0 {
		t.Fatalf("restart reread current limits %d times and could overwrite the first originals", afterRestart.limitReads)
	}

	restartedDownloader, err = restartedStore.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := restartedEngine.Restore(ctx, restartedDownloader); err != nil {
		t.Fatal(err)
	}
	if afterRestart.upload != 700000 || afterRestart.download != 800000 {
		t.Fatalf("restored limits = %d/%d, want 700000/800000", afterRestart.upload, afterRestart.download)
	}
}

func TestShutdownUsesIndependentTimeoutAndContinues(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	blocked := Downloader{Name: "blocked", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	healthy := Downloader{Name: "healthy", Kind: DownloaderQB, Enabled: true, PollSeconds: 5}
	for _, downloader := range []*Downloader{&blocked, &healthy} {
		if err := store.SaveDownloader(ctx, downloader); err != nil {
			t.Fatal(err)
		}
		upload, download := int64(1000), int64(2000)
		if err := store.CaptureOriginalLimits(ctx, downloader.ID, &upload, &download); err != nil {
			t.Fatal(err)
		}
	}

	// Downloaders are restored by descending ID, so block the first item and
	// verify the following item still receives a fresh timeout context.
	adapter := &restoreAdapter{blockID: healthy.ID, limitsByID: map[int64][2]int64{}}
	engine := NewEngine(store)
	engine.Downloaders = adapter
	started := time.Now()
	err = engine.Shutdown(ctx, 100*time.Millisecond)
	if err == nil {
		t.Fatal("Shutdown returned nil after the blocked downloader timed out")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatalf("independent restore timeout took too long: %s", time.Since(started))
	}
	if !slices.Contains(adapter.calls, blocked.ID) || !slices.Contains(adapter.calls, healthy.ID) {
		t.Fatalf("restore calls = %v, want both downloader IDs", adapter.calls)
	}
}

func TestSchedulerRetriesPendingRestoreForDisabledDownloader(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	downloader := Downloader{Name: "disabled", Kind: DownloaderQB, Enabled: false, PollSeconds: 5}
	if err := store.SaveDownloader(ctx, &downloader); err != nil {
		t.Fatal(err)
	}
	originalUpload, originalDownload := int64(300000), int64(400000)
	if err := store.CaptureOriginalLimits(ctx, downloader.ID, &originalUpload, &originalDownload); err != nil {
		t.Fatal(err)
	}
	adapter := &restoreAdapter{upload: 1000000, download: 2000000}
	engine := NewEngine(store)
	engine.Downloaders = adapter

	engine.Tick(ctx)
	if len(adapter.calls) != 1 || adapter.upload != originalUpload || adapter.download != originalDownload {
		t.Fatalf("pending restore calls=%v limits=%d/%d", adapter.calls, adapter.upload, adapter.download)
	}
	persisted, err := store.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.OriginalUploadBytes != nil || persisted.OriginalDownloadBytes != nil {
		t.Fatal("disabled downloader retained takeover state after retry succeeded")
	}
}
