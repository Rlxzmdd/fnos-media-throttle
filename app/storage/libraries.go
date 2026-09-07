package storage

import (
	"context"
	"database/sql"
	"time"
)

const libraryCols = `id,name,kind,base_url,api_key,poll_seconds,activity_seconds,count_local_clients,enabled,last_count,last_checked_at,last_error`

func (s *Store) Libraries(ctx context.Context) ([]Library, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+libraryCols+` FROM libraries ORDER BY id DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Library{}
	for rows.Next() {
		var x Library
		var enabled, countLocal int
		var checked sql.NullString
		if e = rows.Scan(&x.ID, &x.Name, &x.Kind, &x.BaseURL, &x.APIKey, &x.PollSeconds, &x.ActivitySeconds, &countLocal, &enabled, &x.LastCount, &checked, &x.LastError); e != nil {
			return nil, e
		}
		x.Enabled = enabled == 1
		x.CountLocalClients = countLocal == 1
		x.LastCheckedAt = parseTime(checked)
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) Library(ctx context.Context, id int64) (Library, error) {
	var x Library
	var enabled, countLocal int
	var checked sql.NullString
	e := s.DB.QueryRowContext(ctx, `SELECT `+libraryCols+` FROM libraries WHERE id=?`, id).Scan(&x.ID, &x.Name, &x.Kind, &x.BaseURL, &x.APIKey, &x.PollSeconds, &x.ActivitySeconds, &countLocal, &enabled, &x.LastCount, &checked, &x.LastError)
	x.Enabled = enabled == 1
	x.CountLocalClients = countLocal == 1
	x.LastCheckedAt = parseTime(checked)
	return x, e
}
func (s *Store) SaveLibrary(ctx context.Context, x *Library) error {
	x.PollSeconds = validPoll(x.PollSeconds)
	x.ActivitySeconds = validActivitySeconds(x.ActivitySeconds)
	if x.ID == 0 {
		r, e := s.DB.ExecContext(ctx, `INSERT INTO libraries(name,kind,base_url,api_key,poll_seconds,activity_seconds,count_local_clients,enabled) VALUES(?,?,?,?,?,?,?,?)`, x.Name, x.Kind, x.BaseURL, x.APIKey, x.PollSeconds, x.ActivitySeconds, b(x.CountLocalClients), b(x.Enabled))
		if e == nil {
			x.ID, _ = r.LastInsertId()
		}
		return e
	}
	_, e := s.DB.ExecContext(ctx, `UPDATE libraries SET name=?,kind=?,base_url=?,api_key=CASE WHEN ?='' THEN api_key ELSE ? END,poll_seconds=?,activity_seconds=?,count_local_clients=?,enabled=? WHERE id=?`, x.Name, x.Kind, x.BaseURL, x.APIKey, x.APIKey, x.PollSeconds, x.ActivitySeconds, b(x.CountLocalClients), b(x.Enabled), x.ID)
	return e
}
func (s *Store) DeleteLibrary(ctx context.Context, id int64) error {
	_, e := s.DB.ExecContext(ctx, `DELETE FROM libraries WHERE id=?`, id)
	return e
}
func (s *Store) SetLibraryStatus(ctx context.Context, id int64, count int, errText string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE libraries SET last_count=?,last_checked_at=?,last_error=? WHERE id=?`, count, time.Now().UTC().Format(time.RFC3339Nano), errText, id)
	return e
}
