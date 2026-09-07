package service

import (
	"context"
	"testing"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func TestReleaseIfUnboundPreservesActiveChain(t *testing.T) {
	e, d, _, _ := releaseFixture(t, domain.ReleaseUnlimited)
	a := &restoreAdapter{upload: 123, download: 456}
	e.Downloaders = a
	if err := e.ReleaseIfUnbound(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if len(a.calls) != 0 {
		t.Fatal("an active chain was released")
	}
}

func TestTakeoverAfterReleaseDoesNotReuseRecordedLimits(t *testing.T) {
	e, d, c, _ := releaseFixture(t, domain.ReleaseUnlimited)
	a := &restoreAdapter{}
	e.Downloaders = a
	ctx := context.Background()
	if err := e.applyChain(ctx, d, c, 0); err != nil {
		t.Fatal(err)
	}
	d, err := e.Store.Downloader(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Release(ctx, d); err != nil {
		t.Fatal(err)
	}
	d, err = e.Store.Downloader(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.applyChain(ctx, d, c, 0); err != nil {
		t.Fatal(err)
	}
	if a.upload != 500000 || a.download != 800000 || len(a.calls) != 3 {
		t.Fatalf("stale cache skipped takeover: limits=%d/%d calls=%v", a.upload, a.download, a.calls)
	}
}
