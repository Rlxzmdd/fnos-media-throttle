package downloader

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransmissionSessionHandshakeStatsAndLimits(t *testing.T) {
	const sessionID = "test-session"
	var setArguments map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") != sessionID {
			w.Header().Set("X-Transmission-Session-Id", sessionID)
			w.WriteHeader(http.StatusConflict)
			return
		}
		var request struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if request.Method == "session-set" {
			setArguments = request.Arguments
			_, _ = w.Write([]byte(`{"result":"success","arguments":{}}`))
			return
		}
		if request.Method == "session-stats" {
			_, _ = w.Write([]byte(`{"result":"success","arguments":{"uploadSpeed":123000,"downloadSpeed":456000}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"success","arguments":{"speed-limit-up":8000,"speed-limit-up-enabled":true,"speed-limit-down":0,"speed-limit-down-enabled":false}}`))
	}))
	defer server.Close()

	adapter := TransmissionAdapter{Client: server.Client()}
	d := Downloader{BaseURL: server.URL}
	stats, err := adapter.Stats(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if stats.UploadSpeedBytes != 123000 || stats.DownloadSpeedBytes != 456000 || stats.UploadLimitBytes != 8000000 || stats.DownloadLimitBytes != 0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if err = adapter.ApplyLimits(context.Background(), d, LimitPatch{UploadBytes: ptr(6000000), DownloadBytes: ptr(0)}); err != nil {
		t.Fatal(err)
	}
	if setArguments["speed-limit-up"] != float64(6000) || setArguments["speed-limit-up-enabled"] != true || setArguments["speed-limit-down-enabled"] != false {
		t.Fatalf("unexpected set arguments: %#v", setArguments)
	}
}

func TestTransmissionEndpointDefaultPath(t *testing.T) {
	got, err := transmissionEndpoint("http://nas:9091")
	if err != nil || got != "http://nas:9091/transmission/rpc" {
		t.Fatalf("got %q, %v", got, err)
	}
	got, err = transmissionEndpoint("http://nas:9091/custom/rpc")
	if err != nil || got != "http://nas:9091/custom/rpc" {
		t.Fatalf("custom path got %q, %v", got, err)
	}
}
