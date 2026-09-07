package media

import (
	"encoding/json"
	"errors"
	"fmt"
)

func parseProxyCount(body []byte) (int, error) {
	return parseProxyCountForLibrary(body, true)
}

func parseProxyCountForLibrary(body []byte, countLocal bool) (int, error) {
	var n int
	if json.Unmarshal(body, &n) == nil {
		if !countLocal {
			return 0, errors.New("HTTP 人数接口只返回总人数，无法排除本地 IP；请改用 viewers 结构化响应")
		}
		return n, nil
	}
	var structured struct {
		Viewers []struct {
			ID       string `json:"id"`
			UserID   string `json:"userId"`
			DeviceID string `json:"deviceId"`
			ClientIP string `json:"clientIp"`
			IsLocal  *bool  `json:"isLocal"`
		} `json:"viewers"`
	}
	if json.Unmarshal(body, &structured) == nil && structured.Viewers != nil {
		seen := map[string]bool{}
		for index, viewer := range structured.Viewers {
			if !countLocal && isLocalClient(viewer.ClientIP, viewer.IsLocal) {
				continue
			}
			key := viewer.ID
			if key == "" {
				key = viewer.UserID + "\x1f" + viewer.DeviceID
			}
			if key == "\x1f" {
				key = fmt.Sprintf("anonymous-%d", index)
			}
			seen[key] = true
		}
		return len(seen), nil
	}
	var v struct {
		ActiveViewerCount *int `json:"activeViewerCount"`
		Count             *int `json:"count"`
	}
	if e := json.Unmarshal(body, &v); e != nil {
		return 0, fmt.Errorf("invalid proxy response: %w", e)
	}
	if v.ActiveViewerCount != nil {
		if !countLocal {
			return 0, errors.New("HTTP 人数接口未返回客户端 IP，无法排除本地 IP")
		}
		return *v.ActiveViewerCount, nil
	}
	if v.Count != nil {
		if !countLocal {
			return 0, errors.New("HTTP 人数接口未返回客户端 IP，无法排除本地 IP")
		}
		return *v.Count, nil
	}
	return 0, errors.New("proxy response needs activeViewerCount")
}
