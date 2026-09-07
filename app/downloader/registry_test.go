package downloader

import (
	"context"
	"testing"
)

type registryFake struct{ calls int }

type cycleFake struct {
	registryFake
	closed bool
}

func (f *cycleFake) BeginCycle(ctx context.Context, _ Downloader) (context.Context, func()) {
	return ctx, func() { f.closed = true }
}

func TestRegistryDelegatesOptionalCycle(t *testing.T) {
	f := &cycleFake{}
	r := NewRegistry(Registration{Kind: "cycle", Impl: f})
	ctx, closeCycle := r.BeginCycle(context.Background(), Downloader{Kind: "cycle"})
	if ctx == nil || f.closed {
		t.Fatal("unexpected cycle state")
	}
	closeCycle()
	if !f.closed {
		t.Fatal("provider cycle not closed")
	}
	_, noOp := r.BeginCycle(context.Background(), Downloader{Kind: "unknown"})
	noOp()
}

func (f *registryFake) Test(context.Context, Downloader) error { f.calls++; return nil }
func (f *registryFake) Limits(context.Context, Downloader) (int64, int64, error) {
	f.calls++
	return 12, 34, nil
}
func (f *registryFake) Stats(context.Context, Downloader) (TransferStats, error) {
	f.calls++
	return TransferStats{}, nil
}
func (f *registryFake) ApplyLimits(context.Context, Downloader, LimitPatch) error {
	f.calls++
	return nil
}

func TestRegistryExtensionAndUnknownKind(t *testing.T) {
	ctx := context.Background()
	f := &registryFake{}
	r := NewRegistry(Registration{Kind: "new-provider", Impl: f})
	d := Downloader{Kind: "new-provider"}
	if err := r.Test(ctx, d); err != nil {
		t.Fatal(err)
	}
	up, down, err := r.Limits(ctx, d)
	if err != nil || up != 12 || down != 34 {
		t.Fatalf("limits=%d/%d %v", up, down, err)
	}
	if _, err := r.Stats(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyLimits(ctx, d, LimitPatch{}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []DownloaderKind{"", "unknown"} {
		d.Kind = kind
		if err := r.Test(ctx, d); err == nil {
			t.Fatal("unknown test accepted")
		}
		if _, _, err := r.Limits(ctx, d); err == nil {
			t.Fatal("unknown limits accepted")
		}
		if _, err := r.Stats(ctx, d); err == nil {
			t.Fatal("unknown stats accepted")
		}
		if err := r.ApplyLimits(ctx, d, LimitPatch{}); err == nil {
			t.Fatal("unknown apply accepted")
		}
	}
	if f.calls != 4 {
		t.Fatalf("unexpected routing: %d calls", f.calls)
	}
}
