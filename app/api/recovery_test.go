package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
	"github.com/fnos-media-throttle/fnos-media-throttle/service"
	"github.com/fnos-media-throttle/fnos-media-throttle/storage"
)

type recoveryDownloader struct {
	upload   int64
	download int64
	writes   int
}

func (adapter *recoveryDownloader) Test(context.Context, domain.Downloader) error { return nil }
func (adapter *recoveryDownloader) Limits(context.Context, domain.Downloader) (int64, int64, error) {
	return adapter.upload, adapter.download, nil
}
func (adapter *recoveryDownloader) Stats(context.Context, domain.Downloader) (domain.TransferStats, error) {
	return domain.TransferStats{}, nil
}
func (adapter *recoveryDownloader) ApplyLimits(_ context.Context, _ domain.Downloader, patch domain.LimitPatch) error {
	adapter.writes++
	if patch.UploadBytes != nil {
		adapter.upload = *patch.UploadBytes
	}
	if patch.DownloadBytes != nil {
		adapter.download = *patch.DownloadBytes
	}
	return nil
}

func TestDeletingBindingChainRestoresOriginalLimits(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	library := domain.Library{Name: "media", Kind: domain.LibraryHTTP, BaseURL: "http://media", Enabled: true}
	if err := store.SaveLibrary(ctx, &library); err != nil {
		t.Fatal(err)
	}
	downloader := domain.Downloader{Name: "downloader", Kind: domain.DownloaderQB, BaseURL: "http://downloader", Enabled: true, PollSeconds: 5}
	if err := store.SaveDownloader(ctx, &downloader); err != nil {
		t.Fatal(err)
	}
	chain := domain.BindingChain{
		Name:          "binding",
		Enabled:       true,
		LibraryIDs:    []int64{library.ID},
		DownloaderIDs: []int64{downloader.ID},
		Rule: domain.ChainRule{
			Upload:   domain.DirectionRule{Enabled: true, BaseKB: 1000, MinKB: 100},
			Download: domain.DirectionRule{Enabled: true, BaseKB: 2000, MinKB: 100},
		},
	}
	if err := store.SaveChain(ctx, &chain); err != nil {
		t.Fatal(err)
	}
	originalUpload, originalDownload := int64(700000), int64(800000)
	if err := store.CaptureOriginalLimits(ctx, downloader.ID, &originalUpload, &originalDownload); err != nil {
		t.Fatal(err)
	}

	adapter := &recoveryDownloader{upload: 1000000, download: 2000000}
	engine := service.NewEngine(store)
	engine.Downloaders = adapter
	request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/chains/%d", chain.ID), nil)
	response := httptest.NewRecorder()
	New(store, engine).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete response = %d, body=%s", response.Code, response.Body.String())
	}
	if adapter.writes != 1 || adapter.upload != originalUpload || adapter.download != originalDownload {
		t.Fatalf("restore writes=%d limits=%d/%d, want 1 and %d/%d", adapter.writes, adapter.upload, adapter.download, originalUpload, originalDownload)
	}
	persisted, err := store.Downloader(ctx, downloader.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.OriginalUploadBytes != nil || persisted.OriginalDownloadBytes != nil {
		t.Fatal("binding deletion restored limits but did not clear takeover state")
	}
}
