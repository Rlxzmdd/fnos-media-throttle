package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

type fixedLibraryAdapter struct{ count int }

func (a fixedLibraryAdapter) Test(context.Context, Library) error { return nil }
func (a fixedLibraryAdapter) ActiveViewerCount(context.Context, Library) (int, error) {
	return a.count, nil
}

func TestLibraryPollingPersistsCountWithoutBinding(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	library := Library{Name: "独立影视库", Kind: LibraryHTTP, BaseURL: "http://example.test", Enabled: true}
	if err = store.SaveLibrary(context.Background(), &library); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(store)
	engine.Media = fixedLibraryAdapter{count: 3}
	engine.Tick(context.Background())
	got, err := store.Library(context.Background(), library.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastCount != 3 || got.LastCheckedAt == nil {
		t.Fatalf("independent library poll was not persisted: %#v", got)
	}
	if got.PollSeconds != 5 {
		t.Fatalf("default poll interval = %d, want 5", got.PollSeconds)
	}
}

func TestFNOSViewerCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trimmedia.db")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`CREATE TABLE item_user_play(user_guid TEXT, visible INTEGER, update_time INTEGER)`)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UnixMilli()
	for _, x := range []struct {
		user    string
		visible int
		updated int64
	}{{"alice", 1, now}, {"alice", 1, now - 1000}, {"bob", 1, now - 2000}, {"old", 1, now - 400000}, {"hidden", 0, now}} {
		if _, e = db.Exec(`INSERT INTO item_user_play VALUES(?,?,?)`, x.user, x.visible, x.updated); e != nil {
			t.Fatal(e)
		}
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	n, e := FNOSMediaAdapter{}.ActiveViewerCount(context.Background(), Library{Kind: LibraryFNOS, BaseURL: path, ActivitySeconds: 300, CountLocalClients: true})
	if e != nil || n != 2 {
		t.Fatalf("got %d, %v", n, e)
	}
}

func TestFNOSViewerCountUsesDeviceWhenAvailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trimmedia.db")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`CREATE TABLE item_user_play(user_guid TEXT, device_id TEXT, visible INTEGER, update_time INTEGER)`); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UnixMilli()
	rows := []struct{ user, device string }{{"same", "tv"}, {"same", "tv"}, {"same", "tv"}, {"same", "phone"}}
	for _, row := range rows {
		if _, e = db.Exec(`INSERT INTO item_user_play VALUES(?,?,1,?)`, row.user, row.device, now); e != nil {
			t.Fatal(e)
		}
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	n, e := FNOSMediaAdapter{}.ActiveViewerCount(context.Background(), Library{Kind: LibraryFNOS, BaseURL: path, ActivitySeconds: 60, CountLocalClients: true})
	if e != nil || n != 2 {
		t.Fatalf("got %d, %v", n, e)
	}
}
