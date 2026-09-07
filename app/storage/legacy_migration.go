package storage

import (
	"context"
	"fmt"
	"time"
)

// migrateLegacyBindings atomically converts the retired pair-table schema.
// Existing modern memberships win over stale pairs left by older versions.
// Dropping the old table in the same transaction is the completion marker.
func (s *Store) migrateLegacyBindings(ctx context.Context) error {
	var exists int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='bindings'`).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,name,enabled,up_base,up_step,up_min,down_base,down_step,down_min FROM downloaders WHERE id IN (SELECT downloader_id FROM bindings) ORDER BY id`)
	if err != nil {
		return err
	}
	type legacy struct {
		id               int64
		name             string
		enabled          int
		upload, download DirectionRule
	}
	var items []legacy
	for rows.Next() {
		var item legacy
		if err := rows.Scan(&item.id, &item.name, &item.enabled, &item.upload.BaseKB, &item.upload.StepKB, &item.upload.MinKB, &item.download.BaseKB, &item.download.StepKB, &item.download.MinKB); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range items {
		var memberships int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM chain_downloaders WHERE downloader_id=?`, item.id).Scan(&memberships); err != nil {
			return err
		}
		if memberships > 0 {
			continue
		}
		if !validDirection(item.upload) || !validDirection(item.download) {
			return fmt.Errorf("旧下载器 %d 的规则无效，迁移已回滚", item.id)
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO binding_chains(name,enabled,upload_enabled,upload_base,upload_step,upload_min,download_enabled,download_base,download_step,download_min,created_at,updated_at) VALUES(?,?,1,?,?,?,1,?,?,?,?,?)`, item.name+" 绑定链", item.enabled, item.upload.BaseKB, item.upload.StepKB, item.upload.MinKB, item.download.BaseKB, item.download.StepKB, item.download.MinKB, now, now)
		if err != nil {
			return fmt.Errorf("迁移旧下载器 %d: %w", item.id, err)
		}
		chainID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO chain_libraries(chain_id,library_id) SELECT ?,library_id FROM bindings WHERE downloader_id=?`, chainID, item.id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO chain_downloaders(chain_id,downloader_id) VALUES(?,?)`, chainID, item.id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE bindings`); err != nil {
		return err
	}
	return tx.Commit()
}
