package storage

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) Event(ctx context.Context, id *int64, level, msg string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE events SET created_at=? WHERE id=(
		SELECT id FROM events WHERE downloader_id IS ? AND level=? AND message=? ORDER BY id DESC LIMIT 1
	)`, now, id, level, msg)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO events(downloader_id,level,message,created_at) VALUES(?,?,?,?)`, id, level, msg, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Events(ctx context.Context) ([]Event, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT MAX(id),downloader_id,level,message,MAX(created_at)
		FROM events
		GROUP BY COALESCE(downloader_id,-1),level,message
		ORDER BY MAX(created_at) DESC,MAX(id) DESC
		LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var x Event
		var did sql.NullInt64
		var at string
		if e = rows.Scan(&x.ID, &did, &x.Level, &x.Message, &at); e != nil {
			return nil, e
		}
		if did.Valid {
			x.DownloaderID = &did.Int64
		}
		x.CreatedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, x)
	}
	return out, rows.Err()
}
