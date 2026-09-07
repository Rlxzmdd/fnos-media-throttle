package api

import (
	"errors"
	"strings"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func normalizeLibrary(x *domain.Library) {
	if strings.TrimSpace(x.Name) != "" {
		return
	}
	switch x.Kind {
	case domain.LibraryFNOS:
		x.Name = "飞牛影视"
	case domain.LibraryEmby:
		x.Name = "Emby"
	case domain.LibraryJellyfin:
		x.Name = "Jellyfin"
	default:
		x.Name = "HTTP 人数接口"
	}
}
func validLibrary(x *domain.Library) error {
	normalizeLibrary(x)
	if strings.TrimSpace(x.BaseURL) == "" {
		return errors.New("地址不能为空")
	}
	if x.Kind != domain.LibraryEmby && x.Kind != domain.LibraryJellyfin && x.Kind != domain.LibraryHTTP && x.Kind != domain.LibraryFNOS {
		return errors.New("不支持的影视库类型")
	}
	return nil
}
func validDownloader(x *domain.Downloader) error {
	if x.Kind == "" {
		x.Kind = domain.DownloaderQB
	}
	if x.Kind != domain.DownloaderQB && x.Kind != domain.DownloaderTransmission && x.Kind != domain.DownloaderFNOS {
		return errors.New("不支持的下载器类型")
	}
	if strings.TrimSpace(x.Name) == "" {
		if x.Kind == domain.DownloaderFNOS {
			x.Name = "飞牛下载"
		} else if x.Kind == domain.DownloaderTransmission {
			x.Name = "Transmission"
		} else {
			x.Name = "qBittorrent"
		}
	}
	if (x.Kind == domain.DownloaderQB || x.Kind == domain.DownloaderTransmission) && strings.TrimSpace(x.BaseURL) == "" {
		return errors.New("下载器地址不能为空")
	}
	if x.Kind == domain.DownloaderFNOS {
		if strings.TrimSpace(x.BaseURL) == "" {
			x.BaseURL = "ws://127.0.0.1:5666/websocket?type=main"
		}
		if strings.TrimSpace(x.Username) == "" {
			return errors.New("飞牛登录用户名不能为空")
		}
		if x.ID == 0 && x.Password == "" {
			return errors.New("飞牛登录密码不能为空")
		}
		if x.UserID <= 0 {
			x.UserID = 1000
		}
	}
	return nil
}
