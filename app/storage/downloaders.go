package storage

import (
	"context"
	"database/sql"
	"time"
)

func scanDownloader(row interface{ Scan(...any) error }) (Downloader, error) {
	var x Downloader
	var en int
	var applied sql.NullString
	var up, down sql.NullInt64
	e := row.Scan(&x.ID, &x.Name, &x.Kind, &x.BaseURL, &x.Username, &x.Password, &x.UserID, &en, &x.PollSeconds, &up, &down, &x.CurrentUploadKB, &x.CurrentDownloadKB, &x.CurrentUploadSpeedKB, &x.CurrentDownloadSpeedKB, &x.ViewerCount, &applied, &x.LastError)
	x.Enabled = en == 1
	if up.Valid {
		x.OriginalUploadBytes = &up.Int64
	}
	if down.Valid {
		x.OriginalDownloadBytes = &down.Int64
	}
	x.LastAppliedAt = parseTime(applied)
	return x, e
}

const downloaderCols = `id,name,kind,base_url,username,password,user_id,enabled,poll_seconds,original_up,original_down,current_up,current_down,current_up_speed,current_down_speed,viewer_count,last_applied_at,last_error`

func (s *Store) Downloaders(ctx context.Context) ([]Downloader, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+downloaderCols+` FROM downloaders ORDER BY id DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Downloader{}
	for rows.Next() {
		x, err := scanDownloader(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) Downloader(ctx context.Context, id int64) (Downloader, error) {
	return scanDownloader(s.DB.QueryRowContext(ctx, `SELECT `+downloaderCols+` FROM downloaders WHERE id=?`, id))
}
func validPoll(v int) int {
	if v < 5 {
		return 5
	}
	if v > 60 {
		return 60
	}
	return v
}
func (s *Store) SaveDownloader(ctx context.Context, x *Downloader) error {
	if x.Kind == "" {
		x.Kind = DownloaderQB
	}
	if x.UserID <= 0 {
		x.UserID = 1000
	}
	x.PollSeconds = validPoll(x.PollSeconds)
	if x.ID == 0 {
		res, e := s.DB.ExecContext(ctx, `INSERT INTO downloaders(name,kind,base_url,username,password,user_id,enabled,poll_seconds) VALUES(?,?,?,?,?,?,?,?)`, x.Name, x.Kind, x.BaseURL, x.Username, x.Password, x.UserID, b(x.Enabled), x.PollSeconds)
		if e == nil {
			x.ID, _ = res.LastInsertId()
		}
		return e
	}
	_, e := s.DB.ExecContext(ctx, `UPDATE downloaders SET name=?,kind=?,base_url=?,username=?,password=CASE WHEN ?='' THEN password ELSE ? END,user_id=?,enabled=?,poll_seconds=? WHERE id=?`, x.Name, x.Kind, x.BaseURL, x.Username, x.Password, x.Password, x.UserID, b(x.Enabled), x.PollSeconds, x.ID)
	return e
}
func (s *Store) DeleteDownloader(ctx context.Context, id int64) error {
	_, e := s.DB.ExecContext(ctx, `DELETE FROM downloaders WHERE id=?`, id)
	return e
}

// CaptureOriginalLimits persists the pre-takeover limits before any external
// write. COALESCE makes retries idempotent: a failed apply must never replace
// the first captured value with a later value.
func (s *Store) CaptureOriginalLimits(ctx context.Context, id int64, upload, download *int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE downloaders SET
		original_up=COALESCE(original_up,?),
		original_down=COALESCE(original_down,?)
		WHERE id=?`, upload, download, id)
	return err
}

// UpdateApplied persists the last attempted policy and any resulting error.
func (s *Store) UpdateApplied(ctx context.Context, id int64, viewers int, upKB, downKB int64, origUp, origDown *int64, errText string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE downloaders SET viewer_count=?,current_up=?,current_down=?,original_up=COALESCE(original_up,?),original_down=COALESCE(original_down,?),last_applied_at=?,last_error=? WHERE id=?`, viewers, upKB, downKB, origUp, origDown, time.Now().UTC().Format(time.RFC3339Nano), errText, id)
	return e
}
func (s *Store) UpdateTransferSpeeds(ctx context.Context, id, upKB, downKB int64) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE downloaders SET current_up_speed=?,current_down_speed=? WHERE id=?`, upKB, downKB, id)
	return e
}

// ClearOriginalDirections releases values for directions a binding chain no
// longer manages, while preserving a possible takeover of the other direction.
func (s *Store) ClearOriginalDirections(ctx context.Context, id int64, upload, download bool) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE downloaders SET
		original_up=CASE WHEN ? THEN NULL ELSE original_up END,
		original_down=CASE WHEN ? THEN NULL ELSE original_down END
		WHERE id=?`, b(upload), b(download), id)
	return e
}
