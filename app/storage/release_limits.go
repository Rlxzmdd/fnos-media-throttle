package storage

import "context"

// ReleaseLimits returns the last enabled chain's base limits in bytes/s.
// This snapshot deliberately outlives chain membership, but not the downloader.
func (s *Store) ReleaseLimits(ctx context.Context, downloaderID int64) (upload, download int64, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT upload,download FROM release_limits WHERE downloader_id=?`, downloaderID).Scan(&upload, &download)
	return
}
