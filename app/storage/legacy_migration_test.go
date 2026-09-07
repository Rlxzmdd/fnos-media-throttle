package storage

import (
	"context"
	"testing"
)

func seedLegacyPairs(t *testing.T, s *Store, library Library, downloaders ...Downloader) {
	t.Helper()
	ctx := context.Background()
	for _, col := range []string{"up_base", "up_step", "up_min", "down_base", "down_step", "down_min"} {
		if _, err := s.DB.ExecContext(ctx, `ALTER TABLE downloaders ADD COLUMN `+col+` INTEGER NOT NULL DEFAULT 0`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE bindings(library_id INTEGER,downloader_id INTEGER,PRIMARY KEY(library_id,downloader_id))`); err != nil {
		t.Fatal(err)
	}
	for _, d := range downloaders {
		if _, err := s.DB.ExecContext(ctx, `INSERT INTO bindings VALUES(?,?)`, library.ID, d.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB.ExecContext(ctx, `UPDATE downloaders SET up_base=100,down_base=200 WHERE id=?`, d.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyMigrationRollsBackAndRetries(t *testing.T) {
	s, l, _, first, second := makeChainFixture(t)
	defer s.Close()
	ctx := context.Background()
	seedLegacyPairs(t, s, l, first, second)
	if _, err := s.DB.ExecContext(ctx, `CREATE TRIGGER fail_second BEFORE INSERT ON binding_chains WHEN NEW.name='Transmission 绑定链' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(ctx); err == nil {
		t.Fatal("expected injected migration failure")
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM binding_chains`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial chains=%d err=%v", count, err)
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bindings`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("lost legacy rows=%d err=%v", count, err)
	}
	if _, err := s.DB.ExecContext(ctx, `DROP TRIGGER fail_second`); err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	chains, err := s.Chains(ctx)
	if err != nil || len(chains) != 2 {
		t.Fatalf("chains=%v err=%v", chains, err)
	}
	for _, chain := range chains {
		if chain.Rule.Upload.BaseKB != 100 || chain.Rule.Download.BaseKB != 200 {
			t.Fatal("lost legacy rules")
		}
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name='bindings'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("old table not retired")
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('downloaders') WHERE name='up_base'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("old fields not retired")
	}
	if err := s.migrate(ctx); err != nil {
		t.Fatal("repeat migration:", err)
	}
}

func TestLegacyMigrationCompletesOldPartialConversion(t *testing.T) {
	s, l, _, first, second := makeChainFixture(t)
	defer s.Close()
	ctx := context.Background()
	modern := BindingChain{Name: "keep", Enabled: true, LibraryIDs: []int64{l.ID}, DownloaderIDs: []int64{first.ID}, Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 900}}}
	if err := s.SaveChain(ctx, &modern); err != nil {
		t.Fatal(err)
	}
	seedLegacyPairs(t, s, l, first, second)
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.ActiveChainForDownloader(ctx, first.ID)
	if err != nil || got.Rule.Upload.BaseKB != 900 {
		t.Fatal("modern chain overwritten", err)
	}
	got, err = s.ActiveChainForDownloader(ctx, second.ID)
	if err != nil || got.Rule.Upload.BaseKB != 100 {
		t.Fatal("unconverted chain skipped", err)
	}
}
