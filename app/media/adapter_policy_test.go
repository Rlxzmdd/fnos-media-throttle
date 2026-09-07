package media

import "testing"

func TestSessionCountCanExcludeLocalPlayback(t *testing.T) {
	body := []byte(`[
  {"UserId":"same","DeviceId":"tv","RemoteEndPoint":"192.168.1.20:8096","NowPlayingItem":{}},
  {"UserId":"same","DeviceId":"tablet","RemoteEndPoint":"203.0.113.7:8096","NowPlayingItem":{}},
  {"UserId":"other","DeviceId":"phone","RemoteEndPoint":"8.8.8.8:8096","NowPlayingItem":{}}
]`)
	withLocal, err := parseSessionsForLibrary(body, true)
	if err != nil || withLocal != 3 {
		t.Fatalf("with local = %d, %v", withLocal, err)
	}
	withoutLocal, err := parseSessionsForLibrary(body, false)
	if err != nil || withoutLocal != 2 {
		t.Fatalf("without local = %d, %v", withoutLocal, err)
	}
}

func TestStructuredProxyCanExcludeLocalPlayback(t *testing.T) {
	body := []byte(`{"viewers":[
  {"userId":"same","deviceId":"tv","clientIp":"127.0.0.1"},
  {"userId":"same","deviceId":"phone","clientIp":"198.51.100.9"},
  {"userId":"same","deviceId":"phone","clientIp":"198.51.100.9"}
]}`)
	count, err := parseProxyCountForLibrary(body, false)
	if err != nil || count != 1 {
		t.Fatalf("filtered structured proxy = %d, %v", count, err)
	}
}

func TestCountOnlyProxyRejectsLocalExclusion(t *testing.T) {
	if _, err := parseProxyCountForLibrary([]byte(`{"activeViewerCount":2}`), false); err == nil {
		t.Fatal("expected an explicit capability error")
	}
}
