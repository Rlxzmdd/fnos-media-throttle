package api

import (
	"errors"
	"net/http"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func (a Handler) downloaders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		x, e := a.store.Downloaders(r.Context())
		if e != nil {
			fail(w, 500, e)
			return
		}
		for i := range x {
			x[i].Password = ""
		}
		jsonOut(w, 200, x)
	case http.MethodPost:
		var x domain.Downloader
		if e := decode(r, &x); e != nil {
			fail(w, 400, e)
			return
		}
		if e := validDownloader(&x); e != nil {
			fail(w, 400, e)
			return
		}
		if e := a.store.SaveDownloader(r.Context(), &x); e != nil {
			fail(w, 500, e)
			return
		}
		x.Password = ""
		jsonOut(w, 201, x)
	default:
		fail(w, 405, errors.New("method not allowed"))
	}
}
func (a Handler) downloader(w http.ResponseWriter, r *http.Request) {
	i, action, e := pathID(r.URL.Path, "/api/v1/downloaders/")
	if e != nil {
		fail(w, 400, e)
		return
	}
	d, e := a.store.Downloader(r.Context(), i)
	if e != nil {
		fail(w, 404, e)
		return
	}
	if action == "test" && r.Method == http.MethodPost {
		if e = a.engine.Downloaders.Test(a.engine.DebugContext(r.Context()), d); e != nil {
			fail(w, 422, e)
			return
		}
		jsonOut(w, 200, map[string]string{"status": "ok"})
		return
	}
	if action == "restore" && r.Method == http.MethodPost {
		if e = a.engine.Restore(r.Context(), d); e != nil {
			fail(w, 422, e)
			return
		}
		jsonOut(w, 200, map[string]string{"status": "ok"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var x domain.Downloader
		if e := decode(r, &x); e != nil {
			fail(w, 400, e)
			return
		}
		x.ID = i
		if e := validDownloader(&x); e != nil {
			fail(w, 400, e)
			return
		}
		// Saved originals belong to the old endpoint/account, never the new one.
		if d.Kind != x.Kind || d.BaseURL != x.BaseURL || d.Username != x.Username || d.UserID != x.UserID || (x.Password != "" && x.Password != d.Password) {
			if e := a.engine.Release(r.Context(), d); e != nil {
				fail(w, 422, e)
				return
			}
		}
		if e := a.store.SaveDownloader(r.Context(), &x); e != nil {
			fail(w, 500, e)
			return
		}
		if !x.Enabled && d.Enabled {
			if e = a.engine.Release(r.Context(), d); e != nil {
				fail(w, 422, e)
				return
			}
		}
		jsonOut(w, 200, map[string]int64{"id": i})
	case http.MethodDelete:
		if e = a.engine.Release(r.Context(), d); e != nil {
			fail(w, 422, e)
			return
		}
		if e = a.store.DeleteDownloader(r.Context(), i); e != nil {
			fail(w, 500, e)
			return
		}
		if _, e = a.store.PruneEmptyChains(r.Context()); e != nil {
			fail(w, 500, e)
			return
		}
		jsonOut(w, 204, nil)
	default:
		fail(w, 405, errors.New("method not allowed"))
	}
}
