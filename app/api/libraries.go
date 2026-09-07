package api

import (
	"errors"
	"net/http"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func (a Handler) libraries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		x, e := a.store.Libraries(r.Context())
		if e != nil {
			fail(w, 500, e)
			return
		}
		for i := range x {
			x[i].APIKey = ""
		}
		jsonOut(w, 200, x)
	case http.MethodPost:
		var x domain.Library
		if e := decode(r, &x); e != nil {
			fail(w, 400, e)
			return
		}
		if e := validLibrary(&x); e != nil {
			fail(w, 400, e)
			return
		}
		if e := a.store.SaveLibrary(r.Context(), &x); e != nil {
			fail(w, 500, e)
			return
		}
		x.APIKey = ""
		jsonOut(w, 201, x)
	default:
		fail(w, 405, errors.New("method not allowed"))
	}
}
func (a Handler) library(w http.ResponseWriter, r *http.Request) {
	i, action, e := pathID(r.URL.Path, "/api/v1/libraries/")
	if e != nil {
		fail(w, 400, e)
		return
	}
	if action == "test" && r.Method == http.MethodPost {
		x, e := a.store.Library(r.Context(), i)
		if e != nil {
			fail(w, 404, e)
			return
		}
		n, collectErr := a.engine.Media.ActiveViewerCount(a.engine.DebugContext(r.Context()), x)
		if collectErr != nil {
			_ = a.store.SetLibraryStatus(r.Context(), x.ID, x.LastCount, collectErr.Error())
			fail(w, 422, collectErr)
			return
		}
		if e = a.store.SetLibraryStatus(r.Context(), x.ID, n, ""); e != nil {
			fail(w, 500, e)
			return
		}
		jsonOut(w, 200, map[string]any{"status": "ok", "viewerCount": n})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var x domain.Library
		if e := decode(r, &x); e != nil {
			fail(w, 400, e)
			return
		}
		x.ID = i
		if e := validLibrary(&x); e != nil {
			fail(w, 400, e)
			return
		}
		if e := a.store.SaveLibrary(r.Context(), &x); e != nil {
			fail(w, 500, e)
			return
		}
		jsonOut(w, 200, map[string]int64{"id": i})
	case http.MethodDelete:
		if e = a.store.DeleteLibrary(r.Context(), i); e != nil {
			fail(w, 500, e)
			return
		}
		restoreIDs, pruneErr := a.store.PruneEmptyChains(r.Context())
		if pruneErr != nil {
			fail(w, 500, pruneErr)
			return
		}
		for _, downloaderID := range restoreIDs {
			if d, loadErr := a.store.Downloader(r.Context(), downloaderID); loadErr == nil {
				if restoreErr := a.engine.ReleaseIfUnbound(r.Context(), d); restoreErr != nil {
					a.store.Event(r.Context(), &downloaderID, "error", "删除影视库后按退出策略释放限速失败: "+restoreErr.Error())
				}
			}
		}
		jsonOut(w, 204, nil)
	default:
		fail(w, 405, errors.New("method not allowed"))
	}
}
