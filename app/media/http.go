package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPMediaAdapter struct{ Client *http.Client }

func (a HTTPMediaAdapter) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}
func endpoint(base, suffix string) (string, error) {
	u, e := url.Parse(base)
	if e != nil {
		return "", e
	}
	u.Path = strings.TrimRight(u.Path, "/") + suffix
	return u.String(), nil
}
func (a HTTPMediaAdapter) Test(ctx context.Context, l Library) error {
	_, e := a.ActiveViewerCount(ctx, l)
	return e
}
func (a HTTPMediaAdapter) ActiveViewerCount(ctx context.Context, l Library) (int, error) {
	path := ""
	if l.Kind == LibraryEmby || l.Kind == LibraryJellyfin {
		path = "/Sessions"
	}
	u, e := endpoint(l.BaseURL, path)
	if e != nil {
		return 0, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if e != nil {
		return 0, e
	}
	if l.APIKey != "" {
		if l.Kind == LibraryHTTP {
			req.Header.Set("Authorization", "Bearer "+l.APIKey)
		} else {
			req.Header.Set("X-Emby-Token", l.APIKey)
		}
	}
	resp, e := a.client().Do(req)
	if e != nil {
		return 0, e
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if e != nil {
		return 0, e
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, fmt.Errorf("media service returned HTTP %d", resp.StatusCode)
	}
	if l.Kind == LibraryHTTP {
		return parseProxyCountForLibrary(body, l.CountLocalClients)
	}
	return parseSessionsForLibrary(body, l.CountLocalClients)
}
