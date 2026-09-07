package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/fnos-media-throttle/fnos-media-throttle/service"
	"github.com/fnos-media-throttle/fnos-media-throttle/storage"
)

type Handler struct {
	store  *storage.Store
	engine *service.Engine
}

// New returns the complete HTTP API and management UI handler.
func New(store *storage.Store, engine *service.Engine) http.Handler {
	return Handler{store: store, engine: engine}.routes()
}

func (a Handler) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) { jsonOut(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("/api/v1/libraries", a.libraries)
	m.HandleFunc("/api/v1/libraries/", a.library)
	m.HandleFunc("/api/v1/downloaders", a.downloaders)
	m.HandleFunc("/api/v1/downloaders/", a.downloader)
	m.HandleFunc("/api/v1/chains", a.chains)
	m.HandleFunc("/api/v1/chains/", a.chain)
	m.HandleFunc("/api/v1/events", a.events)
	m.HandleFunc("/api/v1/settings", a.settings)
	m.HandleFunc("/api/v1/config/export", a.exportConfig)
	m.HandleFunc("/api/v1/config/import", a.importConfig)
	m.Handle("/", static())
	return security(a.commands(m))
}
func static() http.Handler {
	dir := os.Getenv("UI_DIR")
	if dir == "" {
		dir = "./ui/dist"
	}
	if _, e := os.Stat(dir); e == nil {
		files := http.FileServer(http.Dir(dir))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".js") {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			}
			if strings.HasSuffix(r.URL.Path, ".css") {
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			}
			files.ServeHTTP(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>FNOS 影视观看限速器</title><p>前端尚未构建。请执行 scripts/build.sh，或设置 UI_DIR。</p>`)
	})
}

// fnOS desktop embeds this service from port 5666 into an iframe. Do not send
// X-Frame-Options here: its SAMEORIGIN rule treats the app's service port as a
// different origin and makes Chromium report a misleading "connection refused".
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, e error) {
	jsonOut(w, status, map[string]string{"error": e.Error()})
}
func decode(r *http.Request, v any) error {
	return decodeLimit(r, v, 1<<20)
}
func decodeLimit(r *http.Request, v any, limit int64) error {
	d := json.NewDecoder(io.LimitReader(r.Body, limit))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func id(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
func pathID(path, prefix string) (int64, string, error) {
	tail := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, "", errors.New("missing id")
	}
	x, e := id(parts[0])
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	return x, action, e
}
