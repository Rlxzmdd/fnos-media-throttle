package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/fnos-media-throttle/fnos-media-throttle/buildinfo"
	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func (a Handler) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		x, e := a.store.Settings(r.Context())
		if e != nil {
			fail(w, 500, e)
			return
		}
		x.AppVersion = buildinfo.Version
		jsonOut(w, 200, x)
	case http.MethodPut:
		var x domain.Settings
		if e := decode(r, &x); e != nil {
			fail(w, 400, e)
			return
		}
		if e := x.Normalize(); e != nil {
			fail(w, 400, e)
			return
		}
		if e := a.store.SaveSettings(r.Context(), x); e != nil {
			fail(w, 500, e)
			return
		}
		if x.Debug {
			log.Printf("[debug] debug logging enabled by user")
		}
		x.AppVersion = buildinfo.Version
		jsonOut(w, 200, x)
	default:
		fail(w, 405, errors.New("method not allowed"))
	}
}
