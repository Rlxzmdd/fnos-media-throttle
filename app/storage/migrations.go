package storage

import (
	"context"
	"database/sql"
)

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS libraries (id INTEGER PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL, base_url TEXT NOT NULL, api_key TEXT NOT NULL DEFAULT '', poll_seconds INTEGER NOT NULL DEFAULT 5, activity_seconds INTEGER NOT NULL DEFAULT 60, count_local_clients INTEGER NOT NULL DEFAULT 1, enabled INTEGER NOT NULL DEFAULT 1, last_count INTEGER NOT NULL DEFAULT 0, last_checked_at TEXT, last_error TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS downloaders (id INTEGER PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'qbittorrent', base_url TEXT NOT NULL, username TEXT NOT NULL DEFAULT '', password TEXT NOT NULL DEFAULT '', user_id INTEGER NOT NULL DEFAULT 1000, enabled INTEGER NOT NULL DEFAULT 1, poll_seconds INTEGER NOT NULL DEFAULT 5, original_up INTEGER, original_down INTEGER, current_up INTEGER NOT NULL DEFAULT 0, current_down INTEGER NOT NULL DEFAULT 0, current_up_speed INTEGER NOT NULL DEFAULT 0, current_down_speed INTEGER NOT NULL DEFAULT 0, viewer_count INTEGER NOT NULL DEFAULT 0, last_applied_at TEXT, last_error TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY, downloader_id INTEGER REFERENCES downloaders(id) ON DELETE SET NULL, level TEXT NOT NULL, message TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS binding_chains (id INTEGER PRIMARY KEY, name TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, upload_enabled INTEGER NOT NULL DEFAULT 1, upload_base INTEGER NOT NULL DEFAULT 0, upload_step INTEGER NOT NULL DEFAULT 0, upload_min INTEGER NOT NULL DEFAULT 0, download_enabled INTEGER NOT NULL DEFAULT 1, download_base INTEGER NOT NULL DEFAULT 0, download_step INTEGER NOT NULL DEFAULT 0, download_min INTEGER NOT NULL DEFAULT 0, viewer_count INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS chain_libraries (chain_id INTEGER NOT NULL REFERENCES binding_chains(id) ON DELETE CASCADE, library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE, PRIMARY KEY(chain_id, library_id));
CREATE TABLE IF NOT EXISTS chain_downloaders (chain_id INTEGER NOT NULL REFERENCES binding_chains(id) ON DELETE CASCADE, downloader_id INTEGER NOT NULL REFERENCES downloaders(id) ON DELETE CASCADE, PRIMARY KEY(chain_id, downloader_id));
CREATE TABLE IF NOT EXISTS release_limits (downloader_id INTEGER PRIMARY KEY REFERENCES downloaders(id) ON DELETE CASCADE, upload INTEGER NOT NULL, download INTEGER NOT NULL);
`)
	if err != nil {
		return err
	}
	if err = s.ensureColumn(ctx, "libraries", "activity_seconds", "INTEGER NOT NULL DEFAULT 60"); err != nil {
		return err
	}
	if err = s.ensureColumn(ctx, "libraries", "poll_seconds", "INTEGER NOT NULL DEFAULT 5"); err != nil {
		return err
	}
	if err = s.ensureColumn(ctx, "libraries", "count_local_clients", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err = s.ensureColumn(ctx, "downloaders", "current_up_speed", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err = s.ensureColumn(ctx, "downloaders", "current_down_speed", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err = s.ensureColumn(ctx, "downloaders", "user_id", "INTEGER NOT NULL DEFAULT 1000"); err != nil {
		return err
	}
	if err = s.ensureColumn(ctx, "downloaders", "kind", "TEXT NOT NULL DEFAULT 'qbittorrent'"); err != nil {
		return err
	}
	if err = s.migrateLegacyBindings(ctx); err != nil {
		return err
	}
	for _, column := range []string{"up_base", "up_step", "up_min", "down_base", "down_step", "down_min"} {
		if err := s.dropLegacyColumn(ctx, column); err != nil {
			return err
		}
	}
	_, err = s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO release_limits SELECT d.downloader_id,c.upload_base*1000,c.download_base*1000 FROM chain_downloaders d JOIN binding_chains c ON c.id=d.chain_id WHERE c.enabled=1`)
	return err
}

func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.DB.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err = rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
}

// Only retired downloader columns are passed here after successful migration.
func (s *Store) dropLegacyColumn(ctx context.Context, column string) error {
	var count int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('downloaders') WHERE name=?`, column).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `ALTER TABLE downloaders DROP COLUMN `+column)
	return err
}
