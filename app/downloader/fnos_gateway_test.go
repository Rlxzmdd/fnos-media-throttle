package downloader

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFNOSLoginPayloadUsesUsernameSeparatelyFromUID(t *testing.T) {
	payload := fnosLoginPayload("test-user", "secret", "session", "device-id", "request")
	if got := payload["user"]; got != "test-user" {
		t.Fatalf("login user = %v, want configured username", got)
	}
	if _, exists := payload["uid"]; exists {
		t.Fatal("login payload must not contain downloader uid")
	}
	for _, key := range []string{"si", "deviceType", "deviceName", "did"} {
		if payload[key] == nil || payload[key] == "" {
			t.Fatalf("login payload missing %s", key)
		}
	}
	if payload["deviceName"] != "Linux-Google Chrome" || payload["stay"] != true {
		t.Fatalf("unexpected login identity: deviceName=%v stay=%v", payload["deviceName"], payload["stay"])
	}
	if payload["did"] != "device-id" {
		t.Fatalf("unexpected device id: %v", payload["did"])
	}
}

func TestFNOSLiveGateway(t *testing.T) {
	if os.Getenv("FNOS_LIVE") != "1" {
		t.Skip("set FNOS_LIVE=1 to run against a real fnOS host")
	}
	d := Downloader{
		ID:       1,
		Kind:     DownloaderFNOS,
		BaseURL:  os.Getenv("FNOS_WS"),
		Username: os.Getenv("FNOS_USER"),
		Password: os.Getenv("FNOS_PASS"),
		UserID:   1000,
	}
	if d.BaseURL == "" || d.Username == "" || d.Password == "" {
		t.Fatal("FNOS_WS, FNOS_USER and FNOS_PASS are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	gateway, err := dialFNOSGateway(ctx, d)
	if err != nil {
		t.Fatalf("live fnOS login failed: %v", err)
	}
	defer gateway.Close()
	if len(gateway.signKey) == 0 || gateway.uid <= 0 {
		t.Fatalf("login incomplete: signKey=%t uid=%d", len(gateway.signKey) > 0, gateway.uid)
	}
	config, err := gateway.transferConfig(ctx)
	if err != nil {
		t.Fatalf("live fnOS transfer config failed after login: %v", err)
	}
	t.Logf("live fnOS login and transfer-config query succeeded; uid=%d fields=%v", gateway.uid, sortedFNOSKeys(config))
}

func TestFNOSRequestIDsUseTimestampBackIDAndCounter(t *testing.T) {
	gateway := &fnosGateway{backID: "0000000000000000", index: 1}
	first := gateway.requestID()
	if len(first) != 28 {
		t.Fatalf("request id length = %d, want 28", len(first))
	}
	if first[8:24] != "0000000000000000" || first[24:] != "0001" {
		t.Fatalf("unexpected pre-login request id %q", first)
	}
	gateway.backID = "abcd123400000000"
	second := gateway.requestID()
	if second[8:24] != gateway.backID || second[24:] != "0002" {
		t.Fatalf("unexpected authenticated request id %q", second)
	}
}

func TestFNOSRandomAESKeyAndNoSignEndpoints(t *testing.T) {
	key, err := randomAlphaNumeric(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 || strings.Trim(key, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" {
		t.Fatalf("unexpected AES key format %q", key)
	}
	for _, endpoint := range []string{"encrypted", "util.getSI", "util.crypto.getRSAPub"} {
		if !fnosNoSignRequest(endpoint) {
			t.Fatalf("endpoint %s must not be signed", endpoint)
		}
	}
	if fnosNoSignRequest("appcgi.downloadcenter.stat.all") {
		t.Fatal("business requests must be signed")
	}
}

func TestFNOSBusinessUIDPrefersLoginResponse(t *testing.T) {
	uid, source := fnosBusinessUID(1234, 1000)
	if uid != 1234 || source != "login-response" {
		t.Fatalf("business uid = %d (%s), want login response uid", uid, source)
	}
	uid, source = fnosBusinessUID(0, 1000)
	if uid != 1000 || source != "configured" {
		t.Fatalf("fallback uid = %d (%s), want configured uid", uid, source)
	}
}

func TestFNOSSpeedUnitConversion(t *testing.T) {
	if got := appToFNOSBytes(4000 * 1000); got != 4000*1024 {
		t.Fatalf("app to fnOS = %d", got)
	}
	if got := fnosToAppBytes(4000 * 1024); got != 4000*1000 {
		t.Fatalf("fnOS to app = %d", got)
	}
}

func TestUnwrapFNOSBlock(t *testing.T) {
	response := map[string]any{"data": map[string]any{"block": map[string]any{"data": `{"up_speed":123}`}}}
	if got := int64Value(unwrapFNOSMap(response)["up_speed"]); got != 123 {
		t.Fatalf("unwrapped speed = %d", got)
	}
	encoded, _ := json.Marshal(unwrapFNOSMap(response))
	if string(encoded) != `{"up_speed":123}` {
		t.Fatalf("unexpected map: %s", encoded)
	}
}
