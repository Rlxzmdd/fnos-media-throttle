package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
	"github.com/fnos-media-throttle/fnos-media-throttle/service"
	"github.com/fnos-media-throttle/fnos-media-throttle/storage"
)

type coordinatedDownloader struct {
	mu                         sync.Mutex
	entered, resume            chan struct{}
	up, down                   int64
	calls, inFlight, maxFlight int
	writes                     []string
	finishOnCancel             bool
}

func (a *coordinatedDownloader) Test(context.Context, domain.Downloader) error { return nil }
func (a *coordinatedDownloader) Limits(context.Context, domain.Downloader) (int64, int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.up, a.down, nil
}
func (a *coordinatedDownloader) Stats(ctx context.Context, d domain.Downloader) (domain.TransferStats, error) {
	u, v, _ := a.Limits(ctx, d)
	return domain.TransferStats{UploadLimitBytes: u, DownloadLimitBytes: v}, nil
}
func (a *coordinatedDownloader) ApplyLimits(ctx context.Context, d domain.Downloader, p domain.LimitPatch) error {
	a.mu.Lock()
	a.calls++
	first := a.calls == 1
	a.inFlight++
	if a.inFlight > a.maxFlight {
		a.maxFlight = a.inFlight
	}
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.inFlight--; a.mu.Unlock() }()
	if first {
		close(a.entered)
		select {
		case <-a.resume:
		case <-ctx.Done():
			if !a.finishOnCancel {
				return ctx.Err()
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if p.UploadBytes != nil {
		a.up = *p.UploadBytes
	}
	if p.DownloadBytes != nil {
		a.down = *p.DownloadBytes
	}
	a.writes = append(a.writes, d.BaseURL)
	return nil
}

type queuedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *queuedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for synchronization")
	}
}

func concurrencyFixture(t *testing.T) (*service.Engine, *storage.Store, domain.Downloader, domain.BindingChain, *coordinatedDownloader, http.Handler) {
	t.Helper()
	s, err := storage.Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	e := service.NewEngine(s)
	t.Cleanup(func() { e.StopCommands(); s.Close() })
	ctx := context.Background()
	l := domain.Library{Name: "media", Kind: domain.LibraryHTTP, Enabled: true}
	d := domain.Downloader{Name: "download", Kind: domain.DownloaderQB, BaseURL: "http://old.test", Enabled: true, PollSeconds: 5}
	if err := s.SaveLibrary(ctx, &l); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLibraryStatus(ctx, l.ID, 1, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveDownloader(ctx, &d); err != nil {
		t.Fatal(err)
	}
	c := domain.BindingChain{Name: "chain", Enabled: true, LibraryIDs: []int64{l.ID}, DownloaderIDs: []int64{d.ID}, Rule: domain.ChainRule{
		Upload: domain.DirectionRule{Enabled: true, BaseKB: 1000, StepKB: 100}, Download: domain.DirectionRule{Enabled: true, BaseKB: 2000, StepKB: 100},
	}}
	if err := s.SaveChain(ctx, &c); err != nil {
		t.Fatal(err)
	}
	a := &coordinatedDownloader{entered: make(chan struct{}), resume: make(chan struct{}), up: 700000, down: 800000}
	e.Downloaders = a
	return e, s, d, c, a, New(s, e)
}

func queuedRequest(h http.Handler, method, path string, body []byte) (<-chan struct{}, <-chan *httptest.ResponseRecorder) {
	ctx := &queuedContext{Context: context.Background(), observed: make(chan struct{})}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(ctx))
		done <- w
	}()
	return ctx.observed, done
}
func awaitResponse(t *testing.T, ch <-chan *httptest.ResponseRecorder, want int) {
	t.Helper()
	select {
	case w := <-ch:
		if w.Code != want {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request stalled")
	}
}

func TestConcurrentDeleteWaitsForApplyAndRestores(t *testing.T) {
	e, s, d, c, a, h := concurrencyFixture(t)
	applied := make(chan struct{})
	go func() { e.Apply(context.Background(), d); close(applied) }()
	waitSignal(t, a.entered)
	queued, deleted := queuedRequest(h, http.MethodDelete, fmt.Sprintf("/api/v1/chains/%d", c.ID), nil)
	waitSignal(t, queued)
	// Read APIs remain available while the mutating command owns the gate.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/chains", nil))
	if w.Code != 200 {
		t.Fatal("read API blocked/failed")
	}
	close(a.resume)
	waitSignal(t, applied)
	awaitResponse(t, deleted, 204)
	// A scheduled snapshot from before deletion must not reapply the deleted chain.
	e.Apply(context.Background(), d)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.maxFlight != 1 || a.up != 700000 || a.down != 800000 || len(a.writes) != 2 {
		t.Fatalf("bad final state: %+v", a.writes)
	}
	persisted, err := s.Downloader(context.Background(), d.ID)
	if err != nil || persisted.OriginalUploadBytes != nil || persisted.OriginalDownloadBytes != nil {
		t.Fatal("release state not cleared", err)
	}
}

func TestConcurrentRuleEditUsesNewConfiguration(t *testing.T) {
	e, _, d, c, a, h := concurrencyFixture(t)
	applied := make(chan struct{})
	go func() { e.Apply(context.Background(), d); close(applied) }()
	waitSignal(t, a.entered)
	c.Rule.Upload.BaseKB = 3000
	body, _ := json.Marshal(c)
	queued, edited := queuedRequest(h, http.MethodPut, fmt.Sprintf("/api/v1/chains/%d", c.ID), body)
	waitSignal(t, queued)
	close(a.resume)
	waitSignal(t, applied)
	awaitResponse(t, edited, 200)
	e.Apply(context.Background(), d)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.up != 2900000 || a.maxFlight != 1 {
		t.Fatalf("new rule not applied: %d, concurrency=%d", a.up, a.maxFlight)
	}
}

func TestShutdownDrainsApplyRejectsQueuedEditsAndRestores(t *testing.T) {
	e, _, d, _, a, h := concurrencyFixture(t)
	a.finishOnCancel = true // external write can finish just as cancellation arrives
	applied := make(chan struct{})
	go func() { e.Apply(context.Background(), d); close(applied) }()
	waitSignal(t, a.entered)
	queued, edited := queuedRequest(h, http.MethodPut, "/api/v1/settings", []byte(`{"releaseMode":"unlimited"}`))
	waitSignal(t, queued)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx, time.Second); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, applied)
	awaitResponse(t, edited, 503)
	e.Apply(context.Background(), d)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/downloaders/1/restore", nil))
	if w.Code != 503 {
		t.Fatal("stopped engine accepted a write")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.maxFlight != 1 || a.up != 700000 || a.down != 800000 || len(a.writes) != 2 {
		t.Fatalf("shutdown state: %d/%d calls=%d", a.up, a.down, len(a.writes))
	}
}

func TestEndpointEditReleasesOldConnectionFirst(t *testing.T) {
	e, s, d, _, a, h := concurrencyFixture(t)
	close(a.resume)
	e.Apply(context.Background(), d)
	updated, err := s.Downloader(context.Background(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated.BaseURL = "http://new.test"
	body, _ := json.Marshal(updated)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/downloaders/%d", d.ID), bytes.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.writes) != 2 || a.writes[1] != "http://old.test" || a.up != 700000 {
		t.Fatalf("released wrong endpoint: %v", a.writes)
	}
}
