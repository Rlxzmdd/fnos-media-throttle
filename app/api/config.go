package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fnos-media-throttle/fnos-media-throttle/buildinfo"
	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

const maxConfigBytes = 10 << 20

func (a Handler) exportConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var backup domain.ConfigBackup
	err := a.engine.Coordinate(r.Context(), func(ctx context.Context) error {
		var exportErr error
		backup, exportErr = a.store.ExportConfig(ctx, buildinfo.Version)
		return exportErr
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="fnos-media-throttle-config-%s.json"`, time.Now().Format("20060102-150405")))
	jsonOut(w, http.StatusOK, backup)
}

func (a Handler) importConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var backup domain.ConfigBackup
	if err := decodeLimit(r, &backup, maxConfigBytes); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("配置文件无法读取: %w", err))
		return
	}
	if err := validateConfig(&backup); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	releaseCtx, cancel := timeLimitedContext(r, 20*time.Second)
	err := a.engine.PrepareConfigImport(releaseCtx, 3*time.Second)
	cancel()
	if err != nil {
		fail(w, http.StatusConflict, fmt.Errorf("导入前解除现有限速失败，未修改配置: %w", err))
		return
	}
	if err := a.store.ImportConfig(r.Context(), backup); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	a.engine.ResetSchedule()
	_ = a.store.Event(r.Context(), nil, "info", fmt.Sprintf("配置导入完成：来源版本 %s，影视库 %d，下载器 %d，绑定链 %d", backup.AppVersion, len(backup.Libraries), len(backup.Downloaders), len(backup.Chains)))
	jsonOut(w, http.StatusOK, map[string]any{
		"appVersion": buildinfo.Version, "sourceVersion": backup.AppVersion,
		"libraries": len(backup.Libraries), "downloaders": len(backup.Downloaders), "chains": len(backup.Chains),
	})
}

func timeLimitedContext(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}

func validateConfig(backup *domain.ConfigBackup) error {
	if backup.FormatVersion != domain.ConfigFormatVersion {
		return fmt.Errorf("不支持的配置格式版本 %d，当前支持 %d", backup.FormatVersion, domain.ConfigFormatVersion)
	}
	if strings.TrimSpace(backup.AppVersion) == "" {
		return errors.New("配置缺少应用版本号")
	}
	if err := backup.Settings.Normalize(); err != nil {
		return err
	}
	libraryIDs := map[int64]bool{}
	for i := range backup.Libraries {
		item := &backup.Libraries[i]
		if item.ID <= 0 || libraryIDs[item.ID] {
			return fmt.Errorf("影视库 ID 无效或重复: %d", item.ID)
		}
		libraryIDs[item.ID] = true
		candidate := domain.Library{ID: item.ID, Name: item.Name, Kind: item.Kind, BaseURL: item.BaseURL, APIKey: item.APIKey}
		if err := validLibrary(&candidate); err != nil {
			return fmt.Errorf("影视库 %d: %w", item.ID, err)
		}
		item.Name = candidate.Name
		if item.PollSeconds < 5 || item.PollSeconds > 60 || item.ActivitySeconds < 30 || item.ActivitySeconds > 3600 {
			return fmt.Errorf("影视库 %d 的轮询参数超出范围", item.ID)
		}
	}
	downloaderIDs := map[int64]bool{}
	for i := range backup.Downloaders {
		item := &backup.Downloaders[i]
		if item.ID <= 0 || downloaderIDs[item.ID] {
			return fmt.Errorf("下载器 ID 无效或重复: %d", item.ID)
		}
		downloaderIDs[item.ID] = true
		candidate := domain.Downloader{ID: item.ID, Name: item.Name, Kind: item.Kind, BaseURL: item.BaseURL, Username: item.Username, Password: item.Password, UserID: item.UserID}
		if err := validDownloader(&candidate); err != nil {
			return fmt.Errorf("下载器 %d: %w", item.ID, err)
		}
		if item.Kind == domain.DownloaderFNOS && item.Password == "" {
			return fmt.Errorf("下载器 %d: 飞牛登录密码不能为空", item.ID)
		}
		item.Name, item.BaseURL, item.UserID = candidate.Name, candidate.BaseURL, candidate.UserID
		if item.PollSeconds < 5 || item.PollSeconds > 60 {
			return fmt.Errorf("下载器 %d 的轮询周期超出范围", item.ID)
		}
	}
	chainIDs, activeDownloaders := map[int64]bool{}, map[int64]int64{}
	for _, chain := range backup.Chains {
		if chain.ID <= 0 || chainIDs[chain.ID] {
			return fmt.Errorf("绑定链 ID 无效或重复: %d", chain.ID)
		}
		chainIDs[chain.ID] = true
		if strings.TrimSpace(chain.Name) == "" || len(chain.LibraryIDs) == 0 || len(chain.DownloaderIDs) == 0 {
			return fmt.Errorf("绑定链 %d 缺少名称或成员", chain.ID)
		}
		if !chain.Rule.Upload.Enabled && !chain.Rule.Download.Enabled {
			return fmt.Errorf("绑定链 %d 未启用任何限速方向", chain.ID)
		}
		if !validDirectionConfig(chain.Rule.Upload) || !validDirectionConfig(chain.Rule.Download) {
			return fmt.Errorf("绑定链 %d 的限速规则无效", chain.ID)
		}
		chainLibraries := map[int64]bool{}
		for _, id := range chain.LibraryIDs {
			if !libraryIDs[id] {
				return fmt.Errorf("绑定链 %d 引用了不存在的影视库 %d", chain.ID, id)
			}
			if chainLibraries[id] {
				return fmt.Errorf("绑定链 %d 重复引用影视库 %d", chain.ID, id)
			}
			chainLibraries[id] = true
		}
		chainDownloaders := map[int64]bool{}
		for _, id := range chain.DownloaderIDs {
			if !downloaderIDs[id] {
				return fmt.Errorf("绑定链 %d 引用了不存在的下载器 %d", chain.ID, id)
			}
			if chainDownloaders[id] {
				return fmt.Errorf("绑定链 %d 重复引用下载器 %d", chain.ID, id)
			}
			chainDownloaders[id] = true
			if chain.Enabled {
				if owner := activeDownloaders[id]; owner != 0 {
					return fmt.Errorf("下载器 %d 同时属于启用的绑定链 %d 和 %d", id, owner, chain.ID)
				}
				activeDownloaders[id] = chain.ID
			}
		}
	}
	return nil
}

func validDirectionConfig(rule domain.DirectionRule) bool {
	return rule.BaseKB >= rule.MinKB && rule.StepKB >= 0 && rule.MinKB >= 0
}
