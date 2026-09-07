package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) PruneEmptyChains(ctx context.Context) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT cd.downloader_id
		FROM chain_downloaders cd
		WHERE NOT EXISTS (SELECT 1 FROM chain_libraries cl WHERE cl.chain_id=cd.chain_id)`)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM binding_chains
		WHERE NOT EXISTS (SELECT 1 FROM chain_libraries cl WHERE cl.chain_id=binding_chains.id)
		   OR NOT EXISTS (SELECT 1 FROM chain_downloaders cd WHERE cd.chain_id=binding_chains.id)`); err != nil {
		return nil, err
	}
	return ids, nil
}

const chainCols = `id,name,enabled,upload_enabled,upload_base,upload_step,upload_min,download_enabled,download_base,download_step,download_min,viewer_count,last_error,created_at,updated_at`

func scanChain(scanner interface{ Scan(...any) error }) (BindingChain, error) {
	var chain BindingChain
	var enabled, uploadEnabled, downloadEnabled int
	var created, updated string
	err := scanner.Scan(&chain.ID, &chain.Name, &enabled, &uploadEnabled, &chain.Rule.Upload.BaseKB, &chain.Rule.Upload.StepKB, &chain.Rule.Upload.MinKB, &downloadEnabled, &chain.Rule.Download.BaseKB, &chain.Rule.Download.StepKB, &chain.Rule.Download.MinKB, &chain.ViewerCount, &chain.LastError, &created, &updated)
	if err != nil {
		return chain, err
	}
	chain.Enabled = enabled == 1
	chain.Rule.Upload.Enabled = uploadEnabled == 1
	chain.Rule.Download.Enabled = downloadEnabled == 1
	chain.CreatedAt = parseTime(sql.NullString{String: created, Valid: created != ""})
	chain.UpdatedAt = parseTime(sql.NullString{String: updated, Valid: updated != ""})
	return chain, nil
}

func (s *Store) chainMembers(ctx context.Context, chain *BindingChain) error {
	libRows, err := s.DB.QueryContext(ctx, `SELECT library_id FROM chain_libraries WHERE chain_id=? ORDER BY library_id`, chain.ID)
	if err != nil {
		return err
	}
	defer libRows.Close()
	for libRows.Next() {
		var id int64
		if err = libRows.Scan(&id); err != nil {
			return err
		}
		chain.LibraryIDs = append(chain.LibraryIDs, id)
	}
	if err = libRows.Err(); err != nil {
		return err
	}
	downRows, err := s.DB.QueryContext(ctx, `SELECT downloader_id FROM chain_downloaders WHERE chain_id=? ORDER BY downloader_id`, chain.ID)
	if err != nil {
		return err
	}
	defer downRows.Close()
	for downRows.Next() {
		var id int64
		if err = downRows.Scan(&id); err != nil {
			return err
		}
		chain.DownloaderIDs = append(chain.DownloaderIDs, id)
	}
	return downRows.Err()
}

