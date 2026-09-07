package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fnos-media-throttle/fnos-media-throttle/buildinfo"
	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
	"github.com/fnos-media-throttle/fnos-media-throttle/service"
	"github.com/fnos-media-throttle/fnos-media-throttle/storage"
)

func TestConfigEndpointsExposeVersionAndValidateBeforeImport(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, service.NewEngine(store))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), buildinfo.Version) {
		t.Fatalf("settings version missing: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config/export", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Disposition"), ".json") {
		t.Fatalf("export failed: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"events"`) {
		t.Fatal("event log must not be included in configuration backups")
	}
	var backup domain.ConfigBackup
	if err := json.Unmarshal(w.Body.Bytes(), &backup); err != nil {
		t.Fatal(err)
	}
	if backup.AppVersion != buildinfo.Version || backup.FormatVersion != domain.ConfigFormatVersion {
		t.Fatalf("wrong metadata: %+v", backup)
	}

	backup.FormatVersion++
	body, _ := json.Marshal(backup)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/config/import", strings.NewReader(string(body))))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid format accepted: %d %s", w.Code, w.Body.String())
	}

	backup.FormatVersion = domain.ConfigFormatVersion
	body, _ = json.Marshal(backup)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/config/import", strings.NewReader(string(body))))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), buildinfo.Version) {
		t.Fatalf("valid import failed: %d %s", w.Code, w.Body.String())
	}
}
