package api

import (
	"context"
	"net/http"
	"strings"
)

func (a Handler) commands(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") || r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		err := a.engine.Coordinate(r.Context(), func(ctx context.Context) error {
			next.ServeHTTP(w, r.WithContext(ctx))
			return nil
		})
		if err != nil {
			fail(w, http.StatusServiceUnavailable, err)
		}
	})
}