func (s *Store) Chains(ctx context.Context) ([]BindingChain, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+chainCols+` FROM binding_chains ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chains := []BindingChain{}
	for rows.Next() {
		chain, scanErr := scanChain(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		chains = append(chains, chain)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range chains {
		if err := s.chainMembers(ctx, &chains[i]); err != nil {
			return nil, err
		}
	}
	return chains, nil
}

func (s *Store) Chain(ctx context.Context, id int64) (BindingChain, error) {
	chain, err := scanChain(s.DB.QueryRowContext(ctx, `SELECT `+chainCols+` FROM binding_chains WHERE id=?`, id))
	if err != nil {
		return chain, err
	}
	err = s.chainMembers(ctx, &chain)
	return chain, err
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func chainConflict(ctx context.Context, query rowQuerier, chain BindingChain) error {
	if !chain.Enabled {
		return nil
	}
	for _, downloaderID := range chain.DownloaderIDs {
		var id int64
		err := query.QueryRowContext(ctx, `SELECT c.id FROM binding_chains c JOIN chain_downloaders m ON m.chain_id=c.id WHERE c.enabled=1 AND m.downloader_id=? AND c.id<>? LIMIT 1`, downloaderID, chain.ID).Scan(&id)
		if err == nil {
			return fmt.Errorf("下载器 %d 已属于启用的绑定链 %d", downloaderID, id)
		}
		if err != sql.ErrNoRows {
			return err
		}
	}
	return nil
}

func validDirection(rule DirectionRule) bool {
	return rule.BaseKB >= rule.MinKB && rule.StepKB >= 0 && rule.MinKB >= 0
}

func (s *Store) ValidateChain(ctx context.Context, chain BindingChain) error {
	if len(chain.LibraryIDs) == 0 {
		return errors.New("绑定链至少需要一个影视库")
	}
	if len(chain.DownloaderIDs) == 0 {
		return errors.New("绑定链至少需要一个下载器")
	}
	if !chain.Rule.Upload.Enabled && !chain.Rule.Download.Enabled {
		return errors.New("请至少启用上传或下载限制")
	}
	if !validDirection(chain.Rule.Upload) || !validDirection(chain.Rule.Download) {
		return errors.New("限速规则无效：基础速度不得低于最低速度")
	}
	for _, id := range chain.LibraryIDs {
		if _, err := s.Library(ctx, id); err != nil {
			return fmt.Errorf("影视库 %d: %w", id, err)
		}
	}
	for _, id := range chain.DownloaderIDs {
		if _, err := s.Downloader(ctx, id); err != nil {
			return fmt.Errorf("下载器 %d: %w", id, err)
		}
	}
	return chainConflict(ctx, s.DB, chain)
}

func (s *Store) SaveChain(ctx context.Context, chain *BindingChain) error {
	if err := s.ValidateChain(ctx, *chain); err != nil {
		return err
	}
	if strings.TrimSpace(chain.Name) == "" {
		chain.Name = "绑定链"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := chainConflict(ctx, tx, *chain); err != nil {
		return err
	}
	if chain.ID == 0 {
		result, execErr := tx.ExecContext(ctx, `INSERT INTO binding_chains(name,enabled,upload_enabled,upload_base,upload_step,upload_min,download_enabled,download_base,download_step,download_min,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, chain.Name, b(chain.Enabled), b(chain.Rule.Upload.Enabled), chain.Rule.Upload.BaseKB, chain.Rule.Upload.StepKB, chain.Rule.Upload.MinKB, b(chain.Rule.Download.Enabled), chain.Rule.Download.BaseKB, chain.Rule.Download.StepKB, chain.Rule.Download.MinKB, now, now)
		if execErr != nil {
			return execErr
		}
		chain.ID, _ = result.LastInsertId()
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE binding_chains SET name=?,enabled=?,upload_enabled=?,upload_base=?,upload_step=?,upload_min=?,download_enabled=?,download_base=?,download_step=?,download_min=?,updated_at=? WHERE id=?`, chain.Name, b(chain.Enabled), b(chain.Rule.Upload.Enabled), chain.Rule.Upload.BaseKB, chain.Rule.Upload.StepKB, chain.Rule.Upload.MinKB, b(chain.Rule.Download.Enabled), chain.Rule.Download.BaseKB, chain.Rule.Download.StepKB, chain.Rule.Download.MinKB, now, chain.ID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM chain_libraries WHERE chain_id=?`, chain.ID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM chain_downloaders WHERE chain_id=?`, chain.ID); err != nil {
			return err
		}
	}
	for _, id := range chain.LibraryIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO chain_libraries(chain_id,library_id) VALUES(?,?)`, chain.ID, id); err != nil {
			return err
		}
	}
	for _, id := range chain.DownloaderIDs {
		if chain.Enabled {
			if _, err = tx.ExecContext(ctx, `INSERT INTO release_limits(downloader_id,upload,download) VALUES(?,?,?) ON CONFLICT(downloader_id) DO UPDATE SET upload=CASE WHEN ? THEN excluded.upload ELSE release_limits.upload END,download=CASE WHEN ? THEN excluded.download ELSE release_limits.download END`, id, chain.Rule.Upload.BaseKB*1000, chain.Rule.Download.BaseKB*1000, chain.Rule.Upload.Enabled, chain.Rule.Download.Enabled); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO chain_downloaders(chain_id,downloader_id) VALUES(?,?)`, chain.ID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteChain(ctx context.Context, id int64) ([]int64, error) {
	chain, err := s.Chain(ctx, id)
	if err != nil {
		return nil, err
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM binding_chains WHERE id=?`, id)
	return chain.DownloaderIDs, err
}

func (s *Store) ActiveChainForDownloader(ctx context.Context, downloaderID int64) (BindingChain, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT c.`+strings.ReplaceAll(chainCols, ",", ",c.")+` FROM binding_chains c JOIN chain_downloaders m ON m.chain_id=c.id WHERE c.enabled=1 AND m.downloader_id=? ORDER BY c.id`, downloaderID)
	if err != nil {
		return BindingChain{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return BindingChain{}, sql.ErrNoRows
	}
	chain, err := scanChain(rows)
	if err != nil {
		return BindingChain{}, err
	}
	if rows.Next() {
		return BindingChain{}, errors.New("下载器存在多个启用的绑定链")
	}
	if err := rows.Err(); err != nil {
		return BindingChain{}, err
	}
	if err := rows.Close(); err != nil {
		return BindingChain{}, err
	}
	return chain, s.chainMembers(ctx, &chain)
}

func (s *Store) LibrariesForChain(ctx context.Context, chainID int64) ([]Library, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+libraryCols+` FROM libraries l JOIN chain_libraries m ON m.library_id=l.id WHERE m.chain_id=? ORDER BY l.id`, chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Library{}
	for rows.Next() {
		var item Library
		var enabled, countLocal int
		var checked sql.NullString
		if err = rows.Scan(&item.ID, &item.Name, &item.Kind, &item.BaseURL, &item.APIKey, &item.PollSeconds, &item.ActivitySeconds, &countLocal, &enabled, &item.LastCount, &checked, &item.LastError); err != nil {
			return nil, err
		}
		item.Enabled, item.CountLocalClients, item.LastCheckedAt = enabled == 1, countLocal == 1, parseTime(checked)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SetChainStatus(ctx context.Context, id int64, viewers int, errText string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE binding_chains SET viewer_count=?,last_error=?,updated_at=? WHERE id=?`, viewers, errText, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
