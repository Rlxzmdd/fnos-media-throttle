package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fnos-media-throttle/fnos-media-throttle/service"
	"github.com/fnos-media-throttle/fnos-media-throttle/storage"
)

func TestReleaseSettingsValidation(t *testing.T) {
	s, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := New(s, service.NewEngine(s))
	for _, tc := range []struct {
		body   string
		status int
		mode   string
	}{
		{`{"debug":true}`, 200, "original"},
		{`{"releaseMode":"maximum"}`, 200, "maximum"},
		{`{"releaseMode":"unlimited"}`, 200, "unlimited"},
		{`{"releaseMode":"typo"}`, 400, "unlimited"},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/v1/settings", strings.NewReader(tc.body)))
		if w.Code != tc.status {
			t.Fatalf("status=%d: %s", w.Code, w.Body.String())
		}
		settings, err := s.Settings(context.Background())
		if err != nil || settings.ReleaseMode != tc.mode {
			t.Fatalf("settings=%+v error=%v", settings, err)
		}
	}
}
