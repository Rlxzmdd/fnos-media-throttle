package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

// ExportConfig returns portable application data. Takeover snapshots and live
// counters are intentionally omitted because they only describe this process.
func (s *Store) ExportConfig(ctx context.Context, appVersion string) (domain.ConfigBackup, error) {
	backup := domain.ConfigBackup{
		FormatVersion: domain.ConfigFormatVersion, AppVersion: appVersion,
		ExportedAt: time.Now().UTC(), IncludesSecrets: true,
		Libraries: []domain.LibraryConfig{}, Downloaders: []domain.DownloaderConfig{},
		Chains: []domain.ChainConfig{},
	}
	var err error
	if backup.Settings, err = s.Settings(ctx); err != nil {
		return backup, err
	}
	backup.Settings.AppVersion = ""
	libraries, err := s.Libraries(ctx)
	if err != nil {
		return backup, err
	}
	for _, item := range libraries {
		backup.Libraries = append(backup.Libraries, domain.LibraryConfig{
			ID: item.ID, Name: item.Name, Kind: item.Kind, BaseURL: item.BaseURL, APIKey: item.APIKey,
			PollSeconds: item.PollSeconds, ActivitySeconds: item.ActivitySeconds,
			CountLocalClients: item.CountLocalClients, Enabled: item.Enabled,
		})
	}
	downloaders, err := s.Downloaders(ctx)
	if err != nil {
		return backup, err
	}
	for _, item := range downloaders {
		backup.Downloaders = append(backup.Downloaders, domain.DownloaderConfig{
			ID: item.ID, Name: item.Name, Kind: item.Kind, BaseURL: item.BaseURL,
			Username: item.Username, Password: item.Password, UserID: item.UserID,
			Enabled: item.Enabled, PollSeconds: item.PollSeconds,
		})
	}
	chains, err := s.Chains(ctx)
	if err != nil {
		return backup, err
	}
	for _, item := range chains {
		backup.Chains = append(backup.Chains, domain.ChainConfig{
			ID: item.ID, Name: item.Name, Enabled: item.Enabled, Rule: item.Rule,
			LibraryIDs: item.LibraryIDs, DownloaderIDs: item.DownloaderIDs,
		})
	}
	return backup, nil
}

// ImportConfig atomically replaces portable application data. It must only be
// called after active downloader limits have been released.
func (s *Store) ImportConfig(ctx context.Context, backup domain.ConfigBackup) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`DELETE FROM binding_chains`, `DELETE FROM release_limits`,
		`DELETE FROM libraries`, `DELETE FROM downloaders`, `DELETE FROM settings`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES('debug',?),('release_mode',?)`, b(backup.Settings.Debug), backup.Settings.ReleaseMode); err != nil {
		return err
	}
	for _, item := range backup.Libraries {
		if _, err := tx.ExecContext(ctx, `INSERT INTO libraries(id,name,kind,base_url,api_key,poll_seconds,activity_seconds,count_local_clients,enabled) VALUES(?,?,?,?,?,?,?,?,?)`, item.ID, item.Name, item.Kind, item.BaseURL, item.APIKey, item.PollSeconds, item.ActivitySeconds, b(item.CountLocalClients), b(item.Enabled)); err != nil {
			return fmt.Errorf("导入影视库 %d: %w", item.ID, err)
		}
	}
	for _, item := range backup.Downloaders {
		if _, err := tx.ExecContext(ctx, `INSERT INTO downloaders(id,name,kind,base_url,username,password,user_id,enabled,poll_seconds) VALUES(?,?,?,?,?,?,?,?,?)`, item.ID, item.Name, item.Kind, item.BaseURL, item.Username, item.Password, item.UserID, b(item.Enabled), item.PollSeconds); err != nil {
			return fmt.Errorf("导入下载器 %d: %w", item.ID, err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range backup.Chains {
		if _, err := tx.ExecContext(ctx, `INSERT INTO binding_chains(id,name,enabled,upload_enabled,upload_base,upload_step,upload_min,download_enabled,download_base,download_step,download_min,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.Name, b(item.Enabled), b(item.Rule.Upload.Enabled), item.Rule.Upload.BaseKB, item.Rule.Upload.StepKB, item.Rule.Upload.MinKB, b(item.Rule.Download.Enabled), item.Rule.Download.BaseKB, item.Rule.Download.StepKB, item.Rule.Download.MinKB, now, now); err != nil {
			return fmt.Errorf("导入绑定链 %d: %w", item.ID, err)
		}
		for _, libraryID := range item.LibraryIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO chain_libraries(chain_id,library_id) VALUES(?,?)`, item.ID, libraryID); err != nil {
				return err
			}
		}
		for _, downloaderID := range item.DownloaderIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO chain_downloaders(chain_id,downloader_id) VALUES(?,?)`, item.ID, downloaderID); err != nil {
				return err
			}
			if item.Enabled {
				if _, err := tx.ExecContext(ctx, `INSERT INTO release_limits(downloader_id,upload,download) VALUES(?,?,?) ON CONFLICT(downloader_id) DO UPDATE SET upload=excluded.upload,download=excluded.download`, downloaderID, item.Rule.Upload.BaseKB*1000, item.Rule.Download.BaseKB*1000); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
