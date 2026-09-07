package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func releaseFixture(t *testing.T, mode string) (*Engine, Downloader, BindingChain, string) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "release.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	l := Library{Name: "media", Kind: LibraryHTTP, Enabled: true}
	d := Downloader{Name: "download", Kind: DownloaderQB, Enabled: true}
	if err := s.SaveLibrary(ctx, &l); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveDownloader(ctx, &d); err != nil {
		t.Fatal(err)
	}
	c := BindingChain{Name: "chain", Enabled: true, LibraryIDs: []int64{l.ID}, DownloaderIDs: []int64{d.ID},
		Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 500}, Download: DirectionRule{Enabled: true, BaseKB: 800}}}
	if err := s.SaveChain(ctx, &c); err != nil {
		t.Fatal(err)
	}
	up, down := int64(12000), int64(34000)
	if err := s.CaptureOriginalLimits(ctx, d.ID, &up, &down); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSettings(ctx, domain.Settings{ReleaseMode: mode}); err != nil {
		t.Fatal(err)
	}
	d, err = s.Downloader(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(s), d, c, path
}

func TestReleasePolicySurvivesDeletionAndRestart(t *testing.T) {
	for _, tc := range []struct {
		mode     string
		up, down int64
	}{
		{domain.ReleaseOriginal, 12000, 34000}, {domain.ReleaseUnlimited, 0, 0}, {domain.ReleaseMaximum, 500000, 800000},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx := context.Background()
			e, d, c, path := releaseFixture(t, tc.mode)
			if _, err := e.Store.DeleteChain(ctx, c.ID); err != nil {
				t.Fatal(err)
			}
			if err := e.Store.Close(); err != nil {
				t.Fatal(err)
			}
			s, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			e = NewEngine(s)
			a := &restoreAdapter{upload: 1000, download: 2000}
			e.Downloaders = a
			if err := e.Shutdown(ctx, time.Second); err != nil {
				t.Fatal(err)
			}
			if a.upload != tc.up || a.download != tc.down {
				t.Fatalf("limits %d/%d", a.upload, a.download)
			}
			persisted, err := s.Downloader(ctx, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.OriginalUploadBytes != nil || persisted.OriginalDownloadBytes != nil {
				t.Fatal("state not cleared")
			}
			e = NewEngine(s)
			e.Downloaders = a
			if err := e.Release(ctx, persisted); err != nil {
				t.Fatal(err)
			}
			if len(a.calls) != 1 {
				t.Fatal("release repeated a completed write")
			}
		})
	}
}

func TestReleaseFailureRetainsStateAndRetries(t *testing.T) {
	e, d, _, _ := releaseFixture(t, domain.ReleaseUnlimited)
	a := &restoreAdapter{upload: 1, download: 2, applyError: errors.New("offline")}
	e.Downloaders = a
	ctx := context.Background()
	if err := e.Release(ctx, d); err == nil {
		t.Fatal("expected error")
	}
	d, err := e.Store.Downloader(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.OriginalUploadBytes == nil || d.OriginalDownloadBytes == nil {
		t.Fatal("lost retry state")
	}
	a.applyError = nil
	if err := e.Release(ctx, d); err != nil {
		t.Fatal(err)
	}
	if a.upload != 0 || a.download != 0 {
		t.Fatal("retry did not remove limits")
	}
}

func TestReleaseMaximumPreservesDisabledDirectionSnapshot(t *testing.T) {
	e, d, c, _ := releaseFixture(t, domain.ReleaseMaximum)
	ctx := context.Background()
	c.Rule.Upload.Enabled = false
	c.Rule.Upload.BaseKB = 999 // no longer enabled: must not replace the saved upload maximum
	if err := e.Store.SaveChain(ctx, &c); err != nil {
		t.Fatal(err)
	}
	a := &restoreAdapter{upload: 3, download: 4}
	e.Downloaders = a
	if err := e.ReleaseDirections(ctx, d, true, false); err != nil {
		t.Fatal(err)
	}
	if a.upload != 500000 || a.download != 4 {
		t.Fatal("wrong directions or maximum")
	}
	d, err := e.Store.Downloader(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.OriginalUploadBytes != nil || d.OriginalDownloadBytes == nil {
		t.Fatal("wrong retained direction")
	}
}

func TestReleaseMaximumMissingSnapshotDoesNotUnlock(t *testing.T) {
	e, d, _, _ := releaseFixture(t, domain.ReleaseMaximum)
	ctx := context.Background()
	if _, err := e.Store.DB.ExecContext(ctx, `DELETE FROM release_limits`); err != nil {
		t.Fatal(err)
	}
	a := &restoreAdapter{upload: 1, download: 2}
	e.Downloaders = a
	if err := e.Release(ctx, d); err == nil {
		t.Fatal("missing snapshot accepted")
	}
	if len(a.calls) != 0 {
		t.Fatal("limits changed without a snapshot")
	}
}

func TestReleaseMatchingMaximumIsSilent(t *testing.T) {
	e, d, _, _ := releaseFixture(t, domain.ReleaseMaximum)
	a := &restoreAdapter{upload: 500000, download: 800000}
	e.Downloaders = a
	ctx := context.Background()
	if err := e.Release(ctx, d); err != nil {
		t.Fatal(err)
	}
	events, err := e.Store.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.calls) != 0 || len(events) != 0 {
		t.Fatal("unchanged limits produced writes/logs")
	}
}
