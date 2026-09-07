package storage

import (
	"context"
	"database/sql"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func (s *Store) Settings(ctx context.Context) (Settings, error) {
	var value string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='debug'`).Scan(&value)
	if err != nil && err != sql.ErrNoRows {
		return Settings{}, err
	}
	settings := Settings{Debug: value == "1" || value == "true", ReleaseMode: domain.ReleaseOriginal}
	err = s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='release_mode'`).Scan(&settings.ReleaseMode)
	if err != nil && err != sql.ErrNoRows {
		return Settings{}, err
	}
	if err := settings.Normalize(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func (s *Store) SaveSettings(ctx context.Context, settings Settings) error {
	if err := settings.Normalize(); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES('debug',?),('release_mode',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, b(settings.Debug), settings.ReleaseMode)
	return err
}
