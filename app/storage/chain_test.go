package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func makeChainFixture(t *testing.T) (*Store, Library, Library, Downloader, Downloader) {
	t.Helper()
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	first := Library{Name: "Emby", Kind: LibraryEmby, BaseURL: "http://emby", CountLocalClients: true, Enabled: true}
	second := Library{Name: "Jellyfin", Kind: LibraryJellyfin, BaseURL: "http://jellyfin", CountLocalClients: true, Enabled: true}
	left := Downloader{Name: "qB", Kind: DownloaderQB, BaseURL: "http://qb", Enabled: true}
	right := Downloader{Name: "Transmission", Kind: DownloaderTransmission, BaseURL: "http://tr", Enabled: true}
	for _, item := range []any{&first, &second, &left, &right} {
		switch value := item.(type) {
		case *Library:
			if err = store.SaveLibrary(ctx, value); err != nil {
				t.Fatal(err)
			}
		case *Downloader:
			if err = store.SaveDownloader(ctx, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	return store, first, second, left, right
}

func TestBindingChainSupportsManyLibrariesAndDownloaders(t *testing.T) {
	ctx := context.Background()
	store, first, second, left, right := makeChainFixture(t)
	defer store.Close()
	chain := BindingChain{
		Name: "家庭联动", Enabled: true,
		LibraryIDs: []int64{first.ID, second.ID}, DownloaderIDs: []int64{left.ID, right.ID},
		Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 10000, StepKB: 1000, MinKB: 1000}, Download: DirectionRule{Enabled: false}},
	}
	if err := store.SaveChain(ctx, &chain); err != nil {
		t.Fatal(err)
	}
	got, err := store.Chain(ctx, chain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.LibraryIDs) != 2 || len(got.DownloaderIDs) != 2 || got.Rule.Download.Enabled {
		t.Fatalf("unexpected chain: %#v", got)
	}
	if value := got.Rule.Upload.Limit(20); value != 1000 {
		t.Fatalf("minimum rule = %d", value)
	}
	if _, err = store.ActiveChainForDownloader(ctx, left.ID); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledChainPreventsDownloaderConflict(t *testing.T) {
	ctx := context.Background()
	store, first, second, left, _ := makeChainFixture(t)
	defer store.Close()
	firstChain := BindingChain{Name: "first", Enabled: true, LibraryIDs: []int64{first.ID}, DownloaderIDs: []int64{left.ID}, Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 1}, Download: DirectionRule{Enabled: false}}}
	if err := store.SaveChain(ctx, &firstChain); err != nil {
		t.Fatal(err)
	}
	secondChain := BindingChain{Name: "second", Enabled: true, LibraryIDs: []int64{second.ID}, DownloaderIDs: []int64{left.ID}, Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 1}, Download: DirectionRule{Enabled: false}}}
	if err := store.SaveChain(ctx, &secondChain); err == nil {
		t.Fatal("expected downloader conflict")
	}
}

func TestPruneEmptyChainsReturnsAffectedDownloaders(t *testing.T) {
	ctx := context.Background()
	store, first, _, left, _ := makeChainFixture(t)
	defer store.Close()
	chain := BindingChain{Name: "temporary", Enabled: true, LibraryIDs: []int64{first.ID}, DownloaderIDs: []int64{left.ID}, Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 1}}}
	if err := store.SaveChain(ctx, &chain); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteLibrary(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	ids, err := store.PruneEmptyChains(ctx)
	if err != nil || len(ids) != 1 || ids[0] != left.ID {
		t.Fatalf("prune ids=%v, err=%v", ids, err)
	}
	if _, err := store.Chain(ctx, chain.ID); err == nil {
		t.Fatal("empty chain should have been removed")
	}
}
