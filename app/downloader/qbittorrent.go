package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QBittorrentAdapter controls qBittorrent through its Web API.
type QBittorrentAdapter struct{ Client *http.Client }

func (a QBittorrentAdapter) withAuth(ctx context.Context, d Downloader, fn func(*http.Client) error) error {
	jar, _ := cookiejar.New(nil)
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, Jar: jar}
	}
	loginURL, err := httpEndpoint(d.BaseURL, "/api/v2/auth/login")
	if err != nil {
		return err
	}
	form := url.Values{"username": {d.Username}, "password": {d.Password}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent login returned HTTP %d", response.StatusCode)
	}
	body, _ := io.ReadAll(response.Body)
	if strings.TrimSpace(string(body)) != "Ok." {
		return errors.New("qBittorrent login failed")
	}
	return fn(client)
}

func (a QBittorrentAdapter) Test(ctx context.Context, d Downloader) error {
	return a.withAuth(ctx, d, func(client *http.Client) error {
		endpoint, _ := httpEndpoint(d.BaseURL, "/api/v2/app/version")
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("qBittorrent returned HTTP %d", response.StatusCode)
		}
		return nil
	})
}

func (a QBittorrentAdapter) Limits(ctx context.Context, d Downloader) (int64, int64, error) {
	stats, err := a.Stats(ctx, d)
	return stats.UploadLimitBytes, stats.DownloadLimitBytes, err
}

func (a QBittorrentAdapter) Stats(ctx context.Context, d Downloader) (TransferStats, error) {
	var stats TransferStats
	err := a.withAuth(ctx, d, func(client *http.Client) error {
		endpoint, _ := httpEndpoint(d.BaseURL, "/api/v2/transfer/info")
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		response, requestErr := client.Do(request)
		if requestErr != nil {
			return requestErr
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("qBittorrent returned HTTP %d", response.StatusCode)
		}
		var result struct {
			UpSpeed   int64 `json:"up_info_speed"`
			DownSpeed int64 `json:"dl_info_speed"`
			UpLimit   int64 `json:"up_info_speed_limit"`
			DownLimit int64 `json:"dl_info_speed_limit"`
		}
		if decodeErr := json.NewDecoder(response.Body).Decode(&result); decodeErr != nil {
			return decodeErr
		}
		stats = TransferStats{UploadSpeedBytes: result.UpSpeed, DownloadSpeedBytes: result.DownSpeed, UploadLimitBytes: result.UpLimit, DownloadLimitBytes: result.DownLimit}
		return nil
	})
	return stats, err
}

func (a QBittorrentAdapter) ApplyLimits(ctx context.Context, d Downloader, patch LimitPatch) error {
	return a.withAuth(ctx, d, func(client *http.Client) error {
		for _, limit := range []struct {
			path  string
			value *int64
		}{{"/api/v2/transfer/setUploadLimit", patch.UploadBytes}, {"/api/v2/transfer/setDownloadLimit", patch.DownloadBytes}} {
			if limit.value == nil {
				continue
			}
			endpoint, _ := httpEndpoint(d.BaseURL, limit.path)
			form := url.Values{"limit": {strconv.FormatInt(*limit.value, 10)}}
			request, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response, err := client.Do(request)
			if err != nil {
				return err
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				return fmt.Errorf("qBittorrent returned HTTP %d", response.StatusCode)
			}
		}
		return nil
	})
}

// SetLimits is retained for callers compiled against the pre-chain adapter.
