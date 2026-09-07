package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FNOSMediaAdapter struct{}

func (a FNOSMediaAdapter) Test(ctx context.Context, l Library) error {
	_, e := a.ActiveViewerCount(ctx, l)
	return e
}
func (a FNOSMediaAdapter) ActiveViewerCount(ctx context.Context, l Library) (int, error) {
	if !l.CountLocalClients {
		return 0, errors.New("当前飞牛影视数据库不提供客户端 IP，无法排除本地播放")
	}
	path := filepath.Clean(strings.TrimSpace(l.BaseURL))
	if !filepath.IsAbs(path) {
		return 0, errors.New("飞牛影视数据库必须使用绝对路径")
	}
	info, e := os.Stat(path)
	if e != nil {
		return 0, fmt.Errorf("无法访问飞牛影视数据库: %w", e)
	}
	if info.IsDir() {
		return 0, errors.New("飞牛影视数据库路径不能是目录")
	}
	seconds := l.ActivitySeconds
	if seconds < 30 {
		seconds = 60
	}
	if seconds > 3600 {
		seconds = 3600
	}
	cutoff := time.Now().UnixMilli() - int64(seconds)*1000
	debugf(ctx, "飞牛影视采集开始：数据库=%s，活跃窗口=%d 秒，截止时间戳=%d", path, seconds, cutoff)
	n, e := queryFNOSViewers(ctx, path, cutoff)
	if e == nil {
		debugf(ctx, "飞牛影视采集完成：直接只读数据库，活跃用户=%d", n)
		return n, nil
	}
	debugf(ctx, "飞牛影视直接读取失败，将尝试复制 SQLite/WAL 副本：%v", e)
	// Certain SQLite/WAL permission combinations cannot be opened in-place even
	// as read-only. Copy the database and its WAL sidecars to a private temp
	// directory and retry without ever writing to trimmedia.db itself.
	tmp, copyErr := os.MkdirTemp("", "fnos-media-throttle-")
	if copyErr != nil {
		return 0, e
	}
	defer os.RemoveAll(tmp)
	copyPath := filepath.Join(tmp, "trimmedia.db")
	if copyErr = copySQLiteSet(path, copyPath); copyErr != nil {
		return 0, e
	}
	if n, copyErr = queryFNOSViewers(ctx, copyPath, cutoff); copyErr == nil {
		debugf(ctx, "飞牛影视采集完成：使用临时 SQLite/WAL 副本，活跃用户=%d", n)
		return n, nil
	}
	debugf(ctx, "飞牛影视临时副本读取失败：%v", copyErr)
	return 0, fmt.Errorf("读取飞牛影视播放记录失败: %w", e)
}
func sqliteReadOnlyURI(path string) string {
	u := "file:" + filepath.ToSlash(path)
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", "query_only(1)")
	q.Add("_pragma", "busy_timeout(1000)")
	return u + "?" + q.Encode()
}
func queryFNOSViewers(ctx context.Context, path string, cutoff int64) (int, error) {
	db, e := sql.Open("sqlite", sqliteReadOnlyURI(path))
	if e != nil {
		return 0, e
	}
	defer db.Close()
	identity, identityMode, identityErr := fnosIdentityExpression(ctx, db)
	if identityErr != nil {
		return 0, identityErr
	}
	var count, recent, visible int
	var maxUpdate sql.NullInt64
	query := `SELECT
		COUNT(DISTINCT CASE WHEN visible=1 AND update_time>? AND user_guid IS NOT NULL AND user_guid<>'' THEN ` + identity + ` END),
		COUNT(CASE WHEN visible=1 AND update_time>? THEN 1 END),
		COUNT(CASE WHEN visible=1 THEN 1 END),
		MAX(update_time)
		FROM item_user_play`
	e = db.QueryRowContext(ctx, query, cutoff, cutoff).Scan(&count, &recent, &visible, &maxUpdate)
	debugf(ctx, "飞牛影视数据库诊断：统计身份=%s，活跃观看端=%d，窗口内播放记录=%d，可见播放记录=%d，最大更新时间戳=%d，截止时间戳=%d", identityMode, count, recent, visible, maxUpdate.Int64, cutoff)
	return count, e
}

func fnosIdentityExpression(ctx context.Context, db *sql.DB) (string, string, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(item_user_play)`)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	columns := map[string]bool{}
	columnNames := make([]string, 0, 16)
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err = rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return "", "", err
		}
		columnName := strings.ToLower(name)
		columns[columnName] = true
		columnNames = append(columnNames, columnName)
	}
	if err = rows.Err(); err != nil {
		return "", "", err
	}
	debugf(ctx, "飞牛影视播放表字段：%s", strings.Join(columnNames, ","))
	for _, candidate := range []string{"device_guid", "device_id", "client_guid", "client_id", "session_guid", "session_id", "play_session_id", "terminal_id"} {
		if columns[candidate] {
			return `user_guid || char(31) || COALESCE(CAST("` + candidate + `" AS TEXT),'')`, "账号+" + candidate, nil
		}
	}
	return `user_guid`, "账号（数据库未提供设备字段）", nil
}
func copySQLiteSet(source, destination string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := source + suffix
		if _, e := os.Stat(src); e != nil {
			if suffix != "" && os.IsNotExist(e) {
				continue
			}
			return e
		}
		dst := destination + suffix
		if e := copyFile(src, dst); e != nil {
			return e
		}
	}
	return nil
}
func copyFile(source, destination string) error {
	src, e := os.Open(source)
	if e != nil {
		return e
	}
	defer src.Close()
	dst, e := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
