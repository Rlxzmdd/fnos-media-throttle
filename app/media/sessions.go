package media

import (
	"encoding/json"
	"net/netip"
	"strings"
)

func parseSessions(body []byte) (int, error) {
	return parseSessionsForLibrary(body, true)
}

func parseSessionsForLibrary(body []byte, countLocal bool) (int, error) {
	var sessions []struct {
		UserID         string          `json:"UserId"`
		UserName       string          `json:"UserName"`
		DeviceID       string          `json:"DeviceId"`
		SessionID      string          `json:"Id"`
		PlaySessionID  string          `json:"PlaySessionId"`
		NowPlayingItem json.RawMessage `json:"NowPlayingItem"`
		RemoteEndPoint string          `json:"RemoteEndPoint"`
		IsLocal        *bool           `json:"IsLocal"`
	}
	if e := json.Unmarshal(body, &sessions); e != nil {
		return 0, e
	}
	users := map[string]struct{}{}
	anonymous := 0
	for _, s := range sessions {
		if len(s.NowPlayingItem) == 0 || string(s.NowPlayingItem) == "null" {
			continue
		}
		if !countLocal && isLocalClient(s.RemoteEndPoint, s.IsLocal) {
			continue
		}
		userID := s.UserID
		if userID == "" {
			userID = s.UserName
		}
		deviceID := s.DeviceID
		if deviceID == "" {
			deviceID = s.PlaySessionID
		}
		if deviceID == "" {
			deviceID = s.SessionID
		}
		if userID == "" && deviceID == "" {
			anonymous++
			continue
		}
		// A viewer is an account-device pair: the same account playing on two
		// devices consumes two streams, while switching titles on one device
		// remains a single viewer.
		users[userID+"\x1f"+deviceID] = struct{}{}
	}
	return len(users) + anonymous, nil
}

func isLocalClient(endpoint string, explicit *bool) bool {
	if explicit != nil {
		return *explicit
	}
	host := strings.TrimSpace(endpoint)
	if parsed, err := netip.ParseAddrPort(host); err == nil {
		host = parsed.Addr().String()
	}
	host = strings.Trim(host, "[]")
	address, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast()
}
