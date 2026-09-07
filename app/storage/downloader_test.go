package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDownloaderKindPersistsAndLegacyDefaultsToQB(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	downloader := Downloader{Name: "飞牛下载", Kind: DownloaderFNOS, Enabled: true}
	if err = store.SaveDownloader(context.Background(), &downloader); err != nil {
		t.Fatal(err)
	}
	got, err := store.Downloader(context.Background(), downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != DownloaderFNOS || got.PollSeconds != 5 || got.UserID != 1000 {
		t.Fatalf("saved downloader = %#v", got)
	}
}
