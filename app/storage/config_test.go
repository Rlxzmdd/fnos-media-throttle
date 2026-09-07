package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func TestConfigBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	library := Library{Name: "影视", Kind: LibraryEmby, BaseURL: "http://media", APIKey: "secret-key", PollSeconds: 7, ActivitySeconds: 90, Enabled: true}
	downloader := Downloader{Name: "下载", Kind: DownloaderQB, BaseURL: "http://download", Username: "user", Password: "secret-password", UserID: 1000, PollSeconds: 8, Enabled: true}
	if err := store.SaveLibrary(ctx, &library); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDownloader(ctx, &downloader); err != nil {
		t.Fatal(err)
	}
	chain := BindingChain{Name: "家庭", Enabled: true, LibraryIDs: []int64{library.ID}, DownloaderIDs: []int64{downloader.ID}, Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 900, StepKB: 100, MinKB: 200}}}
	if err := store.SaveChain(ctx, &chain); err != nil {
		t.Fatal(err)
	}
	if err := store.Event(ctx, &downloader.ID, "info", "保留的事件"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSettings(ctx, Settings{Debug: true, ReleaseMode: domain.ReleaseMaximum}); err != nil {
		t.Fatal(err)
	}
	backup, err := store.ExportConfig(ctx, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if backup.FormatVersion != 1 || backup.AppVersion != "9.9.9" || !backup.IncludesSecrets {
		t.Fatalf("unexpected metadata: %+v", backup)
	}
	if backup.Libraries[0].APIKey != "secret-key" || backup.Downloaders[0].Password != "secret-password" {
		t.Fatalf("portable data missing: %+v", backup)
	}
	if err := store.ImportConfig(ctx, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := store.ExportConfig(ctx, "9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Libraries) != 1 || len(restored.Downloaders) != 1 || len(restored.Chains) != 1 {
		t.Fatalf("round trip lost data: %+v", restored)
	}
	current, err := store.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.OriginalUploadBytes != nil || current.LastAppliedAt != nil || current.CurrentUploadSpeedKB != 0 {
		t.Fatal("runtime takeover state must not be imported")
	}
	events, err := store.Events(ctx)
	if err != nil || len(events) != 1 || events[0].Message != "保留的事件" {
		t.Fatalf("import should preserve local event log: %+v, %v", events, err)
	}
}
