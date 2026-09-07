package api

import (
	"errors"
	"net/http"
)

func (a Handler) events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, 405, errors.New("method not allowed"))
		return
	}
	x, e := a.store.Events(r.Context())
	if e != nil {
		fail(w, 500, e)
		return
	}
	jsonOut(w, 200, x)
}
