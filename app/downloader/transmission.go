package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TransmissionAdapter controls one Transmission instance through its RPC API.
// Limits are exposed to the rest of the application as bytes/s, just like the
// qBittorrent adapter, while Transmission stores limits as kB/s.
type TransmissionAdapter struct{ Client *http.Client }

func (a TransmissionAdapter) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

type transmissionResponse struct {
	Arguments json.RawMessage `json:"arguments"`
	Result    string          `json:"result"`
}

func transmissionEndpoint(base string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("Transmission 地址无效")
	}
	if strings.TrimRight(u.Path, "/") == "" {
		u.Path = "/transmission/rpc"
	}
	return u.String(), nil
}

func (a TransmissionAdapter) rpc(ctx context.Context, d Downloader, method string, args any, out any) error {
	endpointURL, err := transmissionEndpoint(d.BaseURL)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"method": method, "arguments": args})
	if err != nil {
		return err
	}
	sessionID := ""
	for attempt := 0; attempt < 2; attempt++ {
		req, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
		if requestErr != nil {
			return requestErr
		}
		req.Header.Set("Content-Type", "application/json")
		if sessionID != "" {
			req.Header.Set("X-Transmission-Session-Id", sessionID)
		}
		if d.Username != "" || d.Password != "" {
			req.SetBasicAuth(d.Username, d.Password)
		}
		resp, requestErr := a.client().Do(req)
		if requestErr != nil {
			return requestErr
		}
		if resp.StatusCode == http.StatusConflict && attempt == 0 {
			sessionID = resp.Header.Get("X-Transmission-Session-Id")
			resp.Body.Close()
			if sessionID == "" {
				return errors.New("Transmission 未返回会话 ID")
			}
			continue
		}
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("Transmission 返回 HTTP %d", resp.StatusCode)
		}
		var result transmissionResponse
		if err = json.Unmarshal(payload, &result); err != nil {
			return fmt.Errorf("解析 Transmission 响应: %w", err)
		}
		if result.Result != "success" {
			return fmt.Errorf("Transmission RPC 失败: %s", result.Result)
		}
		if out != nil && len(result.Arguments) > 0 {
			return json.Unmarshal(result.Arguments, out)
		}
		return nil
	}
	return errors.New("Transmission 会话协商失败")
}

type transmissionSession struct {
	SpeedLimitUp          int64 `json:"speed-limit-up"`
	SpeedLimitUpEnabled   bool  `json:"speed-limit-up-enabled"`
	SpeedLimitDown        int64 `json:"speed-limit-down"`
	SpeedLimitDownEnabled bool  `json:"speed-limit-down-enabled"`
}

type transmissionStats struct {
	UploadSpeed   int64 `json:"uploadSpeed"`
	DownloadSpeed int64 `json:"downloadSpeed"`
}

func (a TransmissionAdapter) session(ctx context.Context, d Downloader) (transmissionSession, error) {
	var s transmissionSession
	err := a.rpc(ctx, d, "session-get", map[string]any{"fields": []string{
		"speed-limit-up", "speed-limit-up-enabled", "speed-limit-down", "speed-limit-down-enabled",
	}}, &s)
	return s, err
}

func (a TransmissionAdapter) Test(ctx context.Context, d Downloader) error {
	_, err := a.session(ctx, d)
	return err
}

func (a TransmissionAdapter) Limits(ctx context.Context, d Downloader) (int64, int64, error) {
	s, err := a.session(ctx, d)
	if err != nil {
		return 0, 0, err
	}
	up, down := int64(0), int64(0)
	if s.SpeedLimitUpEnabled {
		up = s.SpeedLimitUp * 1000
	}
	if s.SpeedLimitDownEnabled {
		down = s.SpeedLimitDown * 1000
	}
	return up, down, nil
}

func (a TransmissionAdapter) Stats(ctx context.Context, d Downloader) (TransferStats, error) {
	s, err := a.session(ctx, d)
	if err != nil {
		return TransferStats{}, err
	}
	var live transmissionStats
	if err = a.rpc(ctx, d, "session-stats", map[string]any{}, &live); err != nil {
		return TransferStats{}, err
	}
	up, down := int64(0), int64(0)
	if s.SpeedLimitUpEnabled {
		up = s.SpeedLimitUp * 1000
	}
	if s.SpeedLimitDownEnabled {
		down = s.SpeedLimitDown * 1000
	}
	return TransferStats{UploadSpeedBytes: live.UploadSpeed, DownloadSpeedBytes: live.DownloadSpeed, UploadLimitBytes: up, DownloadLimitBytes: down}, nil
}

func (a TransmissionAdapter) ApplyLimits(ctx context.Context, d Downloader, patch LimitPatch) error {
	if patch.UploadBytes == nil && patch.DownloadBytes == nil {
		return nil
	}
	args := map[string]any{}
	if patch.UploadBytes != nil {
		args["speed-limit-up-enabled"] = *patch.UploadBytes > 0
		if *patch.UploadBytes > 0 {
			args["speed-limit-up"] = *patch.UploadBytes / 1000
		}
	}
	if patch.DownloadBytes != nil {
		args["speed-limit-down-enabled"] = *patch.DownloadBytes > 0
		if *patch.DownloadBytes > 0 {
			args["speed-limit-down"] = *patch.DownloadBytes / 1000
		}
	}
	return a.rpc(ctx, d, "session-set", args, nil)
}
