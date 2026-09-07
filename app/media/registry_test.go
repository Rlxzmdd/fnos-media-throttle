package media

import (
	"context"
	"testing"
)

type registryFake struct{}

func (registryFake) Test(context.Context, Library) error                     { return nil }
func (registryFake) ActiveViewerCount(context.Context, Library) (int, error) { return 3, nil }

func TestRegistryExtension(t *testing.T) {
	r := NewRegistry(Registration{Kind: "new-provider", Impl: registryFake{}})
	ctx := context.Background()
	l := Library{Kind: "new-provider"}
	if err := r.Test(ctx, l); err != nil {
		t.Fatal(err)
	}
	n, err := r.ActiveViewerCount(ctx, l)
	if err != nil || n != 3 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	l.Kind = "unknown"
	if err := r.Test(ctx, l); err == nil {
		t.Fatal("unknown provider accepted")
	}
	if _, err := r.ActiveViewerCount(ctx, l); err == nil {
		t.Fatal("unknown provider counted")
	}
}
