package media

import "testing"

func TestUniqueSessionUsers(t *testing.T) {
	count, err := parseSessions([]byte(`[{"UserId":"a","NowPlayingItem":{}},{"UserId":"a","NowPlayingItem":{}},{"UserId":"b","NowPlayingItem":{}},{"UserId":"c"}]`))
	if err != nil || count != 2 {
		t.Fatalf("got %d, %v", count, err)
	}
}

func TestSessionIdentityUsesAccountAndDevice(t *testing.T) {
	body := []byte(`[
		{"UserId":"same","DeviceId":"tv","NowPlayingItem":{"Id":"movie-1"}},
		{"UserId":"same","DeviceId":"tv","NowPlayingItem":{"Id":"movie-2"}},
		{"UserId":"same","DeviceId":"tv","NowPlayingItem":{"Id":"movie-3"}},
		{"UserId":"same","DeviceId":"phone","NowPlayingItem":{"Id":"movie-1"}}
	]`)
	count, err := parseSessions(body)
	if err != nil || count != 2 {
		t.Fatalf("same account on two devices should be 2 viewers, got %d, %v", count, err)
	}
}

func TestProxyCount(t *testing.T) {
	count, err := parseProxyCount([]byte(`{"activeViewerCount":4}`))
	if err != nil || count != 4 {
		t.Fatalf("got %d, %v", count, err)
	}
}
